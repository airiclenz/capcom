package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/airiclenz/capcom/internal/userconfig"
)

// The lock badge's text colours as SGR parameters: black on a light fill, white on a dark one.
const (
	blackText = "38;2;0;0;0"
	whiteText = "38;2;255;255;255"
)

// badgeWord is the text the lock badge must fill: the word lock with a space either side.
const badgeWord = " lock "

// darkThemeConfig is a user config file that picks the dark preset.
const darkThemeConfig = "ui:\n  theme: dark\n"

// arrangeHeld takes the store's Tree lock for another session.
func arrangeHeld(t *testing.T) func(*Options) {
	return func(options *Options) {
		acquireTestLock(t, &testStore{store: options.Store})
	}
}

// arrangeStale takes the store's Tree lock for another session and ages it three hours.
func arrangeStale(t *testing.T) func(*Options) {
	return func(options *Options) {
		ts := &testStore{store: options.Store}
		acquireTestLock(t, ts)
		ageTestLock(t, ts, 3*time.Hour)
	}
}

// paintedStatusRow returns the last row of the model's view with its ANSI styling: the status bar.
func paintedStatusRow(m model) string {
	rows := strings.Split(m.View().Content, "\n")
	return rows[len(rows)-1]
}

// assertBadge fails the test unless the padded word lock opening row is drawn in an SGR carrying
// fill and text, and the rest of the lock's segment, from the plain space after the word, follows a
// reset.
func assertBadge(t *testing.T, row, fill, text string) {
	t.Helper()
	badge := sgrBefore(t, row, badgeWord)
	for _, want := range []string{fill, text} {
		if !strings.Contains(badge, want) {
			t.Errorf("lock in row %q is drawn in %q, want it to carry %s", row, badge, want)
		}
	}
	assertBadgeEndsAtTheWord(t, row)
}

// assertBadgeEndsAtTheWord fails the test unless the padded word lock in row is followed at once by
// a reset and a plain space, so the badge covers exactly the word, and the row holds no colon right
// after lock.
func assertBadgeEndsAtTheWord(t *testing.T, row string) {
	t.Helper()
	start := strings.Index(row, badgeWord)
	if start < 0 {
		t.Fatalf("row %q lacks %q", row, badgeWord)
	}
	rest := row[start+len(badgeWord):]
	reset := sgrSequence.FindStringSubmatchIndex(rest)
	if reset == nil || reset[0] != 0 {
		t.Fatalf("the badge in row %q runs past %q, want a reset right after it", row, badgeWord)
	}
	if code := rest[reset[2]:reset[3]]; code != "" && code != "0" {
		t.Errorf("%q in row %q is followed by %q, want a reset", badgeWord, row, code)
	}
	if !strings.HasPrefix(rest[reset[1]:], " ") {
		t.Errorf("row %q does not follow the badge with a plain space", row)
	}
	assertNoColonAfterLock(t, row)
}

// assertNoColonAfterLock fails the test if row, stripped of its styling, holds lock with a colon
// directly after it.
func assertNoColonAfterLock(t *testing.T, row string) {
	t.Helper()
	if plain := ansi.Strip(row); strings.Contains(plain, "lock:") {
		t.Errorf("status bar %q holds lock:, want no colon after lock", plain)
	}
}

// assertPlainLock fails the test unless row opens with lockText whole, no SGR sequence before or
// within it.
func assertPlainLock(t *testing.T, row, lockText string) {
	t.Helper()
	end := strings.Index(row, lockText)
	if end < 0 {
		t.Fatalf("row %q lacks %q whole: lock is painted", row, lockText)
	}
	if found := sgrSequence.FindAllString(row[:end+len(lockText)], -1); len(found) > 0 {
		t.Errorf("%q in row %q is drawn in %q, want it plain", lockText, row, found)
	}
	assertNoColonAfterLock(t, row)
}

func TestStatusBar_LockBadge_FillsTheWordByLockState(t *testing.T) {
	// dark's lock-held is #f0883e and its lock-free #43d389, both light fills.
	testCases := []struct {
		name    string
		arrange func(t *testing.T) func(*Options)
		fill    string
	}{
		{name: "held", arrange: arrangeHeld, fill: "48;2;240;136;62"},
		{name: "stale", arrange: arrangeStale, fill: "48;2;240;136;62"},
		{name: "free", arrange: func(*testing.T) func(*Options) { return nil }, fill: "48;2;67;211;137"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newThemeModel(t, nil, darkThemeConfig, false, testCase.arrange(t))

			row := paintedStatusRow(m)

			assertBadge(t, row, testCase.fill, blackText)
		})
	}
}

func TestStatusBar_LockBadge_OverrideChangesTheFill(t *testing.T) {
	configText := darkThemeConfig + "  colors:\n    lock-held: \"#102030\"\n    lock-free: \"#f0f0f0\"\n"
	testCases := []struct {
		name    string
		arrange func(t *testing.T) func(*Options)
		fill    string
		text    string
	}{
		{name: "held", arrange: arrangeHeld, fill: "48;2;16;32;48", text: whiteText},
		{name: "free", arrange: func(*testing.T) func(*Options) { return nil }, fill: "48;2;240;240;240", text: blackText},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newThemeModel(t, nil, configText, false, testCase.arrange(t))

			row := paintedStatusRow(m)

			assertBadge(t, row, testCase.fill, testCase.text)
		})
	}
}

func TestStatusBar_LockBadge_InvalidLockIsPlain(t *testing.T) {
	arrange := func(options *Options) {
		writeTestLockRecord(t, &testStore{store: options.Store}, []byte("not json"))
	}
	m := newThemeModel(t, nil, darkThemeConfig, false, arrange)

	row := paintedStatusRow(m)

	assertPlainLock(t, row, " lock  invalid")
}

func TestStatusBar_LockBadge_ReadingPlaceholderIsPlain(t *testing.T) {
	options := newTestOptions(t, newTestStore(t, nil))
	m := newModel(options)

	m = send(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})

	assertPlainLock(t, paintedStatusRow(m), lockReadingText)
}

func TestStatusBar_LockBadge_NoColorHeldIsReverseFreeIsPlain(t *testing.T) {
	t.Run("held", func(t *testing.T) {
		m := newThemeModel(t, nil, darkThemeConfig, true, arrangeHeld(t))

		row := paintedStatusRow(m)

		if got := sgrBefore(t, row, badgeWord); got != "7" {
			t.Errorf("lock in row %q is drawn in %q, want Reverse (7)", row, got)
		}
		assertBadgeEndsAtTheWord(t, row)
	})
	t.Run("free", func(t *testing.T) {
		m := newThemeModel(t, nil, darkThemeConfig, true, nil)

		row := paintedStatusRow(m)

		assertPlainLock(t, row, " lock  free")
	})
}

func TestStatusBar_LockBadge_NarrowRowCutsBeforeTheHints(t *testing.T) {
	const width = 50
	m := newThemeModel(t, nil, darkThemeConfig, false, arrangeHeld(t))

	m = press(t, m, tea.WindowSizeMsg{Width: width, Height: 24})

	row := paintedStatusRow(m)
	plain := ansi.Strip(row)
	if got := ansi.StringWidth(plain); got > width {
		t.Errorf("status bar %q is %d columns wide, want at most %d", plain, got, width)
	}
	if !strings.HasPrefix(plain, " lock  held by") || !strings.HasSuffix(plain, statusSeparator+hintsPanes) {
		t.Errorf("status bar = %q, want the lock cut before intact hints %q", plain, hintsPanes)
	}
	assertBadge(t, row, "48;2;240;136;62", blackText)
}

// withLayoutButton returns the status row row with the layout button label at its right edge,
// blanks between them, width columns in all.
func withLayoutButton(row, label string, width int) string {
	padding := width - ansi.StringWidth(row) - ansi.StringWidth(label)
	return row + strings.Repeat(" ", padding) + label
}

// arrangeLayoutChoice stores choice as the layout choice in the user state file.
func arrangeLayoutChoice(t *testing.T, choice string) func(*Options) {
	return func(options *Options) {
		writeTestFile(t, options.UserStatePath, layoutState(choice))
	}
}

func TestStatusBar_LayoutButton_FillsTheLastThreeColumns(t *testing.T) {
	cases := []struct {
		name   string
		choice string
		want   string
	}{
		{name: "split", choice: userconfig.LayoutSplit, want: layoutButtonToStacked},
		{name: "stacked", choice: userconfig.LayoutStacked, want: layoutButtonToSplit},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newThemeModel(t, nil, darkThemeConfig, false, arrangeLayoutChoice(t, testCase.choice))

			got := statusRow(m)

			want := withLayoutButton(" lock  free"+statusSeparator+hintsPanes, testCase.want, 120)
			if got != want {
				t.Errorf("status bar = %q, want %q", got, want)
			}
		})
	}
}

func TestStatusBar_LayoutButton_HiddenWhereBothLayoutsDoNotFit(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
	}{
		{name: "89x30", width: 89, height: 30},
		{name: "120x24", width: 120, height: 24},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newTestModel(t, newTestStore(t, readyFiles), testCase.width, testCase.height)

			if got, want := statusRow(m), " lock  free"+statusSeparator+hintsPanes; got != want {
				t.Errorf("status bar = %q, want %q unpadded, no layout button", got, want)
			}
		})
	}
}

func TestStatusBar_LayoutButton_FollowsTheOverlays(t *testing.T) {
	cases := []struct {
		name       string
		open       func(t *testing.T, m model) model
		wantButton bool
	}{
		{name: "help screen", open: func(t *testing.T, m model) model { return press(t, m, runeKey('?')) }},
		{
			name: "confirm prompt",
			open: func(t *testing.T, m model) model {
				m.prompt = newConfirmPrompt("go on?", nil)
				return m
			},
		},
		{name: "text prompt", open: func(t *testing.T, m model) model { return press(t, m, runeKey('L')) }},
		{
			name:       "filter line",
			open:       func(t *testing.T, m model) model { return press(t, m, runeKey('/')) },
			wantButton: true,
		},
		{
			name:       "status picker",
			open:       func(t *testing.T, m model) model { return press(t, m, runeKey('s')) },
			wantButton: true,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := testCase.open(t, newTestModel(t, newTestStore(t, readyFiles), 120, 30))

			got := statusRow(m)

			hasButton := strings.HasSuffix(got, layoutButtonToStacked) && ansi.StringWidth(got) == 120
			if hasButton != testCase.wantButton {
				t.Errorf("status bar = %q: layout button drawn %v, want %v", got, hasButton, testCase.wantButton)
			}
		})
	}
}

func TestStatusBar_LayoutButton_LongNoticeIsCutBeforeTheHints(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), 120, 30)
	m.notice = strings.Repeat("a long notice ", 20)

	got := statusRow(m)

	if width := ansi.StringWidth(got); width != 120 {
		t.Errorf("status bar %q is %d columns wide, want 120", got, width)
	}
	if want := statusSeparator + hintsPanes + layoutButtonToStacked; !strings.HasSuffix(got, want) {
		t.Errorf("status bar = %q, want the notice cut before intact %q", got, want)
	}
}

func TestStatusBar_LayoutButton_PaintedInTheLayoutButtonRole(t *testing.T) {
	cases := []struct {
		name       string
		configText string
		isNoColor  bool
		want       string
	}{
		{name: "dark", configText: darkThemeConfig, want: "48;2;74;74;74"},
		{name: "light", configText: "ui:\n  theme: light\n", want: "48;2;208;215;222"},
		{name: "NO_COLOR", configText: darkThemeConfig, isNoColor: true, want: "7"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			m := newThemeModel(t, nil, testCase.configText, testCase.isNoColor, nil)

			row := paintedStatusRow(m)

			got := sgrBefore(t, row, layoutButtonToStacked)
			if !strings.Contains(got, testCase.want) {
				t.Errorf("layout button in row %q is drawn in %q, want it to carry %s", row, got, testCase.want)
			}
		})
	}
}
