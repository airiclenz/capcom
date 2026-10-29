package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// reportTimeColumn is the index of the Time cell among a report row's cells, the one cell that
// differs between two runs of the same script.
const reportTimeColumn = 5

// executeCommand runs the agenteval command tree with args, as main does, and returns the exit
// code main would exit with, and the command's stdout and its error text.
func executeCommand(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	root := newRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetArgs(args)
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	err := root.ExecuteContext(context.Background())

	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	return exitCodeFor(err), stdout.String(), stderr.String() + err.Error()
}

// runNextClaimClose runs `run --only next-claim-close` with the report at out, the test binary
// copied as the real capcom and as the fake agent on script, and flags added before `--`, and
// returns the exit code and the report written.
func runNextClaimClose(t *testing.T, out string, script [][]string, flags ...string) (int, string) {
	t.Helper()
	setHermeticGitEnv(t)
	dir := t.TempDir()
	t.Setenv(fakeScriptVariable, writeScript(t, dir, script))

	args := append([]string{
		"run", "--out", out, "--capcom", copyTestBinary(t, dir, realCapcomName), "--only", "next-claim-close",
	}, flags...)

	exitCode, _, errorText := executeCommand(t, append(args, "--", copyTestBinary(t, dir, fakeAgentName))...)

	if exitCode != 0 {
		t.Fatalf("run exit = %d, want 0: %s", exitCode, errorText)
	}
	report, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read the report: %v", err)
	}
	return exitCode, string(report)
}

// reportRow returns the cells of the report's row for task, without the Time cell.
func reportRow(t *testing.T, report, task string) []string {
	t.Helper()
	for _, line := range strings.Split(report, "\n") {
		if !strings.HasPrefix(line, "| "+task+" |") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for index := range cells {
			cells[index] = strings.TrimSpace(cells[index])
		}
		return slices.Delete(cells, reportTimeColumn, reportTimeColumn+1)
	}
	t.Fatalf("the report has no row for %s:\n%s", task, report)
	return nil
}

func TestRun_ACorrectScriptPassesTheTask(t *testing.T) {
	out := filepath.Join(t.TempDir(), "eval.md")

	_, report := runNextClaimClose(t, out, nextClaimCloseScript)

	if row := reportRow(t, report, "next-claim-close"); row[1] != resultPassed {
		t.Errorf("next-claim-close row = %q, want a pass\n%s", row, report)
	}
}

func TestRun_WritesTheTranscriptAndCallLogBesideTheReport(t *testing.T) {
	dir := t.TempDir()

	runNextClaimClose(t, filepath.Join(dir, "eval.md"), nextClaimCloseScript)

	transcripts := filepath.Join(dir, "eval"+transcriptsDirSuffix)
	transcript, err := os.ReadFile(filepath.Join(transcripts, "next-claim-close"+transcriptExtension))
	if err != nil || !strings.Contains(string(transcript), "fix-login-timeout") {
		t.Errorf("transcript = %q, %v; want the agent's output", transcript, err)
	}
	calls := mustReadCallLog(t, filepath.Join(transcripts, "next-claim-close"+callLogExtension))
	if len(calls) != len(nextClaimCloseScript) {
		t.Errorf("call log has %d calls, want %d: %+v", len(calls), len(nextClaimCloseScript), calls)
	}
}

func TestRun_AnEmptyScriptFailsTheTaskWithAWhy(t *testing.T) {
	out := filepath.Join(t.TempDir(), "eval.md")

	_, report := runNextClaimClose(t, out, nil)

	row := reportRow(t, report, "next-claim-close")
	if row[1] != resultFailed || !strings.Contains(row[len(row)-1], "not closed") {
		t.Errorf("next-claim-close row = %q, want a fail saying the issue is not closed", row)
	}
}

func TestRun_AnAgentPastTheTimeoutFailsTheTask(t *testing.T) {
	out := filepath.Join(t.TempDir(), "eval.md")

	_, report := runNextClaimClose(t, out, nextClaimCloseScript, "--timeout", "1ns")

	row := reportRow(t, report, "next-claim-close")
	if row[1] != resultFailed || !strings.Contains(row[len(row)-1], "timed out") {
		t.Errorf("next-claim-close row = %q, want a fail saying it timed out", row)
	}
}

func TestRun_ASecondRunWithTheSameOutCountsTheSame(t *testing.T) {
	out := filepath.Join(t.TempDir(), "eval.md")
	_, first := runNextClaimClose(t, out, nextClaimCloseScript)

	_, second := runNextClaimClose(t, out, nextClaimCloseScript)

	firstRow, secondRow := reportRow(t, first, "next-claim-close"), reportRow(t, second, "next-claim-close")
	if !slices.Equal(firstRow, secondRow) {
		t.Errorf("second run row = %q, want the first's %q", secondRow, firstRow)
	}
}

func TestRunCommand_BadUsageExits2(t *testing.T) {
	out := filepath.Join(t.TempDir(), "eval.md")
	tests := []struct {
		name string
		args []string
	}{
		{name: "no agent command", args: []string{"run", "--out", out}},
		{name: "nothing after --", args: []string{"run", "--out", out, "--"}},
		{name: "an argument before --", args: []string{"run", "--out", out, "stray", "--", "agent"}},
		{name: "an unknown --only id", args: []string{"run", "--out", out, "--only", "no-such-task", "--", "agent"}},
		{name: "no --out", args: []string{"run", "--", "agent"}},
		{name: "a zero timeout", args: []string{"run", "--out", out, "--timeout", "0s", "--", "agent"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exitCode, _, errorText := executeCommand(t, test.args...)

			if exitCode != exitBadUsage {
				t.Errorf("exit = %d, want %d: %s", exitCode, exitBadUsage, errorText)
			}
			if _, err := os.Stat(out); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("the report exists after a usage error: %v", err)
			}
		})
	}
}

func TestResolveExecutable_MakesARelativePathAbsolute(t *testing.T) {
	dir := t.TempDir()
	copied := copyTestBinary(t, dir, realCapcomName)
	t.Chdir(dir)

	resolved, err := resolveExecutable("." + string(filepath.Separator) + filepath.Base(copied))

	if err != nil {
		t.Fatalf("resolveExecutable: %v", err)
	}
	resolvedInfo, resolvedErr := os.Stat(resolved)
	copiedInfo, copiedErr := os.Stat(copied)
	if !filepath.IsAbs(resolved) || resolvedErr != nil || copiedErr != nil || !os.SameFile(resolvedInfo, copiedInfo) {
		t.Errorf("resolved = %q, want the absolute path of %s", resolved, copied)
	}
}

func TestRun_AnAgentThatCannotStartFailsTheRun(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "no-such-agent")

	exitCode, _, errorText := executeCommand(t,
		"run", "--out", filepath.Join(dir, "eval.md"), "--capcom", copyTestBinary(t, dir, realCapcomName),
		"--", missing,
	)

	if exitCode != exitRunFailed || !strings.Contains(errorText, "agent command") {
		t.Errorf("exit = %d, error = %q; want %d naming the agent command", exitCode, errorText, exitRunFailed)
	}
}
