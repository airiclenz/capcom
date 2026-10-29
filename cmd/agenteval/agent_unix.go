//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// shellArgv is the argv that runs command through the POSIX shell.
func shellArgv(command string) []string {
	return []string{"sh", "-c", command}
}

// isolateProcessGroup starts process in a process group of its own and has its cancellation kill
// that whole group, so a background job or a child holding the output pipe dies with it.
func isolateProcessGroup(process *exec.Cmd) {
	process.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	process.Cancel = func() error {
		return syscall.Kill(-process.Process.Pid, syscall.SIGKILL)
	}
}
