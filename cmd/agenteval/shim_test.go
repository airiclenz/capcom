package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom"
)

// shimEnvironment is a getenv over a fixed set of variables, the way runShim reads its own.
func shimEnvironment(variables map[string]string) func(string) string {
	return func(name string) string { return variables[name] }
}

// runShimInProcess runs runShim with args against the test binary copied as the real capcom,
// and returns its exit code, its stdout and stderr, and the call log's path.
func runShimInProcess(t *testing.T, args ...string) (int, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	callLog := filepath.Join(dir, "calls.jsonl")
	environment := shimEnvironment(map[string]string{
		capcomVariable:  copyTestBinary(t, dir, realCapcomName),
		callLogVariable: callLog,
	})
	var stdout, stderr bytes.Buffer

	exitCode := runShim(args, environment, strings.NewReader(""), &stdout, &stderr)

	return exitCode, stdout.String(), stderr.String(), callLog
}

func TestRunShim_PassesTheExitCodeThrough(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "success", args: []string{"version"}, want: 0},
		{name: "usage error", args: []string{"no-such-command"}, want: exitBadUsage},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exitCode, _, _, _ := runShimInProcess(t, test.args...)

			if exitCode != test.want {
				t.Errorf("runShim(%q) = %d, want %d", test.args, exitCode, test.want)
			}
		})
	}
}

func TestRunShim_PassesStdoutThrough(t *testing.T) {
	_, stdout, _, _ := runShimInProcess(t, "version")

	if want := "capcom " + capcom.Version() + "\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestRunShim_LogsOneLinePerCallWithItsArgs(t *testing.T) {
	dir := t.TempDir()
	callLog := filepath.Join(dir, "calls.jsonl")
	environment := shimEnvironment(map[string]string{
		capcomVariable:  copyTestBinary(t, dir, realCapcomName),
		callLogVariable: callLog,
	})
	calls := [][]string{{"version"}, {"version", "--json"}, {"no-such-command", "an argument"}}

	for _, args := range calls {
		runShim(args, environment, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
	}

	logged := readCallLog(t, callLog)
	if len(logged) != len(calls) {
		t.Fatalf("call log has %d lines, want %d: %+v", len(logged), len(calls), logged)
	}
	for index, args := range calls {
		if !slices.Equal(logged[index].Args, args) {
			t.Errorf("line %d args = %q, want %q", index+1, logged[index].Args, args)
		}
	}
	if logged[2].Exit != exitBadUsage {
		t.Errorf("line 3 exit = %d, want %d", logged[2].Exit, exitBadUsage)
	}
}

func TestRunShim_LogsNoArgsAsAnEmptyArray(t *testing.T) {
	_, _, _, callLog := runShimInProcess(t)

	content, err := os.ReadFile(callLog)
	if err != nil {
		t.Fatalf("read the call log: %v", err)
	}
	if !strings.HasPrefix(string(content), `{"args":[],`) {
		t.Errorf("call log = %q, want it to start with {\"args\":[],", content)
	}
}

func TestRunShim_MissingVariableIsAnErrorNamingIt(t *testing.T) {
	dir := t.TempDir()
	callLog := filepath.Join(dir, "calls.jsonl")
	tests := []struct {
		name      string
		variables map[string]string
		missing   string
	}{
		{name: "no capcom", variables: map[string]string{callLogVariable: callLog}, missing: capcomVariable},
		{name: "no call log", variables: map[string]string{capcomVariable: "capcom"}, missing: callLogVariable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer

			exitCode := runShim([]string{"version"}, shimEnvironment(test.variables), strings.NewReader(""), &bytes.Buffer{}, &stderr)

			if exitCode != exitRunFailed {
				t.Errorf("exit = %d, want %d", exitCode, exitRunFailed)
			}
			if !strings.Contains(stderr.String(), test.missing) {
				t.Errorf("stderr = %q, want it to name %s", stderr.String(), test.missing)
			}
			if _, err := os.Stat(callLog); !os.IsNotExist(err) {
				t.Errorf("call log exists after a refused call (stat: %v)", err)
			}
		})
	}
}

func TestRunShim_UnstartableBuildExitsOneAndIsLogged(t *testing.T) {
	dir := t.TempDir()
	missingBuild := filepath.Join(dir, "no-such-capcom")
	callLog := filepath.Join(dir, "calls.jsonl")
	environment := shimEnvironment(map[string]string{capcomVariable: missingBuild, callLogVariable: callLog})
	var stderr bytes.Buffer

	exitCode := runShim([]string{"version"}, environment, strings.NewReader(""), &bytes.Buffer{}, &stderr)

	if exitCode != exitRunFailed {
		t.Errorf("exit = %d, want %d", exitCode, exitRunFailed)
	}
	if !strings.Contains(stderr.String(), missingBuild) {
		t.Errorf("stderr = %q, want it to name %s", stderr.String(), missingBuild)
	}
	if logged := readCallLog(t, callLog); len(logged) != 1 || logged[0].Exit != exitRunFailed {
		t.Errorf("call log = %+v, want one call with exit %d", logged, exitRunFailed)
	}
}

func TestRunShim_UnwritableCallLogExitsOneNamingIt(t *testing.T) {
	dir := t.TempDir()
	callLog := filepath.Join(dir, "no-such-directory", "calls.jsonl")
	environment := shimEnvironment(map[string]string{
		capcomVariable:  copyTestBinary(t, dir, realCapcomName),
		callLogVariable: callLog,
	})
	var stderr bytes.Buffer

	exitCode := runShim([]string{"version"}, environment, strings.NewReader(""), &bytes.Buffer{}, &stderr)

	if exitCode != exitRunFailed {
		t.Errorf("exit = %d, want %d", exitCode, exitRunFailed)
	}
	if !strings.Contains(stderr.String(), callLog) {
		t.Errorf("stderr = %q, want it to name %s", stderr.String(), callLog)
	}
}

func TestProgramName_MatchesTheShimWithOrWithoutExeSuffix(t *testing.T) {
	tests := []struct {
		program string
		isShim  bool
	}{
		{program: "capcom", isShim: true},
		{program: "capcom.exe", isShim: true},
		{program: filepath.Join("bin", "capcom"), isShim: true},
		{program: filepath.Join("bin", "capcom.exe"), isShim: true},
		{program: "agenteval", isShim: false},
		{program: "capcom-real", isShim: false},
		{program: "capcom.exe.old", isShim: false},
	}
	for _, test := range tests {
		t.Run(test.program, func(t *testing.T) {
			if isShim := programName(test.program) == shimName; isShim != test.isShim {
				t.Errorf("programName(%q) == %q is %v, want %v", test.program, shimName, isShim, test.isShim)
			}
		})
	}
}

// TestShim_AsSubprocessRunsCapcomAndLogsTheCall runs the test binary copied as capcom, the way
// an agent reaches the shim from PATH.
func TestShim_AsSubprocessRunsCapcomAndLogsTheCall(t *testing.T) {
	setHermeticGitEnv(t)
	dir := t.TempDir()
	callLog := filepath.Join(dir, "calls.jsonl")
	shim := copyTestBinary(t, dir, shimName)
	command := exec.Command(shim, "version")
	command.Dir = dir
	command.Env = append(os.Environ(),
		capcomVariable+"="+copyTestBinary(t, dir, realCapcomName),
		callLogVariable+"="+callLog,
	)

	stdout, err := command.Output()

	if err != nil {
		t.Fatalf("run the shim: %v", err)
	}
	if want := "capcom " + capcom.Version() + "\n"; string(stdout) != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	logged := readCallLog(t, callLog)
	if len(logged) != 1 || !slices.Equal(logged[0].Args, []string{"version"}) || logged[0].Exit != 0 {
		t.Errorf("call log = %+v, want one call of [version] with exit 0", logged)
	}
}
