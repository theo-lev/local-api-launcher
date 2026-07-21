package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
	"unicode/utf8"
)

type managedProcess struct {
	cmd         *exec.Cmd
	pid         int
	runID       string
	state       string
	reconnected bool
	envID       string
	envName     string
}

type ProcessManager struct {
	mu       sync.Mutex
	procs    map[string]*managedProcess
	sessions map[string]*LogSession
	store    *Store
}

func NewProcessManager(store *Store) *ProcessManager {
	return &ProcessManager{procs: make(map[string]*managedProcess), sessions: make(map[string]*LogSession), store: store}
}

func mavenExe(configured string) string {
	if configured != "" {
		return configured
	}
	if runtime.GOOS == "windows" {
		return "mvn.cmd"
	}
	return "mvn"
}

func (pm *ProcessManager) Start(id, repoPath, mavenPath, jdkPath, envID, envName string, envVars []string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if proc, ok := pm.procs[id]; ok {
		return fmt.Errorf("cannot start while run %s is %s", proc.runID, proc.state)
	}

	runID := newID()
	cmd := exec.Command(mavenExe(mavenPath), "spring-boot:run", "-DskipTests")
	cmd.Dir = repoPath
	if len(envVars) > 0 || jdkPath != "" {
		env := append(os.Environ(), envVars...)
		if jdkPath != "" {
			env = append(env, "JAVA_HOME="+jdkPath)
		}
		cmd.Env = env
	}
	setProcAttr(cmd)

	stdout, stdoutWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	stderr, stderrWriter, err := os.Pipe()
	if err != nil {
		stdout.Close()
		stdoutWriter.Close()
		return err
	}
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	if err := cmd.Start(); err != nil {
		stdout.Close()
		stdoutWriter.Close()
		stderr.Close()
		stderrWriter.Close()
		return err
	}
	// These are user-supplied files rather than StdoutPipe/StderrPipe, so Wait
	// cannot close the read ends before the readers deliver final buffered data.
	stdoutWriter.Close()
	stderrWriter.Close()

	session := newLogSession(runID)
	proc := &managedProcess{cmd: cmd, pid: cmd.Process.Pid, runID: runID, state: "running", envID: envID, envName: envName}
	pm.procs[id] = proc
	pm.sessions[id] = session // a successful new launch replaces the retained old session
	if err := pm.store.StorePid(id, ProcInfo{Pid: proc.pid, RunID: runID, EnvID: envID, EnvName: envName}); err != nil {
		log.Printf("failed to persist process repo=%s run=%s pid=%d: %v", id, runID, proc.pid, err)
	}
	log.Printf("process started repo=%s run=%s pid=%d", id, runID, proc.pid)

	var pipeWG sync.WaitGroup
	pipeWG.Add(2)
	go func() { defer pipeWG.Done(); pipeLines(stdout, session) }()
	go func() { defer pipeWG.Done(); pipeLines(stderr, session) }()
	go pm.waitForProcess(id, proc, session, &pipeWG)
	return nil
}

func (pm *ProcessManager) waitForProcess(id string, proc *managedProcess, session *LogSession, pipeWG *sync.WaitGroup) {
	waitErr := proc.cmd.Wait()
	pipeWG.Wait()
	session.closeAll()
	pm.cleanup(id, proc, waitErr)
}

func (pm *ProcessManager) cleanup(id string, proc *managedProcess, waitErr error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	current, ok := pm.procs[id]
	if !ok || current != proc || current.runID != proc.runID {
		currentRun := ""
		if ok {
			currentRun = current.runID
		}
		log.Printf("process cleanup skipped repo=%s old_run=%s pid=%d current_run=%s wait_error=%v", id, proc.runID, proc.pid, currentRun, waitErr)
		return
	}
	delete(pm.procs, id)
	removed, err := pm.store.RemovePidIfRun(id, proc.runID)
	log.Printf("process cleaned repo=%s run=%s pid=%d persisted_removed=%t wait_error=%v cleanup_error=%v", id, proc.runID, proc.pid, removed, waitErr, err)
}

func (pm *ProcessManager) Stop(id string) error {
	pm.mu.Lock()
	proc, ok := pm.procs[id]
	if !ok {
		pm.mu.Unlock()
		return fmt.Errorf("not running")
	}
	if proc.state == "stopping" {
		pm.mu.Unlock()
		return fmt.Errorf("already stopping")
	}
	proc.state = "stopping"
	pm.mu.Unlock()

	var err error
	if proc.reconnected {
		err = killByPid(proc.pid)
	} else {
		err = killProc(proc.cmd)
	}
	if err != nil {
		pm.mu.Lock()
		if pm.procs[id] == proc {
			proc.state = "running"
		}
		pm.mu.Unlock()
		log.Printf("process termination failed repo=%s run=%s pid=%d: %v", id, proc.runID, proc.pid, err)
		return fmt.Errorf("failed to terminate process tree: %w", err)
	}
	log.Printf("process termination requested repo=%s run=%s pid=%d", id, proc.runID, proc.pid)
	if proc.reconnected {
		go pm.reapReconnected(id, proc)
	}
	return nil
}

func (pm *ProcessManager) reapReconnected(id string, proc *managedProcess) {
	for isAlive(proc.pid) {
		time.Sleep(100 * time.Millisecond)
	}
	pm.cleanup(id, proc, nil)
}

func (pm *ProcessManager) Reconnect(id string, pid int, envID, envName string, runIDs ...string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	runID := ""
	if len(runIDs) != 0 {
		runID = runIDs[0]
	}
	if runID == "" {
		runID = newID()
	}
	pm.procs[id] = &managedProcess{pid: pid, runID: runID, state: "running", reconnected: true, envID: envID, envName: envName}
	if err := pm.store.StorePid(id, ProcInfo{Pid: pid, RunID: runID, EnvID: envID, EnvName: envName}); err != nil {
		log.Printf("failed to persist reconnected process repo=%s run=%s pid=%d: %v", id, runID, pid, err)
	}
	log.Printf("process reconnected repo=%s run=%s pid=%d", id, runID, pid)
}

func (pm *ProcessManager) Status(id string) (status, runID, envID, envName string, reconnected bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	proc, ok := pm.procs[id]
	if !ok {
		return "stopped", "", "", "", false
	}
	if proc.reconnected && proc.state == "running" && !isAlive(proc.pid) {
		delete(pm.procs, id)
		pm.store.RemovePidIfRun(id, proc.runID)
		return "stopped", "", "", "", false
	}
	return proc.state, proc.runID, proc.envID, proc.envName, proc.reconnected
}

func (pm *ProcessManager) RunningEnv(id string) (string, string) {
	_, _, envID, envName, _ := pm.Status(id)
	return envID, envName
}

func (pm *ProcessManager) IsRunning(id string) bool {
	status, _, _, _, _ := pm.Status(id)
	return status != "stopped"
}

func (pm *ProcessManager) IsReconnected(id string) bool {
	_, _, _, _, reconnected := pm.Status(id)
	return reconnected
}

func (pm *ProcessManager) GetSession(id string) *LogSession {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.sessions[id]
}

func pipeLines(r io.Reader, s *LogSession) {
	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			if line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
				if len(line) > 0 && line[len(line)-1] == '\r' {
					line = line[:len(line)-1]
				}
			}
			s.append(decodeProcessOutput(line))
		}
		if err != nil {
			return
		}
	}
}

// Java versions before JDK 18 commonly use Windows-1252 for redirected
// console output on Windows. JSON requires UTF-8 and replaces invalid bytes
// with U+FFFD, so normalize process output before retaining it. Valid UTF-8
// sequences are preserved, allowing UTF-8 and Windows-1252 output to coexist.
func decodeProcessOutput(input string) string {
	if utf8.ValidString(input) {
		return input
	}
	runes := make([]rune, 0, len(input))
	for len(input) > 0 {
		r, size := utf8.DecodeRuneInString(input)
		if r == utf8.RuneError && size == 1 {
			r = windows1252Rune(input[0])
		}
		runes = append(runes, r)
		input = input[size:]
	}
	return string(runes)
}

func windows1252Rune(b byte) rune {
	if b < 0x80 || b >= 0xa0 {
		return rune(b)
	}
	// Undefined Windows-1252 bytes retain their C1 code point rather than
	// becoming a replacement character and destroying the original byte.
	return [...]rune{
		'€', 0x81, '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', 0x8d, 'Ž', 0x8f,
		0x90, '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', 0x9d, 'ž', 'Ÿ',
	}[b-0x80]
}
