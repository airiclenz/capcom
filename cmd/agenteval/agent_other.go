//go:build !unix

package main

import "os/exec"

// processGroupLedger is the file, named by AGENTEVAL_PROCESS_GROUPS, where on Unix the agent loop
// lists the process groups of its shell commands. Without Unix process groups there is nothing to
// list, and it records nothing.
type processGroupLedger string

// shellArgv is the argv that runs command through cmd.exe, the shell on Windows.
func shellArgv(command string) []string {
	return []string{"cmd", "/C", command}
}

// isolateProcessGroup leaves process as it is: without Unix process groups, cancellation kills
// the shell alone, and the wait delay bounds how long a child left holding the pipe can stall it.
func isolateProcessGroup(*exec.Cmd) {}

// isolateAgentProcessGroup leaves agent as it is: without Unix process groups, cancellation kills
// the agent alone, and the sweep it returns has nothing to kill, in the agent's group or in the
// ledger.
func isolateAgentProcessGroup(*exec.Cmd, processGroupLedger) (sweep func() error) {
	return func() error { return nil }
}

// recordStarted records nothing: without Unix process groups there is no group to sweep.
func (processGroupLedger) recordStarted(int) error { return nil }

// recordEnded records nothing: without Unix process groups there is no group to sweep.
func (processGroupLedger) recordEnded(int) error { return nil }
