package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/store"
)

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
