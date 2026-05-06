//go:build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

func setProcAttr(_ *exec.Cmd) {}

func killProc(cmd *exec.Cmd) {
	cmd.Process.Kill()
}

func isAlive(pid int) bool {
	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	const STILL_ACTIVE = 259
	handle, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	var code uint32
	if err := syscall.GetExitCodeProcess(handle, &code); err != nil {
		return false
	}
	return code == STILL_ACTIVE
}

func killByPid(pid int) {
	if p, err := os.FindProcess(pid); err == nil {
		p.Kill()
	}
}
