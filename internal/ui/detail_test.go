package ui

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

func TestIssueHeader_RendersEachSetKeyInOrder(t *testing.T) {
	tests := []struct {
		name  string
		entry index.Entry
		want  string
	}{
		{
			name: "every key set",
			entry: index.Entry{
				ID:       "child",
				ParentID: "epic",
				Title:    "Child task",
				Status:   "done",
				Type:     "bug",
				Priority: 1,
				Labels:   []string{"ui", "tui"},
				Deps:     []string{"base", "shipped"},
				Related:  []string{"cousin"},
				Plan:     "2026-10-16 - 03 - some-plan",
				Aliases:  []string{"old-child"},
				Created:  time.Date(2026, 10, 11, 9, 0, 59, 0, time.UTC),
				Closed:   time.Date(2026, 10, 12, 11, 30, 0, 0, time.FixedZone("CEST", 2*60*60)),
				Reason:   "shipped\n  in two lines",
			},
			want: "# Child task\n\n" +
				"status: done · P1 · bug\n" +
				"labels: ui, tui\n" +
				"parent: `epic`\n" +
				"deps: `base`, `shipped`\n" +
				"related: `cousin`\n" +
				"plan: 2026-10-16 - 03 - some-plan\n" +
				"aliases: old-child\n" +
				"created: 2026-10-11 09:00\n" +
				"closed: 2026-10-12 09:30\n" +
				"reason: shipped in two lines\n" +
				"\n",
		},
		{
			name:  "standing mark after the plan",
			entry: index.Entry{ID: "area", Title: "Area", Status: "open", Priority: 2, Plan: "area-plan", IsStanding: true},
			want:  "# Area\n\nstatus: open · P2\nplan: area-plan\nstanding: yes\n\n",
		},
		{
			name:  "only title, status and priority",
			entry: index.Entry{ID: "bare", Title: "Bare task", Status: "open", Priority: 2},
			want:  "# Bare task\n\nstatus: open · P2\n\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := issueHeader(test.entry); got != test.want {
				t.Errorf("issueHeader() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDetailPane_IssueWithPlanAndRelated_ShowsBothLines(t *testing.T) {
	files := map[string]string{
		"issues/alpha.md": issueFile(
			"title: Alpha task\nstatus: open\npriority: 1\nrelated: [beta]\nplan: alpha-plan\n",
			"Alpha's body.\n",
		),
		"issues/beta.md": issueFile("title: Beta task\nstatus: open\npriority: 2\n", ""),
	}
	m := newIssuesModel(t, newTestStore(t, files), "alpha", 120, 30)

	text := detailText(m)
	for _, want := range []string{"related: beta", "plan: alpha-plan", "status: open · P1 · task", "created: 2026-10-11 09:00"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail pane = %q, want a line %q", text, want)
		}
	}
}

func TestDetailPane_StandingIssue_ShowsTheMark(t *testing.T) {
	ts := newTestStore(t, standingFiles)

	text := detailText(newIssuesModel(t, ts, "area", 120, 30))
	if !strings.Contains(text, "standing: yes") {
		t.Errorf("detail pane of a standing issue = %q, want a line %q", text, "standing: yes")
	}
	if text := detailText(newIssuesModel(t, ts, "child", 120, 30)); strings.Contains(text, "standing:") {
		t.Errorf("detail pane of an unmarked issue = %q, want no standing line", text)
	}
}

// A link the detail pane wraps at a hyphen ends each of its rows with its underline closed: the
// upstream wrap carried the reset after the hyphen onto the next row, so the row ended with the
// underline open and it ran on past the link into whatever the screen painted after it.
func TestDetailPane_LinkWrappedAtAHyphen_EndsEveryRowWithNoStyleOpen(t *testing.T) {
	files := map[string]string{
		"issues/alpha.md": issueFile(
			"title: Alpha task\nstatus: open\npriority: 1\n",
			"Every issue the tracker files is named by a short id, as in [agents-md-no-block](beta).\n",
		),
		"issues/beta.md": issueFile("title: Beta task\nstatus: open\npriority: 2\n", ""),
	}

	m := newIssuesModel(t, newTestStore(t, files), "alpha", 120, 30)

	if !slices.ContainsFunc(m.detail, func(line string) bool {
		return strings.HasSuffix(strings.TrimSpace(ansi.Strip(line)), "agents-md-no-")
	}) {
		t.Fatalf("detail pane = %q; want the link broken at a hyphen", detailText(m))
	}
	for i, line := range m.detail {
		if isStyleOpenAtEnd(line) {
			t.Errorf("detail row %d %q ends with a style in effect", i, line)
		}
	}
}

// isStyleOpenAtEnd reports whether line, read from a terminal with no style in effect, leaves an
// SGR style in effect after its last byte: its last SGR sequence carries a parameter other than 0.
func isStyleOpenAtEnd(line string) bool {
	matches := sgrSequence.FindAllStringSubmatch(line, -1)
	return len(matches) > 0 && strings.Trim(matches[len(matches)-1][1], "0;") != ""
}

func TestNoteHeader_RendersEachSetKeyInOrder(t *testing.T) {
	tests := []struct {
		name  string
		entry index.NoteEntry
		want  string
	}{
		{
			name: "every key set",
			entry: index.NoteEntry{
				ID:           "old-guide",
				Title:        "Old guide",
				Labels:       []string{"howto", "cli"},
				Refs:         []string{"internal/ui/detail.go", "guide"},
				IsRetired:    true,
				SupersededBy: []string{"guide", "setup"},
				Aliases:      []string{"handbook"},
			},
			want: "# Old guide\n\n" +
				"labels: howto, cli\n" +
				"refs: `internal/ui/detail.go`, `guide`\n" +
				"retired: true\n" +
				"superseded-by: `guide`, `setup`\n" +
				"aliases: handbook\n" +
				"\n",
		},
		{
			name: "not retired, superseded or aliased",
			entry: index.NoteEntry{
				ID:     "guide",
				Title:  "Guide",
				Labels: []string{"howto"},
				Refs:   []string{"internal/ui/detail.go"},
			},
			want: "# Guide\n\n" +
				"labels: howto\n" +
				"refs: `internal/ui/detail.go`\n" +
				"\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := noteHeader(test.entry); got != test.want {
				t.Errorf("noteHeader() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestKnowledgeTab_RetiredNoteDetail_LinksItsSuccessor(t *testing.T) {
	files := map[string]string{
		"knowledge/guide.md": noteFile("title: Guide\n", "The guide.\n"),
		"knowledge/old.md": noteFile(
			"title: Old\nretired: true\nsuperseded-by: [guide]\naliases: [handbook]\n",
			"Retired.\n",
		),
	}
	m := newKnowledgeModel(t, newTestStore(t, files), "old", 120, 30)

	text := detailText(m)
	for _, want := range []string{"retired: true", "superseded-by: guide", "aliases: handbook"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail pane = %q, want a line %q", text, want)
		}
	}
	targets := make([]string, 0, len(m.issue.rendered.Links))
	for _, link := range m.issue.rendered.Links {
		targets = append(targets, link.Target)
	}
	if !slices.Contains(targets, "guide") {
		t.Errorf("detail links = %v, want a link to guide", targets)
	}
}

// pendingFiles is a store of two open issues, alpha above beta in the Issues tree.
var pendingFiles = map[string]string{
	"issues/alpha.md": issueFile("title: Alpha task\nstatus: open\npriority: 1\n", "Alpha first.\n"),
	"issues/beta.md":  issueFile("title: Beta task\nstatus: open\npriority: 2\n", "Beta's body.\n"),
}

// rewriteAlpha writes alpha's file again with body.
func rewriteAlpha(t *testing.T, ts *testStore, body string) {
	t.Helper()
	writeStoreFile(t, ts, "issues/alpha.md", issueFile("title: Alpha task\nstatus: open\npriority: 1\n", body))
}

// reloadIssues hands the model the Issues tab's reload read, as a live refresh does, and returns
// the command it answers with, unrun: the re-read of the shown issue.
func reloadIssues(t *testing.T, m model) (model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(loadIssues(m.session, m.reloadRead(TabIssues))())
	next, isModel := updated.(model)
	if !isModel {
		t.Fatalf("Update returned %T, want model", updated)
	}
	if cmd == nil {
		t.Fatal("the reload returned no command, want the shown issue's re-read")
	}
	return next, cmd
}

// updateHeld hands msg to the model's Update and returns the updated model and its command, unrun.
func updateHeld(t *testing.T, m model, msg tea.Msg) (model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	next, isModel := updated.(model)
	if !isModel {
		t.Fatalf("Update returned %T, want model", updated)
	}
	return next, cmd
}

func TestDetailPane_TabSwitchBeforeTheRead_SaysItIsLoading(t *testing.T) {
	m := newTestModel(t, newTestStore(t, pendingFiles), 120, 30)

	m = send(t, m, runeKey('1'))

	if m.tab != TabIssues {
		t.Fatalf("tab = %v after 1, want the Issues tab", m.tab)
	}
	want := []string{m.theme.mutedRow.Render("loading alpha…")}
	if !slices.Equal(m.detail, want) {
		t.Errorf("detail pane = %q, want the one muted line %q", m.detail, want)
	}
	if frame := ansi.Strip(m.View().Content); !strings.Contains(frame, "loading alpha…") {
		t.Errorf("frame = %q, want it to hold %q", frame, "loading alpha…")
	}
}

func TestDetailPane_ReadDelivered_ReplacesTheLoadingLine(t *testing.T) {
	m := newTestModel(t, newTestStore(t, pendingFiles), 120, 30)

	m, cmd := updateHeld(t, m, runeKey('1'))
	m = drive(t, m, cmd)

	if text := detailText(m); strings.Contains(text, "loading") || !strings.Contains(text, "Alpha first.") {
		t.Errorf("detail pane = %q, want alpha's body and no loading line", text)
	}
}

func TestDetailPane_TwoReReadsOfTheShownIssueInReverseOrder_KeepsTheNewerBody(t *testing.T) {
	ts := newTestStore(t, pendingFiles)
	m := newIssuesModel(t, ts, "alpha", 120, 30)
	rewriteAlpha(t, ts, "Alpha second.\n")
	m, olderRead := reloadIssues(t, m)
	older := olderRead()
	rewriteAlpha(t, ts, "Alpha third.\n")
	m, newerRead := reloadIssues(t, m)

	m = send(t, m, newerRead())
	m = send(t, m, older)

	if text := detailText(m); !strings.Contains(text, "Alpha third.") {
		t.Errorf("detail pane = %q, want the newer read's body %q", text, "Alpha third.")
	}
}

func TestDetailPane_StaleReadOfTheShownIssue_LeavesTheNewerOnePending(t *testing.T) {
	ts := newTestStore(t, pendingFiles)
	m := newIssuesModel(t, ts, "alpha", 120, 30)
	rewriteAlpha(t, ts, "Alpha second.\n")
	m, olderRead := reloadIssues(t, m)
	older := olderRead()
	rewriteAlpha(t, ts, "Alpha third.\n")
	m, newerRead := reloadIssues(t, m)

	m = send(t, m, older)

	if text := detailText(m); !strings.Contains(text, "Alpha first.") {
		t.Errorf("detail pane after the stale read = %q, want the shown body %q kept", text, "Alpha first.")
	}
	m = send(t, m, newerRead())
	if text := detailText(m); !strings.Contains(text, "Alpha third.") {
		t.Errorf("detail pane after the pending read = %q, want %q", text, "Alpha third.")
	}
}

func TestDetailPane_BackOnTheShownIssueBeforeTheNextAnswers_DropsTheNextsRead(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, pendingFiles), "alpha", 120, 30)

	m, betaRead := updateHeld(t, m, runeKey('j'))
	m = press(t, m, runeKey('k'))
	m = send(t, m, betaRead())

	if m.issue.id != "alpha" || m.detailPending.isPending() {
		t.Errorf("shown %q, pending %+v; want alpha shown and nothing pending", m.issue.id, m.detailPending)
	}
	if text := detailText(m); !strings.Contains(text, "Alpha first.") {
		t.Errorf("detail pane after beta's late read = %q, want alpha's body", text)
	}
	m, cmd := updateHeld(t, m, runeKey('j'))
	if cmd == nil {
		t.Fatal("j back onto beta returned no command, want beta read again")
	}
	m = drive(t, m, cmd)
	if text := detailText(m); !strings.Contains(text, "Beta's body.") {
		t.Errorf("detail pane after the second j = %q, want beta's body", text)
	}
}

func TestDetailPane_ReReadHeldOverAKeyThatKeepsTheSelection_ShowsTheEditedBody(t *testing.T) {
	ts := newTestStore(t, pendingFiles)
	m := newIssuesModel(t, ts, "alpha", 120, 30)
	rewriteAlpha(t, ts, "Alpha edited.\n")
	m, reRead := reloadIssues(t, m)

	m = press(t, m, runeKey('k'))
	m = drive(t, m, reRead)

	if text := detailText(m); !strings.Contains(text, "Alpha edited.") {
		t.Errorf("detail pane = %q, want the re-read's body %q", text, "Alpha edited.")
	}
}

func TestDetailPane_ReadNoRequestAskedFor_StillAppliesToTheSelection(t *testing.T) {
	ts := newTestStore(t, pendingFiles)
	m := newIssuesModel(t, ts, "alpha", 120, 30)
	m, _ = reloadIssues(t, m)

	m = send(t, m, detailMsg{id: "alpha", source: "# Alpha as the conflict read it\n"})

	if text := detailText(m); !strings.Contains(text, "Alpha as the conflict read it") {
		t.Errorf("detail pane = %q, want the conflict's read applied", text)
	}
}

// watchClock is the time the model's clock reads in the tests of a detail read that waits.
var watchClock = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

// pendingWatch fixes m's clock at watchClock and returns the detailWatchMsg of its pending read,
// asked for detailWatchDelay before then.
func pendingWatch(t *testing.T, m model) (model, detailWatchMsg) {
	t.Helper()
	if !m.detailPending.isPending() {
		t.Fatal("no detail read pending, want one to watch")
	}
	m.now = func() time.Time { return watchClock }
	return m, detailWatchMsg{seq: m.detailPending.seq, requested: watchClock.Add(-detailWatchDelay)}
}

// holdSession holds s's mutex for purpose from another goroutine and returns once the holder is
// recorded. The function it returns frees the mutex and waits for that goroutine to end.
func holdSession(t *testing.T, s *session, purpose string) func() {
	t.Helper()
	held := make(chan struct{})
	freed := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- s.doFor(purpose, func(*store.Store, *index.Index) error {
			close(held)
			<-freed
			return nil
		})
	}()
	<-held
	return func() {
		close(freed)
		if err := <-result; err != nil {
			t.Errorf("holding the session for %q: %v", purpose, err)
		}
	}
}

// watchedBeta returns the Issues tab showing alpha with beta's read pending and reported in the
// status bar as waiting.
func watchedBeta(t *testing.T) model {
	t.Helper()
	m := newIssuesModel(t, newTestStore(t, pendingFiles), "alpha", 120, 30)
	m, _ = updateHeld(t, m, runeKey('j'))
	m, watch := pendingWatch(t, m)
	m = send(t, m, watch)
	if got := statusRow(m); !strings.Contains(got, "reading beta: waiting 2s") {
		t.Fatalf("status bar = %q, want beta's read reported as waiting", got)
	}
	return m
}

func TestDetailWatch_PendingWhileThePollHoldsTheSession_ReportsTheWaitAndTheHolder(t *testing.T) {
	m := newTestModel(t, newTestStore(t, pendingFiles), 120, 30)
	m = send(t, m, runeKey('1'))
	m, watch := pendingWatch(t, m)
	release := holdSession(t, m.session, purposePoll)
	defer release()

	m = send(t, m, watch)

	want := "reading alpha: waiting 2s on the index poll"
	if got := statusRow(m); !strings.Contains(got, want) {
		t.Errorf("status bar = %q, want it to hold %q", got, want)
	}
}

func TestDetailWatch_PendingWithTheSessionFree_ReportsTheWaitAlone(t *testing.T) {
	m := newTestModel(t, newTestStore(t, pendingFiles), 120, 30)
	m = send(t, m, runeKey('1'))
	m, watch := pendingWatch(t, m)

	m = send(t, m, watch)

	got := statusRow(m)
	if !strings.Contains(got, "reading alpha: waiting 2s") || strings.Contains(got, "waiting 2s on") {
		t.Errorf("status bar = %q, want %q with no holder", got, "reading alpha: waiting 2s")
	}
}

func TestDetailWatch_StillPending_AsksAgainEachSecond(t *testing.T) {
	m := newTestModel(t, newTestStore(t, pendingFiles), 120, 30)
	var delays []time.Duration
	var watches []detailWatchMsg
	m.detailTick = func(delay time.Duration, message func(time.Time) tea.Msg) tea.Cmd {
		delays = append(delays, delay)
		if watch, isWatch := message(watchClock).(detailWatchMsg); isWatch {
			watches = append(watches, watch)
		}
		return nil
	}
	m = send(t, m, runeKey('1'))
	m.now = func() time.Time { return watchClock.Add(detailWatchDelay) }

	m = send(t, m, watches[0])

	if want := []time.Duration{detailWatchDelay, detailWatchInterval}; !slices.Equal(delays, want) {
		t.Errorf("ticks scheduled after %v, want %v", delays, want)
	}
	if len(watches) != 2 || watches[1] != watches[0] {
		t.Errorf("watches = %+v, want the second asking after the same read as the first", watches)
	}
}

func TestDetailWatch_StaleSequence_ChangesNothing(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, pendingFiles), "alpha", 120, 30)
	m, _ = updateHeld(t, m, runeKey('j'))
	m, watch := pendingWatch(t, m)
	watch.seq--

	m, cmd := updateHeld(t, m, watch)

	if got := statusRow(m); strings.Contains(got, "reading") || cmd != nil {
		t.Errorf("status bar = %q, command %v; want no report and no further tick", got, cmd != nil)
	}
}

func TestDetailWatch_ReadDelivered_ClearsTheReport(t *testing.T) {
	m := newTestModel(t, newTestStore(t, pendingFiles), 120, 30)
	m, read := updateHeld(t, m, runeKey('1'))
	m, watch := pendingWatch(t, m)
	m = send(t, m, watch)
	if got := statusRow(m); !strings.Contains(got, "reading alpha") {
		t.Fatalf("status bar = %q, want alpha's read reported before it is delivered", got)
	}

	m = drive(t, m, read)

	if got := statusRow(m); strings.Contains(got, "reading alpha") {
		t.Errorf("status bar = %q, want the report gone once the read applied", got)
	}
}

func TestDetailWatch_PendingReadDroppedOrReplaced_ClearsTheReport(t *testing.T) {
	cases := []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{name: "a tab with an empty list", key: runeKey('4')},
		{name: "a tab whose selection is read anew", key: runeKey('2')},
		{name: "back on the issue already shown", key: runeKey('k')},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := watchedBeta(t)

			m = send(t, m, testCase.key)

			if got := statusRow(m); strings.Contains(got, "reading") {
				t.Errorf("status bar = %q, want the report of beta's read gone", got)
			}
		})
	}
}
