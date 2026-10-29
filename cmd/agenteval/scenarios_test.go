package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// scenarioTestTimeout bounds one fake agent run of a scenario test; the fake agent's script ends
// on its own long before it.
const scenarioTestTimeout = 2 * time.Minute

// nextClaimCloseScript does next-claim-close the way it is asked: find the most urgent issue,
// claim it, log progress, close it with a reason.
var nextClaimCloseScript = [][]string{
	{"next"},
	{"start", "fix-login-timeout"},
	{"log", "fix-login-timeout", "Raised the login timeout to thirty seconds"},
	{"finish", "fix-login-timeout", "--reason", "The login no longer times out"},
}

// writeScript writes script into dir as the fake agent's script file, one JSON argv array per
// line, and returns its path.
func writeScript(t *testing.T, dir string, script [][]string) string {
	t.Helper()
	var lines strings.Builder
	for _, argv := range script {
		line, err := json.Marshal(argv)
		if err != nil {
			t.Fatalf("encode the script line %q: %v", argv, err)
		}
		lines.Write(append(line, '\n'))
	}
	path := filepath.Join(dir, "script.jsonl")
	if err := os.WriteFile(path, []byte(lines.String()), 0o600); err != nil {
		t.Fatalf("write the script: %v", err)
	}
	return path
}

// seedScenario prepares the workspace of the scenario with id, seeded by the test binary copied
// into dir as the real capcom, and removes it when the test ends. It returns the scenario, the
// workspace and the real capcom's path.
func seedScenario(t *testing.T, dir, id string) (scenario, workspace, string) {
	t.Helper()
	setHermeticGitEnv(t)
	task, isFound := findScenario(id)
	if !isFound {
		t.Fatalf("no scenario %q", id)
	}
	capcomPath := copyTestBinary(t, dir, realCapcomName)
	seeded, err := prepareWorkspace(capcomPath, task)
	if err != nil {
		t.Fatalf("prepare the workspace: %v", err)
	}
	t.Cleanup(func() {
		if err := seeded.remove(); err != nil {
			t.Errorf("remove the workspace: %v", err)
		}
	})
	return task, seeded, capcomPath
}

// runScenario seeds the scenario with id, runs the test binary as the fake agent on script in its
// workspace, behind the shim, and returns the scenario's verdict on what the agent left. Each
// scenario's tests drive it through here.
func runScenario(t *testing.T, id string, script [][]string) outcome {
	t.Helper()
	dir := t.TempDir()
	task, seeded, capcomPath := seedScenario(t, dir, id)
	t.Setenv(fakeScriptVariable, writeScript(t, dir, script))
	transcript, err := os.Create(filepath.Join(dir, "transcript.log"))
	if err != nil {
		t.Fatalf("create the transcript: %v", err)
	}
	defer transcript.Close()
	callLog := filepath.Join(dir, "calls.jsonl")
	agentArgv := []string{copyTestBinary(t, dir, fakeAgentName)}

	run, err := seeded.runAgent(
		context.Background(), agentArgv, task.prompt, capcomPath, callLog, transcript, scenarioTestTimeout,
	)

	if err != nil || run.IsTimedOut {
		t.Fatalf("run the fake agent: %v (timed out: %t)", err, run.IsTimedOut)
	}
	return task.check(seeded.repo, mustReadCallLog(t, callLog))
}

func TestNextClaimClose_JudgesWhatTheAgentLeft(t *testing.T) {
	tests := []struct {
		name    string
		script  [][]string
		wantWhy string
	}{
		{name: "closes the most urgent issue", script: nextClaimCloseScript},
		{
			name: "closes it without a Log entry",
			script: [][]string{
				{"start", "fix-login-timeout"},
				{"finish", "fix-login-timeout", "--reason", "Done"},
			},
			wantWhy: "no Log entry",
		},
		{
			name: "closes a less urgent one too",
			script: append(
				append([][]string{}, nextClaimCloseScript...),
				[]string{"close", "update-readme", "--reason", "Done as well"},
			),
			wantWhy: "update-readme",
		},
		{
			name:    "closes a less urgent one instead",
			script:  [][]string{{"close", "tidy-log-output", "--reason", "Done"}},
			wantWhy: "tidy-log-output",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verdict := runScenario(t, "next-claim-close", test.script)

			if test.wantWhy == "" && !verdict.Passed {
				t.Errorf("verdict = fail (%s), want pass", verdict.Why)
			}
			if test.wantWhy != "" && (verdict.Passed || !strings.Contains(verdict.Why, test.wantWhy)) {
				t.Errorf("verdict = %+v, want a fail naming %q", verdict, test.wantWhy)
			}
		})
	}
}

func TestCheckNextClaimClose_AnUnreadableStoreFails(t *testing.T) {
	verdict := checkNextClaimClose(t.TempDir(), nil)

	if verdict.Passed || !strings.Contains(verdict.Why, "cannot read the issues") {
		t.Errorf("verdict = %+v, want a fail saying the issues cannot be read", verdict)
	}
}

func TestLoadNotes_ReadsANoteTheAgentAdded(t *testing.T) {
	_, seeded, capcomPath := seedScenario(t, t.TempDir(), "next-claim-close")
	if _, err := runIn(seeded.repo, capcomPath, "know", "add", "--id", "release-steps", "--title", "Cutting a release"); err != nil {
		t.Fatalf("add the Note: %v", err)
	}

	notes, err := loadNotes(seeded.repo)

	if err != nil {
		t.Fatalf("loadNotes: %v", err)
	}
	if got := notes["release-steps"].Title; got != "Cutting a release" {
		t.Errorf("release-steps title = %q, want %q (notes: %v)", got, "Cutting a release", notes)
	}
}

func TestLockState_ReadsALockTheAgentTook(t *testing.T) {
	_, seeded, capcomPath := seedScenario(t, t.TempDir(), "next-claim-close")
	if _, err := runIn(seeded.repo, capcomPath, "lock", "acquire", "--purpose", "eval"); err != nil {
		t.Fatalf("acquire the lock: %v", err)
	}

	state, err := lockState(seeded.repo)

	if err != nil {
		t.Fatalf("lockState: %v", err)
	}
	if !state.IsHeld || state.Record.Purpose != "eval" {
		t.Errorf("lock state = %+v, want held for eval", state)
	}
}
