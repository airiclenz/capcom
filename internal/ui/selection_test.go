package ui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/ui/markdown"
)

// selectionTestWidth is the pane width the selection tests paint and copy in.
const selectionTestWidth = 40

// selectionTestStyle is the style the selection tests shade in: Reverse, which needs no colour.
var selectionTestStyle = lipgloss.NewStyle().Reverse(true)

// newDetailSel returns an active selection from the anchor to the head, each a line and a column.
func newDetailSel(anchorLine, anchorCol, headLine, headCol int) detailSel {
	return detailSel{
		isActive: true,
		anchor:   contentCell{line: anchorLine, col: anchorCol},
		head:     contentCell{line: headLine, col: headCol},
	}
}

// shadedText returns the plain text row carries in selectionTestStyle, failing the test when the
// row carries none.
func shadedText(t *testing.T, row string) string {
	t.Helper()
	rendered := selectionTestStyle.Render("X")
	prefix, suffix, _ := strings.Cut(rendered, "X")
	_, after, isFound := strings.Cut(row, prefix)
	if !isFound {
		t.Fatalf("row %q carries no shading", row)
	}
	shaded, _, isFound := strings.Cut(after, suffix)
	if !isFound {
		t.Fatalf("row %q opens its shading but never closes it", row)
	}
	return ansi.Strip(shaded)
}

func TestSelectionText_TwoStyledLines_CopiesPlainTextWithoutTrailingSpaces(t *testing.T) {
	bold := lipgloss.NewStyle().Bold(true)
	lines := []string{
		"plain " + bold.Render("bold") + "   ",
		bold.Render("next") + " line    ",
	}
	selection := newDetailSel(0, 2, 1, 9)

	copied := selection.text(markdown.Measure{}, lines, selectionTestWidth)

	if want := "ain bold\nnext line"; copied != want {
		t.Errorf("text = %q, want %q", copied, want)
	}
}

func TestSelection_ReversedDrag_EqualsTheForwardOne(t *testing.T) {
	var measure markdown.Measure
	lines := []string{"first line", "second line", "third line"}
	forward := newDetailSel(0, 3, 2, 5)
	reversed := newDetailSel(2, 5, 0, 3)

	forwardText := forward.text(measure, lines, selectionTestWidth)
	reversedText := reversed.text(measure, lines, selectionTestWidth)
	forwardPaint := forward.paint(measure, lines, 0, len(lines), selectionTestWidth, selectionTestStyle)
	reversedPaint := reversed.paint(measure, lines, 0, len(lines), selectionTestWidth, selectionTestStyle)

	if forwardText != reversedText {
		t.Errorf("reversed text = %q, forward %q", reversedText, forwardText)
	}
	if !slices.Equal(forwardPaint, reversedPaint) {
		t.Errorf("reversed paint = %q, forward %q", reversedPaint, forwardPaint)
	}
}

func TestSelection_WideRunes_CutWholeRunesAlikeInPaintAndText(t *testing.T) {
	var measure markdown.Measure
	lines := []string{"ab世界🙂cd"}
	cases := []struct {
		name     string
		c0, c1   int
		wantText string
	}{
		{name: "both ends on a rune's first cell", c0: 2, c1: 6, wantText: "世界"},
		{name: "both ends mid-rune", c0: 3, c1: 7, wantText: "世界"},
		{name: "to the line's end", c0: 5, c1: 10, wantText: "界🙂cd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selection := newDetailSel(0, tc.c0, 0, tc.c1)

			copied := selection.text(measure, lines, selectionTestWidth)
			painted := selection.paint(measure, lines, 0, 1, selectionTestWidth, selectionTestStyle)

			if copied != tc.wantText {
				t.Errorf("text = %q, want %q", copied, tc.wantText)
			}
			if shaded := shadedText(t, painted[0]); shaded != copied {
				t.Errorf("paint shades %q, text copies %q", shaded, copied)
			}
			if plain := ansi.Strip(painted[0]); plain != lines[0] {
				t.Errorf("painted row reads %q, want %q", plain, lines[0])
			}
		})
	}
}

func TestSelectionPaint_Collapsed_PaintsNothing(t *testing.T) {
	lines := []string{"one", "two"}
	selection := newDetailSel(1, 2, 1, 2)

	painted := selection.paint(markdown.Measure{}, lines, 0, len(lines), selectionTestWidth, selectionTestStyle)

	if !slices.Equal(painted, lines) {
		t.Errorf("paint = %q, want the lines unchanged %q", painted, lines)
	}
}

func TestSelectionSpanUnchanged(t *testing.T) {
	old := []string{"zero", "one", "two", "three", "four"}
	cases := []struct {
		name        string
		selection   detailSel
		changedLine int
		want        bool
	}{
		{name: "a change outside the span keeps it", selection: newDetailSel(1, 0, 2, 3), changedLine: 4, want: true},
		{name: "a change inside the span drops it", selection: newDetailSel(1, 0, 3, 3), changedLine: 2, want: false},
		{name: "a reversed span is judged alike", selection: newDetailSel(3, 3, 1, 0), changedLine: 2, want: false},
		{name: "a collapsed selection is kept", selection: newDetailSel(2, 1, 2, 1), changedLine: 2, want: true},
		{name: "an inactive selection is not", selection: detailSel{}, changedLine: 4, want: false},
		{name: "a span past the lines is dropped", selection: newDetailSel(3, 0, 7, 0), changedLine: 0, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := slices.Clone(old)
			changed[tc.changedLine] = "rewritten"

			isKept := tc.selection.spanUnchanged(old, changed)

			if isKept != tc.want {
				t.Errorf("spanUnchanged = %v, want %v", isKept, tc.want)
			}
		})
	}
}

func TestSelectionPaint_ScrolledWindow_ShadesOnlyTheVisibleRowsOfAFullLengthCopy(t *testing.T) {
	lines := []string{"line 0", "line 1", "line 2", "line 3", "line 4", "line 5", "line 6"}
	original := slices.Clone(lines)
	selection := newDetailSel(1, 2, 5, 4)
	const offset, rows = 2, 3

	painted := selection.paint(markdown.Measure{}, lines, offset, rows, selectionTestWidth, selectionTestStyle)

	if len(painted) != len(lines) {
		t.Fatalf("len(paint) = %d, want %d", len(painted), len(lines))
	}
	if !slices.Equal(lines, original) {
		t.Errorf("paint changed its input: %q", lines)
	}
	for row := range painted {
		isShaded := painted[row] != lines[row]
		if wantShaded := row >= offset && row < offset+rows; isShaded != wantShaded {
			t.Errorf("row %d shaded = %v, want %v: %q", row, isShaded, wantShaded, painted[row])
		}
	}
}
