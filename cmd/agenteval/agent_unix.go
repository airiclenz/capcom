//go:build unix

package main

import (
	"errors"
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

// isolateAgentProcessGroup starts agent in a process group of its own and has its cancellation
// send that whole group SIGTERM, so the agent and everything it started can wind down. The sweep
// it returns sends SIGKILL to whatever is still in the group: call it once agent has returned, so
// no process the agent started outlives the run. The sweep does nothing for an agent that never
// started, and a group already empty is no error.
func isolateAgentProcessGroup(agent *exec.Cmd) (sweep func() error) {
	agent.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	agent.Cancel = func() error {
		return syscall.Kill(-agent.Process.Pid, syscall.SIGTERM)
	}
	return func() error {
		if agent.Process == nil {
			return nil
		}
		err := syscall.Kill(-agent.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
}
