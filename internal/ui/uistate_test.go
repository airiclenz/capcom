package ui

import (
	"maps"
	"strings"
	"testing"
)

// worktreeStateFiles is readyFiles, whose Ready queue is alpha, beta then delta and whose finished
// is done, with the Notes of knowledgeFiles but its alpha, so no Note shares an issue's id.
func worktreeStateFiles() map[string]string {
	files := maps.Clone(readyFiles)
	for name, content := range knowledgeFiles {
		if name != "knowledge/alpha.md" {
			files[name] = content
		}
	}
	return files
}

// worktreeStateWidth fits the whole status bar of the worktree state tests, notices included.
const worktreeStateWidth = 240

// startWithWorktreeState returns a model started over ts with the worktree state file holding
// content, written only when content is not empty, after configure has adjusted the options.
func startWithWorktreeState(t *testing.T, ts *testStore, content string, configure func(*Options)) model {
	t.Helper()
	options := newTestOptions(t, ts)
	if content != "" {
		writeTestFile(t, options.UIStatePath, content)
	}
	configure(&options)
	return startModel(t, options, worktreeStateWidth, 30)
}

// keepOptions leaves the test options as newTestOptions built them.
func keepOptions(*Options) {}

func TestWorktreeState_MissingFile_OpensOnTheOptionsTab(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())

	m := startWithWorktreeState(t, ts, "", func(options *Options) { options.Tab = TabIssues })

	if m.tab != TabIssues {
		t.Errorf("tab = %v, want issues: no file keeps the options' tab", m.tab)
	}
}

func TestWorktreeState_StoredTab_OpensThereOnTheStoredNote(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())
	content := "ui:\n  tab: \"knowledge\"\n  selected:\n    knowledge: \"setup\"\n"

	m := startWithWorktreeState(t, ts, content, keepOptions)

	if m.tab != TabKnowledge {
		t.Errorf("tab = %v, want knowledge", m.tab)
	}
	if got := selectedNoteID(m); got != "setup" {
		t.Errorf("selected Note = %q, want setup", got)
	}
}

func TestWorktreeState_GivenTab_WinsAndSelectsItsStoredID(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())
	content := "ui:\n  tab: \"ready\"\n  selected:\n    issues: \"beta\"\n"

	m := startWithWorktreeState(t, ts, content, func(options *Options) {
		options.Tab = TabIssues
		options.IsTabGiven = true
	})

	if m.tab != TabIssues {
		t.Errorf("tab = %v, want issues: the command line's tab wins", m.tab)
	}
	if got := selectedIssueID(m); got != "beta" {
		t.Errorf("selected issue = %q, want the stored beta", got)
	}
}

func TestWorktreeState_CommandLineID_BeatsTheStoredOne(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())
	content := "ui:\n  selected:\n    issues: \"beta\"\n    knowledge: \"setup\"\n"

	m := startWithWorktreeState(t, ts, content, func(options *Options) {
		options.Tab = TabIssues
		options.IsTabGiven = true
		options.ID = "delta"
	})

	if got := selectedIssueID(m); got != "delta" {
		t.Errorf("selected issue = %q, want delta from the command line", got)
	}
	if got := selectedNoteID(m); got != "setup" {
		t.Errorf("selected Note = %q, want the stored setup on the other tab", got)
	}
}

func TestWorktreeState_StoredIDThatDoesNotApply_SelectsTheFirstRowQuietly(t *testing.T) {
	cases := []struct {
		name string
		tab  Tab
		id   string
	}{
		{name: "unknown on ready", tab: TabReady, id: "no-such-id"},
		{name: "not in the Ready queue", tab: TabReady, id: "gamma"},
		{name: "unknown on issues", tab: TabIssues, id: "no-such-id"},
		{name: "unknown on knowledge", tab: TabKnowledge, id: "no-such-id"},
		{name: "an issue on knowledge", tab: TabKnowledge, id: "beta"},
	}
	ts := newTestStore(t, worktreeStateFiles())
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			onTab := func(options *Options) { options.Tab = testCase.tab }
			baseline := startWithWorktreeState(t, ts, "", onTab)
			content := "ui:\n  selected:\n    " + testCase.tab.String() + ": \"" + testCase.id + "\"\n"

			m := startWithWorktreeState(t, ts, content, onTab)

			want, _ := baseline.sidebarSelection()
			if got, _ := m.sidebarSelection(); got != want {
				t.Errorf("selected = %q, want the first row %q", got, want)
			}
			if got := statusRow(m); strings.Contains(got, testCase.id) {
				t.Errorf("status bar = %q, want no notice naming %s", got, testCase.id)
			}
		})
	}
}

func TestWorktreeState_UnknownTabName_OpensReadyQuietly(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())

	m := startWithWorktreeState(t, ts, "ui:\n  tab: \"bogus\"\n", func(options *Options) { options.Tab = TabIssues })

	if m.tab != TabReady {
		t.Errorf("tab = %v, want ready for a tab name no tab has", m.tab)
	}
	if got := statusRow(m); strings.Contains(got, "bogus") {
		t.Errorf("status bar = %q, want no notice naming bogus", got)
	}
}

func TestWorktreeState_StoredClosedIssue_IsSelectedOnIssues(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())
	content := "ui:\n  tab: \"issues\"\n  selected:\n    issues: \"finished\"\n"

	m := startWithWorktreeState(t, ts, content, keepOptions)

	if got := selectedIssueID(m); got != "finished" {
		t.Errorf("selected issue = %q, want the closed finished", got)
	}
}

func TestWorktreeState_MalformedFile_ShowsItsPathAndRunsOnTheDefaults(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())
	var path string

	m := startWithWorktreeState(t, ts, "ui:\n  tab: [issues]\n", func(options *Options) {
		path = options.UIStatePath
	})

	if got := statusRow(m); !strings.Contains(got, worktreeStateNoticePrefix+path+":2: ") {
		t.Errorf("status bar = %q, want the worktree state notice naming %s", got, path)
	}
	if m.tab != TabReady {
		t.Errorf("tab = %v, want the options' ready", m.tab)
	}
}
