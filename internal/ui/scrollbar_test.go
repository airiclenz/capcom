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

// queueLength is how many issues longQueueFiles holds, more than the sidebar shows at either size.
const queueLength = 40

// longQueueFiles returns a store of queueLength ready issues, q00 to q39, and no Note.
func longQueueFiles() map[string]string {
	files := make(map[string]string, queueLength)
	for n := range queueLength {
		files[fmt.Sprintf("issues/q%02d.md", n)] = issueFile(
			fmt.Sprintf("title: Queue %02d\nstatus: open\npriority: 2\n", n), "A queued issue.\n")
	}
	return files
}

// sidebarBarCells returns the cell in the sidebar pane's last column on every body row, and the
// cell left of it, as the painter measures them.
func sidebarBarCells(t *testing.T, m model) ([]string, []string) {
	t.Helper()
	paneWidth := m.sidebarPaneWidth() + scrollbarColumns
	rows := strings.Split(ansi.Strip(m.View().Content), "\n")
	measure := m.markdownTheme.Measure
	bars := make([]string, 0, m.bodyHeight())
	gutters := make([]string, 0, m.bodyHeight())
	for row := bodyTop; row < bodyTop+m.bodyHeight(); row++ {
		bars = append(bars, measure.Cut(rows[row], paneWidth-1, paneWidth))
		gutters = append(gutters, measure.Cut(rows[row], paneWidth-scrollbarColumns, paneWidth-1))
	}
	return bars, gutters
}

// assertSidebarBar fails the test unless every cell of bars is the thumb or the track, both
// present, and every cell of gutters is blank.
func assertSidebarBar(t *testing.T, bars, gutters []string) {
	t.Helper()
	counts := map[string]int{}
	for row, bar := range bars {
		counts[bar]++
		if gutters[row] != " " {
			t.Errorf("row %d holds %q in the sidebar's gutter, want a blank", row, gutters[row])
		}
	}
	if counts[scrollThumbGlyph] == 0 || counts[scrollTrackGlyph] == 0 ||
		counts[scrollThumbGlyph]+counts[scrollTrackGlyph] != len(bars) {
		t.Errorf("the sidebar's last column holds %v, want only %q and %q, both present",
			counts, scrollThumbGlyph, scrollTrackGlyph)
	}
}

// assertBlankBar fails the test unless every cell of bars and gutters is blank.
func assertBlankBar(t *testing.T, bars, gutters []string) {
	t.Helper()
	for row := range bars {
		if bars[row] != " " || gutters[row] != " " {
			t.Errorf("row %d holds %q%q in the sidebar's scrollbar columns, want blanks", row, gutters[row], bars[row])
		}
	}
}

func TestSidebar_LongReadyList_PaintsAThumbFlushBottomAtTheLastEntry(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := newTestModel(t, newTestStore(t, longQueueFiles()), size[0], size[1])
			bars, gutters := sidebarBarCells(t, m)
			assertSidebarBar(t, bars, gutters)
			if bars[0] != scrollThumbGlyph {
				t.Errorf("the bar's top cell is %q on the first entry, want the thumb %q", bars[0], scrollThumbGlyph)
			}

			for range queueLength {
				m = press(t, m, runeKey('j'))
			}
			bars, gutters = sidebarBarCells(t, m)

			assertSidebarBar(t, bars, gutters)
			if last := bars[len(bars)-1]; last != scrollThumbGlyph {
				t.Errorf("the bar's bottom cell is %q on the last entry, want the thumb %q", last, scrollThumbGlyph)
			}
		})
	}
}

func TestSidebar_ShortReadyList_LeavesTheScrollbarColumnBlank(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := newTestModel(t, newTestStore(t, readyFiles), size[0], size[1])

			bars, gutters := sidebarBarCells(t, m)

			assertBlankBar(t, bars, gutters)
		})
	}
}

func TestSidebar_FilterOpen_LeavesTheFilterRowsBarCellBlank(t *testing.T) {
	m := newTestModel(t, newTestStore(t, longQueueFiles()), wideWidth, wideHeight)
	m = press(t, m, runeKey('/'))

	bars, gutters := sidebarBarCells(t, m)

	assertBlankBar(t, bars[:1], gutters[:1])
	assertSidebarBar(t, bars[1:], gutters[1:])
	if bars[1] != scrollThumbGlyph {
		t.Errorf("the bar's top cell under the filter line is %q, want the thumb %q", bars[1], scrollThumbGlyph)
	}
}

func TestSidebar_LongTree_PaintsAThumbOnATrack(t *testing.T) {
	for _, size := range [][2]int{{wideWidth, wideHeight}, {smallWidth, smallHeight}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := newIssuesModel(t, newTestStore(t, longQueueFiles()), "q00", size[0], size[1])

			bars, gutters := sidebarBarCells(t, m)

			assertSidebarBar(t, bars, gutters)
		})
	}
}

func TestSidebar_ShortTree_LeavesTheScrollbarColumnBlank(t *testing.T) {
	m := newIssuesModel(t, newTestStore(t, treeFiles), "solo", wideWidth, wideHeight)

	bars, gutters := sidebarBarCells(t, m)

	assertBlankBar(t, bars, gutters)
}

func TestSidebar_SwitchFromALongListToAnEmptyTab_PaintsNoThumb(t *testing.T) {
	m := newTestModel(t, newTestStore(t, longQueueFiles()), wideWidth, wideHeight)
	for range queueLength {
		m = press(t, m, runeKey('j'))
	}

	m = press(t, m, runeKey('3'))

	if m.tab != TabKnowledge {
		t.Fatalf("tab = %v, want Knowledge", m.tab)
	}
	bars, gutters := sidebarBarCells(t, m)
	assertBlankBar(t, bars, gutters)
}
