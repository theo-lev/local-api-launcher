package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

type managedProcess struct {
	cmd         *exec.Cmd
	pid         int
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
	return &ProcessManager{
		procs:    make(map[string]*managedProcess),
		sessions: make(map[string]*LogSession),
		store:    store,
	}
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
	if _, ok := pm.procs[id]; ok {
		return fmt.Errorf("already running")
	}

	cmd := exec.Command(mavenExe(mavenPath), "spring-boot:run", "-DskipTests")
	cmd.Dir = repoPath
	// Build the child env only when we have something to add, so the plain
	// "None + no JDK" case still inherits the parent env unchanged. Order
	// matters: active-set vars override the inherited env, and JAVA_HOME from
	// jdkPath is appended last so it wins over a set's JAVA_HOME.
	if len(envVars) > 0 || jdkPath != "" {
		env := append(os.Environ(), envVars...)
		if jdkPath != "" {
			env = append(env, "JAVA_HOME="+jdkPath)
		}
		cmd.Env = env
	}
	setProcAttr(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	session := newLogSession()
	pm.sessions[id] = session

	if err := cmd.Start(); err != nil {
		delete(pm.sessions, id)
		return err
	}
	pm.procs[id] = &managedProcess{cmd: cmd, pid: cmd.Process.Pid, envID: envID, envName: envName}
	pm.store.StorePid(id, ProcInfo{Pid: cmd.Process.Pid, EnvID: envID, EnvName: envName})

	go pipeLines(stdout, session)
	go pipeLines(stderr, session)

	go func() {
		cmd.Wait()
		pm.mu.Lock()
		delete(pm.procs, id)
		pm.mu.Unlock()
		pm.store.RemovePid(id)
		session.closeAll()
	}()

	return nil
}

func (pm *ProcessManager) Stop(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	proc, ok := pm.procs[id]
	if !ok {
		return fmt.Errorf("not running")
	}
	if proc.reconnected {
		killByPid(proc.pid)
	} else {
		killProc(proc.cmd)
	}
	delete(pm.procs, id)
	pm.store.RemovePid(id)
	return nil
}

func (pm *ProcessManager) Reconnect(id string, pid int, envID, envName string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.procs[id] = &managedProcess{pid: pid, reconnected: true, envID: envID, envName: envName}
}

// RunningEnv returns the environment id and name a running process was launched
// under, or empty strings if it isn't tracked.
func (pm *ProcessManager) RunningEnv(id string) (string, string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if proc, ok := pm.procs[id]; ok {
		return proc.envID, proc.envName
	}
	return "", ""
}

func (pm *ProcessManager) IsRunning(id string) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	proc, ok := pm.procs[id]
	if !ok {
		return false
	}
	if proc.reconnected && !isAlive(proc.pid) {
		delete(pm.procs, id)
		pm.store.RemovePid(id)
		return false
	}
	return true
}

func (pm *ProcessManager) IsReconnected(id string) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	proc, ok := pm.procs[id]
	return ok && proc.reconnected
}

func (pm *ProcessManager) GetSession(id string) *LogSession {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.sessions[id]
}

func pipeLines(r io.Reader, s *LogSession) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		s.append(scanner.Text())
	}
}
