package ui

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/index"
)

// entryList is a flat tab's issue list: the issues of the last read, those the tab's filter keeps,
// the cursor over those and the first of them on screen. isRead reports a read taken in, and err
// the error that kept the last read from being taken. It owns what every flat tab does the same
// way: narrowing to a filter, keeping the selection across a read, seating a wanted id, and moving
// the cursor by key, click and wheel. Loading the issues, painting a row and the texts of an empty
// or unread list stay with the tab that embeds it. It is a plain value the value-copied model
// holds inline; its zero value is a list not yet read.
type entryList struct {
	all     []index.Entry
	entries []index.Entry
	err     error
	isRead  bool
	cursor  listCursor
	top     int
}

// selection returns the selected issue, and false when the list keeps none.
func (l entryList) selection() (index.Entry, bool) {
	row := l.cursor.highlight(len(l.entries))
	if row < 0 {
		return index.Entry{}, false
	}
	return l.entries[row], true
}

// rowOf returns the row of the issue id among the issues the filter keeps, -1 when it is not
// among them.
func (l entryList) rowOf(id string) int {
	return slices.IndexFunc(l.entries, func(entry index.Entry) bool { return entry.ID == id })
}

// take takes in a read: its issues, narrowed to filter, and err, the error that kept it from
// being read. The cursor stays on the issue it was on or, when the read dropped that, on its
// replacement among the issues listed before; with neither listed it stays clamped.
func (l *entryList) take(entries []index.Entry, err error, filter sidebarFilter) {
	previous, wasSelected := l.selection()
	previousIDs := make([]string, 0, len(l.entries))
	for _, entry := range l.entries {
		previousIDs = append(previousIDs, entry.ID)
	}
	l.all = entries
	l.entries = keepMatched(entries, entryID, filter)
	l.err = err
	l.isRead = true
	l.cursor.clampSelection(len(l.entries))
	if wasSelected {
		l.keepSelection(previous, previousIDs)
	}
}

// keepSelection seats the cursor, after a read replaced the issues, on previous, the issue it was
// on, or on its replacement among the issues previousIDs listed when the read dropped it. With
// neither listed the cursor stays where it is.
func (l *entryList) keepSelection(previous index.Entry, previousIDs []string) {
	id := previous.ID
	if l.rowOf(id) < 0 {
		id = replacementID(previous.ID, previous.ParentID, previousIDs, func(id string) bool { return l.rowOf(id) >= 0 })
	}
	if row := l.rowOf(id); row >= 0 {
		l.cursor.seat(row, len(l.entries))
	}
}

// seatID puts the cursor on the issue id and reports whether the issues the filter keeps hold it;
// when they do not, the cursor stays where it is.
func (l *entryList) seatID(id string) bool {
	row := l.rowOf(id)
	if row < 0 {
		return false
	}
	l.cursor.seat(row, len(l.entries))
	return true
}

// refilter narrows the issues of the last read to filter, from the top of the list, keeping the
// selection on the same issue when it is still listed.
func (l *entryList) refilter(filter sidebarFilter) {
	selected, isSelected := l.selection()
	l.entries = keepMatched(l.all, entryID, filter)
	l.cursor = listCursor{}
	l.top = 0
	if isSelected {
		l.seatID(selected.ID)
	}
}

// count returns the counts of the tab's label: matched, the issues the filter keeps, and total,
// the issues of the last read. It reports false while no read is in or the last read failed.
func (l entryList) count() (matched int, total int, isCounted bool) {
	if !l.isRead || l.err != nil {
		return 0, 0, false
	}
	return len(l.entries), len(l.all), true
}

// key moves the cursor for an up or down key, stopping at the ends, and reports whether it took
// the key; any other key it leaves untouched.
func (l *entryList) key(msg tea.KeyPressMsg) bool {
	return l.cursor.key(msg, len(l.entries), listStopsAtEnds) == listSwallowed
}

// click seats the cursor on the issue at row, a row of the sidebar counted from its top on
// screen, and reports whether one is there: a list that could not be read, and a row below the
// last issue, select nothing.
func (l *entryList) click(row int) bool {
	position := l.top + row
	if l.err != nil || position >= len(l.entries) {
		return false
	}
	l.cursor.seat(position, len(l.entries))
	return true
}

// wheel moves the cursor one row for a wheel notch, stopping at the ends.
func (l *entryList) wheel(msg tea.MouseWheelMsg) {
	l.cursor.wheel(msg, len(l.entries))
}

// window scrolls the list so the selection stays inside height rows on screen and returns the
// rows to paint, from start up to end, and the selected one.
func (l *entryList) window(height int) (start int, end int, selected int) {
	selected = l.cursor.highlight(len(l.entries))
	l.top = clampInt(l.top, selected-height+1, selected)
	return l.top, min(l.top+height, len(l.entries)), selected
}
