//go:build !unix

package main

import "os/exec"

// shellArgv is the argv that runs command through cmd.exe, the shell on Windows.
func shellArgv(command string) []string {
	return []string{"cmd", "/C", command}
}

// isolateProcessGroup leaves process as it is: without Unix process groups, cancellation kills
// the shell alone, and the wait delay bounds how long a child left holding the pipe can stall it.
func isolateProcessGroup(*exec.Cmd) {}

// isolateAgentProcessGroup leaves agent as it is: without Unix process groups, cancellation kills
// the agent alone, and the sweep it returns has nothing to kill.
func isolateAgentProcessGroup(*exec.Cmd) (sweep func() error) {
	return func() error { return nil }
}
