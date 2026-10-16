package ui

import (
	"maps"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// filterFiles is a store whose issue outline is epic (labelled ui, with alpha, holding leaf, and
// the done shipped), solo (labelled ui), user (labelled ui, waiting on wrapped) and wrapped, whose
// child one is done; the parser appears in alpha, leaf and shipped. Its Knowledge outline is alpha,
// guide (with setup, holding deep) and the background zeta; the parser appears in guide and setup.
var filterFiles = map[string]string{
	"issues/epic.md":       issueFile("title: Epic\nstatus: open\npriority: 1\nlabels: [ui]\n", "The epic plans the rollout.\n"),
	"issues/epic/alpha.md": issueFile("title: Alpha\nstatus: active\npriority: 2\n", "Alpha wires the parser.\n"),
	"issues/epic/alpha/leaf.md": issueFile(
		"title: Leaf\nstatus: open\npriority: 3\nlabels: [parser]\n",
		"Leaf tunes the parser cache.\n",
	),
	"issues/epic/shipped.md": issueFile(
		"title: Shipped\nstatus: done\npriority: 2\nclosed: 2026-10-12T09:00:00Z\nreason: shipped\n",
		"Shipped the parser.\n",
	),
	"issues/solo.md":                issueFile("title: Solo\nstatus: open\npriority: 2\nlabels: [ui]\n", "Solo draws the screen.\n"),
	"issues/user.md":                issueFile("title: User\nstatus: open\npriority: 2\nlabels: [ui]\ndeps: [wrapped]\n", ""),
	"issues/wrapped.md":             issueFile("title: Wrapped\nstatus: open\npriority: 2\n", "Wrapped holds the rest.\n"),
	"issues/wrapped/one.md":         closedIssueFile("One"),
	"knowledge/alpha.md":            noteFile("title: Alpha\nlabels: [testing]\n", "Testing notes.\n"),
	"knowledge/guide.md":            noteFile("title: Guide\n", "How the parser works.\n"),
	"knowledge/guide/setup.md":      noteFile("title: Setup\nlabels: [howto]\n", "Set up the parser.\n"),
	"knowledge/guide/setup/deep.md": noteFile("title: Deep\n", "Deep down.\n"),
	"knowledge/zeta.md":             noteFile("title: Zeta\nlabels: [background]\n", "The parser in the background.\n"),
}

// typeKeys types text into the model, one printable key per rune.
func typeKeys(t *testing.T, m model, text string) model {
	t.Helper()
	for _, r := range text {
		m = press(t, m, runeKey(r))
	}
	return m
}

// applyFilterText opens the filter line, types text and keeps it with Enter.
func applyFilterText(t *testing.T, m model, text string) model {
	t.Helper()
	m = typeKeys(t, press(t, m, runeKey('/')), text)
	return press(t, m, specialKey(tea.KeyEnter))
}

// sidebarText returns the sidebar's rows, their styling stripped.
func sidebarText(m model) []string {
	rows := make([]string, 0, len(m.sidebar))
	for _, row := range m.sidebar {
		rows = append(rows, ansi.Strip(row))
	}
	return rows
}

// matchedIDs returns the ids the open tab's filter matched, sorted.
func matchedIDs(m model) []string {
	return slices.Sorted(maps.Keys(m.filters[m.tab].matched))
}

func TestFilter_IssuesTerms_ShowMatchesWithAncestorsExpanded(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantMatched []string
		wantVisible []string
	}{
		{name: "text", text: "parser", wantMatched: []string{"alpha", "leaf", "shipped"}, wantVisible: []string{"epic", "alpha", "leaf"}},
		{name: "status", text: "status:active", wantMatched: []string{"alpha"}, wantVisible: []string{"epic", "alpha"}},
		{name: "label", text: "label:parser", wantMatched: []string{"leaf"}, wantVisible: []string{"epic", "alpha", "leaf"}},
		{name: "combined", text: "status:open label:ui screen", wantMatched: []string{"solo"}, wantVisible: []string{"solo"}},
		{name: "closed status", text: "status:done", wantMatched: []string{"one", "shipped"}, wantVisible: []string{"epic", "shipped", "wrapped", "one"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newIssuesModel(t, newTestStore(t, filterFiles), "", 120, 30)

			m = applyFilterText(t, m, tt.text)

			if got := matchedIDs(m); !slices.Equal(got, tt.wantMatched) {
				t.Errorf("matched = %v, want %v", got, tt.wantMatched)
			}
			if got := visibleIDs(m); !slices.Equal(got, tt.wantVisible) {
				t.Errorf("visible rows = %v, want %v", got, tt.wantVisible)
			}
			if got := mutedIDs(m); len(got) != 0 {
				t.Errorf("muted rows = %v, want none", got)
			}
		})
	}
}

func TestFilter_IssuesText_MatchesSearchIssues(t *testing.T) {
	ts := newTestStore(t, filterFiles)
	m := newIssuesModel(t, ts, "", 120, 30)

	m = applyFilterText(t, m, "the parser")

	hits, err := ts.index.SearchIssues("the parser")
	if err != nil {
		t.Fatalf("SearchIssues: %v", err)
	}
	want := make([]string, 0, len(hits))
	for _, hit := range hits {
		want = append(want, hit.ID)
	}
	slices.Sort(want)
	if got := matchedIDs(m); !slices.Equal(got, want) {
		t.Errorf("matched = %v, want SearchIssues' %v", got, want)
	}
}

func TestFilter_KnowledgeText_MatchesSearchNotes(t *testing.T) {
	ts := newTestStore(t, filterFiles)
	m := newKnowledgeModel(t, ts, "", 120, 30)

	m = applyFilterText(t, m, "parser")

	hits, err := ts.index.SearchNotes("parser", nil, false)
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}
	want := make([]string, 0, len(hits))
	for _, hit := range hits {
		want = append(want, hit.ID)
	}
	slices.Sort(want)
	if got := matchedIDs(m); !slices.Equal(got, want) {
		t.Errorf("matched = %v, want SearchNotes' %v", got, want)
	}
	// zeta matches too, but a filter keeps the tree's default visibility: background stays hidden.
	if got, want := visibleNoteIDs(m), []string{"guide", "setup"}; !slices.Equal(got, want) {
		t.Errorf("visible rows = %v, want %v", got, want)
	}
}

func TestFilter_KnowledgeTerms_KeepAncestorsExpanded(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantVisible []string
	}{
		{name: "text", text: "deep", wantVisible: []string{"guide", "setup", "deep"}},
		{name: "label", text: "label:howto", wantVisible: []string{"guide", "setup"}},
		{name: "combined", text: "label:howto parser", wantVisible: []string{"guide", "setup"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newKnowledgeModel(t, newTestStore(t, filterFiles), "", 120, 30)

			m = applyFilterText(t, m, tt.text)

			if got := visibleNoteIDs(m); !slices.Equal(got, tt.wantVisible) {
				t.Errorf("visible rows = %v, want %v", got, tt.wantVisible)
			}
		})
	}
}

func TestFilter_ReadyLabel_NarrowsTheQueue(t *testing.T) {
	m := newTestModel(t, newTestStore(t, filterFiles), 120, 30)
	var want []string
	for _, entry := range m.ready.entries {
		if slices.Contains(entry.Labels, "ui") {
			want = append(want, entry.ID)
		}
	}
	if len(want) == 0 || len(want) == len(m.ready.entries) {
		t.Fatalf("fixture: ready %d issues, %d labelled ui; want some of each", len(m.ready.entries), len(want))
	}

	m = applyFilterText(t, m, "label:ui")

	got := make([]string, 0, len(m.ready.entries))
	for _, entry := range m.ready.entries {
		got = append(got, entry.ID)
	}
	if !slices.Equal(got, want) {
		t.Errorf("ready rows = %v, want %v", got, want)
	}
}

func TestFilter_Problems_ShowAMessage(t *testing.T) {
	tests := []struct {
		name string
		tab  Tab
		text string
		want string
	}{
		{name: "unknown status", tab: TabIssues, text: "status:nope", want: `status "nope" is not declared in config.yaml`},
		{name: "unknown label", tab: TabIssues, text: "parser label:nope", want: `no issue carries label "nope"`},
		{name: "unknown status on ready", tab: TabReady, text: "status:nope", want: `status "nope" is not declared in config.yaml`},
		{name: "unknown note label", tab: TabKnowledge, text: "label:nope", want: `no Note carries label "nope"`},
		{name: "status on notes", tab: TabKnowledge, text: "status:open", want: "status: applies to issues, not Notes"},
		{name: "no match", tab: TabIssues, text: "zebra", want: "No issue matches the filter."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := newTestOptions(t, newTestStore(t, filterFiles))
			options.Tab = tt.tab
			m := startModel(t, options, 120, 30)

			m = applyFilterText(t, m, tt.text)

			if got := sidebarText(m); !slices.Equal(got, []string{tt.want}) {
				t.Errorf("sidebar = %q, want %q", got, tt.want)
			}
			if _, isSelected := m.sidebarSelection(); isSelected {
				t.Error("a row is selected, want none")
			}
		})
	}
}

func TestFilter_ClosedStatus_RowIsSelectable(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, filterFiles), "", 120, 30)

	m = applyFilterText(t, m, "status:done")
	m = press(t, m, runeKey('j'))

	if got := selectedIssueID(m); got != "shipped" {
		t.Fatalf("selected = %q, want shipped", got)
	}
	if text := detailText(m); !strings.Contains(text, "Shipped the parser.") {
		t.Errorf("detail pane = %q, want shipped's body", text)
	}
}

func TestFilter_OpenLine_TakesEveryKey(t *testing.T) {
	m := newTestModel(t, newTestStore(t, filterFiles), 120, 30)
	m = press(t, m, runeKey('/'))

	for _, key := range []tea.KeyPressMsg{runeKey('q'), runeKey('1'), runeKey('d'), specialKey(tea.KeyTab), runeKey('?')} {
		updated, cmd := m.Update(key)
		if cmd != nil {
			if _, isQuit := cmd().(tea.QuitMsg); isQuit {
				t.Fatalf("%s quit the UI", key)
			}
		}
		m = updated.(model)
	}

	if m.tab != TabReady || m.deps.isOpen || m.isHelpOpen || !m.isFilterOpen {
		t.Errorf("tab %v, deps open %v, help open %v, filter open %v; want the Ready tab with only the line open",
			m.tab, m.deps.isOpen, m.isHelpOpen, m.isFilterOpen)
	}
	if got := m.filterLine.value(); got != "q1d?" {
		t.Errorf("line = %q, want q1d?", got)
	}
}

func TestFilter_StatusBar_ShowsTheOpenTabsFilter(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, filterFiles), "", 120, 30)

	m = applyFilterText(t, m, "parser")

	if got := statusRow(m); !strings.Contains(got, "filter: parser") {
		t.Errorf("status bar = %q, want filter: parser", got)
	}
	m = press(t, m, runeKey('2'))
	if got := statusRow(m); strings.Contains(got, "filter:") {
		t.Errorf("status bar on the Ready tab = %q, want no filter", got)
	}
}

func TestFilter_EscOnTheLine_ClearsTheFilter(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, filterFiles), "", 120, 30)
	m = applyFilterText(t, m, "parser")

	m = pressKeys(t, m, runeKey('/'), specialKey(tea.KeyEscape))

	if m.isFilterOpen || m.filters[TabIssues].isActive() {
		t.Errorf("filter open %v, filter %q; want both gone", m.isFilterOpen, m.filters[TabIssues].text)
	}
	if got, want := visibleIDs(m), []string{"epic", "solo", "user", "wrapped"}; !slices.Equal(got, want) {
		t.Errorf("visible rows = %v, want the collapsed tree %v", got, want)
	}
}

func TestFilter_EscFromTheSidebar_ClearsAndRestoresTheTree(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, filterFiles), "", 120, 30)
	m = applyFilterText(t, m, "label:parser")

	m = press(t, m, specialKey(tea.KeyEscape))

	if m.filters[TabIssues].isActive() {
		t.Errorf("filter %q still in effect", m.filters[TabIssues].text)
	}
	if got, want := visibleIDs(m), []string{"epic", "solo", "user", "wrapped"}; !slices.Equal(got, want) {
		t.Errorf("visible rows = %v, want the collapsed tree %v", got, want)
	}
	if got := statusRow(m); strings.Contains(got, "filter:") {
		t.Errorf("status bar = %q, want no filter", got)
	}
}

func TestFilter_EscWithDepViewOrNarrowDetail_KeepsTheFilter(t *testing.T) {
	tests := []struct {
		name  string
		width int
		open  tea.KeyPressMsg
	}{
		{name: "dep view", width: 120, open: runeKey('d')},
		{name: "narrow detail", width: 80, open: specialKey(tea.KeyEnter)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newIssuesModel(t, newTestStore(t, filterFiles), "", tt.width, 30)
			m = applyFilterText(t, m, "label:ui")

			m = pressKeys(t, m, tt.open, specialKey(tea.KeyEscape))

			if m.deps.isOpen || m.isDetailOpen {
				t.Fatalf("deps open %v, detail open %v; want Esc to have closed them", m.deps.isOpen, m.isDetailOpen)
			}
			if got := m.filters[TabIssues].text; got != "label:ui" {
				t.Errorf("filter = %q, want label:ui kept", got)
			}
		})
	}
}

func TestFilter_DepViewEnterOnHiddenIssue_ClearsTheIssuesFilter(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, filterFiles), "", 120, 30)
	m = applyFilterText(t, m, "label:ui")
	m = pressKeys(t, m, runeKey('j'), runeKey('j'))
	if got := selectedIssueID(m); got != "user" {
		t.Fatalf("selected = %q, want user", got)
	}

	m = pressKeys(t, m, runeKey('d'), specialKey(tea.KeyEnter))

	if m.filters[TabIssues].isActive() {
		t.Errorf("filter %q still in effect", m.filters[TabIssues].text)
	}
	if got := selectedIssueID(m); got != "wrapped" {
		t.Errorf("selected = %q, want wrapped", got)
	}
}

func TestFilter_OpenLine_MatchesGolden(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, filterFiles), "", 120, 30)

	m = typeKeys(t, press(t, m, runeKey('/')), "parser")

	golden(t, "filter-line-120x30", m.View())
}

func TestFilter_KeptFilter_MatchesGolden(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, filterFiles), "", 120, 30)

	m = applyFilterText(t, m, "parser")

	golden(t, "filter-kept-120x30", m.View())
}
