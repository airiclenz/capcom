package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// wideBodyFiles is a store whose Ready queue opens on wide, a body of long lines that fill the
// detail pane's width, each opening with a grapheme the layout and the painter measure apart.
var wideBodyFiles = map[string]string{
	"issues/wide.md": issueFile("title: Wide\nstatus: open\npriority: 0\n", wideBody()),
}

// wideBody returns fillerLines paragraphs, each a ⚠️ and a word too long for any pane, so it is
// hard-wrapped to the pane's full width.
func wideBody() string {
	var body strings.Builder
	for range fillerLines {
		fmt.Fprintf(&body, "⚠️%s\n\n", strings.Repeat("x", 300))
	}
	return body.String()
}

// detailBarCells returns the cell in the last screen column of every detail pane row, and the
// two cells left of it, as the painter measures them.
func detailBarCells(t *testing.T, m model) ([]string, []string) {
	t.Helper()
	rows := strings.Split(ansi.Strip(m.View().Content), "\n")
	bars := make([]string, 0, m.bodyHeight())
	gutters := make([]string, 0, m.bodyHeight())
	for row := bodyTop; row < bodyTop+m.bodyHeight(); row++ {
		measure := m.markdownTheme.Measure
		if got := measure.Width(rows[row]); got != m.width {
			t.Errorf("row %d is %d columns in the painter's measure, want %d: %q", row, got, m.width, rows[row])
		}
		bars = append(bars, measure.Cut(rows[row], m.width-1, m.width))
		gutters = append(gutters, measure.Cut(rows[row], m.width-scrollbarColumns, m.width-1))
	}
	return bars, gutters
}

func TestScrollbarThumb_Geometry(t *testing.T) {
	cases := []struct {
		name         string
		start        int
		window       int
		total        int
		height       int
		wantPosition int
		wantSize     int
	}{
		{name: "lines fit", start: 0, window: 10, total: 10, height: 10, wantPosition: 0, wantSize: 0},
		{name: "fewer lines than rows", start: 0, window: 10, total: 4, height: 10, wantPosition: 0, wantSize: 0},
		{name: "no rows", start: 0, window: 0, total: 40, height: 0, wantPosition: 0, wantSize: 0},
		{name: "flush top at offset 0", start: 0, window: 10, total: 40, height: 10, wantPosition: 0, wantSize: 2},
		{name: "flush bottom at the last offset", start: 30, window: 10, total: 40, height: 10, wantPosition: 8, wantSize: 2},
		{name: "past the last offset stays flush bottom", start: 99, window: 10, total: 40, height: 10, wantPosition: 8, wantSize: 2},
		{name: "minimum one row", start: 0, window: 10, total: 1000, height: 10, wantPosition: 0, wantSize: 1},
		{name: "minimum row flush bottom", start: 990, window: 10, total: 1000, height: 10, wantPosition: 9, wantSize: 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			position, size := scrollbarThumb(testCase.start, testCase.window, testCase.total, testCase.height)

			if position != testCase.wantPosition || size != testCase.wantSize {
				t.Errorf("scrollbarThumb(%d, %d, %d, %d) = %d, %d, want %d, %d",
					testCase.start, testCase.window, testCase.total, testCase.height,
					position, size, testCase.wantPosition, testCase.wantSize)
			}
		})
	}
}

func TestDetailPane_LongBody_PaintsAThumbOnATrackInTheLastColumn(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := openDetail(t, newTestModel(t, newTestStore(t, linkFiles), size[0], size[1]))

			bars, gutters := detailBarCells(t, m)

			counts := map[string]int{}
			for row, bar := range bars {
				counts[bar]++
				if gutters[row] != " " {
					t.Errorf("row %d holds %q in the scrollbar's gutter, want a blank", row, gutters[row])
				}
			}
			if counts[scrollThumbGlyph] == 0 || counts[scrollTrackGlyph] == 0 ||
				counts[scrollThumbGlyph]+counts[scrollTrackGlyph] != len(bars) {
				t.Errorf("the last column holds %v, want only %q and %q, both present", counts, scrollThumbGlyph, scrollTrackGlyph)
			}
			if bars[0] != scrollThumbGlyph {
				t.Errorf("the bar's top cell is %q at offset 0, want the thumb %q", bars[0], scrollThumbGlyph)
			}
		})
	}
}

func TestDetailPane_ShortBody_LeavesTheScrollbarColumnBlank(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := openDetail(t, newTestModel(t, newTestStore(t, readyFiles), size[0], size[1]))

			bars, gutters := detailBarCells(t, m)

			for row := range bars {
				if bars[row] != " " || gutters[row] != " " {
					t.Errorf("row %d holds %q%q in the scrollbar's columns, want blanks", row, gutters[row], bars[row])
				}
			}
		})
	}
}

func TestDetailPane_WideText_StopsShortOfTheScrollbar(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := openDetail(t, newTestModel(t, newTestStore(t, wideBodyFiles), size[0], size[1]))

			bars, gutters := detailBarCells(t, m)

			for row := range bars {
				if gutters[row] != " " {
					t.Errorf("row %d holds %q in the scrollbar's gutter, want a blank", row, gutters[row])
				}
				if bars[row] != scrollThumbGlyph && bars[row] != scrollTrackGlyph {
					t.Errorf("row %d holds %q in the last column, want the bar", row, bars[row])
				}
			}
		})
	}
}

func TestDetailPane_EndOfBody_SeatsTheThumbFlushBottom(t *testing.T) {
	m := newTestModel(t, newTestStore(t, linkFiles), wideWidth, wideHeight)
	m = m.scrollDetail(len(m.detail))

	bars, _ := detailBarCells(t, m)

	if last := bars[len(bars)-1]; last != scrollThumbGlyph {
		t.Errorf("the bar's bottom cell is %q at the last offset, want the thumb %q", last, scrollThumbGlyph)
	}
}
