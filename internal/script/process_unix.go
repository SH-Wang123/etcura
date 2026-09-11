//go:build !windows

package script

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup puts the child in its own process group so a timeout
// can terminate the whole tree. Without this, killing only the direct child
// leaves grandchildren (e.g. a shell's `sleep`) holding the output pipes, and
// Wait() blocks until they finish — a "timeout" that does not actually bound
// the wait.
func configureProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcessGroup terminates the child's whole process group (best effort).
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	if pgid, err := syscall.Getpgid(pid); err == nil && pgid > 0 {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		return
	}
	_ = cmd.Process.Kill()
}
