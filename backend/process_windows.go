//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"syscall"
)

// CREATE_NO_WINDOW: children write to our pipes, they never need a console window.
const createNoWindow = 0x08000000

func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

// killTree terminates pid and all of its descendants. mvn.cmd is a cmd.exe
// wrapper that spawns java as a child, so killing only the wrapper leaves
// the API running (issue #3); taskkill /T walks the whole tree.
func killTree(pid int) {
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	kill.Run()
}

func killProc(cmd *exec.Cmd) {
	killTree(cmd.Process.Pid)
}

func killByPid(pid int) {
	killTree(pid)
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

func pauseOnExit() {
	// Keep the console window open so the user can read the error before
	// the window closes (double-clicked exe spawns its own console).
	print("\nPress Enter to exit...")
	var buf [1]byte
	syscall.Read(syscall.Stdin, buf[:])
}
