//go:build windows

package main

import "os/exec"

func setProcAttr(_ *exec.Cmd) {}

func killProc(cmd *exec.Cmd) {
	cmd.Process.Kill()
}
