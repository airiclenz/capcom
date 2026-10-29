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

// assertVerdict fails t unless verdict passes when wantWhy is empty, or fails with a why holding
// wantWhy otherwise.
func assertVerdict(t *testing.T, verdict outcome, wantWhy string) {
	t.Helper()
	if wantWhy == "" && !verdict.Passed {
		t.Errorf("verdict = fail (%s), want pass", verdict.Why)
	}
	if wantWhy != "" && (verdict.Passed || !strings.Contains(verdict.Why, wantWhy)) {
		t.Errorf("verdict = %+v, want a fail naming %q", verdict, wantWhy)
	}
}

// scenarioCase is one fake agent script a scenario test runs, with the why its verdict must name;
// an empty wantWhy means the verdict must pass.
type scenarioCase struct {
	name    string
	script  [][]string
	wantWhy string
}

// runScenarioCases runs each of cases against the scenario with id as its own subtest.
func runScenarioCases(t *testing.T, id string, cases []scenarioCase) {
	t.Helper()
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assertVerdict(t, runScenario(t, id, test.script), test.wantWhy)
		})
	}
}

func TestFileWithDepScenario_JudgesWhatTheAgentLeft(t *testing.T) {
	runScenarioCases(t, "file-with-dep", []scenarioCase{
		{
			name: "files the issue with its dep and Acceptance",
			script: [][]string{{
				"create", "--id", "rate-limit-login", "--title", "Rate limit the login endpoint",
				"--dep", "auth-tokens", "--why", "Repeated failed logins go unchecked",
				"--acceptance", "A sixth failed login within a minute is answered 429",
			}},
		},
		{name: "files nothing", wantWhy: "0 issues were added"},
		{
			name: "files it without the dep",
			script: [][]string{{
				"create", "--id", "rate-limit-login", "--title", "Rate limit the login endpoint",
				"--acceptance", "A sixth failed login within a minute is answered 429",
			}},
			wantWhy: "does not depend on auth-tokens",
		},
		{
			name: "files it without an Acceptance section",
			script: [][]string{{
				"create", "--id", "rate-limit-login", "--title", "Rate limit the login endpoint",
				"--dep", "auth-tokens", "--body", "Limit the failed logins per client",
			}},
			wantWhy: "no \"## Acceptance\" section",
		},
	})
}

func TestFindLockNoteScenario_JudgesWhatTheAgentLeft(t *testing.T) {
	runScenarioCases(t, "find-lock-note", []scenarioCase{
		{
			name: "records the Note's id",
			script: [][]string{
				{"know", "search", "session"},
				{"log", "onboarding", "Read tree-lock before the first change"},
			},
		},
		{name: "records nothing", wantWhy: "no Log entry of onboarding names tree-lock"},
		{
			name:    "records another Note's id",
			script:  [][]string{{"log", "onboarding", "Read working-practices first"}},
			wantWhy: "names tree-lock",
		},
	})
}

func TestFindLockNoteScenario_AgentsBlockDoesNotNameTheNote(t *testing.T) {
	_, seeded, _ := seedScenario(t, t.TempDir(), "find-lock-note")

	data, err := os.ReadFile(filepath.Join(seeded.repo, "AGENTS.md"))

	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if strings.Contains(string(data), findLockNoteTarget) {
		t.Errorf("AGENTS.md names %s, want it found only by searching:\n%s", findLockNoteTarget, data)
	}
	if !strings.Contains(string(data), "working-practices") {
		t.Errorf("AGENTS.md does not list the seeded top-level Note working-practices:\n%s", data)
	}
}

// resumeLostTokenScript does resume-lost-token the way it is asked: log progress, resume the lock
// by starting the issue again, then release it with the token that start printed.
var resumeLostTokenScript = [][]string{
	{"log", "cache-layer", "Cached the catalogue queries for a minute"},
	{"start", "cache-layer"},
	{"lock", "release", "--token", "{prev:token ([0-9a-f]{32})}"},
}

func TestResumeLostTokenScenario_JudgesWhatTheAgentLeft(t *testing.T) {
	runScenarioCases(t, "resume-lost-token", []scenarioCase{
		{name: "resumes, logs and releases", script: resumeLostTokenScript},
		{name: "does nothing", wantWhy: "the Tree lock is still held"},
		{
			name: "breaks the lock instead",
			script: [][]string{
				{"--json", "lock", "break"},
				{"log", "cache-layer", "Cached the catalogue queries for a minute"},
			},
			wantWhy: "the lock was broken",
		},
		{
			name:    "releases without logging",
			script:  resumeLostTokenScript[1:],
			wantWhy: "cache-layer has no Log entry",
		},
	})
}

func TestIsLockBreak_FindsTheVerbPastFlags(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{"lock", "break"}, want: true},
		{args: []string{"--json", "lock", "break", "--kv"}, want: true},
		{args: []string{"lock", "release", "--token", "break"}, want: false},
		{args: []string{"lock"}, want: false},
		{args: nil, want: false},
	}
	for _, test := range tests {
		if got := isLockBreak(shimCall{Args: test.args}); got != test.want {
			t.Errorf("isLockBreak(%q) = %t, want %t", test.args, got, test.want)
		}
	}
}

func TestFindByContentScenario_JudgesWhatTheAgentLeft(t *testing.T) {
	runScenarioCases(t, "find-by-content", []scenarioCase{
		{
			name: "raises the issue the prompt describes",
			script: [][]string{
				{"search", "flaky upload retries"},
				{"update", "storage-client-jitter", "--priority", "1", "--add-label", "urgent"},
			},
		},
		{name: "changes nothing", wantWhy: "storage-client-jitter has priority 3"},
		{
			name:    "raises it without the label",
			script:  [][]string{{"update", "storage-client-jitter", "--priority", "1"}},
			wantWhy: "not labelled urgent",
		},
		{
			name: "raises another issue too",
			script: [][]string{
				{"update", "storage-client-jitter", "--priority", "1", "--add-label", "urgent"},
				{"update", "avatar-size-limit", "--priority", "1", "--add-label", "urgent"},
			},
			wantWhy: "avatar-size-limit changed priority",
		},
	})
}

func TestHasHeading_MatchesOnlyTheWholeLine(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{
		{body: "intro\n\n## Acceptance\n\n- done\n", want: true},
		{body: "## Acceptance  \r\n", want: true},
		{body: "### Acceptance\n", want: false},
		{body: "See ## Acceptance below\n", want: false},
		{body: "", want: false},
	}
	for _, test := range tests {
		if got := hasHeading(test.body, acceptanceHeading); got != test.want {
			t.Errorf("hasHeading(%q) = %t, want %t", test.body, got, test.want)
		}
	}
}

func TestScenarioChecks_AnUnreadableStoreFails(t *testing.T) {
	tests := []struct {
		name    string
		check   func(repo string, calls []shimCall) outcome
		wantWhy string
	}{
		{name: "file-with-dep", check: checkFileWithDep, wantWhy: "cannot read the issues"},
		{name: "find-lock-note", check: checkFindLockNote, wantWhy: "cannot read the issues"},
		{name: "resume-lost-token", check: checkResumeLostToken, wantWhy: "cannot read the Tree lock"},
		{name: "find-by-content", check: checkFindByContent, wantWhy: "cannot read the issues"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertVerdict(t, test.check(t.TempDir(), nil), test.wantWhy)
		})
	}
}
