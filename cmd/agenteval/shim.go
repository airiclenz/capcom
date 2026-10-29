package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The shim's contract with the harness: the name it answers to, and the environment variables
// naming the capcom build under test and the call log it appends to.
const (
	shimName        = "capcom"
	capcomVariable  = "AGENTEVAL_CAPCOM"
	callLogVariable = "AGENTEVAL_CALL_LOG"
)

// windowsExecutableSuffix is the suffix every executable carries on Windows; programName strips
// it so the shim answers to capcom and capcom.exe alike.
const windowsExecutableSuffix = ".exe"

// callLogFileMode is the permission a call log is created with.
const callLogFileMode = 0o644

// shimCall is one line of the call log: one capcom call the agent made.
type shimCall struct {
	Args         []string `json:"args"`
	Exit         int      `json:"exit"`
	Milliseconds int64    `json:"ms"`
}

// programName reports the name a program was started under: the base name of program (os.Args[0])
// without its Windows executable suffix.
func programName(program string) string {
	return strings.TrimSuffix(filepath.Base(program), windowsExecutableSuffix)
}

// runShim runs the capcom build named by AGENTEVAL_CAPCOM with args and the given streams, then
// appends the call to the file named by AGENTEVAL_CALL_LOG, and returns the build's exit code. It
// returns 1 without running anything when either variable is unset, and 1 when the build cannot
// be started (the call is still logged, with that exit code) or the call log cannot be written.
func runShim(args []string, getenv func(string) string, stdin io.Reader, stdout, stderr io.Writer) int {
	capcomPath, callLogPath := getenv(capcomVariable), getenv(callLogVariable)
	for _, required := range []struct{ name, value string }{
		{capcomVariable, capcomPath},
		{callLogVariable, callLogPath},
	} {
		if required.value == "" {
			fmt.Fprintf(stderr, "agenteval shim: %s is not set\n", required.name)
			return exitRunFailed
		}
	}

	exitCode, elapsed := runCapcom(capcomPath, args, stdin, stdout, stderr)

	call := shimCall{Args: append([]string{}, args...), Exit: exitCode, Milliseconds: elapsed.Milliseconds()}
	if err := appendCall(callLogPath, call); err != nil {
		fmt.Fprintf(stderr, "agenteval shim: %s: %v\n", callLogPath, err)
		return exitRunFailed
	}
	return exitCode
}

// runCapcom runs the build at capcomPath with args and the given streams and reports its exit
// code and how long it ran. A build that cannot be started, or is killed by a signal, reports 1
// with the failure printed on stderr.
func runCapcom(capcomPath string, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, time.Duration) {
	command := exec.Command(capcomPath, args...)
	command.Stdin, command.Stdout, command.Stderr = stdin, stdout, stderr

	started := time.Now()
	err := command.Run()
	elapsed := time.Since(started)

	if err == nil {
		return 0, elapsed
	}
	var exited *exec.ExitError
	if errors.As(err, &exited) && exited.ExitCode() >= 0 {
		return exited.ExitCode(), elapsed
	}
	fmt.Fprintf(stderr, "agenteval shim: %s: %v\n", capcomPath, err)
	return exitRunFailed, elapsed
}

// appendCall appends call to the call log at path as one JSON line, creating the file when it
// does not exist yet. The line goes out in a single write, so concurrent calls never interleave
// within a line.
func appendCall(path string, call shimCall) error {
	line, err := json.Marshal(call)
	if err != nil {
		return fmt.Errorf("encode the call: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, callLogFileMode)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		return errors.Join(err, file.Close())
	}
	return file.Close()
}
