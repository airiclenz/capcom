package ui

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/outline"
	"github.com/airiclenz/capcom/internal/store"
)

// keyFilter opens the filter line over the sidebar.
const keyFilter = "/"

// The filter's terms, and its texts.
const (
	// statusTermPrefix and labelTermPrefix open the terms that narrow by field; every other word
	// matches title and body.
	statusTermPrefix = "status:"
	labelTermPrefix  = "label:"
	// filterPrompt opens the filter line, filterStatusPrefix the status bar's account of the
	// filter in effect.
	filterPrompt       = "/"
	filterStatusPrefix = "filter: "
	// filterFailurePrefix opens the sidebar's one row when the filter could not be applied.
	filterFailurePrefix = "filter: "
	// unknownStatusFormat is the sidebar's one row for a status term naming no declared status.
	unknownStatusFormat = "status %q is not declared in config.yaml"
	// unknownLabelFormat is the sidebar's one row for a label term no issue or Note carries.
	unknownLabelFormat = "no %s carries label %q"
	// statusOnNotesText is the Knowledge sidebar's one row for a status term.
	statusOnNotesText = "status: applies to issues, not Notes"
	// hintsFilter are the key hints while the filter line is open.
	hintsFilter = "enter keep · esc clear"
)

// noMatchTexts are each tab's one sidebar row when the filter in effect matches nothing, indexed
// by Tab.
var noMatchTexts = [...]string{
	TabReady:     "No ready issue matches the filter.",
	TabIssues:    "No issue matches the filter.",
	TabKnowledge: "No Note matches the filter.",
}

// filterTerms are the terms of one filter line: the statuses and labels it narrows by and the
// words that match title and body. Every term must hold for an issue or Note to match.
type filterTerms struct {
	statuses []string
	labels   []string
	words    []string
}

// parseFilter splits a filter line into its terms at white space: `status:<name>` and
// `label:<name>` name a field, and every other word is a text word.
func parseFilter(text string) filterTerms {
	var terms filterTerms
	for _, word := range strings.Fields(text) {
		if name, isStatus := strings.CutPrefix(word, statusTermPrefix); isStatus {
			terms.statuses = append(terms.statuses, name)
			continue
		}
		if name, isLabel := strings.CutPrefix(word, labelTermPrefix); isLabel {
			terms.labels = append(terms.labels, name)
			continue
		}
		terms.words = append(terms.words, word)
	}
	return terms
}

// sidebarFilter is the filter in effect over one tab's sidebar. Its zero value is no filter.
type sidebarFilter struct {
	// text is the filter line as it was kept; "" when no filter is in effect.
	text string
	// matched holds the ids of the issues or Notes every term holds for, before the tree's
	// visibility rule.
	matched map[string]bool
	// isClosedShown reports a status term naming a status of a closed category: Closed issues
	// then show in the Issues tree with their ancestors.
	isClosedShown bool
	// problem is the sidebar's one row for a term that names no status or label, or a status
	// term on the Knowledge tab; err is the error that kept the filter from being applied. Either
	// leaves matched empty.
	problem string
	err     error
}

// isActive reports a filter in effect.
func (f sidebarFilter) isActive() bool {
	return f.text != ""
}

// filterMsg carries the filter one tab's filter line was kept with, matched against the index.
type filterMsg struct {
	tab    Tab
	filter sidebarFilter
}

// loadFilter returns a command that matches the filter line text against the issues, for the
// Ready and Issues tabs, or the Notes, for the Knowledge tab, through the session.
func loadFilter(s *session, tab Tab, text string) tea.Cmd {
	return func() tea.Msg {
		var filter sidebarFilter
		err := s.do(func(opened *store.Store, issueIndex *index.Index) error {
			var matchErr error
			filter, matchErr = matchFilter(opened.Config, issueIndex, tab, parseFilter(text))
			return matchErr
		})
		if err != nil {
			filter = sidebarFilter{err: err}
		}
		filter.text = text
		return filterMsg{tab: tab, filter: filter}
	}
}

// matchFilter matches terms against the Notes on the Knowledge tab and against the issues on the
// others.
func matchFilter(cfg config.Config, issueIndex *index.Index, tab Tab, terms filterTerms) (sidebarFilter, error) {
	if tab == TabKnowledge {
		return matchNotes(issueIndex, terms)
	}
	return matchIssues(cfg, issueIndex, terms)
}

// matchIssues returns the filter of terms over every issue, Closed ones included: the issues
// whose status is every status term's, that carry every label term's label and, when there are
// text words, that capcom search finds for them. A status cfg does not declare, and a label no
// issue carries, is the filter's problem instead.
func matchIssues(cfg config.Config, issueIndex *index.Index, terms filterTerms) (sidebarFilter, error) {
	var filter sidebarFilter
	for _, name := range terms.statuses {
		category, isDeclared := cfg.CategoryOf(name)
		if !isDeclared {
			filter.problem = fmt.Sprintf(unknownStatusFormat, name)
			return filter, nil
		}
		filter.isClosedShown = filter.isClosedShown || category.IsClosed()
	}
	entries, err := issueIndex.List(index.Filter{IsClosedIncluded: true})
	if err != nil {
		return filter, err
	}
	labelsOf := func(entry index.Entry) []string { return entry.Labels }
	if label, isUnknown := unknownLabel(entries, labelsOf, terms.labels); isUnknown {
		filter.problem = fmt.Sprintf(unknownLabelFormat, "issue", label)
		return filter, nil
	}
	var searched map[string]bool
	if len(terms.words) > 0 {
		hits, err := issueIndex.SearchIssues(strings.Join(terms.words, " "))
		if err != nil {
			return filter, err
		}
		searched = make(map[string]bool, len(hits))
		for _, hit := range hits {
			searched[hit.ID] = true
		}
	}
	filter.matched = map[string]bool{}
	for _, entry := range entries {
		isStatusMatch := !slices.ContainsFunc(terms.statuses, func(name string) bool { return name != entry.Status })
		isTextMatch := searched == nil || searched[entry.ID]
		if isStatusMatch && isTextMatch && hasEvery(entry.Labels, terms.labels) {
			filter.matched[entry.ID] = true
		}
	}
	return filter, nil
}

// matchNotes returns the filter of terms over every Note, retired ones included: the Notes that
// carry every label term's label and, when there are text words, that capcom know search finds
// for them. A status term, and a label no Note carries, is the filter's problem instead.
func matchNotes(issueIndex *index.Index, terms filterTerms) (sidebarFilter, error) {
	var filter sidebarFilter
	if len(terms.statuses) > 0 {
		filter.problem = statusOnNotesText
		return filter, nil
	}
	notes, err := issueIndex.Notes(true)
	if err != nil {
		return filter, err
	}
	labelsOf := func(note index.NoteEntry) []string { return note.Labels }
	if label, isUnknown := unknownLabel(notes, labelsOf, terms.labels); isUnknown {
		filter.problem = fmt.Sprintf(unknownLabelFormat, "Note", label)
		return filter, nil
	}
	var searched map[string]bool
	if len(terms.words) > 0 {
		hits, err := issueIndex.SearchNotes(strings.Join(terms.words, " "), nil, false)
		if err != nil {
			return filter, err
		}
		searched = make(map[string]bool, len(hits))
		for _, hit := range hits {
			searched[hit.ID] = true
		}
	}
	filter.matched = map[string]bool{}
	for _, note := range notes {
		isTextMatch := searched == nil || searched[note.ID]
		if isTextMatch && hasEvery(note.Labels, terms.labels) {
			filter.matched[note.ID] = true
		}
	}
	return filter, nil
}

// unknownLabel returns the first of labels that no entry carries, and false when every one is on
// some entry.
func unknownLabel[E any](entries []E, labelsOf func(E) []string, labels []string) (string, bool) {
	for _, label := range labels {
		isCarried := slices.ContainsFunc(entries, func(entry E) bool { return slices.Contains(labelsOf(entry), label) })
		if !isCarried {
			return label, true
		}
	}
	return "", false
}

// hasEvery reports every one of wanted among labels.
func hasEvery(labels, wanted []string) bool {
	for _, label := range wanted {
		if !slices.Contains(labels, label) {
			return false
		}
	}
	return true
}

// keepMatched returns the entries filter matches, in order; with no filter in effect, every one.
func keepMatched[E any](entries []E, idOf func(E) string, filter sidebarFilter) []E {
	if !filter.isActive() {
		return entries
	}
	kept := make([]E, 0, len(entries))
	for _, entry := range entries {
		if filter.matched[idOf(entry)] {
			kept = append(kept, entry)
		}
	}
	return kept
}

// filteredOutline returns the outline a filter leaves of visible's entries: every matched entry
// visible prints with all its nodes expanded, and that entry's ancestors, grouped and ordered as
// parentOf and compare give. Every entry stays in it, so a row's progress still counts all its
// children.
func filteredOutline[E any](
	visible *outline.Tree[E],
	entries []E,
	idOf func(E) string,
	parentOf func(E) string,
	compare func(left E, right E) int,
	matched map[string]bool,
) *outline.Tree[E] {
	rows := visible.Rows(nil, outline.UnlimitedDepth)
	parents := make(map[string]string, len(rows))
	for _, row := range rows {
		parents[row.ID] = row.ParentID
	}
	kept := map[string]bool{}
	for _, row := range rows {
		if !matched[row.ID] {
			continue
		}
		for id := row.ID; id != "" && !kept[id]; id = parents[id] {
			kept[id] = true
		}
	}
	return outline.NewTree(entries, idOf, parentOf, compare, func(entry E) bool { return kept[idOf(entry)] })
}

// expandAll expands every node the outline shows, keeping the selection on the same node.
func (w *treeWidget[E]) expandAll() {
	selected, _ := w.selection()
	w.expanded = maps.Clone(w.isShown)
	w.rebuild(selected.ID)
}

// openFilter opens the filter line over the open tab's sidebar, holding the filter in effect there,
// when the sidebar is on screen.
func (m model) openFilter() model {
	isSidebarShown := !m.isNarrow() || !m.isDetailOpen
	if !isSidebarShown {
		return m
	}
	m.isFilterOpen = true
	m.filterLine = newLineInput(filterPrompt, m.filters[m.tab].text, m.sidebarPaneWidth())
	return m.paintSidebar()
}

// filterLineKey gives one key to the open filter line. Enter closes it and keeps its text as the
// tab's filter, matched against the index, or clears the filter when the text is blank; Esc
// closes it and clears the filter.
func (m model) filterLineKey(msg tea.KeyPressMsg) (model, tea.Cmd) {
	line, cmd, verdict := m.filterLine.key(msg)
	m.filterLine = line
	switch verdict {
	case lineSubmitted:
		m.isFilterOpen = false
		text := strings.Join(strings.Fields(line.value()), " ")
		if text == "" {
			return m.clearFilter(m.tab).showSelection()
		}
		m.filterAsked[m.tab] = text
		return m.paintSidebar(), loadFilter(m.session, m.tab, text)
	case lineCancelled:
		m.isFilterOpen = false
		return m.clearFilter(m.tab).showSelection()
	}
	return m, cmd
}

// applyFilter takes in a filter matched against the index, unless the tab's filter line has been
// kept with other text or cleared since, rebuilds that tab's sidebar under it and shows the open
// tab's selection. A dependency view opened over that tab since closes first, since the filter
// can move the selection it lists.
func (m model) applyFilter(msg filterMsg) (model, tea.Cmd) {
	if m.filterAsked[msg.tab] != msg.filter.text {
		return m, nil
	}
	if m.deps.isOpen && msg.tab == m.tab {
		m = m.closeDeps()
	}
	return m.withFilter(msg.tab, msg.filter).showSelection()
}

// clearFilter drops tab's filter and rebuilds its sidebar without one.
func (m model) clearFilter(tab Tab) model {
	m.filterAsked[tab] = ""
	if !m.filters[tab].isActive() {
		return m
	}
	return m.withFilter(tab, sidebarFilter{})
}

// withFilter puts filter in effect over tab and rebuilds the tab's rows under it, keeping the
// selection on the same issue or Note when it is still listed. A tree a new filter applies to
// opens fully expanded; one the filter leaves gets back the nodes expanded before the first
// filter.
func (m model) withFilter(tab Tab, filter sidebarFilter) model {
	wasActive := m.filters[tab].isActive()
	m.filters[tab] = filter
	switch tab {
	case TabReady:
		return m.refilterReady()
	case TabIssues:
		m.issues.tree, m.issues.unfilteredExpanded = refilterTree(
			m.issues.tree, m.issues.unfilteredExpanded, wasActive, filter.isActive(), m.issuesTree,
		)
	case TabKnowledge:
		m.knowledge.tree, m.knowledge.unfilteredExpanded = refilterTree(
			m.knowledge.tree, m.knowledge.unfilteredExpanded, wasActive, filter.isActive(), m.knowledgeTree,
		)
	}
	return m
}

// refilterTree rebuilds tree with build after its tab's filter changed, and returns it with the
// expanded nodes to restore once no filter is in effect. A first filter saves tree's expanded
// nodes; a filter in effect expands the whole rebuilt tree; dropping the filter restores the saved
// nodes. The selection stays on the same node when the rebuilt tree shows it, or, with no filter
// left, can reveal it.
func refilterTree[E any](
	tree treeWidget[E],
	saved map[string]bool,
	wasActive bool,
	isActive bool,
	build func(expanded map[string]bool) treeWidget[E],
) (treeWidget[E], map[string]bool) {
	selected, isSelected := tree.selection()
	if !wasActive {
		saved = tree.expanded
	}
	if !isActive {
		rebuilt := build(saved)
		if isSelected {
			rebuilt.selectID(selected.ID)
		}
		return rebuilt, nil
	}
	rebuilt := build(nil)
	rebuilt.expandAll()
	if isSelected && rebuilt.isShown[selected.ID] {
		rebuilt.selectID(selected.ID)
	}
	return rebuilt, saved
}

// refilterReady narrows the Ready queue of the last read to the Ready filter, keeping the
// selection on the same issue when it is still listed.
func (m model) refilterReady() model {
	selected, isSelected := m.readySelection()
	m.ready.entries = keepMatched(m.ready.all, entryID, m.filters[TabReady])
	m.ready.cursor = listCursor{}
	m.ready.top = 0
	if !isSelected {
		return m
	}
	if row := slices.IndexFunc(m.ready.entries, func(entry index.Entry) bool { return entry.ID == selected.ID }); row >= 0 {
		m.ready.cursor.seat(row, len(m.ready.entries))
	}
	return m
}

// issuesTree returns the Issues tree over the issues of the last read, with expanded expanded:
// the outline capcom list --tree prints or, with a filter in effect, what the filter leaves of
// it, Closed issues included when a status term names a closed status.
func (m model) issuesTree(expanded map[string]bool) treeWidget[index.Entry] {
	entries := m.issues.entries
	filter := m.filters[TabIssues]
	tree := outline.Issues(entries, false)
	if filter.isActive() {
		tree = filteredOutline(outline.Issues(entries, filter.isClosedShown), entries, entryID, entryParentID, nil, filter.matched)
	}
	return newTreeWidget(tree, entryID, expanded)
}

// knowledgeTree returns the Knowledge tree over the Notes of the last read, with expanded
// expanded: the outline capcom know tree prints or, with a filter in effect, what the filter
// leaves of it.
func (m model) knowledgeTree(expanded map[string]bool) treeWidget[index.NoteEntry] {
	notes := m.knowledge.notes
	filter := m.filters[TabKnowledge]
	tree := noteOutline(notes)
	if filter.isActive() {
		tree = filteredOutline(tree, notes, noteID, noteParentID, compareNoteIDs, filter.matched)
	}
	return newTreeWidget(tree, noteID, expanded)
}

// emptySidebarText returns the one row of tab's sidebar when it lists nothing: the filter's
// failure or problem, or that it matches nothing, with a filter in effect, and emptyText without
// one.
func (m model) emptySidebarText(tab Tab, emptyText string) string {
	filter := m.filters[tab]
	switch {
	case !filter.isActive():
		return emptyText
	case filter.err != nil:
		return filterFailurePrefix + filter.err.Error()
	case filter.problem != "":
		return filter.problem
	}
	return noMatchTexts[tab]
}

// sidebarHeight returns the rows the sidebar's list gets: the body's, less the filter line's row
// while it is open.
func (m model) sidebarHeight() int {
	if m.isFilterOpen {
		return max(m.bodyHeight()-1, 0)
	}
	return m.bodyHeight()
}

// sidebarPaneWidth returns the columns of the sidebar pane: the whole screen on a narrow
// terminal, its share of it otherwise.
func (m model) sidebarPaneWidth() int {
	if m.isNarrow() {
		return m.width
	}
	return m.sidebarWidth()
}

// sidebarLines returns the rows the sidebar pane draws: its list, under the filter line while it
// is open.
func (m model) sidebarLines() []string {
	if !m.isFilterOpen {
		return m.sidebar
	}
	return append([]string{m.filterLine.view()}, m.sidebar...)
}

// filterStatus returns the status bar's account of the open tab's filter, `filter: <text>`, and
// "" when none is in effect.
func (m model) filterStatus() string {
	filter := m.filters[m.tab]
	if !filter.isActive() {
		return ""
	}
	return filterStatusPrefix + filter.text
}

// entryID returns the id of an issue.
func entryID(entry index.Entry) string {
	return entry.ID
}

// entryParentID returns the id of an issue's parent, "" at the top level.
func entryParentID(entry index.Entry) string {
	return entry.ParentID
}

// noteID returns the id of a Note.
func noteID(note index.NoteEntry) string {
	return note.ID
}

// noteParentID returns the id of a Note's parent, "" at the top level.
func noteParentID(note index.NoteEntry) string {
	return note.ParentID
}

// compareNoteIDs orders Notes by id, as capcom know tree does.
func compareNoteIDs(left, right index.NoteEntry) int {
	return cmp.Compare(left.ID, right.ID)
}
