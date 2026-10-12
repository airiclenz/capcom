package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These tests drive the production execRunner against a real child process: the test binary
// itself, which TestMain (main_test.go) turns into runRunnerHelper when runnerHelperVariable is
// set. No other program runs, so the tests need no tool on PATH and reach nothing outside the
// temp directory.

const (
	// runnerHelperVariable names the helper mode; only a child started by these tests has it.
	runnerHelperVariable = "CAPCOM_RELEASE_RUNNER_HELPER"
	// runnerProbeVariable is the variable the env test sets both inherited and through env.
	runnerProbeVariable = "CAPCOM_RELEASE_RUNNER_PROBE"
	// runnerStartedFile is the file the sleeping helper creates in its directory once it runs.
	runnerStartedFile = "started"
	// runnerHelperExitCode is the status the failing helper exits with.
	runnerHelperExitCode = 3
)

// runnerVerbatimOutput is what the stdout helper writes: blank lines and leading and trailing
// whitespace that any trimming would lose.
const runnerVerbatimOutput = "\n  indented line\twith a tab  \n\nlast line without a newline   "

// runnerFailureDetail is what the failing helper writes to stderr.
const runnerFailureDetail = "helper refused: the probe was told to fail"

// runRunnerHelper is the child process's whole program: it acts out mode and returns its exit
// status.
func runRunnerHelper(mode string) int {
	switch mode {
	case "env":
		fmt.Print(os.Getenv(runnerProbeVariable))
	case "cwd":
		directory, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "getwd: %v", err)
			return 1
		}
		fmt.Print(directory)
	case "stdout":
		fmt.Print(runnerVerbatimOutput)
	case "fail":
		fmt.Fprint(os.Stderr, runnerFailureDetail)
		return runnerHelperExitCode
	case "sleep":
		if err := os.WriteFile(runnerStartedFile, nil, 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v", runnerStartedFile, err)
			return 1
		}
		time.Sleep(10 * time.Minute)
	default:
		fmt.Fprintf(os.Stderr, "unknown runner helper mode %q", mode)
		return 2
	}
	return 0
}

// helperExecutable returns the path of the running test binary, the helper child's program.
func helperExecutable(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	return executable
}

// runHelper runs the test binary in helper mode through execRunner, in dir, with extra appended
// to the environment after the mode.
func runHelper(t *testing.T, dir, mode string, extra ...string) (string, error) {
	t.Helper()
	env := append([]string{runnerHelperVariable + "=" + mode}, extra...)
	return execRunner{}.run(t.Context(), dir, env, helperExecutable(t))
}

// TestExecRunnerEnvEntryOverridesAnInheritedVariable pins that env is appended to the inherited
// environment and wins over it.
func TestExecRunnerEnvEntryOverridesAnInheritedVariable(t *testing.T) {
	t.Setenv(runnerProbeVariable, "inherited")

	output, err := runHelper(t, t.TempDir(), "env", runnerProbeVariable+"=from env")

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if output != "from env" {
		t.Errorf("child saw %s=%q, want %q", runnerProbeVariable, output, "from env")
	}
}

// TestExecRunnerRunsTheCommandInDir pins that the child's working directory is dir. The paths
// are compared as files, not strings: a temp directory can be reached through a symlink (macOS
// /var -> /private/var).
func TestExecRunnerRunsTheCommandInDir(t *testing.T) {
	dir := t.TempDir()

	output, err := runHelper(t, dir, "cwd")

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	want, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat %s: %v", dir, err)
	}
	got, err := os.Stat(output)
	if err != nil {
		t.Fatalf("stat the child's cwd %q: %v", output, err)
	}
	if !os.SameFile(got, want) {
		t.Errorf("child ran in %q, want %q", output, dir)
	}
}

// TestExecRunnerReturnsStdoutVerbatim pins that the result is the child's stdout byte for byte,
// never trimmed.
func TestExecRunnerReturnsStdoutVerbatim(t *testing.T) {
	output, err := runHelper(t, t.TempDir(), "stdout")

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if output != runnerVerbatimOutput {
		t.Errorf("stdout = %q, want %q", output, runnerVerbatimOutput)
	}
}

// TestExecRunnerFailureCarriesStderr pins that a command exiting non-zero is an error that
// carries what it wrote to stderr.
func TestExecRunnerFailureCarriesStderr(t *testing.T) {
	_, err := runHelper(t, t.TempDir(), "fail")

	if err == nil {
		t.Fatal("run succeeded, want an error for a non-zero exit")
	}
	if !strings.Contains(err.Error(), runnerFailureDetail) {
		t.Errorf("error %q does not carry the child's stderr %q", err, runnerFailureDetail)
	}
}

// TestExecRunnerCancelledContextKillsTheChild pins that cancelling ctx kills a running child
// promptly instead of waiting for it to finish: the child sleeps for minutes, run must return
// within seconds of the cancel.
func TestExecRunnerCancelledContextKillsTheChild(t *testing.T) {
	const (
		startDeadline = 30 * time.Second
		killDeadline  = 10 * time.Second
	)
	dir := t.TempDir()
	executable := helperExecutable(t)
	env := []string{runnerHelperVariable + "=sleep"}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := execRunner{}.run(ctx, dir, env, executable)
		result <- err
	}()
	waitForFile(t, filepath.Join(dir, runnerStartedFile), startDeadline)

	cancel()

	select {
	case err := <-result:
		if err == nil {
			t.Error("run succeeded, want an error for a killed child")
		}
	case <-time.After(killDeadline):
		t.Fatalf("run still waiting %v after the context was cancelled", killDeadline)
	}
}

// waitForFile polls until path exists, failing the test after deadline.
func waitForFile(t *testing.T, path string, deadline time.Duration) {
	t.Helper()
	const interval = 10 * time.Millisecond
	for waited := time.Duration(0); waited < deadline; waited += interval {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("%s did not appear within %v", path, deadline)
}
