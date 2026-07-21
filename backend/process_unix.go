//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProc(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}

func isAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func killByPid(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}

func pauseOnExit() {}
