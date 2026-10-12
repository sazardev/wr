//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// ownGroup puts the browser and everything it spawns in a process group of
// their own, and makes cancelling the command kill the whole group: Chromium
// forks renderers and a zygote that would otherwise outlive a timeout.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
