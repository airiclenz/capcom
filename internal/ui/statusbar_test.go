package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
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
