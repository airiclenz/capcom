package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/store"
)

// keyDeps opens and closes the dependency view.
const keyDeps = "d"

// The dependency view's texts, laid out as capcom dep list prints its sections with no flags.
const (
	// depsWaitsOnHeading opens the issues the shown issue depends on, depsBlocksHeading the
	// issues that depend on it.
	depsWaitsOnHeading = "waits on:"
	depsBlocksHeading  = "blocks:"
	// depsRowIndent indents a section's rows under its heading, one outline level.
	depsRowIndent = "  "
	// depsNoneText is the one row of a section with no issue.
	depsNoneText = depsRowIndent + "none"
	// depsNeitherFormat is the pane's one row for an issue with neither deps nor dependents.
	depsNeitherFormat = "%s waits on no issue and blocks none."
	// depsMissingLabel closes the row of a dep that names no issue, as dep list marks it.
	depsMissingLabel = "missing"
	// depsMissingFormat is the notice for Enter on a dep that names no issue.
	depsMissingFormat = "%s is missing"
	// depsFailurePrefix opens the pane's one row when the dependencies could not be read.
	depsFailurePrefix = "deps: "
)

// depsDepth is the depth capcom dep list walks with no --depth: the direct dependents only.
const depsDepth = 1

// depsMsg carries one read of an issue's dependencies for the dependency view: the issue's id,
// the rows of what it waits on and of what it blocks, or the error that kept them from being read.
type depsMsg struct {
	id      string
	waitsOn []depRow
	blocks  []depRow
	err     error
}

// depRow is one row of the dependency view: the issue it names, its id and its entry, or, for a
// dep that names no issue, that dep as the file writes it in missing and an empty id.
type depRow struct {
	id      string
	missing string
	entry   index.Entry
}

// text returns the row's spans: `id  status  P<n>  title`, unaligned, its status painted in
// status, or for a missing dep that dep and depsMissingLabel.
func (r depRow) text(status lipgloss.Style) []span {
	if r.id == "" {
		return []span{{text: oneLine(r.missing) + "  " + depsMissingLabel}}
	}
	return readyRow(r.entry, status, columnWidths{})
}

// depView is the dependency view's state: whether it covers the detail pane, the issue it lists,
// the rows of its last read, the cursor over its waits-on rows followed by its blocks rows, and the
// detail pane's scroll it hid, to restore when it closes.
type depView struct {
	isOpen bool
	// isLoaded reports a read applied; until then the pane keeps what it showed.
	isLoaded   bool
	id         string
	waitsOn    []depRow
	blocks     []depRow
	err        error
	cursor     listCursor
	bodyOffset int
}

// rows returns the view's selectable rows: the waits-on rows, then the blocks rows.
func (v depView) rows() []depRow {
	return append(append([]depRow{}, v.waitsOn...), v.blocks...)
}

// loadDeps returns a command that reads the dependencies of the issue with id through the
// session, from every issue in the index, Closed ones included, as capcom dep list does.
func loadDeps(s *session, id string) tea.Cmd {
	return func() tea.Msg {
		msg := depsMsg{id: id}
		msg.err = s.do(func(_ *store.Store, issueIndex *index.Index) error {
			entries, err := issueIndex.List(index.Filter{IsClosedIncluded: true})
			if err != nil {
				return err
			}
			msg.waitsOn, msg.blocks = depRows(entries, id)
			return nil
		})
		return msg
	}
}

// depRows returns the rows of what the issue with id waits on and what it blocks among entries,
// as capcom dep list with no flags lists them: its deps verbatim in the file's order, a dep naming
// no issue marked missing, then the issues that depend on it in tree order.
func depRows(entries []index.Entry, id string) ([]depRow, []depRow) {
	byID := make(map[string]index.Entry, len(entries))
	for _, entry := range entries {
		byID[entry.ID] = entry
	}
	graph := issue.NewDepGraph(index.Nodes(entries))

	var waitsOn []depRow
	for _, dep := range byID[id].Deps {
		resolved, isFound := graph.Resolve(dep)
		if !isFound {
			waitsOn = append(waitsOn, depRow{missing: dep})
			continue
		}
		waitsOn = append(waitsOn, depRow{id: resolved, entry: byID[resolved]})
	}

	var blocks []depRow
	for _, step := range graph.Walk(id, issue.WalkDependents, depsDepth) {
		blocks = append(blocks, depRow{id: step.ID, entry: byID[step.ID]})
	}
	return waitsOn, blocks
}

// depViewKey handles the keys of the dependency view, ahead of the model's own keys. While the
// help screen is open it takes none. With the view closed, d on the Ready, Issues or Closed tab
// opens it over the selected issue, filling a narrow, short screen. With it open, d and Esc close
// it, Enter follows the selected row, and the up and down arrows and j k move its cursor; a tab
// key closes it and goes on to switch the tab, / closes it and goes on to open the filter line
// where the sidebar is on screen, and any other key the sidebar takes closes it before moving the
// sidebar. It reports false for every key it leaves to the model.
func (m model) depViewKey(msg tea.KeyPressMsg) (model, tea.Cmd, bool) {
	if m.isHelpOpen {
		return m, nil, false
	}
	key := msg.String()
	if !m.deps.isOpen {
		if key != keyDeps {
			return m, nil, false
		}
		return m.openDeps()
	}

	switch key {
	case keyDeps, keyBack:
		return m.closeDeps(), nil, true
	case keyOpen:
		next, cmd := m.followDep()
		return next, cmd, true
	case keyNextTab:
		return m.closeDeps(), nil, false
	}
	if _, isTabKey := tabForKey(key); isTabKey {
		return m.closeDeps(), nil, false
	}
	if key == keyFilter && (!m.isSinglePane() || !m.isDetailOpen) {
		return m.closeDeps(), nil, false
	}
	if m.deps.cursor.key(msg, len(m.deps.rows()), listStopsAtEnds) == listSwallowed {
		return m.paintDeps(), nil, true
	}
	return m.sidebarKeyClosingDeps(msg)
}

// sidebarKeyClosingDeps gives a key the dependency view leaves to the sidebar, such as h on the
// Issues tree, with the view closed first, so the detail pane never lists one issue's dependencies
// beside another's selection. It reports false, with the view still open, for a key the sidebar
// does not take or while the sidebar is off screen.
func (m model) sidebarKeyClosingDeps(msg tea.KeyPressMsg) (model, tea.Cmd, bool) {
	isSidebarShown := !m.isSinglePane() || !m.isDetailOpen
	if !isSidebarShown {
		return m, nil, false
	}
	if next, cmd, isTaken := m.closeDeps().sidebarKey(msg); isTaken {
		return next, cmd, true
	}
	return m, nil, false
}

// openDeps opens the dependency view over the issue the Ready, Issues or Closed sidebar selects,
// clearing the detail selection, and returns the command that reads its dependencies. It takes
// nothing on the Knowledge tab or with nothing selected.
func (m model) openDeps() (model, tea.Cmd, bool) {
	if m.tab == TabKnowledge {
		return m, nil, false
	}
	id, isSelected := m.sidebarSelection()
	if !isSelected {
		return m, nil, false
	}
	m.deps = depView{isOpen: true, id: id, bodyOffset: m.detailOffset}
	m.sel = detailSel{}
	if m.isSinglePane() {
		m.isDetailOpen = true
	}
	return m, loadDeps(m.session, id), true
}

// closeDeps closes the dependency view and puts the shown issue's body back in the detail pane,
// scrolled as it was.
func (m model) closeDeps() model {
	m.detailOffset = m.deps.bodyOffset
	m.deps = depView{}
	m.detail = nil
	return m.renderDetail().scrollDetail(0)
}

// applyDeps takes in a read of an issue's dependencies, unless the view has closed or moved to
// another issue since it was asked for, and paints it with the cursor on its first row; a read
// over one already applied, as a live refresh takes, keeps the cursor and the scroll.
func (m model) applyDeps(msg depsMsg) model {
	if !m.deps.isOpen || m.deps.id != msg.id {
		return m
	}
	wasLoaded := m.deps.isLoaded
	m.deps.isLoaded = true
	m.deps.waitsOn, m.deps.blocks, m.deps.err = msg.waitsOn, msg.blocks, msg.err
	if !wasLoaded {
		m.deps.cursor = listCursor{}
		m.detailOffset = 0
	}
	m.deps.cursor.clampSelection(len(m.deps.rows()))
	return m.paintDeps()
}

// followDep follows the selected row of the dependency view: it closes the view and switches to
// the Issues tab with that issue selected, its ancestors expanded and, when the tree hides it,
// revealed; an issue the Issues filter hides clears that filter first. A dep that names no issue
// says so in the status bar and changes nothing; an issue the tree does not hold is reported there
// as capcom ui issues reports one.
func (m model) followDep() (model, tea.Cmd) {
	rows := m.deps.rows()
	selected := m.deps.cursor.highlight(len(rows))
	if selected < 0 {
		return m, nil
	}
	row := rows[selected]
	if row.id == "" {
		m.notice = fmt.Sprintf(depsMissingFormat, row.missing)
		return m, nil
	}
	return m.revealIssue(row.id)
}

// revealIssue closes the dependency view and switches to the Issues tab with the issue id names
// selected, its ancestors expanded and, when the tree hides it, revealed; an issue the Issues
// filter hides clears that filter first. An issue the tree does not hold is reported in the status
// bar as capcom ui issues reports one.
func (m model) revealIssue(id string) (model, tea.Cmd) {
	m.deps = depView{}
	if m.filters[TabIssues].isActive() && !m.issues.tree.isShown[id] {
		m = m.clearFilter(TabIssues)
	}
	if !m.issues.tree.selectID(id) {
		m.notice = fmt.Sprintf(notInTreeFormat, id)
	}
	return m.openTab(TabIssues)
}

// paintDeps fills the detail pane with the dependency view once its read has applied, scrolled so
// the selected row stays on screen. It does nothing while the view is closed.
func (m model) paintDeps() model {
	if !m.deps.isOpen || !m.deps.isLoaded {
		return m
	}
	lines, selectedLine, _ := m.deps.lines(m.theme)
	m.detail = lines
	if selectedLine >= 0 {
		height := max(m.detailPaneRows(), 1)
		m.detailOffset = clampInt(m.detailOffset, selectedLine-height+1, selectedLine)
	}
	return m.scrollDetail(0)
}

// lines returns the dependency view's rows of text, the position of the selected one, -1 when
// it has none, and, for each line, the position in rows of the row it draws, -1 for a heading or
// a none line: the waits-on heading and rows, then the blocks heading and rows, a section with no
// issue saying none, each status in its category's colour of th and the selected row marked by
// th's markOf. An issue with neither, and a read that failed, each take one row.
func (v depView) lines(th theme) ([]string, int, []int) {
	switch {
	case v.err != nil:
		return []string{depsFailurePrefix + v.err.Error()}, -1, []int{-1}
	case len(v.waitsOn) == 0 && len(v.blocks) == 0:
		return []string{fmt.Sprintf(depsNeitherFormat, v.id)}, -1, []int{-1}
	}
	selected := v.cursor.highlight(len(v.waitsOn) + len(v.blocks))
	lines := make([]string, 0, len(v.waitsOn)+len(v.blocks)+4)
	rowOfLine := make([]int, 0, cap(lines))
	selectedLine := -1
	position := 0
	for _, section := range []struct {
		heading string
		rows    []depRow
	}{{depsWaitsOnHeading, v.waitsOn}, {depsBlocksHeading, v.blocks}} {
		lines = append(lines, section.heading)
		rowOfLine = append(rowOfLine, -1)
		if len(section.rows) == 0 {
			lines = append(lines, depsNoneText)
			rowOfLine = append(rowOfLine, -1)
		}
		for _, row := range section.rows {
			if position == selected {
				selectedLine = len(lines)
			}
			mark := th.markOf(false, position == selected)
			spans := append([]span{{text: depsRowIndent}}, row.text(th.status(row.entry.Category))...)
			lines = append(lines, mark.render(spans...))
			rowOfLine = append(rowOfLine, position)
			position++
		}
	}
	return lines, selectedLine, rowOfLine
}

// rowAtLine returns the position in rows of the row the view draws on line, and -1 when line
// holds a heading, a none line or nothing.
func (v depView) rowAtLine(line int) int {
	// Where a row sits does not depend on its styles, so the zero theme serves.
	_, _, rowOfLine := v.lines(theme{})
	if line < 0 || line >= len(rowOfLine) {
		return -1
	}
	return rowOfLine[line]
}
