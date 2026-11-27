package ui

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/ui/markdown"
	"github.com/airiclenz/capcom/internal/userconfig"
)

// priorityFiles is a store with one open issue per priority, zero (P0) to four (P4), and one closed
// issue per priority, shut-zero (P0) to shut-four (P4), closed newest first from shut-zero. zero
// waits on every closed one, so it stays Ready and its dependency view lists one row per priority.
var priorityFiles = map[string]string{
	"issues/zero.md": issueFile(
		"title: Zero task\nstatus: open\npriority: 0\n"+
			"deps: [shut-zero, shut-one, shut-two, shut-three, shut-four]\n",
		"Zero's body.\n",
	),
	"issues/one.md":        issueFile("title: One task\nstatus: open\npriority: 1\n", "One's body.\n"),
	"issues/two.md":        issueFile("title: Two task\nstatus: open\npriority: 2\n", "Two's body.\n"),
	"issues/three.md":      issueFile("title: Three task\nstatus: open\npriority: 3\n", "Three's body.\n"),
	"issues/four.md":       issueFile("title: Four task\nstatus: open\npriority: 4\n", "Four's body.\n"),
	"issues/shut-zero.md":  shutIssueFile("Shut zero", 0, "2026-10-15T10:00:00Z"),
	"issues/shut-one.md":   shutIssueFile("Shut one", 1, "2026-10-14T10:00:00Z"),
	"issues/shut-two.md":   shutIssueFile("Shut two", 2, "2026-10-13T10:00:00Z"),
	"issues/shut-three.md": shutIssueFile("Shut three", 3, "2026-10-12T10:00:00Z"),
	"issues/shut-four.md":  shutIssueFile("Shut four", 4, "2026-10-11T10:00:00Z"),
}

// shutIssueFile returns the file of a done issue titled title, of priority, closed at closed.
func shutIssueFile(title string, priority int, closed string) string {
	return issueFile(
		fmt.Sprintf("title: %s\nstatus: done\npriority: %d\nclosed: %s\nreason: shipped\n", title, priority, closed),
		"",
	)
}

// darkPriorityColours are the SGR foreground parameters of the dark preset's priority roles, by
// priority; P2 has none.
var darkPriorityColours = map[int]string{
	0: "38;2;248;81;73",
	1: "38;2;227;179;65",
	3: "38;2;138;138;138",
	4: "38;2;92;92;92",
}

// darkSelectionFill is the SGR background parameters of the dark preset's selection, #3a5fcd.
const darkSelectionFill = "48;2;58;95;205"

var (
	// sgrForeground matches SGR parameters that set a foreground colour.
	sgrForeground = regexp.MustCompile(`(^|;)38;`)
	// sgrBold matches SGR parameters that set bold.
	sgrBold = regexp.MustCompile(`(^|;)1(;|$)`)
)

// checkDarkPriority fails the test unless the SGR parameters got, which paint the P<n> of an issue
// of priority, carry the dark preset's colour for it, none for P2, and bold for P0 alone.
func checkDarkPriority(t *testing.T, where string, priority int, got string) {
	t.Helper()
	want, isColoured := darkPriorityColours[priority]
	switch {
	case isColoured && !strings.Contains(got, want):
		t.Errorf("%s: P%d is drawn in %q, want it to carry %s", where, priority, got, want)
	case !isColoured && sgrForeground.MatchString(got):
		t.Errorf("%s: P%d is drawn in %q, want no foreground colour", where, priority, got)
	}
	if isBold := sgrBold.MatchString(got); isBold != (priority == 0) {
		t.Errorf("%s: P%d is drawn in %q, bold = %t, want %t", where, priority, got, isBold, priority == 0)
	}
}

// lineWith returns the line of lines that holds text, failing the test when none does.
func lineWith(t *testing.T, lines []string, text string) string {
	t.Helper()
	index := slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, text) })
	if index < 0 {
		t.Fatalf("no line holds %q in %q", text, lines)
	}
	return lines[index]
}

func TestPriority_EveryIssueTab_PaintsEachLevelInItsColour(t *testing.T) {
	openTitles := []string{"Zero task", "One task", "Two task", "Three task", "Four task"}
	shutTitles := []string{"Shut zero", "Shut one", "Shut two", "Shut three", "Shut four"}
	testCases := []struct {
		name   string
		tab    Tab
		isDeps bool
		titles []string
		// selected is the priority of the row the tab selects as it opens.
		selected int
	}{
		{name: "ready", tab: TabReady, titles: openTitles},
		{name: "issues", tab: TabIssues, titles: openTitles},
		{name: "closed", tab: TabClosed, titles: shutTitles},
		{name: "dependency view", tab: TabReady, isDeps: true, titles: shutTitles},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newThemeModel(t, priorityFiles, "ui:\n  theme: dark\n", false, func(options *Options) {
				options.Tab = testCase.tab
				options.IsTabGiven = true
				if testCase.tab != TabClosed {
					options.ID = "zero"
				}
			})
			lines := m.sidebar
			if testCase.isDeps {
				m = press(t, m, runeKey('d'))
				lines = m.detail
			}

			for priority, title := range testCase.titles {
				row := lineWith(t, lines, title)
				got := sgrBefore(t, row, priorityText(priority))
				checkDarkPriority(t, testCase.name, priority, got)
				if priority == testCase.selected && !strings.Contains(got, darkSelectionFill) {
					t.Errorf("%s: selected P%d is drawn in %q, want it to carry the fill %s",
						testCase.name, priority, got, darkSelectionFill)
				}
			}
		})
	}
}

func TestPriority_DetailHeader_PaintsEachLevelInItsColour(t *testing.T) {
	for priority, id := range []string{"zero", "one", "two", "three", "four"} {
		t.Run(id, func(t *testing.T) {
			m := newThemeModel(t, priorityFiles, "ui:\n  theme: dark\n", false, func(options *Options) {
				options.Tab = TabIssues
				options.IsTabGiven = true
				options.ID = id
			})

			line := lineWith(t, m.detail, "status: ")
			want := fmt.Sprintf("status: open · P%d · task", priority)
			if got := strings.TrimSpace(ansi.Strip(line)); got != want {
				t.Fatalf("status line = %q, want %q", got, want)
			}
			checkDarkPriority(t, "detail header", priority, sgrBefore(t, line, priorityText(priority)))
		})
	}
}

func TestPriority_NoteDetail_IsNotRepainted(t *testing.T) {
	files := map[string]string{
		"knowledge/rules.md": "---\ntitle: Rules\n---\n\nstatus: open · P0 · task\n",
	}
	m := newThemeModel(t, files, "ui:\n  theme: dark\n", false, func(options *Options) {
		options.Tab = TabKnowledge
		options.IsTabGiven = true
		options.ID = "rules"
	})

	line := lineWith(t, m.detail, "P0")

	if got := sgrBefore(t, line, "P0"); got != "" {
		t.Errorf("a Note's line %q has P0 drawn in %q, want it plain", line, got)
	}
}

func TestPriority_Override_PaintsTheDefaultPriority(t *testing.T) {
	m := newThemeModel(t, priorityFiles, "ui:\n  colors:\n    priority-2: \"#010203\"\n", false, func(options *Options) {
		options.ID = "two"
	})

	for where, line := range map[string]string{
		"ready row":     lineWith(t, m.sidebar, "Two task"),
		"detail header": lineWith(t, m.detail, "status: "),
	} {
		if got := sgrBefore(t, line, "P2"); !strings.Contains(got, "38;2;1;2;3") {
			t.Errorf("%s: P2 is drawn in %q, want the priority-2 override's 38;2;1;2;3", where, got)
		}
	}
}

func TestPriority_NoColor_P0IsBoldWithoutColour(t *testing.T) {
	// zero, P0, opens selected; one step down selects one, so zero's row is unmarked: under
	// NO_COLOR a selected row is drawn whole in reverse, its spans' own styles dropped.
	withZeroSelected := newThemeModel(t, priorityFiles, "ui:\n  theme: dark\n", true, nil)
	withOneSelected := press(t, withZeroSelected, specialKey(tea.KeyDown))

	for where, line := range map[string]string{
		"ready row":     lineWith(t, withOneSelected.sidebar, "Zero task"),
		"detail header": lineWith(t, withZeroSelected.detail, "status: "),
	} {
		got := sgrBefore(t, line, "P0")
		if !sgrBold.MatchString(got) {
			t.Errorf("%s: P0 is drawn in %q, want bold", where, got)
		}
		if found := colourSequences(line); len(found) > 0 {
			t.Errorf("%s: line %q has colour sequences %q under NO_COLOR", where, line, found)
		}
	}
}

func TestPriority_DetailHeader_PaintedAtEveryWidthThatKeepsItOnTheStatusLine(t *testing.T) {
	th := newTheme(userconfig.UI{Theme: userconfig.ThemeDark}, false, false)
	source := issueHeader(index.Entry{ID: "zero", Title: "Zero", Status: "open", Priority: 0, Type: "feature"})

	for width := ansi.StringWidth("status: open · P0"); width <= 40; width++ {
		lines := markdown.Render(th.markdown, source, width, markdown.Options{}).Lines

		line := lineWith(t, paintHeaderPriority(th, lines), "status: ")
		if !strings.Contains(ansi.Strip(line), "P0") {
			t.Fatalf("width %d: status line %q lacks P0", width, line)
		}
		checkDarkPriority(t, fmt.Sprintf("width %d", width), 0, sgrBefore(t, line, "P0"))
	}
}

func TestPriority_DetailHeader_BlankTitle_StillPaintsTheStatusLine(t *testing.T) {
	th := newTheme(userconfig.UI{Theme: userconfig.ThemeDark}, false, false)
	source := issueHeader(index.Entry{ID: "blank", Title: "   ", Status: "open", Priority: 0, Type: "task"})
	lines := markdown.Render(th.markdown, source, 60, markdown.Options{}).Lines

	line := lineWith(t, paintHeaderPriority(th, lines), "status: ")

	checkDarkPriority(t, "blank title", 0, sgrBefore(t, line, "P0"))
}
