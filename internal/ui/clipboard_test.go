package ui

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// copyRecorder records what a copy hands each route: the text OSC 52, the native clipboard and
// tmux were given, and the clears of the copy notice it scheduled, each with its delay. nativeErr
// and tmuxErr are what the native and tmux writes return.
type copyRecorder struct {
	osc52     []string
	native    []string
	tmux      []string
	delays    []time.Duration
	clears    []tea.Msg
	nativeErr error
	tmuxErr   error
}

// recordCopies returns m with every copy route and the notice's clear recorded by recorder: no
// route writes anything and no clear is scheduled.
func recordCopies(m model, recorder *copyRecorder) model {
	m.setClipboard = func(text string) tea.Cmd {
		recorder.osc52 = append(recorder.osc52, text)
		return nil
	}
	m.writeNative = func(text string) error {
		recorder.native = append(recorder.native, text)
		return recorder.nativeErr
	}
	m.writeTmux = func(text string) error {
		recorder.tmux = append(recorder.tmux, text)
		return recorder.tmuxErr
	}
	m.flashTick = func(delay time.Duration, clear func(time.Time) tea.Msg) tea.Cmd {
		recorder.delays = append(recorder.delays, delay)
		recorder.clears = append(recorder.clears, clear(time.Time{}))
		return nil
	}
	return m
}

// assertCopied fails the test unless every route was handed exactly want, once each.
func assertCopied(t *testing.T, recorder *copyRecorder, want string) {
	t.Helper()
	for name, got := range map[string][]string{
		"OSC 52": recorder.osc52,
		"native": recorder.native,
		"tmux":   recorder.tmux,
	} {
		if !slices.Equal(got, []string{want}) {
			t.Errorf("%s copies = %q, want [%q]", name, got, want)
		}
	}
}

// wideFiles is an issue whose body holds double-width runes.
var wideFiles = map[string]string{
	"issues/alpha.md": issueFile("title: Alpha task\nstatus: open\npriority: 1\n", "Wide 漢字かな text.\n"),
}

func TestClipboard_TwoLineDrag_CopiesOverEveryRouteAndShowsTheNotice(t *testing.T) {
	recorder := &copyRecorder{}
	m := recordCopies(newSelectModel(t, newTestStore(t, selectFiles)), recorder)

	m = dragSelect(t, m, "first point", 0, "second point", 3)

	want := "first point\n• sec"
	assertCopied(t, recorder, want)
	if got := statusRow(m); !strings.Contains(got, "copied 17 chars") {
		t.Errorf("status bar = %q, want it to show copied 17 chars", got)
	}
	if !m.sel.isActive || len(shadedLines(t, m)) == 0 {
		t.Error("the copied selection is gone, want its highlight kept")
	}
	if !slices.Equal(recorder.delays, []time.Duration{flashDuration}) {
		t.Errorf("notice clears scheduled after %v, want once after %v", recorder.delays, flashDuration)
	}
}

func TestClipboard_OneCharDrag_ShowsTheSingularNotice(t *testing.T) {
	recorder := &copyRecorder{}
	m := recordCopies(newSelectModel(t, newTestStore(t, selectFiles)), recorder)

	m = dragSelect(t, m, "first point", 0, "first point", 1)

	assertCopied(t, recorder, "f")
	if got := statusRow(m); !strings.Contains(got, "copied 1 char ") {
		t.Errorf("status bar = %q, want it to show copied 1 char", got)
	}
}

func TestClipboard_DragOverBlankCells_CopiesNothingAndClears(t *testing.T) {
	recorder := &copyRecorder{}
	m := recordCopies(newSelectModel(t, newTestStore(t, selectFiles)), recorder)
	blank := slices.IndexFunc(m.detail, func(line string) bool { return strings.TrimSpace(line) == "" })
	if blank < 0 {
		t.Fatalf("no blank detail line in %q", detailText(m))
	}
	y := bodyTop + blank - m.detailOffset

	m = press(t, m, click(detailLeft(m), y))
	m = press(t, m, motion(detailLeft(m)+5, y))
	m = press(t, m, release(detailLeft(m)+5, y))

	if calls := len(recorder.osc52) + len(recorder.native) + len(recorder.tmux) + len(recorder.clears); calls != 0 {
		t.Errorf("recorded %+v, want no copy", recorder)
	}
	if m.sel.isActive {
		t.Error("the blank selection is kept, want it cleared")
	}
	if got := statusRow(m); strings.Contains(got, "copied") {
		t.Errorf("status bar = %q, want no copy notice", got)
	}
}

func TestClipboard_FailingNativeAndTmuxWrites_StillShowTheNotice(t *testing.T) {
	recorder := &copyRecorder{
		nativeErr: errors.New("no clipboard program"),
		tmuxErr:   errors.New("tmux refused"),
	}
	m := recordCopies(newSelectModel(t, newTestStore(t, selectFiles)), recorder)

	m = dragSelect(t, m, "first point", 0, "first point", 5)

	assertCopied(t, recorder, "first")
	if got := statusRow(m); !strings.Contains(got, "copied 5 chars") {
		t.Errorf("status bar = %q, want it to show copied 5 chars", got)
	}
}

func TestClipboard_StaleClear_KeepsTheNewerNotice(t *testing.T) {
	recorder := &copyRecorder{}
	m := recordCopies(newSelectModel(t, newTestStore(t, selectFiles)), recorder)
	m = dragSelect(t, m, "first point", 0, "first point", 5)
	m = dragSelect(t, m, "second point", 0, "second point", 3)
	if len(recorder.clears) != 2 {
		t.Fatalf("scheduled %d clears, want 2", len(recorder.clears))
	}

	m = press(t, m, recorder.clears[0])

	if got := statusRow(m); !strings.Contains(got, "copied 3 chars") {
		t.Errorf("after the first copy's clear the status bar = %q, want copied 3 chars kept", got)
	}

	m = press(t, m, recorder.clears[1])

	if got := statusRow(m); strings.Contains(got, "copied") {
		t.Errorf("after the second copy's clear the status bar = %q, want no copy notice", got)
	}
}

func TestClipboard_WideRuneDrag_CopiesTheHighlightedRunes(t *testing.T) {
	recorder := &copyRecorder{}
	m := recordCopies(newSelectModel(t, newTestStore(t, wideFiles)), recorder)

	m = dragSelect(t, m, "字", 0, " text", 0)

	assertCopied(t, recorder, "字かな")
	if got := statusRow(m); !strings.Contains(got, "copied 3 chars") {
		t.Errorf("status bar = %q, want it to show copied 3 chars", got)
	}
	if got := shadedLines(t, m); !slices.Equal(slices.Collect(maps.Values(got)), []string{"字かな"}) {
		t.Errorf("shaded lines = %v, want only 字かな", got)
	}
}

func TestClipboard_CopyNotice_LeadsTheNotices(t *testing.T) {
	m := newSelectModel(t, newTestStore(t, selectFiles))
	m.writeNotice = "closed alpha"

	m = dragSelect(t, m, "first point", 0, "first point", 5)

	got := statusRow(m)
	copied, written := strings.Index(got, "copied 5 chars"), strings.Index(got, "closed alpha")
	if copied < 0 || written < 0 || copied > written {
		t.Errorf("status bar = %q, want copied 5 chars before closed alpha", got)
	}
}
