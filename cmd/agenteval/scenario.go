package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/store"
)

// scenario is one seeded capcom task: the prompt the agent reads on stdin, the seed that puts the
// task's starting state into a fresh repo, and the check that judges what the agent left behind.
//
// A prompt names ids and goals, never a capcom command: the agent learns capcom from the AGENTS.md
// block the workspace writes. A check reads only the repo's .capcom files, its lock state and the
// task's call log, never the agent's transcript.
type scenario struct {
	id     string
	prompt string
	// seed adds the task's starting state to a repo that already has a capcom store and its
	// AGENTS.md block; the workspace commits whatever it leaves.
	seed func(env seedEnv) error
	// check judges the repo after the agent ran, given the calls the agent made.
	check func(repo string, calls []shimCall) outcome
}

// seedEnv is what a scenario's seed works with: the repo, and a way to run the capcom build under
// test in it, never through the shim, so seeding is not counted as the agent's calls.
type seedEnv struct {
	repo string
	// capcom runs the build under test with args in repo and returns its stdout; a non-zero exit is
	// an error carrying its stderr.
	capcom func(args ...string) (string, error)
}

// outcome is a check's verdict on one task: whether the agent did it, and why not when it did not.
type outcome struct {
	Passed bool
	Why    string
}

// passed is the verdict of a task the agent did.
func passed() outcome {
	return outcome{Passed: true}
}

// failed is the verdict of a task the agent did not do, with the reason formatted from format and
// args.
func failed(format string, args ...any) outcome {
	return outcome{Why: fmt.Sprintf(format, args...)}
}

// scenarios returns every scenario, in the order a run runs them and the report lists them. Ids are
// stable: a report row, a transcript file and `run --only` all name a scenario by its id.
func scenarios() []scenario {
	return []scenario{
		nextClaimCloseScenario(),
		fileWithDepScenario(),
		findLockNoteScenario(),
		resumeLostTokenScenario(),
		findByContentScenario(),
		contextBeforeStartScenario(),
		blockedStartScenario(),
		moveUnderParentScenario(),
		renameIDScenario(),
		addNoteScenario(),
		reopenWithReasonScenario(),
		closeChildrenScenario(),
	}
}

// findScenario returns the scenario with id, and false when there is none.
func findScenario(id string) (scenario, bool) {
	for _, candidate := range scenarios() {
		if candidate.id == id {
			return candidate, true
		}
	}
	return scenario{}, false
}

// seededIssue is an issue a seed creates: its id, title, body and priority, the ids of the issues
// it depends on, the basename of its plan (empty for none), the id of its Parent (empty for the top
// level), whether it is a standing Parent, and the reason it is closed with (empty to leave it
// open).
type seededIssue struct {
	id          string
	title       string
	body        string
	priority    int
	deps        []string
	plan        string
	parent      string
	isStanding  bool
	closeReason string
}

// createIssues creates each of issues in env's repo, in order, so an issue must come after the
// issues it depends on and its Parent; one with a close reason is closed right after it is created.
func createIssues(env seedEnv, issues []seededIssue) error {
	for _, seeded := range issues {
		args := []string{
			"create", "--id", seeded.id, "--title", seeded.title, "--body", seeded.body,
			"--priority", strconv.Itoa(seeded.priority),
		}
		for _, dep := range seeded.deps {
			args = append(args, "--dep", dep)
		}
		if seeded.plan != "" {
			args = append(args, "--plan", seeded.plan)
		}
		if seeded.parent != "" {
			args = append(args, "--parent", seeded.parent)
		}
		if seeded.isStanding {
			args = append(args, "--standing")
		}
		if _, err := env.capcom(args...); err != nil {
			return fmt.Errorf("create %s: %w", seeded.id, err)
		}
		if seeded.closeReason == "" {
			continue
		}
		if _, err := env.capcom("close", seeded.id, "--reason", seeded.closeReason); err != nil {
			return fmt.Errorf("close %s: %w", seeded.id, err)
		}
	}
	return nil
}

// seededNote is a Note a seed adds: its id, the id of the Note it sits below (empty for the top
// level), its title and its body.
type seededNote struct {
	id     string
	parent string
	title  string
	body   string
}

// createNotes adds each of notes to env's repo, in order, so a parent must come before its
// children. The body is appended after the add, since know add takes a body only from a file.
func createNotes(env seedEnv, notes []seededNote) error {
	for _, seeded := range notes {
		args := []string{"know", "add", "--id", seeded.id, "--title", seeded.title}
		if seeded.parent != "" {
			args = append(args, "--parent", seeded.parent)
		}
		if _, err := env.capcom(args...); err != nil {
			return fmt.Errorf("add the Note %s: %w", seeded.id, err)
		}
		if _, err := env.capcom("know", "update", seeded.id, "--body-append", seeded.body); err != nil {
			return fmt.Errorf("write the body of the Note %s: %w", seeded.id, err)
		}
	}
	return nil
}

// hasHeading reports whether body holds a line that is exactly heading, such as "## Acceptance".
func hasHeading(body, heading string) bool {
	for line := range strings.Lines(body) {
		if strings.TrimRight(line, " \t\r\n") == heading {
			return true
		}
	}
	return false
}

// nextClaimCloseIssues are the three Ready issues of next-claim-close, one each of priority 1, 2
// and 3, listed in an order that is not their priority's, so an agent taking the first one it sees
// picks the wrong one.
var nextClaimCloseIssues = []seededIssue{
	{
		id:       "tidy-log-output",
		title:    "Tidy the log output",
		body:     "The debug log repeats the request id on every line.",
		priority: 3,
	},
	{
		id:       "fix-login-timeout",
		title:    "Fix the login timeout",
		body:     "A login that takes longer than five seconds fails with a blank page.",
		priority: 1,
	},
	{
		id:       "update-readme",
		title:    "Update the README",
		body:     "The README still names the old install command.",
		priority: 2,
	},
}

// nextClaimCloseUrgent is the id of next-claim-close's most urgent issue, the one the agent must
// close.
const nextClaimCloseUrgent = "fix-login-timeout"

// nextClaimCloseScenario asks the agent to work the most urgent of three Ready issues through:
// claim it, note progress, close it with a reason. It passes when that issue alone is closed, with
// a reason and at least one Log entry.
func nextClaimCloseScenario() scenario {
	return scenario{
		id: "next-claim-close",
		prompt: "Work on the most urgent ready issue in this repository: claim it, note your " +
			"progress on it, and close it with a reason. Leave the other issues alone.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, nextClaimCloseIssues)
		},
		check: checkNextClaimClose,
	}
}

// checkNextClaimClose judges next-claim-close from the issue files.
func checkNextClaimClose(repo string, _ []shimCall) outcome {
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	for _, seeded := range nextClaimCloseIssues {
		stored, isFound := issues[seeded.id]
		switch {
		case !isFound:
			return failed("%s is gone", seeded.id)
		case seeded.id != nextClaimCloseUrgent && stored.IsClosed:
			return failed("%s (priority %d) was closed, not only the most urgent", seeded.id, seeded.priority)
		case seeded.id != nextClaimCloseUrgent:
			continue
		case !stored.IsClosed:
			return failed("%s, the most urgent issue, is not closed", seeded.id)
		case stored.Issue.Reason == "":
			return failed("%s was closed without a reason", seeded.id)
		case len(logEntries(stored)) == 0:
			return failed("%s has no Log entry", seeded.id)
		}
	}
	return passed()
}

// fileWithDepBlocker is the id of file-with-dep's seeded issue, the one the new issue must wait
// for.
const fileWithDepBlocker = "auth-tokens"

// acceptanceHeading is the heading of the section that says how to tell an issue is done.
const acceptanceHeading = "## Acceptance"

// fileWithDepIssues is file-with-dep's seed: the one issue the new issue depends on.
var fileWithDepIssues = []seededIssue{
	{
		id:       fileWithDepBlocker,
		title:    "Issue signed auth tokens",
		body:     "Sessions live in server memory; the login endpoint should issue signed tokens instead.",
		priority: 2,
	},
}

// fileWithDepScenario asks the agent to file an issue that waits for a seeded one, with a why and an
// Acceptance section. It passes when exactly one issue was added, depending on auth-tokens, with an
// Acceptance section.
func fileWithDepScenario() scenario {
	return scenario{
		id: "file-with-dep",
		prompt: "File a new issue for rate limiting the login endpoint, so that a client is slowed " +
			"down after repeated failed logins. The work cannot start before the issue auth-tokens " +
			"is done. Say in the issue why it is needed, and give it an Acceptance section that " +
			"says how to tell it is done.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, fileWithDepIssues)
		},
		check: checkFileWithDep,
	}
}

// checkFileWithDep judges file-with-dep from the issue files.
func checkFileWithDep(repo string, _ []shimCall) outcome {
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	if _, isFound := issues[fileWithDepBlocker]; !isFound {
		return failed("%s is gone", fileWithDepBlocker)
	}
	var added []storedIssue
	for id, stored := range issues {
		if id != fileWithDepBlocker {
			added = append(added, stored)
		}
	}
	if len(added) != 1 {
		return failed("%d issues were added, want one", len(added))
	}
	filed := added[0]
	switch {
	case !slices.Contains(filed.Issue.Deps, fileWithDepBlocker):
		return failed("%s does not depend on %s (deps: %v)", filed.ID, fileWithDepBlocker, filed.Issue.Deps)
	case !hasHeading(filed.Issue.Body, acceptanceHeading):
		return failed("%s has no %q section", filed.ID, acceptanceHeading)
	}
	return passed()
}

// The ids find-lock-note works with: the Note that explains the Tree lock, which the agent must
// find, and the issue it records that Note's id on.
const (
	findLockNoteTarget = "tree-lock"
	findLockNoteIssue  = "onboarding"
)

// findLockNoteNotes are find-lock-note's five Notes. The one that explains the Tree lock has a
// title that does not name it, and sits below a neutral top-level Note: the AGENTS.md block lists
// only the top level, so it never names the Note the agent must find.
var findLockNoteNotes = []seededNote{
	{
		id:    "working-practices",
		title: "How we work in this repository",
		body:  "The habits this repository keeps; the Notes below this one go into detail.",
	},
	{
		id:     findLockNoteTarget,
		parent: "working-practices",
		title:  "Before a change that spans several commits",
		body: "Claim the worktree with a Tree lock first, so that no other session edits the same " +
			"files while the change is under way: acquire it with a purpose, keep the Lock token it " +
			"prints, and release it with that token when the work is done.",
	},
	{
		id:    "release-checklist",
		title: "Cutting a release",
		body:  "Releases are cut from main by the release tool; never tag by hand.",
	},
	{
		id:    "test-commands",
		title: "Running the tests",
		body:  "Run one package at a time with a memory limit; the full suite runs in CI.",
	},
	{
		id:    "error-wording",
		title: "How error messages read",
		body:  "An error names what failed and what to do next, in one sentence without a stack trace.",
	},
}

// findLockNoteIssues is find-lock-note's one issue, the one the agent records the Note's id on.
var findLockNoteIssues = []seededIssue{
	{
		id:       findLockNoteIssue,
		title:    "Onboard a new contributor",
		body:     "Collect the Notes a new contributor reads before their first change.",
		priority: 2,
	},
}

// findLockNoteScenario asks the agent to find the Note that explains the Tree lock by what it says
// and record its id on an issue. It passes when a Log entry of onboarding names tree-lock.
func findLockNoteScenario() scenario {
	return scenario{
		id: "find-lock-note",
		prompt: "Find the knowledge Note in this repository that explains how to keep other sessions " +
			"from editing this working copy while a change is under way, and record that Note's id " +
			"in a Log entry on the issue onboarding.\n",
		seed: func(env seedEnv) error {
			if err := createNotes(env, findLockNoteNotes); err != nil {
				return err
			}
			return createIssues(env, findLockNoteIssues)
		},
		check: checkFindLockNote,
	}
}

// checkFindLockNote judges find-lock-note from onboarding's Log.
func checkFindLockNote(repo string, _ []shimCall) outcome {
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	stored, isFound := issues[findLockNoteIssue]
	if !isFound {
		return failed("%s is gone", findLockNoteIssue)
	}
	for _, entry := range logEntries(stored) {
		if strings.Contains(entry, findLockNoteTarget) {
			return passed()
		}
	}
	return failed("no Log entry of %s names %s", findLockNoteIssue, findLockNoteTarget)
}

// resumeLostTokenIssue is the id of the issue resume-lost-token's seed claims, Tree lock and all.
const resumeLostTokenIssue = "cache-layer"

// resumeLostTokenIssues is resume-lost-token's one issue. It has no Log entry, so any entry the
// check finds is the agent's.
var resumeLostTokenIssues = []seededIssue{
	{
		id:       resumeLostTokenIssue,
		title:    "Put a cache in front of the catalogue queries",
		body:     "Every page view runs the same three catalogue queries against the database.",
		priority: 2,
	},
}

// resumeLostTokenScenario seeds a claimed issue whose Lock token the agent is never given, and asks
// it to resume the work, log progress and release the lock. It passes when the lock is free,
// cache-layer has a Log entry, and the agent never broke the lock to get there.
func resumeLostTokenScenario() scenario {
	return scenario{
		id: "resume-lost-token",
		prompt: "You were working on the issue cache-layer when your session restarted, and the Lock " +
			"token you held is lost. Pick the work up again where you left it, note your progress on " +
			"cache-layer, and release the lock when you are done.\n",
		seed: func(env seedEnv) error {
			if err := createIssues(env, resumeLostTokenIssues); err != nil {
				return err
			}
			if _, err := env.capcom("start", resumeLostTokenIssue); err != nil {
				return fmt.Errorf("start %s: %w", resumeLostTokenIssue, err)
			}
			return nil
		},
		check: checkResumeLostToken,
	}
}

// checkResumeLostToken judges resume-lost-token from the lock state, cache-layer's Log and the
// call log.
func checkResumeLostToken(repo string, calls []shimCall) outcome {
	for _, call := range calls {
		if isLockBreak(call) {
			return failed("the lock was broken (capcom %s), not resumed", strings.Join(call.Args, " "))
		}
	}
	state, err := lockState(repo)
	if err != nil {
		return failed("cannot read the Tree lock: %v", err)
	}
	if state.IsHeld {
		return failed("the Tree lock is still held (%s)", state.Record.Purpose)
	}
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	stored, isFound := issues[resumeLostTokenIssue]
	switch {
	case !isFound:
		return failed("%s is gone", resumeLostTokenIssue)
	case len(logEntries(stored)) == 0:
		return failed("%s has no Log entry", resumeLostTokenIssue)
	}
	return passed()
}

// isLockBreak reports whether call ran `capcom lock break`: its first two arguments that are not
// flags are lock and break, so a global flag before them does not hide it.
func isLockBreak(call shimCall) bool {
	var words []string
	for _, arg := range call.Args {
		if !strings.HasPrefix(arg, "-") {
			words = append(words, arg)
		}
	}
	return len(words) >= 2 && words[0] == "lock" && words[1] == "break"
}

// The change find-by-content asks for on the issue it names by content.
const (
	findByContentPriority = 1
	findByContentLabel    = "urgent"
)

// findByContentTarget is the id of the issue find-by-content's prompt describes; neither it nor
// the title holds the prompt's words, only the body does.
const findByContentTarget = "storage-client-jitter"

// findByContentIssues are find-by-content's eight issues, none of priority 1 or labelled. One
// other issue is about uploads too, so a search for "upload" alone does not settle it.
var findByContentIssues = []seededIssue{
	{
		id:       "dark-mode-toggle",
		title:    "Add a dark mode toggle",
		body:     "The settings page has no way to switch the theme.",
		priority: 3,
	},
	{
		id:       "avatar-size-limit",
		title:    "Raise the avatar size limit",
		body:     "An avatar upload over one megabyte is refused; allow five.",
		priority: 2,
	},
	{
		id:       "csv-export-encoding",
		title:    "Fix the CSV export encoding",
		body:     "Exported names with accents come out garbled in spreadsheet tools.",
		priority: 2,
	},
	{
		id:    findByContentTarget,
		title: "Jitter in the storage client",
		body: "The upload retries are flaky: when the bucket answers 503 the client retries at once, " +
			"three times in a row, and the upload fails.",
		priority: 3,
	},
	{
		id:       "session-expiry-banner",
		title:    "Warn before a session expires",
		body:     "Users lose a half-filled form when their session ends without notice.",
		priority: 4,
	},
	{
		id:       "search-index-rebuild",
		title:    "Rebuild the search index nightly",
		body:     "Renamed products keep their old names in search results for days.",
		priority: 3,
	},
	{
		id:       "password-reset-mail",
		title:    "Shorten the password reset mail",
		body:     "The reset link sits below three paragraphs of legal text.",
		priority: 2,
	},
	{
		id:       "api-docs-examples",
		title:    "Add examples to the API docs",
		body:     "No endpoint in the reference shows a request and its response.",
		priority: 4,
	},
}

// findByContentScenario names one of eight issues only by what its body says and asks the agent to
// make it priority 1 and label it urgent. It passes when that issue alone has both and every other
// issue keeps its priority, has no label and stays open.
func findByContentScenario() scenario {
	return scenario{
		id: "find-by-content",
		prompt: "One of the open issues in this repository is about flaky upload retries. Make it " +
			"priority 1 and give it the label urgent. Leave every other issue as it is.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, findByContentIssues)
		},
		check: checkFindByContent,
	}
}

// checkFindByContent judges find-by-content from the issue files.
func checkFindByContent(repo string, _ []shimCall) outcome {
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	for _, seeded := range findByContentIssues {
		stored, isFound := issues[seeded.id]
		isTarget := seeded.id == findByContentTarget
		switch {
		case !isFound:
			return failed("%s is gone", seeded.id)
		case isTarget && stored.Issue.Priority != findByContentPriority:
			return failed("%s has priority %d, want %d", seeded.id, stored.Issue.Priority, findByContentPriority)
		case isTarget && !slices.Contains(stored.Issue.Labels, findByContentLabel):
			return failed("%s is not labelled %s (labels: %v)", seeded.id, findByContentLabel, stored.Issue.Labels)
		case isTarget:
			continue
		case stored.Issue.Priority != seeded.priority:
			return failed("%s changed priority from %d to %d", seeded.id, seeded.priority, stored.Issue.Priority)
		case len(stored.Issue.Labels) != 0:
			return failed("%s was labelled %v", seeded.id, stored.Issue.Labels)
		case stored.IsClosed:
			return failed("%s was closed", seeded.id)
		}
	}
	return passed()
}

// isActive reports whether stored is being worked on: its status falls in the doing category of
// cfg.
func isActive(cfg config.Config, stored storedIssue) bool {
	category, _ := cfg.CategoryOf(stored.Issue.Status)
	return category == config.CategoryDoing
}

// loadConfigAndIssues returns the config of repo's store and its issues, as loadIssues reads them.
func loadConfigAndIssues(repo string) (config.Config, map[string]storedIssue, error) {
	opened, err := store.Open(repo)
	if err != nil {
		return config.Config{}, nil, err
	}
	issues, err := loadIssues(repo)
	if err != nil {
		return config.Config{}, nil, err
	}
	return opened.Config, issues, nil
}

// The ids and the plan context-before-start works with: the issue the agent starts, and the
// basename of the plan it is linked to, which only the issue's plan field names.
const (
	contextBeforeStartIssue = "export-csv"
	contextBeforeStartPlan  = "orders-export-plan"
)

// plansDir is the plans directory of a seeded repo, relative to its root: the first of the plans
// directories config.yaml names by default.
const plansDir = "docs/plans"

// contextBeforeStartPlanText is the plan file context-before-start seeds under plansDir.
const contextBeforeStartPlanText = "# Plan: export the orders report as CSV\n\n" +
	"## 1. Write the CSV encoder\n\nOne row per order, a header row first.\n"

// contextBeforeStartIssues is context-before-start's one issue, linked to the seeded plan.
var contextBeforeStartIssues = []seededIssue{
	{
		id:       contextBeforeStartIssue,
		title:    "Export the orders report as CSV",
		body:     "Finance copies the orders report into a spreadsheet by hand every month.",
		priority: 2,
		plan:     contextBeforeStartPlan,
	},
}

// contextBeforeStartNotes is context-before-start's one Note, which applies to export-csv: a
// search on the issue's title finds it.
var contextBeforeStartNotes = []seededNote{
	{
		id:    "csv-conventions",
		title: "How a CSV export is written",
		body:  "A CSV export is UTF-8 with a byte order mark, comma-separated, with a header row first.",
	},
}

// contextBeforeStartScenario seeds an issue linked to a plan, with a Note that applies, and asks the
// agent to start it and log the name of its plan. It passes when export-csv is active and a Log
// entry of it holds the plan's basename.
func contextBeforeStartScenario() scenario {
	return scenario{
		id: "context-before-start",
		prompt: "Start work on the issue export-csv. Before you change anything, find out what the " +
			"issue needs, and record the name of the plan it belongs to in a Log entry on it.\n",
		seed: func(env seedEnv) error {
			planPath := filepath.Join(env.repo, filepath.FromSlash(plansDir), contextBeforeStartPlan+markdownExtension)
			if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
				return fmt.Errorf("create the plans directory: %w", err)
			}
			if err := os.WriteFile(planPath, []byte(contextBeforeStartPlanText), 0o644); err != nil {
				return fmt.Errorf("write the plan: %w", err)
			}
			if err := createNotes(env, contextBeforeStartNotes); err != nil {
				return err
			}
			return createIssues(env, contextBeforeStartIssues)
		},
		check: checkContextBeforeStart,
	}
}

// checkContextBeforeStart judges context-before-start from export-csv's status and Log.
func checkContextBeforeStart(repo string, _ []shimCall) outcome {
	cfg, issues, err := loadConfigAndIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	stored, isFound := issues[contextBeforeStartIssue]
	switch {
	case !isFound:
		return failed("%s is gone", contextBeforeStartIssue)
	case !isActive(cfg, stored):
		return failed("%s is not active (status %q)", contextBeforeStartIssue, stored.Issue.Status)
	}
	for _, entry := range logEntries(stored) {
		if strings.Contains(entry, contextBeforeStartPlan) {
			return passed()
		}
	}
	return failed("no Log entry of %s names its plan %s", contextBeforeStartIssue, contextBeforeStartPlan)
}

// The ids blocked-start works with: the issue the prompt asks to start, and the open issue it
// waits for, which the agent must start instead.
const (
	blockedStartIssue   = "deploy"
	blockedStartBlocker = "ci-green"
)

// blockedStartIssues are blocked-start's two issues: deploy waits for the open ci-green.
var blockedStartIssues = []seededIssue{
	{
		id:       blockedStartBlocker,
		title:    "Make the CI pipeline green",
		body:     "Two integration tests fail on every run since the database upgrade.",
		priority: 2,
	},
	{
		id:       blockedStartIssue,
		title:    "Deploy the new release",
		body:     "Roll the new release out to production.",
		priority: 1,
		deps:     []string{blockedStartBlocker},
	},
}

// blockedStartScenario asks the agent to start an issue that waits for an open one, and to start
// what blocks it when it cannot. It passes when ci-green is active and deploy is not.
func blockedStartScenario() scenario {
	return scenario{
		id: "blocked-start",
		prompt: "Start work on the issue deploy. If it cannot be started yet, start the work that " +
			"blocks it instead.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, blockedStartIssues)
		},
		check: checkBlockedStart,
	}
}

// checkBlockedStart judges blocked-start from the two issues' statuses.
func checkBlockedStart(repo string, _ []shimCall) outcome {
	cfg, issues, err := loadConfigAndIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	blocked, isBlockedFound := issues[blockedStartIssue]
	blocker, isBlockerFound := issues[blockedStartBlocker]
	switch {
	case !isBlockedFound:
		return failed("%s is gone", blockedStartIssue)
	case !isBlockerFound:
		return failed("%s is gone", blockedStartBlocker)
	case isActive(cfg, blocked):
		return failed("%s is active, though it waits for %s", blockedStartIssue, blockedStartBlocker)
	case !isActive(cfg, blocker):
		return failed(
			"%s, which blocks %s, is not active (status %q)",
			blockedStartBlocker, blockedStartIssue, blocker.Issue.Status,
		)
	}
	return passed()
}

// The ids move-under-parent works with: the Parent and the top-level issue the agent moves below
// it.
const (
	moveUnderParentParent = "billing"
	moveUnderParentChild  = "invoice-pdf"
)

// moveUnderParentIssues are move-under-parent's two top-level issues.
var moveUnderParentIssues = []seededIssue{
	{
		id:       moveUnderParentParent,
		title:    "Billing",
		body:     "Everything about charging customers and the documents they receive.",
		priority: 2,
	},
	{
		id:       moveUnderParentChild,
		title:    "Attach the invoice as a PDF",
		body:     "The invoice mail links to a web page; customers want the PDF attached.",
		priority: 3,
	},
}

// moveUnderParentScenario asks the agent to move a top-level issue below another. It passes when
// invoice-pdf's file sits in billing's children directory and no longer at the top level.
func moveUnderParentScenario() scenario {
	return scenario{
		id: "move-under-parent",
		prompt: "The issue invoice-pdf belongs under the issue billing. Put it there, keeping its " +
			"id.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, moveUnderParentIssues)
		},
		check: checkMoveUnderParent,
	}
}

// checkMoveUnderParent judges move-under-parent from where invoice-pdf's file is.
func checkMoveUnderParent(repo string, _ []shimCall) outcome {
	opened, err := store.Open(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	fileName := moveUnderParentChild + markdownExtension
	movedPath := filepath.Join(opened.IssuesDir(), moveUnderParentParent, fileName)
	oldPath := filepath.Join(opened.IssuesDir(), fileName)
	isMoved, err := fileExists(movedPath)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	isLeft, err := fileExists(oldPath)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	switch {
	case !isMoved:
		return failed("%s does not exist", opened.Relative(movedPath))
	case isLeft:
		return failed("%s still exists", opened.Relative(oldPath))
	}
	return passed()
}

// fileExists reports whether path names a file or directory; an error other than its absence is
// returned.
func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// The ids rename-id works with: the issue's id before and after the rename.
const (
	renameIDOld = "fix-thing"
	renameIDNew = "fix-upload-timeout"
)

// renameIDIssues are rename-id's seed: the issue to rename, then the two issues that depend on
// it.
var renameIDIssues = []seededIssue{
	{
		id:       renameIDOld,
		title:    "Fix the upload timeout",
		body:     "An upload over a hundred megabytes times out after thirty seconds.",
		priority: 2,
	},
	{
		id:       "upload-progress-bar",
		title:    "Show a progress bar for uploads",
		body:     "A long upload shows nothing until it is done.",
		priority: 3,
		deps:     []string{renameIDOld},
	},
	{
		id:       "resumable-uploads",
		title:    "Resume an interrupted upload",
		body:     "A dropped connection restarts the upload from the first byte.",
		priority: 3,
		deps:     []string{renameIDOld},
	},
}

// renameIDScenario asks the agent to rename an issue two others depend on, keeping references to
// it working. It passes when fix-upload-timeout exists with the alias fix-thing and both
// dependents' deps name fix-upload-timeout.
func renameIDScenario() scenario {
	return scenario{
		id: "rename-id",
		prompt: "The id of the issue fix-thing says nothing. Rename it to fix-upload-timeout, so that " +
			"everything that refers to it keeps working.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, renameIDIssues)
		},
		check: checkRenameID,
	}
}

// checkRenameID judges rename-id from the renamed issue's file and its dependents' deps.
func checkRenameID(repo string, _ []shimCall) outcome {
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	renamed, isFound := issues[renameIDNew]
	switch {
	case !isFound:
		return failed("%s does not exist", renameIDNew)
	case !slices.Contains(renamed.Issue.Aliases, renameIDOld):
		return failed("%s has no alias %s (aliases: %v)", renameIDNew, renameIDOld, renamed.Issue.Aliases)
	}
	for _, seeded := range renameIDIssues[1:] {
		dependent, isFound := issues[seeded.id]
		switch {
		case !isFound:
			return failed("%s is gone", seeded.id)
		case !slices.Contains(dependent.Issue.Deps, renameIDNew):
			return failed("%s does not depend on %s (deps: %v)", seeded.id, renameIDNew, dependent.Issue.Deps)
		}
	}
	return passed()
}

// The ids and the fact add-note works with: the Note the new one goes below, and the text its body
// must hold.
const (
	addNoteParent = "testing"
	addNoteFact   = "GOMEMLIMIT=2GiB"
)

// addNoteNotes is add-note's seed: the one Note the agent adds below.
var addNoteNotes = []seededNote{
	{
		id:    addNoteParent,
		title: "How the tests are run",
		body:  "How the tests in this repository are run; the Notes below this one go into detail.",
	},
}

// addNoteScenario asks the agent to record a fact as a new Note below a seeded one. It passes when
// a Note below testing has a body that holds GOMEMLIMIT=2GiB.
func addNoteScenario() scenario {
	return scenario{
		id: "add-note",
		prompt: "Record in this repository's knowledge, as a new Note below the Note testing, that " +
			"the tests here run with GOMEMLIMIT=2GiB.\n",
		seed: func(env seedEnv) error {
			return createNotes(env, addNoteNotes)
		},
		check: checkAddNote,
	}
}

// checkAddNote judges add-note from the Note files below testing's children directory.
func checkAddNote(repo string, _ []shimCall) outcome {
	opened, err := store.Open(repo)
	if err != nil {
		return failed("cannot read the Notes: %v", err)
	}
	isRecorded := false
	err = walkMarkdown(filepath.Join(opened.KnowledgeDir(), addNoteParent), func(_, path string, data []byte) error {
		note, err := knowledge.Parse(path, data)
		if err != nil {
			return err
		}
		isRecorded = isRecorded || strings.Contains(note.Body, addNoteFact)
		return nil
	})
	switch {
	case err != nil:
		return failed("cannot read the Notes: %v", err)
	case !isRecorded:
		return failed("no Note below %s holds %s in its body", addNoteParent, addNoteFact)
	}
	return passed()
}

// reopenWithReasonIssue is the id of the issue reopen-with-reason's seed closes by mistake.
const reopenWithReasonIssue = "login-bug"

// reopenWithReasonIssues is reopen-with-reason's one issue, closed with the reason "fixed". Closing
// writes no Log entry, so any entry the check finds is the agent's.
var reopenWithReasonIssues = []seededIssue{
	{
		id:          reopenWithReasonIssue,
		title:       "Login fails after a password change",
		body:        "A user who changed their password is sent back to the login page on every attempt.",
		priority:    2,
		closeReason: "fixed",
	},
}

// reopenWithReasonScenario seeds an issue closed by mistake and asks the agent to reopen it, saying
// why. It passes when login-bug is open and has a Log entry.
func reopenWithReasonScenario() scenario {
	return scenario{
		id: "reopen-with-reason",
		prompt: "The issue login-bug was closed by mistake: the bug still reproduces. Reopen it, " +
			"saying why.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, reopenWithReasonIssues)
		},
		check: checkReopenWithReason,
	}
}

// checkReopenWithReason judges reopen-with-reason from login-bug's status and Log.
func checkReopenWithReason(repo string, _ []shimCall) outcome {
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	stored, isFound := issues[reopenWithReasonIssue]
	switch {
	case !isFound:
		return failed("%s is gone", reopenWithReasonIssue)
	case stored.IsClosed:
		return failed("%s is still closed (status %q)", reopenWithReasonIssue, stored.Issue.Status)
	case len(logEntries(stored)) == 0:
		return failed("%s has no Log entry", reopenWithReasonIssue)
	}
	return passed()
}

// closeChildrenParent is the id of close-children's standing Parent, the grouping issue the agent
// must leave open.
const closeChildrenParent = "v2-migration"

// closeChildrenIssues are close-children's seed: the standing Parent, then its four children, the
// last of them already closed.
var closeChildrenIssues = []seededIssue{
	{
		id:         closeChildrenParent,
		title:      "Migrate to the v2 API",
		body:       "Groups the work of moving every client of the v1 API to v2.",
		priority:   2,
		isStanding: true,
	},
	{
		id:       "v2-auth-endpoints",
		title:    "Move the auth endpoints to v2",
		body:     "Login and token refresh still call the v1 routes.",
		priority: 2,
		parent:   closeChildrenParent,
	},
	{
		id:       "v2-orders-endpoints",
		title:    "Move the orders endpoints to v2",
		body:     "The order list and order detail pages still call the v1 routes.",
		priority: 2,
		parent:   closeChildrenParent,
	},
	{
		id:       "v2-client-sdk",
		title:    "Release the client SDK for v2",
		body:     "The published SDK only speaks v1.",
		priority: 3,
		parent:   closeChildrenParent,
	},
	{
		id:          "v2-docs",
		title:       "Document the v2 API",
		body:        "The reference pages describe only the v1 routes.",
		priority:    3,
		parent:      closeChildrenParent,
		closeReason: "Published with the v2 beta",
	},
}

// closeChildrenScenario asks the agent to close the remaining children of a standing Parent and
// leave the Parent itself alone. It passes when every child is closed with a reason and
// v2-migration is open and still standing.
func closeChildrenScenario() scenario {
	return scenario{
		id: "close-children",
		prompt: "The remaining work under the issue v2-migration is done. Close all of it, but not " +
			"v2-migration itself: it only groups the work and stays open.\n",
		seed: func(env seedEnv) error {
			return createIssues(env, closeChildrenIssues)
		},
		check: checkCloseChildren,
	}
}

// checkCloseChildren judges close-children from the Parent's and its children's issue files.
func checkCloseChildren(repo string, _ []shimCall) outcome {
	issues, err := loadIssues(repo)
	if err != nil {
		return failed("cannot read the issues: %v", err)
	}
	parent, isFound := issues[closeChildrenParent]
	switch {
	case !isFound:
		return failed("%s is gone", closeChildrenParent)
	case parent.IsClosed:
		return failed("%s, the grouping issue, was closed", closeChildrenParent)
	case !parent.Issue.IsStanding:
		return failed("%s is no longer standing", closeChildrenParent)
	}
	for _, seeded := range closeChildrenIssues[1:] {
		child, isFound := issues[seeded.id]
		switch {
		case !isFound:
			return failed("%s is gone", seeded.id)
		case !child.IsClosed:
			return failed("%s is not closed", seeded.id)
		case child.Issue.Reason == "":
			return failed("%s was closed without a reason", seeded.id)
		}
	}
	return passed()
}
