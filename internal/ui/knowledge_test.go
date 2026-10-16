package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/outline"
)

// knowledgeFiles is a store whose Knowledge outline is alpha, then guide (with setup, holding
// deep, and the retired old, holding relic), then the background zeta; setup carries labels and
// refs naming a file and the issue solo.
var knowledgeFiles = map[string]string{
	"knowledge/alpha.md":            noteFile("title: Alpha\nlabels: [testing]\n", "Alpha's *body*.\n"),
	"knowledge/guide.md":            noteFile("title: Guide\naliases: [handbook]\n", "The guide.\n"),
	"knowledge/guide/setup.md":      noteFile("title: Setup\nlabels: [howto, cli]\nrefs: [internal/ui/knowledge.go, solo]\n", "Set it up.\n"),
	"knowledge/guide/setup/deep.md": noteFile("title: Deep\n", ""),
	"knowledge/guide/old.md":        noteFile("title: Old\nretired: true\n", "Retired.\n"),
	"knowledge/guide/old/relic.md":  noteFile("title: Relic\n", ""),
	"knowledge/zeta.md":             noteFile("title: Zeta\nlabels: [background]\n", ""),
	"issues/solo.md":                issueFile("title: Solo\nstatus: open\npriority: 2\n", ""),
}

// noteFile returns the file of a Note with the frontmatter fields given, then body.
func noteFile(fields, body string) string {
	return "---\n" + fields + "---\n" + body
}

// newKnowledgeModel returns a started model over the test store on the Knowledge tab, selecting
// id when it is not empty, sized width by height.
func newKnowledgeModel(t *testing.T, ts *testStore, id string, width, height int) model {
	t.Helper()
	options := newTestOptions(t, ts)
	options.Tab = TabKnowledge
	options.ID = id
	return startModel(t, options, width, height)
}

// visibleNoteIDs returns the ids of the Knowledge tree's visible rows, top to bottom.
func visibleNoteIDs(m model) []string {
	ids := make([]string, 0, len(m.knowledge.tree.rows))
	for _, row := range m.knowledge.tree.rows {
		ids = append(ids, row.ID)
	}
	return ids
}

// mutedNoteIDs returns the ids of the Knowledge tree's muted rows, top to bottom.
func mutedNoteIDs(m model) []string {
	var ids []string
	for _, row := range m.knowledge.tree.rows {
		if row.isMuted {
			ids = append(ids, row.ID)
		}
	}
	return ids
}

// selectedNoteID returns the id of the Knowledge tree's selected Note, "" when there is none.
func selectedNoteID(m model) string {
	note, _ := m.knowledgeSelection()
	return note.ID
}

func TestKnowledgeTab_NestedTree_MatchesGolden(t *testing.T) {
	m := newKnowledgeModel(t, newTestStore(t, knowledgeFiles), "", 120, 30)

	m = pressKeys(t, m, runeKey('j'), runeKey('l'), runeKey('j'), runeKey('l'))

	golden(t, "knowledge-nested-120x30", m.View())
}

func TestKnowledgeTab_NoteWithRefs_MatchesGolden(t *testing.T) {
	m := newKnowledgeModel(t, newTestStore(t, knowledgeFiles), "setup", 120, 30)

	golden(t, "knowledge-refs-120x30", m.View())
}

func TestKnowledgeTab_OpensCollapsedToTheTopLevel(t *testing.T) {
	m := newKnowledgeModel(t, newTestStore(t, knowledgeFiles), "", 120, 30)

	if got, want := visibleNoteIDs(m), []string{"alpha", "guide"}; !slices.Equal(got, want) {
		t.Errorf("visible rows = %v, want %v", got, want)
	}
	if text := detailText(m); !strings.HasPrefix(text, "Alpha") || !strings.Contains(text, "labels: testing") {
		t.Errorf("detail = %q, want alpha's title and labels", text)
	}
}

func TestKnowledgeTab_FullyExpanded_RowOrderEqualsTheKnowTreeOutline(t *testing.T) {
	ts := newTestStore(t, knowledgeFiles)
	notes, err := ts.index.Notes(false)
	if err != nil {
		t.Fatalf("Notes: %v", err)
	}
	var want []string
	for _, row := range outline.Notes(notes, false).Rows(nil, outline.UnlimitedDepth) {
		want = append(want, row.ID)
	}
	m := newKnowledgeModel(t, ts, "", 120, 30)

	for position := 0; position < len(m.knowledge.tree.rows); position++ {
		m.knowledge.tree.cursor.seat(position, len(m.knowledge.tree.rows))
		m = press(t, m, runeKey('l'))
	}

	if got := visibleNoteIDs(m); !slices.Equal(got, want) {
		t.Errorf("fully expanded tree = %v, want the outline of capcom know tree %v", got, want)
	}
}

func TestKnowledgeTab_NoteDetail_ShowsLabelsRefsAndBody_RefsNamingAnIDAreLinks(t *testing.T) {
	m := newKnowledgeModel(t, newTestStore(t, knowledgeFiles), "setup", 120, 30)

	text := detailText(m)
	for _, want := range []string{"Setup", "labels: howto, cli", "refs: internal/ui/knowledge.go, solo", "Set it up."} {
		if !strings.Contains(text, want) {
			t.Errorf("detail = %q, want it to hold %q", text, want)
		}
	}
	targets := make([]string, 0, len(m.issue.rendered.Links))
	for _, link := range m.issue.rendered.Links {
		targets = append(targets, link.Target)
	}
	if !slices.Equal(targets, []string{"solo"}) {
		t.Errorf("links = %v, want only solo: the file ref stays inert", targets)
	}
	if m.issue.etag == "" {
		t.Errorf("ETag of a parsed Note file is empty, want the file's")
	}
}

func TestKnowledgeTab_OpenedOnAnID_SelectsItWithItsAncestorsExpanded(t *testing.T) {
	cases := []struct {
		name         string
		id           string
		wantVisible  []string
		wantSelected string
		wantNotice   string
	}{
		{name: "nested id", id: "deep", wantVisible: []string{"alpha", "guide", "setup", "deep"}, wantSelected: "deep"},
		{name: "alias", id: "handbook", wantVisible: []string{"alpha", "guide"}, wantSelected: "guide"},
		{
			name:         "unknown",
			id:           "no-such-note",
			wantVisible:  []string{"alpha", "guide"},
			wantSelected: "alpha",
			wantNotice:   "select no-such-note: ",
		},
	}
	ts := newTestStore(t, knowledgeFiles)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newKnowledgeModel(t, ts, testCase.id, 120, 30)

			if got := visibleNoteIDs(m); !slices.Equal(got, testCase.wantVisible) {
				t.Errorf("capcom ui knowledge %s: visible rows = %v, want %v", testCase.id, got, testCase.wantVisible)
			}
			if got := selectedNoteID(m); got != testCase.wantSelected {
				t.Errorf("capcom ui knowledge %s: selected = %q, want %q", testCase.id, got, testCase.wantSelected)
			}
			if !strings.Contains(statusRow(m), testCase.wantNotice) {
				t.Errorf("capcom ui knowledge %s: status bar %q, want %q", testCase.id, statusRow(m), testCase.wantNotice)
			}
		})
	}
}

func TestKnowledgeTab_OpenedOnAHiddenNote_RevealsAndSelectsIt(t *testing.T) {
	cases := []struct {
		name        string
		id          string
		wantVisible []string
		wantMuted   []string
		wantTitle   string
	}{
		{
			name:        "retired",
			id:          "old",
			wantVisible: []string{"alpha", "guide", "old", "setup"},
			wantMuted:   []string{"old"},
			wantTitle:   "Old",
		},
		{
			name:        "under a retired Note",
			id:          "relic",
			wantVisible: []string{"alpha", "guide", "old", "relic", "setup"},
			wantMuted:   []string{"old", "relic"},
			wantTitle:   "Relic",
		},
		{
			name:        "background",
			id:          "zeta",
			wantVisible: []string{"alpha", "guide", "zeta"},
			wantMuted:   []string{"zeta"},
			wantTitle:   "Zeta",
		},
	}
	ts := newTestStore(t, knowledgeFiles)
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newKnowledgeModel(t, ts, testCase.id, 120, 30)

			if got := visibleNoteIDs(m); !slices.Equal(got, testCase.wantVisible) {
				t.Errorf("capcom ui knowledge %s: visible rows = %v, want %v", testCase.id, got, testCase.wantVisible)
			}
			if got := mutedNoteIDs(m); !slices.Equal(got, testCase.wantMuted) {
				t.Errorf("capcom ui knowledge %s: muted rows = %v, want %v", testCase.id, got, testCase.wantMuted)
			}
			if text := detailText(m); selectedNoteID(m) != testCase.id || !strings.HasPrefix(text, testCase.wantTitle) {
				t.Errorf("capcom ui knowledge %s: selected %q with detail %q, want %s", testCase.id, selectedNoteID(m), text, testCase.id)
			}
		})
	}
}

func TestKnowledgeTab_SelectionLeavesARevealedNote_HidesItAgain(t *testing.T) {
	m := newKnowledgeModel(t, newTestStore(t, knowledgeFiles), "old", 120, 30)

	m = press(t, m, runeKey('j'))

	if got, want := visibleNoteIDs(m), []string{"alpha", "guide", "setup"}; !slices.Equal(got, want) ||
		selectedNoteID(m) != "setup" || len(mutedNoteIDs(m)) != 0 {
		t.Errorf("j onto setup: visible %v selecting %q, want %v selecting setup, nothing muted",
			got, selectedNoteID(m), want)
	}
}

// selectedNoteRow returns the Knowledge sidebar row of the selected Note and that Note's tree
// row, failing the test when the selection is not want.
func selectedNoteRow(t *testing.T, m model, want string) (string, treeRow[index.NoteEntry]) {
	t.Helper()
	selected, isSelected := m.knowledge.tree.selection()
	if !isSelected || selected.ID != want {
		t.Fatalf("selected Note = %q, want %q", selected.ID, want)
	}
	text := selected.ID + noteRowSeparator + selected.Entry.Title
	for _, row := range m.sidebar {
		if strings.Contains(row, text) {
			return row, selected
		}
	}
	t.Fatalf("sidebar %q has no row %q", m.sidebar, text)
	return "", selected
}

// sgrAttributes returns the parameters of the SGR parameter list sgr that stand on their own: a
// colour's sub-parameters (the 2;r;g;b or 5;n after a 38, 48 or 58) are skipped.
func sgrAttributes(sgr string) []string {
	parameters := strings.Split(sgr, ";")
	attributes := make([]string, 0, len(parameters))
	for position := 0; position < len(parameters); position++ {
		parameter := parameters[position]
		attributes = append(attributes, parameter)
		if parameter != "38" && parameter != "48" && parameter != "58" || position+1 >= len(parameters) {
			continue
		}
		switch parameters[position+1] {
		case "2":
			position += 4
		case "5":
			position += 2
		}
	}
	return attributes
}

func TestKnowledgeTab_SelectedRow_FillsTheBackgroundAfterTheConnector(t *testing.T) {
	// dark's selection is #3a5fcd.
	const fill = "48;2;58;95;205"
	m := newThemeModel(t, knowledgeFiles, "ui:\n  theme: dark\n", false, func(options *Options) {
		options.Tab = TabKnowledge
		options.ID = "setup"
	})

	row, selected := selectedNoteRow(t, m, "setup")

	for _, text := range []string{treeArrow(selected), "setup  Setup"} {
		if got := sgrBefore(t, row, text); !strings.Contains(got, fill) {
			t.Errorf("%q in row %q is drawn in %q, want it to carry %s", text, row, got, fill)
		}
	}
	if selected.connector == "" {
		t.Fatalf("setup has no tree connector: the check cannot tell")
	}
	lead := row[:strings.Index(row, selected.connector)+len(selected.connector)]
	if strings.Contains(lead, fill) {
		t.Errorf("connector %q in row %q is drawn with the fill %s, want the fill to start after it", selected.connector, row, fill)
	}
}

func TestKnowledgeTab_SelectedRetiredRow_IsFaintOverTheFill(t *testing.T) {
	// dark's selection is #3a5fcd; SGR 2 is faint.
	const (
		fill  = "48;2;58;95;205"
		faint = "2"
	)
	// relic, under the retired old, is revealed, muted and selected.
	m := newThemeModel(t, knowledgeFiles, "ui:\n  theme: dark\n", false, func(options *Options) {
		options.Tab = TabKnowledge
		options.ID = "relic"
	})

	row, selected := selectedNoteRow(t, m, "relic")

	for _, text := range []string{treeArrow(selected), "relic  Relic"} {
		got := sgrBefore(t, row, text)
		if !strings.Contains(got, fill) {
			t.Errorf("%q in row %q is drawn in %q, want it to carry %s", text, row, got, fill)
		}
		if !slices.Contains(sgrAttributes(got), faint) {
			t.Errorf("%q in row %q is drawn in %q, want it faint (SGR %s)", text, row, got, faint)
		}
	}
}
