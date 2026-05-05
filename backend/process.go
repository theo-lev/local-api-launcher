package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync"
)

type ProcessManager struct {
	mu       sync.Mutex
	procs    map[string]*exec.Cmd
	sessions map[string]*LogSession
}

func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		procs:    make(map[string]*exec.Cmd),
		sessions: make(map[string]*LogSession),
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

func (pm *ProcessManager) Start(id, repoPath, mavenPath string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if _, ok := pm.procs[id]; ok {
		return fmt.Errorf("already running")
	}

	cmd := exec.Command(mavenExe(mavenPath), "spring-boot:run", "-DskipTests")
	cmd.Dir = repoPath
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
	pm.procs[id] = cmd

	go pipeLines(stdout, session)
	go pipeLines(stderr, session)

	go func() {
		cmd.Wait()
		pm.mu.Lock()
		delete(pm.procs, id)
		pm.mu.Unlock()
		session.closeAll()
	}()

	return nil
}

func (pm *ProcessManager) Stop(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	cmd, ok := pm.procs[id]
	if !ok {
		return fmt.Errorf("not running")
	}
	killProc(cmd)
	delete(pm.procs, id)
	return nil
}

func (pm *ProcessManager) IsRunning(id string) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	_, ok := pm.procs[id]
	return ok
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
