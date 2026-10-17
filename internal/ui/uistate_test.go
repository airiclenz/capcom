package ui

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/userconfig"
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

// readWorktreeState returns what the worktree state file at path holds, the zero WorktreeState
// when it is missing.
func readWorktreeState(t *testing.T, path string) userconfig.WorktreeState {
	t.Helper()
	state, err := userconfig.LoadWorktreeState(path)
	if err != nil {
		t.Fatalf("LoadWorktreeState: %v", err)
	}
	return state
}

// captureSaveTicks makes m's debounce ticks schedule nothing and returns the messages they would
// have sent, in the order scheduled, so a test sends them itself.
func captureSaveTicks(m *model) *[]tea.Msg {
	captured := &[]tea.Msg{}
	m.saveTick = func(_ time.Duration, message func(time.Time) tea.Msg) tea.Cmd {
		*captured = append(*captured, message(time.Time{}))
		return nil
	}
	return captured
}

func TestWorktreeState_TabChange_SavesAtOnce(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())
	options := newTestOptions(t, ts)
	m := startModel(t, options, worktreeStateWidth, 30)

	m = press(t, m, runeKey('1'))

	if got := readWorktreeState(t, options.UIStatePath).Tab; got != "issues" {
		t.Errorf("stored tab = %q, want issues", got)
	}
}

func TestWorktreeState_SelectionChange_SavesOnItsTick(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())
	options := newTestOptions(t, ts)
	m := startModel(t, options, worktreeStateWidth, 30)
	ticks := captureSaveTicks(&m)

	m = press(t, m, runeKey('j'))
	beforeTick := readWorktreeState(t, options.UIStatePath).Ready
	press(t, m, (*ticks)[len(*ticks)-1])

	if beforeTick != "" {
		t.Errorf("stored Ready id before the tick = %q, want none", beforeTick)
	}
	if got := readWorktreeState(t, options.UIStatePath).Ready; got != "beta" {
		t.Errorf("stored Ready id = %q, want beta", got)
	}
}

func TestWorktreeState_TwoMovesWithinTheDelay_SaveOnlyTheLastID(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())
	options := newTestOptions(t, ts)
	m := startModel(t, options, worktreeStateWidth, 30)
	ticks := captureSaveTicks(&m)

	m = press(t, m, runeKey('j'))
	m = press(t, m, runeKey('j'))
	m = press(t, m, (*ticks)[0])
	afterStaleTick := readWorktreeState(t, options.UIStatePath).Ready
	press(t, m, (*ticks)[1])

	if afterStaleTick != "" {
		t.Errorf("stored Ready id after the first tick = %q, want none: a later move replaced it", afterStaleTick)
	}
	if got := readWorktreeState(t, options.UIStatePath).Ready; got != "delta" {
		t.Errorf("stored Ready id = %q, want delta", got)
	}
}

func TestWorktreeState_QuitWithAPendingSelection_FlushWritesIt(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())
	options := newTestOptions(t, ts)
	m := startModel(t, options, worktreeStateWidth, 30)
	captureSaveTicks(&m)

	m = press(t, m, runeKey('j'))
	m = press(t, m, runeKey('q'))
	err := m.flushWorktreeState()

	if err != nil {
		t.Fatalf("flushWorktreeState: %v", err)
	}
	if got := readWorktreeState(t, options.UIStatePath).Ready; got != "beta" {
		t.Errorf("stored Ready id = %q, want beta", got)
	}
}

func TestWorktreeState_ReloadBeforeKnowledgesFirstRead_KeepsItsStoredID(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())
	options := newTestOptions(t, ts)
	writeTestFile(t, options.UIStatePath, "ui:\n  selected:\n    knowledge: \"setup\"\n")
	m := newModel(options)
	m.tick = noTick
	m.saveTick = noFlashTick
	m = drive(t, m, loadReady(m.session, m.initialRead(TabReady)))
	m = drive(t, m, loadIssues(m.session, m.initialRead(TabIssues)))
	m = drive(t, m, loadKnowledge(m.session, m.reloadRead(TabKnowledge)))
	m = press(t, m, tea.WindowSizeMsg{Width: worktreeStateWidth, Height: 30})

	press(t, m, runeKey('1'))

	if got := readWorktreeState(t, options.UIStatePath).Knowledge; got != "setup" {
		t.Errorf("stored Knowledge id = %q, want setup: its first read has not applied", got)
	}
}

func TestWorktreeState_UnwritablePath_ReportsOnlyTheFirstFailure(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())
	options := newTestOptions(t, ts)
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	writeTestFile(t, parent, "a regular file\n")
	options.UIStatePath = filepath.Join(parent, "ui-state.yaml")
	m := startModel(t, options, worktreeStateWidth, 30)

	m = press(t, m, runeKey('1'))
	firstStatus := statusRow(m)
	m.worktreeStateNotice = ""
	m = press(t, m, runeKey('3'))

	if !strings.Contains(firstStatus, worktreeSaveNoticePrefix) {
		t.Errorf("status bar = %q, want the notice %q", firstStatus, worktreeSaveNoticePrefix)
	}
	if m.worktreeStateNotice != "" {
		t.Errorf("worktreeStateNotice = %q, want none: only the first failure is reported", m.worktreeStateNotice)
	}
}

func TestWorktreeState_TwoWorktrees_KeepSeparateFiles(t *testing.T) {
	ts := newTestStore(t, worktreeStateFiles())
	linked := addTestWorktree(t, ts)
	mainOptions := newTestOptions(t, ts)
	mainOptions.UIStatePath = ""
	linkedOptions := newTestOptions(t, linked)
	linkedOptions.UIStatePath = ""
	mainModel := startModel(t, mainOptions, worktreeStateWidth, 30)
	linkedModel := startModel(t, linkedOptions, worktreeStateWidth, 30)

	press(t, mainModel, runeKey('1'))
	press(t, linkedModel, runeKey('3'))

	for _, check := range []struct {
		name string
		ts   *testStore
		want string
	}{
		{name: "main", ts: ts, want: "issues"},
		{name: "linked", ts: linked, want: "knowledge"},
	} {
		path := filepath.Join(check.ts.store.GitDir, "capcom", worktreeStateFileName)
		if got := readWorktreeState(t, path).Tab; got != check.want {
			t.Errorf("%s worktree: stored tab in %s = %q, want %s", check.name, path, got, check.want)
		}
	}
}

// addTestWorktree commits ts's repository, adds a linked worktree of it beside its root, inits a
// store there and returns that store with its index, closed when the test ends.
func addTestWorktree(t *testing.T, ts *testStore) *testStore {
	t.Helper()
	worktree := filepath.Join(filepath.Dir(ts.root), filepath.Base(ts.root)+"-linked")
	gitCommands := [][]string{
		{"commit", "-q", "--allow-empty", "-m", "base"},
		{"worktree", "add", "-q", "-b", "linked", worktree},
	}
	for _, arguments := range gitCommands {
		command := exec.Command("git", arguments...)
		command.Dir = ts.root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(worktree); err != nil {
			t.Errorf("remove %s: %v", worktree, err)
		}
	})
	if _, _, err := store.Init(worktree); err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	opened, err := store.Open(worktree)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	issueIndex, err := index.Open(opened, index.ModeRefresh)
	if err != nil {
		t.Fatalf("index.Open: %v", err)
	}
	t.Cleanup(func() {
		if err := issueIndex.Close(); err != nil {
			t.Errorf("close index: %v", err)
		}
	})
	return &testStore{root: worktree, store: opened, index: issueIndex}
}
