package ui

import (
	tea "charm.land/bubbletea/v2"
)

// listVerdict is what a listCursor did with one key press, and the whole of what it tells the
// sidebar that owns it.
type listVerdict int

// The verdicts of listCursor.key.
const (
	// listSwallowed: the cursor took the key, moving the selection or, at an end, deliberately not.
	listSwallowed listVerdict = iota
	// listUnclaimed: none of the cursor's keys; the model's own keys get it.
	listUnclaimed
)

// listWrap says what up on the first row and down on the last one do: stop there, or wrap round
// to the other end.
type listWrap bool

// The two answers to what a move past an end does.
const (
	// listStopsAtEnds keeps down on the last row on it.
	listStopsAtEnds listWrap = false
	// listWrapsAround takes down on the last row to the first.
	listWrapsAround listWrap = true
)

// listCursor is where a sidebar list's selection stands: an index into the rows the sidebar
// paints, clamped rather than trusted, since the rows can change under it. It is a plain value the
// value-copied model holds inline; its zero value selects the first row.
type listCursor struct {
	selected int
}

// clampSelection keeps the selection inside a list of n rows. An empty list pins it at zero, and
// highlight marks no row for it.
func (l *listCursor) clampSelection(n int) {
	switch {
	case n == 0:
		l.selected = 0
	case l.selected >= n:
		l.selected = n - 1
	case l.selected < 0:
		l.selected = 0
	}
}

// move walks the selection delta rows through a list of n by wrap's answer at the ends. An empty
// list moves nowhere. The caller has already clamped, so a wrapping move is the modulo of a seated
// index and a stopping one cannot leave the range.
func (l *listCursor) move(delta, n int, wrap listWrap) {
	if n == 0 {
		return
	}
	next := l.selected + delta
	if wrap == listWrapsAround {
		l.selected = ((next % n) + n) % n
		return
	}
	l.selected = clampInt(next, 0, n-1)
}

// highlight returns the row of a list of n that the sidebar marks: the clamped selection, or -1
// when the list is empty and there is nothing to mark.
func (l listCursor) highlight(n int) int {
	if n == 0 {
		return -1
	}
	return clampInt(l.selected, 0, n-1)
}

// seat puts the selection on row, clamped into a list of count rows: the one way anything but a
// key moves the cursor, such as an id named on the command line or a click. An empty list seats
// nothing.
func (l *listCursor) seat(row, count int) {
	if count == 0 {
		return
	}
	l.selected = clampInt(row, 0, count-1)
}

// key routes one key press through the cursor over a list of n rows and reports what it did: up
// and k move the selection a row back, down and j a row on, by wrap's answer at the ends, and
// every other key is listUnclaimed. The selection is clamped first, since the rows can have
// changed since the last key.
func (l *listCursor) key(msg tea.KeyPressMsg, n int, wrap listWrap) listVerdict {
	l.clampSelection(n)
	switch msg.String() {
	case keyUp, keyUpVim:
		l.move(-1, n, wrap)
		return listSwallowed
	case keyDown, keyDownVim:
		l.move(1, n, wrap)
		return listSwallowed
	}
	return listUnclaimed
}

// wheel walks the cursor one row per wheel notch over a list of n rows: tea.MouseWheelUp a row
// back, tea.MouseWheelDown a row on, any other button nothing. It stops at the ends whatever the
// keys do: a wheel is a scroll, and rolling past the last row onto the first would land somewhere
// the user did not aim.
func (l *listCursor) wheel(msg tea.MouseWheelMsg, n int) {
	if n < 1 {
		return
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		l.move(-1, n, listStopsAtEnds)
	case tea.MouseWheelDown:
		l.move(1, n, listStopsAtEnds)
	}
}

// clampInt returns n held within lo and hi.
func clampInt(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
