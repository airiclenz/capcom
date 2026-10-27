// Package outline computes the issue and Note outlines as data: which entries an outline prints,
// in what order, at what depth, and where a depth limit cut them. Both front ends render from it,
// `capcom list --tree` and `capcom know tree` in internal/cli and the trees of `capcom ui`, so
// the visibility rules live in one place. Rendering is the caller's.
package outline

import (
	"cmp"
	"slices"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/knowledge"
)

// UnlimitedDepth is the maxDepth of Tree.Rows that walks every level.
const UnlimitedDepth = 0

// Row is one line of an outline: the entry, its id, the id of its parent in the outline ("" at the
// top level), its depth below the start of the outline, whether it has shown children and, when
// maxDepth cut those children, how many it cut (More) and the depth that would show them
// (MoreDepth). More and MoreDepth are 0 on a row that cut nothing. IsContext is true on a row of
// a Tree that Matching narrowed when the row's id is not among the matched ids: the row prints only
// as an ancestor of a match, or as the start of the outline. A Tree NewTree built has no context
// rows.
type Row[E any] struct {
	Entry       E
	ID          string
	ParentID    string
	Depth       int
	HasChildren bool
	More        int
	MoreDepth   int
	IsContext   bool
}

// Tree is a hierarchy of entries grouped under their parents, with the visibility rule of the
// outline that prints it. Grouping covers every entry, hidden ones included, so a hidden entry's
// children stay in its group and are hidden with it, never promoted to the top level. matched is
// the set of ids a Tree that Matching narrowed was narrowed to, nil on a Tree NewTree built.
type Tree[E any] struct {
	children map[string][]E
	parentOf map[string]string
	idOf     func(E) string
	isShown  func(E) bool
	matched  map[string]bool
}

// NewTree groups entries by the id parentOf gives, under "" for the top level. An entry whose
// parent is not among entries (its file unusable or left out) joins the top level. Each group is
// in the order compare gives, or in the order of entries when compare is nil. isShown decides
// which entries the outline prints; nil shows every entry.
func NewTree[E any](
	entries []E,
	idOf func(E) string,
	parentOf func(E) string,
	compare func(left E, right E) int,
	isShown func(E) bool,
) *Tree[E] {
	isListed := make(map[string]bool, len(entries))
	for _, entry := range entries {
		isListed[idOf(entry)] = true
	}
	children := map[string][]E{}
	parents := make(map[string]string, len(entries))
	for _, entry := range entries {
		parentID := parentOf(entry)
		if !isListed[parentID] {
			parentID = ""
		}
		children[parentID] = append(children[parentID], entry)
		parents[idOf(entry)] = parentID
	}
	if compare != nil {
		for _, group := range children {
			slices.SortStableFunc(group, compare)
		}
	}
	return &Tree[E]{children: children, parentOf: parents, idOf: idOf, isShown: isShown}
}

// Issues returns the outline of `capcom list --tree`: every issue of entries under its parent, in
// the order of entries. Closed issues and their subtrees are hidden unless isAll; Children still
// returns them, so they count in their parent's progress.
func Issues(entries []index.Entry, isAll bool) *Tree[index.Entry] {
	return NewTree(
		entries,
		func(entry index.Entry) string { return entry.ID },
		func(entry index.Entry) string { return entry.ParentID },
		nil,
		func(entry index.Entry) bool { return isAll || !entry.Category.IsClosed() },
	)
}

// Notes returns the outline of `capcom know tree`: every Note of notes under its parent, each group
// in id order. Background Notes and their subtrees are hidden unless isBackgroundIncluded; which
// retired Notes take part is the caller's choice of notes.
func Notes(notes []index.NoteEntry, isBackgroundIncluded bool) *Tree[index.NoteEntry] {
	return NewTree(
		notes,
		func(entry index.NoteEntry) string { return entry.ID },
		func(entry index.NoteEntry) string { return entry.ParentID },
		func(left, right index.NoteEntry) int { return cmp.Compare(left.ID, right.ID) },
		func(entry index.NoteEntry) bool {
			return isBackgroundIncluded || !knowledge.IsBackground(entry.Labels)
		},
	)
}

// Matching returns the outline t narrowed to matched: every entry t shows whose id is in matched,
// and that entry's ancestors, grouped and ordered as in t. Only an entry t shows can match; a
// matched id t hides, by itself or through an ancestor, keeps nothing. The narrowed Tree marks a
// row IsContext when its id is not in matched. Children still returns every child, hidden ones
// included, so a row's progress counts all its children; a depth cut counts the kept ones only.
func (t *Tree[E]) Matching(matched map[string]bool) *Tree[E] {
	rows := t.Rows(nil, UnlimitedDepth)
	kept := make(map[string]bool, len(rows))
	for _, row := range rows {
		if !matched[row.ID] {
			continue
		}
		for id := row.ID; id != "" && !kept[id]; id = t.parentOf[id] {
			kept[id] = true
		}
	}
	return &Tree[E]{
		children: t.children,
		parentOf: t.parentOf,
		idOf:     t.idOf,
		isShown:  func(entry E) bool { return kept[t.idOf(entry)] },
		matched:  matchedSet(matched),
	}
}

// matchedSet returns a copy of matched that is never nil, so a Tree narrowed to no ids still marks
// its start row as context.
func matchedSet(matched map[string]bool) map[string]bool {
	set := make(map[string]bool, len(matched))
	for id, isMatched := range matched {
		if isMatched {
			set[id] = true
		}
	}
	return set
}

// Children returns every direct child of parentID, hidden ones included, in group order.
func (t *Tree[E]) Children(parentID string) []E {
	return t.children[parentID]
}

// Shown returns the direct children of parentID the outline prints, in group order.
func (t *Tree[E]) Shown(parentID string) []E {
	if t.isShown == nil {
		return t.children[parentID]
	}
	var shown []E
	for _, child := range t.children[parentID] {
		if t.isShown(child) {
			shown = append(shown, child)
		}
	}
	return shown
}

// Rows returns the outline, cut maxDepth levels below its start (UnlimitedDepth for none). With a
// nil start the outline is every shown top-level entry and its shown subtree, each at its depth
// below the top. Otherwise start comes first at depth 0, cutting nothing, whether or not it is
// shown, then its shown subtree one depth deeper.
func (t *Tree[E]) Rows(start *E, maxDepth int) []Row[E] {
	var rows []Row[E]
	add := func(entry E, depth int, more int, moreDepth int) {
		id := t.idOf(entry)
		rows = append(rows, Row[E]{
			Entry:       entry,
			ID:          id,
			ParentID:    t.parentOf[id],
			Depth:       depth,
			HasChildren: len(t.Shown(id)) > 0,
			More:        more,
			MoreDepth:   moreDepth,
			IsContext:   t.matched != nil && !t.matched[id],
		})
	}
	if start == nil {
		walkOutline(t.Shown, t.idOf, "", 1, 0, maxDepth, add)
		return rows
	}
	add(*start, 0, 0, 0)
	walkOutline(t.Shown, t.idOf, t.idOf(*start), 1, 1, maxDepth, add)
	return rows
}

// walkOutline walks the outline below parentID, whose shown children sit level levels below the
// start of the outline: it calls add for each shown child at indent, in the order shownChildren
// gives, then walks that child's subtree one level and one indent deeper. On the last level
// maxDepth allows (every level when maxDepth is UnlimitedDepth) it descends no further: add gets
// the number of the child's shown children it cut (more) and the depth that would show them
// (moreDepth). Elsewhere add gets 0 for both. The caller adds the start of the outline itself.
func walkOutline[E any](
	shownChildren func(parentID string) []E,
	idOf func(E) string,
	parentID string,
	level int,
	indent int,
	maxDepth int,
	add func(entry E, indent int, more int, moreDepth int),
) {
	isLastLevel := maxDepth != UnlimitedDepth && level >= maxDepth
	for _, child := range shownChildren(parentID) {
		if isLastLevel {
			add(child, indent, len(shownChildren(idOf(child))), level+1)
			continue
		}
		add(child, indent, 0, 0)
		walkOutline(shownChildren, idOf, idOf(child), level+1, indent+1, maxDepth, add)
	}
}
