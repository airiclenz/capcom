package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
)

// recordedCall is one external command a verb asked the runner for.
type recordedCall struct {
	dir  string
	env  []string
	name string
	args []string
}

// fakeRunner is the recording stand-in for execRunner: it answers each command from respond and
// remembers every call, so a test pins exactly what a verb would have run without running it.
type fakeRunner struct {
	respond func(call recordedCall) (string, error)

	mu    sync.Mutex
	calls []recordedCall
}

// run records the call and answers it.
func (f *fakeRunner) run(_ context.Context, dir string, env []string, name string, args ...string) (string, error) {
	call := recordedCall{dir: dir, env: slices.Clone(env), name: name, args: slices.Clone(args)}
	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.mu.Unlock()
	return f.respond(call)
}

// recorded returns the calls made so far whose command is name.
func (f *fakeRunner) recorded(name string) []recordedCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var matching []recordedCall
	for _, call := range f.calls {
		if call.name == name {
			matching = append(matching, call)
		}
	}
	return matching
}

// execute runs the program's command tree over commands with args, the checkout's repo the
// execRepo over commands as in production, returning the error main would turn into an exit code
// and everything the command printed.
func execute(t *testing.T, commands runner, args ...string) (string, error) {
	t.Helper()
	return executeWith(t, commands, execRepos(commands), args...)
}

// executeWith runs the program's command tree as execute does, every verb opening its repo with
// openRepo.
func executeWith(t *testing.T, commands runner, openRepo repoOpener, args ...string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	root := newRootCommand(commands, openRepo)
	root.SetArgs(args)
	root.SetOut(&output)
	root.SetErr(&output)
	err := root.ExecuteContext(t.Context())
	return output.String(), err
}

// TestBadCommandLineIsAUsageError pins the difference between the two ways this program can
// fail: a command line it cannot parse never starts a run, so it exits 2 — with the usage text
// when a verb's flags or arguments are wrong — and the runner is never reached.
func TestBadCommandLineIsAUsageError(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args         []string
		isUsageShown bool
	}{
		"unknown flag":   {args: []string{"dist", "--no-such-flag"}, isUsageShown: true},
		"stray argument": {args: []string{"dist", "extra"}, isUsageShown: true},
		"unknown verb":   {args: []string{"no-such-verb"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			commands := &fakeRunner{respond: func(recordedCall) (string, error) {
				return "", errors.New("the runner must not be reached")
			}}

			output, err := execute(t, commands, tc.args...)

			if err == nil {
				t.Fatalf("release %v succeeded", tc.args)
			}
			if code := exitCodeFor(err); code != exitBadUsage {
				t.Errorf("exit code = %d, want %d", code, exitBadUsage)
			}
			if tc.isUsageShown && !strings.Contains(output, "Usage:") {
				t.Errorf("output does not print the usage:\n%s", output)
			}
			if len(commands.calls) != 0 {
				t.Errorf("runner was called %d times, want none", len(commands.calls))
			}
		})
	}
}

// TestCommandTreeOpensTheInjectedRepo pins the repo factory's wiring: a verb run through the
// command tree reads the checkout through the repo newRootCommand was handed, never through
// argv of its own — here status's gate, judged from a memRepo whose tree is dirty while every
// runner command fails.
func TestCommandTreeOpensTheInjectedRepo(t *testing.T) {
	t.Parallel()
	checkout := memRepo{commits: linearHistory(), head: bumpCommit, originMain: bumpCommit,
		isDirty: true, greenRuns: []string{featureCommit}}
	var openedDir string
	openRepo := func(dir string) repo {
		openedDir = dir
		return checkout
	}
	commands := &fakeRunner{respond: func(call recordedCall) (string, error) {
		return "", errors.New(call.name + ": unavailable")
	}}

	output, err := executeWith(t, commands, openRepo, "status", "--json", "v0.1.0")

	if err == nil {
		t.Fatal("status succeeded over a dirty tree")
	}
	var report statusReport
	if decodeErr := json.Unmarshal([]byte(output), &report); decodeErr != nil {
		t.Fatalf("status --json printed no JSON report: %v\n%s", decodeErr, output)
	}
	want := gateReport{Commit: featureCommit, Failures: []string{"the working tree is not clean"}}
	if report.Gate.IsOK || report.Gate.Commit != want.Commit || !slices.Equal(report.Gate.Failures, want.Failures) {
		t.Errorf("gate = %+v, want %+v", report.Gate, want)
	}
	if openedDir != "." {
		t.Errorf("repo opened at %q, want the checkout %q", openedDir, ".")
	}
	gateCommands := []string{"git fetch", "git status", "git rev-parse", "git rev-list --parents",
		"git diff-tree", "git merge-base", "gh run list"}
	for _, call := range commands.calls {
		command := call.name + " " + strings.Join(call.args, " ")
		if slices.ContainsFunc(gateCommands, func(gate string) bool { return strings.HasPrefix(command, gate) }) {
			t.Errorf("status ran %q itself instead of asking the repo", command)
		}
	}
}
