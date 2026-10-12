//go:build windows

package main

import "os/exec"

// ownGroup is a no-op on Windows: killing the process is all that is attempted.
func ownGroup(cmd *exec.Cmd) {}
