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
		{name: "context-before-start", check: checkContextBeforeStart, wantWhy: "cannot read the issues"},
		{name: "blocked-start", check: checkBlockedStart, wantWhy: "cannot read the issues"},
		{name: "move-under-parent", check: checkMoveUnderParent, wantWhy: "cannot read the issues"},
		{name: "rename-id", check: checkRenameID, wantWhy: "cannot read the issues"},
		{name: "add-note", check: checkAddNote, wantWhy: "cannot read the Notes"},
		{name: "reopen-with-reason", check: checkReopenWithReason, wantWhy: "cannot read the issues"},
		{name: "close-children", check: checkCloseChildren, wantWhy: "cannot read the issues"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertVerdict(t, test.check(t.TempDir(), nil), test.wantWhy)
		})
	}
}

func TestContextBeforeStartScenario_JudgesWhatTheAgentLeft(t *testing.T) {
	runScenarioCases(t, "context-before-start", []scenarioCase{
		{
			name: "starts it and logs its plan",
			script: [][]string{
				{"context", "export-csv"},
				{"start", "export-csv"},
				{"log", "export-csv", "Working to the plan orders-export-plan"},
			},
		},
		{name: "does nothing", wantWhy: "export-csv is not active"},
		{
			name:    "logs the plan without starting",
			script:  [][]string{{"log", "export-csv", "Working to the plan orders-export-plan"}},
			wantWhy: "export-csv is not active",
		},
		{
			name: "starts it and logs something else",
			script: [][]string{
				{"start", "export-csv"},
				{"log", "export-csv", "Started on the CSV encoder"},
			},
			wantWhy: "no Log entry of export-csv names its plan orders-export-plan",
		},
	})
}

func TestContextBeforeStartScenario_ContextNamesThePlanAndTheNote(t *testing.T) {
	_, seeded, capcomPath := seedScenario(t, t.TempDir(), "context-before-start")

	printed, err := runIn(seeded.repo, capcomPath, "context", "export-csv")

	if err != nil {
		t.Fatalf("capcom context: %v", err)
	}
	for _, want := range []string{"docs/plans/orders-export-plan.md", "csv-conventions"} {
		if !strings.Contains(printed, want) {
			t.Errorf("capcom context does not name %s:\n%s", want, printed)
		}
	}
}

func TestBlockedStartScenario_JudgesWhatTheAgentLeft(t *testing.T) {
	runScenarioCases(t, "blocked-start", []scenarioCase{
		{
			name:   "starts what blocks it",
			script: [][]string{{"start", "deploy"}, {"start", "ci-green"}},
		},
		{name: "does nothing", wantWhy: "ci-green, which blocks deploy, is not active"},
		{
			name:    "stops at the refusal",
			script:  [][]string{{"start", "deploy"}},
			wantWhy: "ci-green, which blocks deploy, is not active",
		},
		{
			name:    "drops the dep and starts deploy",
			script:  [][]string{{"dep", "rm", "deploy", "ci-green"}, {"start", "deploy"}},
			wantWhy: "deploy is active",
		},
	})
}

func TestMoveUnderParentScenario_JudgesWhatTheAgentLeft(t *testing.T) {
	runScenarioCases(t, "move-under-parent", []scenarioCase{
		{
			name:   "moves it below billing",
			script: [][]string{{"mv", "invoice-pdf", "--parent", "billing"}},
		},
		{name: "does nothing", wantWhy: "billing/invoice-pdf.md does not exist"},
		{
			name:    "renames it instead",
			script:  [][]string{{"mv", "invoice-pdf", "billing-invoice-pdf"}},
			wantWhy: "billing/invoice-pdf.md does not exist",
		},
	})
}

func TestMoveUnderParentScenario_AFileLeftAtTheTopFails(t *testing.T) {
	_, seeded, _ := seedScenario(t, t.TempDir(), "move-under-parent")
	issuesDir := filepath.Join(seeded.repo, ".capcom", "issues")
	if err := os.MkdirAll(filepath.Join(issuesDir, "billing"), 0o755); err != nil {
		t.Fatalf("create billing's children directory: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(issuesDir, "invoice-pdf.md"))
	if err != nil {
		t.Fatalf("read invoice-pdf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(issuesDir, "billing", "invoice-pdf.md"), data, 0o600); err != nil {
		t.Fatalf("copy invoice-pdf below billing: %v", err)
	}

	verdict := checkMoveUnderParent(seeded.repo, nil)

	assertVerdict(t, verdict, "invoice-pdf.md still exists")
}

func TestRenameIDScenario_JudgesWhatTheAgentLeft(t *testing.T) {
	runScenarioCases(t, "rename-id", []scenarioCase{
		{
			name:   "renames it",
			script: [][]string{{"mv", "fix-thing", "fix-upload-timeout"}},
		},
		{name: "does nothing", wantWhy: "fix-upload-timeout does not exist"},
		{
			name:    "files a new issue instead",
			script:  [][]string{{"create", "--id", "fix-upload-timeout", "--title", "Fix the upload timeout"}},
			wantWhy: "fix-upload-timeout has no alias fix-thing",
		},
		{
			name: "drops a dependent's dep first",
			script: [][]string{
				{"dep", "rm", "resumable-uploads", "fix-thing"},
				{"mv", "fix-thing", "fix-upload-timeout"},
			},
			wantWhy: "resumable-uploads does not depend on fix-upload-timeout",
		},
	})
}

// wantScenarioCount is how many scenarios a run runs.
const wantScenarioCount = 12

func TestScenarios_HaveTwelveUniqueIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, task := range scenarios() {
		if seen[task.id] {
			t.Errorf("scenario id %q is used twice", task.id)
		}
		seen[task.id] = true
	}

	if len(seen) != wantScenarioCount {
		t.Errorf("%d unique scenario ids, want %d", len(seen), wantScenarioCount)
	}
}

func TestAddNoteScenario_JudgesWhatTheAgentLeft(t *testing.T) {
	runScenarioCases(t, "add-note", []scenarioCase{
		{
			name: "adds a Note below testing",
			script: [][]string{
				{"know", "add", "--id", "test-memory-limit", "--title", "Tests run with a memory limit", "--parent", "testing"},
				{"know", "update", "test-memory-limit", "--body-append", "Run the tests with GOMEMLIMIT=2GiB."},
			},
		},
		{name: "adds nothing", wantWhy: "no Note below testing holds GOMEMLIMIT=2GiB"},
		{
			name: "adds it at the top level",
			script: [][]string{
				{"know", "add", "--id", "test-memory-limit", "--title", "Tests run with a memory limit"},
				{"know", "update", "test-memory-limit", "--body-append", "Run the tests with GOMEMLIMIT=2GiB."},
			},
			wantWhy: "no Note below testing",
		},
		{
			name:    "appends it to testing itself",
			script:  [][]string{{"know", "update", "testing", "--body-append", "Run the tests with GOMEMLIMIT=2GiB."}},
			wantWhy: "no Note below testing",
		},
		{
			name: "adds a Note below testing without the fact",
			script: [][]string{
				{"know", "add", "--id", "test-memory-limit", "--title", "Tests run with a memory limit", "--parent", "testing"},
			},
			wantWhy: "no Note below testing",
		},
	})
}

func TestReopenWithReasonScenario_JudgesWhatTheAgentLeft(t *testing.T) {
	runScenarioCases(t, "reopen-with-reason", []scenarioCase{
		{
			name:   "reopens it with a reason",
			script: [][]string{{"reopen", "login-bug", "--reason", "Closed by mistake: the bug still reproduces"}},
		},
		{name: "does nothing", wantWhy: "login-bug is still closed"},
		{
			name:    "logs why without reopening",
			script:  [][]string{{"log", "login-bug", "The bug still reproduces"}},
			wantWhy: "login-bug is still closed",
		},
	})
}

func TestReopenWithReasonScenario_AnOpenIssueWithoutALogEntryFails(t *testing.T) {
	_, seeded, _ := seedScenario(t, t.TempDir(), "reopen-with-reason")
	path := filepath.Join(seeded.repo, ".capcom", "issues", "login-bug.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read login-bug: %v", err)
	}
	var reopened strings.Builder
	for line := range strings.Lines(string(data)) {
		switch {
		case strings.HasPrefix(line, "status:"):
			reopened.WriteString("status: open\n")
		case strings.HasPrefix(line, "closed:"), strings.HasPrefix(line, "reason:"):
		default:
			reopened.WriteString(line)
		}
	}
	if err := os.WriteFile(path, []byte(reopened.String()), 0o600); err != nil {
		t.Fatalf("write login-bug: %v", err)
	}

	verdict := checkReopenWithReason(seeded.repo, nil)

	assertVerdict(t, verdict, "login-bug has no Log entry")
}

// closeChildrenOpenChildren are the children of v2-migration close-children's seed leaves open.
var closeChildrenOpenChildren = []string{"v2-auth-endpoints", "v2-orders-endpoints", "v2-client-sdk"}

func TestCloseChildrenScenario_JudgesWhatTheAgentLeft(t *testing.T) {
	closeAll := append(append([]string{"close"}, closeChildrenOpenChildren...), "--reason", "Shipped with v2")
	runScenarioCases(t, "close-children", []scenarioCase{
		{name: "closes the open children", script: [][]string{{"list", "--tree"}, closeAll}},
		{name: "does nothing", wantWhy: "v2-auth-endpoints is not closed"},
		{
			name:    "leaves one child open",
			script:  [][]string{{"close", "v2-auth-endpoints", "v2-orders-endpoints", "--reason", "Shipped with v2"}},
			wantWhy: "v2-client-sdk is not closed",
		},
		{
			name: "closes the grouping issue too",
			script: [][]string{
				closeAll,
				{"update", "v2-migration", "--no-standing"},
				{"close", "v2-migration", "--reason", "All done"},
			},
			wantWhy: "v2-migration, the grouping issue, was closed",
		},
		{
			name:    "clears the standing mark",
			script:  [][]string{closeAll, {"update", "v2-migration", "--no-standing"}},
			wantWhy: "v2-migration is no longer standing",
		},
	})
}
