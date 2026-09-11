//go:build windows

package script

import "os/exec"

// Windows has no process groups in the Unix sense; CommandContext kills the
// direct child and Cmd.WaitDelay bounds the wait for its pipes.
func configureProcessGroup(_ *exec.Cmd) {}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
