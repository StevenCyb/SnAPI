//go:build !windows

package cmd

import (
	"os/exec"
	"syscall"
)

// setProcessGroup sets the SysProcAttr of cmd to create a new process group.
// This allows us to kill the entire process group later, which is necessary
// on Unix-like systems.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
