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
