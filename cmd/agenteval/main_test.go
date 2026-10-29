package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom"
	"github.com/airiclenz/capcom/internal/cli"
)

// The names TestMain dispatches on: the test binary copied under one of them, by copyTestBinary,
// runs as that program instead of running the tests, so a test drives real subprocesses without
// a go build. shimName itself makes the copy the shim.
const (
	realCapcomName = "capcom-real"
	fakeAgentName  = "fake-agent"
)

// fakeScriptVariable names the fake agent's script: one JSON argv array per line, each run as
// `capcom <argv>` from PATH. Blank lines are skipped but counted, so an error's line number is
// the file's own.
const fakeScriptVariable = "AGENTEVAL_FAKE_SCRIPT"

// previousPlaceholderPrefix and previousPlaceholderSuffix delimit the fake agent's {prev:<regex>}
// placeholder: a whole argv element replaced by the first submatch of <regex> over the previous
// call's stdout. The regex is everything up to the element's last }, so a {n} quantifier inside
// it holds.
const (
	previousPlaceholderPrefix = "{prev:"
	previousPlaceholderSuffix = "}"
)

// executableFileMode is the permission copyTestBinary gives a copy.
const executableFileMode = 0o755

// TestMain runs the test binary as the program its basename names (shim, real capcom or fake
// agent), and runs the tests otherwise.
func TestMain(m *testing.M) {
	switch programName(os.Args[0]) {
	case shimName:
		os.Exit(runShim(os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr))
	case realCapcomName:
		os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
	case fakeAgentName:
		os.Exit(runFakeAgent(os.Getenv, os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

// runFakeAgent runs the script AGENTEVAL_FAKE_SCRIPT names, echoing every call's stdout to
// stdout and its stderr to stderr. A capcom call's own failure does not stop the script; a
// script error does, printed on stderr, exit 1.
func runFakeAgent(getenv func(string) string, stdout, stderr io.Writer) int {
	scriptPath := getenv(fakeScriptVariable)
	if scriptPath == "" {
		fmt.Fprintf(stderr, "fake agent: %s is not set\n", fakeScriptVariable)
		return exitRunFailed
	}
	if err := runFakeScript(scriptPath, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "fake agent: %v\n", err)
		return exitRunFailed
	}
	return 0
}

// runFakeScript runs each line of the script at scriptPath as one capcom call from PATH, its
// {prev:<regex>} placeholders filled from the previous call's stdout. An error names the script
// file and line.
func runFakeScript(scriptPath string, stdout, stderr io.Writer) error {
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return err
	}
	var previousStdout string
	for index, line := range strings.Split(string(script), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var argv []string
		if err := json.Unmarshal([]byte(line), &argv); err != nil {
			return fmt.Errorf("%s:%d: %w", scriptPath, index+1, err)
		}
		expanded, err := expandPrevious(argv, previousStdout)
		if err != nil {
			return fmt.Errorf("%s:%d: %w", scriptPath, index+1, err)
		}
		previousStdout, err = runScriptedCall(expanded, stdout, stderr)
		if err != nil {
			return fmt.Errorf("%s:%d: %w", scriptPath, index+1, err)
		}
	}
	return nil
}

// expandPrevious returns argv with every {prev:<regex>} element replaced by the first submatch
// of <regex> over previousStdout. A regex that does not compile, has no submatch, or does not
// match is an error naming it.
func expandPrevious(argv []string, previousStdout string) ([]string, error) {
	expanded := make([]string, 0, len(argv))
	for _, element := range argv {
		if !strings.HasPrefix(element, previousPlaceholderPrefix) || !strings.HasSuffix(element, previousPlaceholderSuffix) {
			expanded = append(expanded, element)
			continue
		}
		pattern := strings.TrimSuffix(strings.TrimPrefix(element, previousPlaceholderPrefix), previousPlaceholderSuffix)
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("placeholder %s: %w", element, err)
		}
		match := compiled.FindStringSubmatch(previousStdout)
		if len(match) < 2 {
			return nil, fmt.Errorf("placeholder %s: no submatch in the previous call's stdout", element)
		}
		expanded = append(expanded, match[1])
	}
	return expanded, nil
}

// runScriptedCall runs `capcom <argv>` from PATH with empty stdin, echoing its streams, and
// returns its stdout. A call that exits non-zero is no error: the script goes on.
func runScriptedCall(argv []string, stdout, stderr io.Writer) (string, error) {
	var captured bytes.Buffer
	command := exec.Command(shimName, argv...)
	command.Stdout = io.MultiWriter(&captured, stdout)
	command.Stderr = stderr
	err := command.Run()
	var exited *exec.ExitError
	if err != nil && !errors.As(err, &exited) {
		return "", err
	}
	return captured.String(), nil
}

// copyTestBinary copies the running test binary into dir as name, with the .exe suffix on
// Windows, and returns the copy's path.
func copyTestBinary(t *testing.T, dir, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += windowsExecutableSuffix
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	source, err := os.Open(executable)
	if err != nil {
		t.Fatalf("open the test binary: %v", err)
	}
	defer source.Close()
	path := filepath.Join(dir, name)
	target, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, executableFileMode)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		t.Fatalf("copy the test binary to %s: %v", path, err)
	}
	if err := target.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
	return path
}

// setHermeticGitEnv isolates git from user and system configuration and fixes the author and
// committer identity, so git-backed tests behave the same on every machine. It is a copy of
// internal/cli's test helper, which a test of another package cannot import.
func setHermeticGitEnv(t *testing.T) {
	t.Helper()
	environment := map[string]string{
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "Test Author",
		"GIT_AUTHOR_EMAIL":    "author@example.com",
		"GIT_COMMITTER_NAME":  "Test Committer",
		"GIT_COMMITTER_EMAIL": "committer@example.com",
	}
	for key, value := range environment {
		t.Setenv(key, value)
	}
}

// readCallLog returns the calls of the call log at path, one per line.
func readCallLog(t *testing.T, path string) []shimCall {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the call log: %v", err)
	}
	var calls []shimCall
	for _, line := range strings.Split(strings.TrimSuffix(string(content), "\n"), "\n") {
		var call shimCall
		if err := json.Unmarshal([]byte(line), &call); err != nil {
			t.Fatalf("call log line %q: %v", line, err)
		}
		calls = append(calls, call)
	}
	return calls
}

func TestExitCodeFor_TellsARunFailureFromBadUsage(t *testing.T) {
	failing := runE(func(*cobra.Command, []string) error { return errors.New("the run failed") })
	runFailure := failing(&cobra.Command{}, nil)

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "run failure", err: runFailure, want: exitRunFailed},
		{name: "usage error", err: errors.New("unknown flag"), want: exitBadUsage},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := exitCodeFor(test.err); got != test.want {
				t.Errorf("exitCodeFor(%v) = %d, want %d", test.err, got, test.want)
			}
		})
	}
}

func TestRunE_PassesSuccessThrough(t *testing.T) {
	succeeding := runE(func(*cobra.Command, []string) error { return nil })

	if err := succeeding(&cobra.Command{}, nil); err != nil {
		t.Errorf("runE over a succeeding body = %v, want nil", err)
	}
}

func TestExpandPrevious_FillsThePlaceholderFromThePreviousStdout(t *testing.T) {
	previousStdout := "acquired\ntoken: 0123456789abcdef0123456789abcdef\n"
	argv := []string{"lock", "release", "--token", "{prev:token: ([0-9a-f]{32})}"}

	expanded, err := expandPrevious(argv, previousStdout)

	if err != nil {
		t.Fatalf("expandPrevious: %v", err)
	}
	want := []string{"lock", "release", "--token", "0123456789abcdef0123456789abcdef"}
	if !slices.Equal(expanded, want) {
		t.Errorf("expanded = %q, want %q", expanded, want)
	}
}

func TestExpandPrevious_LeavesOtherElementsAlone(t *testing.T) {
	argv := []string{"{prev:", "prev:x}", "{not a placeholder}", "plain"}

	expanded, err := expandPrevious(argv, "")

	if err != nil {
		t.Fatalf("expandPrevious: %v", err)
	}
	if !slices.Equal(expanded, argv) {
		t.Errorf("expanded = %q, want %q unchanged", expanded, argv)
	}
}

func TestExpandPrevious_RefusesABadPlaceholder(t *testing.T) {
	tests := []struct {
		name    string
		element string
	}{
		{name: "no match", element: "{prev:token: ([0-9a-f]{32})}"},
		{name: "no submatch", element: "{prev:capcom}"},
		{name: "bad regex", element: "{prev:token: ([0-9a-f]}"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := expandPrevious([]string{test.element}, "capcom 1.2.3\n")

			if err == nil || !strings.Contains(err.Error(), test.element) {
				t.Errorf("expandPrevious(%q) error = %v, want one naming the placeholder", test.element, err)
			}
		})
	}
}

func TestRunFakeAgent_NoMatchNamesTheScriptLine(t *testing.T) {
	scriptPath := filepath.Join(t.TempDir(), "script.jsonl")
	script := "\n" + `["version", "{prev:capcom (\\S+)}"]` + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write the script: %v", err)
	}
	var stderr bytes.Buffer

	exitCode := runFakeAgent(shimEnvironment(map[string]string{fakeScriptVariable: scriptPath}), io.Discard, &stderr)

	if exitCode != exitRunFailed {
		t.Errorf("exit = %d, want %d", exitCode, exitRunFailed)
	}
	if want := scriptPath + ":2:"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr = %q, want it to name %s", stderr.String(), want)
	}
}

func TestRunFakeAgent_MissingScriptVariableNamesIt(t *testing.T) {
	var stderr bytes.Buffer

	exitCode := runFakeAgent(shimEnvironment(nil), io.Discard, &stderr)

	if exitCode != exitRunFailed || !strings.Contains(stderr.String(), fakeScriptVariable) {
		t.Errorf("exit = %d, stderr = %q, want exit %d naming %s", exitCode, stderr.String(), exitRunFailed, fakeScriptVariable)
	}
}

// TestFakeAgent_PassesAPreviousCallsOutputToTheNext runs the test binary as the fake agent, with
// the shim first on its PATH, so the call log shows the second call's argument filled from the
// first call's stdout by a regex holding a {n} quantifier.
func TestFakeAgent_PassesAPreviousCallsOutputToTheNext(t *testing.T) {
	setHermeticGitEnv(t)
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatalf("create the bin directory: %v", err)
	}
	callLog := filepath.Join(dir, "calls.jsonl")
	scriptPath := filepath.Join(dir, "script.jsonl")
	script := `["version"]` + "\n" + `["version", "{prev:^capcom (\\S{1,64})\\n}"]` + "\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write the script: %v", err)
	}
	copyTestBinary(t, binDir, shimName)
	command := exec.Command(copyTestBinary(t, dir, fakeAgentName))
	command.Dir = dir
	command.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		capcomVariable+"="+copyTestBinary(t, dir, realCapcomName),
		callLogVariable+"="+callLog,
		fakeScriptVariable+"="+scriptPath,
	)

	output, err := command.CombinedOutput()

	if err != nil {
		t.Fatalf("run the fake agent: %v\n%s", err, output)
	}
	logged := readCallLog(t, callLog)
	if len(logged) != 2 {
		t.Fatalf("call log has %d lines, want 2: %+v", len(logged), logged)
	}
	if want := []string{"version", capcom.Version()}; !slices.Equal(logged[1].Args, want) {
		t.Errorf("second call args = %q, want %q", logged[1].Args, want)
	}
}
