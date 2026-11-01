package stage

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/ops"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// issueExtension is the extension of an issue file.
const issueExtension = ".md"

// Catalog returns the ops.Catalog of the issues as the Overlay holds them, over x, the index of the
// Overlay's store as the disk holds it: every issue file staged in the issue tree stands in place of
// the index's entry for it, and every staged create is an issue the index does not know yet. It
// reads the Overlay on every call, so it sees each write staged before it; x is read, never
// refreshed. ops.Staged over the Overlay, its RaiseFormat and this Catalog runs the writes as they
// run on disk, with nothing written until Plan's tree.Plan is applied.
func (o *Overlay) Catalog(x *index.Index) ops.Catalog {
	return catalog{overlay: o, index: x}
}

// catalog is the ops.Catalog Overlay.Catalog returns.
type catalog struct {
	overlay *Overlay
	index   *index.Index
}

// stagedIssue is one issue file the Overlay stages, as the index would store it.
type stagedIssue struct {
	entry index.Entry
	// aliases are the Aliases the file records, as nodestore.Reading.Aliases reads them: the
	// node's when it parses, and those still recovered from its raw frontmatter when it does not.
	aliases []string
	// isNew says whether a staged create made the file, so the index does not know it.
	isNew bool
	// isUsable says whether the file is usable, as the index keeps only such a file.
	isUsable bool
}

// resolved returns the issue ref resolved to, or, for a file that is not usable, a
// *tree.UnusableError quoting ref and naming the file, as the index refuses one.
func (i stagedIssue) resolved(ref string) (index.Entry, error) {
	if !i.isUsable {
		return index.Entry{}, &tree.UnusableError{Arg: ref, Path: i.entry.Path}
	}
	return i.entry, nil
}

// Resolve finds the usable issue ref names, as index.Index.Resolve does, among the issues as the
// Overlay holds them. A staged issue whose id, or else one of whose Aliases, is exactly the name ref
// stands for (tree.NormalizeArg) is the match; otherwise the index resolves ref, its refusals kept,
// and an issue it finds that is staged is the staged one. Where the index matched ref by an Alias
// that the staged file of its issue no longer records, ref resolves as the index will once the
// stage is applied: as an id prefix alone (index.Index.ResolvePrefix). Staged creates count as
// prefix candidates: beside the one id the index matched by prefix, or beside the several of its
// *tree.AmbiguousError, they make ref a *tree.AmbiguousError naming every candidate, and where
// the index found nothing (a *tree.NotFoundError) the one staged create ref is a prefix of is the
// match.
func (c catalog) Resolve(ref string) (index.Entry, error) {
	staged := c.staged()
	name := tree.NormalizeArg(ref)
	if match, isMatched := exactMatch(staged, name); isMatched {
		return match.resolved(ref)
	}
	entry, err := c.index.Resolve(ref)
	if err == nil && isAliasDropped(staged, entry, name) {
		entry, err = c.index.ResolvePrefix(ref)
	}
	return mergeStaged(ref, name, staged, entry, err)
}

// isAliasDropped reports whether entry, the index's match for name, is an issue the index matched
// by its Alias name, and whose staged file no longer records that Alias.
func isAliasDropped(staged []stagedIssue, entry index.Entry, name string) bool {
	if entry.ID == name || !slices.Contains(entry.Aliases, name) {
		return false
	}
	match, isStaged := stagedByID(staged, entry.ID)
	return isStaged && !slices.Contains(match.aliases, name)
}

// mergeStaged returns what ref, whose name is name, resolves to among the staged issues, given
// entry and err, what the index resolved it to: the staged issue in place of the one the index
// found, and the staged creates name is a prefix of beside the index's prefix candidates.
func mergeStaged(
	ref string, name string, staged []stagedIssue, entry index.Entry, err error,
) (index.Entry, error) {
	created := createdPrefixMatches(staged, name)
	var notFound *tree.NotFoundError
	var ambiguous *tree.AmbiguousError
	switch {
	case err == nil:
		isPrefix := entry.ID != name && !slices.Contains(entry.Aliases, name)
		if isPrefix && len(created) > 0 {
			return index.Entry{}, ambiguousError(ref, []string{entry.ID}, created)
		}
		if match, isStaged := stagedByID(staged, entry.ID); isStaged {
			return match.resolved(ref)
		}
		return entry, nil
	case errors.As(err, &notFound):
		switch len(created) {
		case 0:
			return index.Entry{}, err
		case 1:
			return created[0].resolved(ref)
		default:
			return index.Entry{}, ambiguousError(ref, nil, created)
		}
	case errors.As(err, &ambiguous) && len(created) > 0:
		return index.Entry{}, ambiguousError(ref, ambiguous.Candidates, created)
	default:
		return index.Entry{}, err
	}
}

// List returns the usable issues filter selects, as index.Index.List does, among the issues as the
// Overlay holds them: the index's entries with every staged file's entry in place of the index's,
// and the staged creates added, each staged entry kept only when filter selects it, all in tree
// order (by path, byte by byte, as the index orders them) and capped at filter.Limit.
func (c catalog) List(filter index.Filter) ([]index.Entry, error) {
	unlimited := filter
	unlimited.Limit = 0
	entries, err := c.index.List(unlimited)
	if err != nil {
		return nil, err
	}
	stagedPaths := map[string]bool{}
	var merged []index.Entry
	for _, staged := range c.staged() {
		stagedPaths[staged.entry.Path] = true
		if staged.isUsable && isSelected(filter, staged.entry) {
			merged = append(merged, staged.entry)
		}
	}
	for _, entry := range entries {
		if !stagedPaths[entry.Path] {
			merged = append(merged, entry)
		}
	}
	slices.SortFunc(merged, func(a index.Entry, b index.Entry) int {
		return strings.Compare(a.Path, b.Path)
	})
	if filter.Limit > 0 && len(merged) > filter.Limit {
		merged = merged[:filter.Limit]
	}
	return merged, nil
}

// staged returns every issue file the Overlay stages in the issue tree, in the order each was first
// written, decoded as the index decodes it: its id from its name, its parent from its directory.
func (c catalog) staged() []stagedIssue {
	s := c.overlay.store
	issuesDir := filepath.Clean(s.IssuesDir())
	reader := nodestore.IssueReader(s.Config)
	var issues []stagedIssue
	for _, path := range c.overlay.order {
		if !isIssueFile(issuesDir, path) {
			continue
		}
		file := c.overlay.files[path]
		relPath, _ := store.RelativePath(s.WorktreeRoot, path)
		reading := reader.Decode(relPath, file.data)
		parentID := ""
		if dir := filepath.Dir(path); dir != issuesDir {
			parentID = tree.IDOf(dir)
		}
		issues = append(issues, stagedIssue{
			entry:    index.EntryOf(relPath, tree.IDOf(path), parentID, reading.Node, s.Config),
			aliases:  reading.Aliases,
			isNew:    file.isNew,
			isUsable: reading.IsUsable(),
		})
	}
	return issues
}

// isIssueFile reports whether the clean absolute path is a node file inside issuesDir.
func isIssueFile(issuesDir string, path string) bool {
	if !strings.HasSuffix(path, issueExtension) {
		return false
	}
	relative, err := filepath.Rel(issuesDir, path)
	return err == nil && filepath.IsLocal(relative)
}

// exactMatch returns the staged issue whose id is name, or else the first one with name among its
// Aliases.
func exactMatch(staged []stagedIssue, name string) (stagedIssue, bool) {
	if match, isStaged := stagedByID(staged, name); isStaged {
		return match, true
	}
	for _, candidate := range staged {
		if slices.Contains(candidate.entry.Aliases, name) {
			return candidate, true
		}
	}
	return stagedIssue{}, false
}

// stagedByID returns the staged issue whose id is id.
func stagedByID(staged []stagedIssue, id string) (stagedIssue, bool) {
	for _, candidate := range staged {
		if candidate.entry.ID == id {
			return candidate, true
		}
	}
	return stagedIssue{}, false
}

// createdPrefixMatches returns the staged creates whose id name is a prefix of; an empty name has
// none.
func createdPrefixMatches(staged []stagedIssue, name string) []stagedIssue {
	if name == "" {
		return nil
	}
	var matches []stagedIssue
	for _, candidate := range staged {
		if candidate.isNew && strings.HasPrefix(candidate.entry.ID, name) {
			matches = append(matches, candidate)
		}
	}
	return matches
}

// ambiguousError returns the *tree.AmbiguousError of ref matching the index's candidates and the
// ids of the staged creates, each once and in order.
func ambiguousError(ref string, candidates []string, created []stagedIssue) *tree.AmbiguousError {
	merged := slices.Clone(candidates)
	for _, issue := range created {
		merged = append(merged, issue.entry.ID)
	}
	slices.Sort(merged)
	return &tree.AmbiguousError{Arg: ref, Candidates: slices.Compact(merged)}
}

// isSelected reports whether filter selects entry, as the index's query selects the issue: every
// set field must match, Labels all present and NoLabels all absent, and a Closed issue only with
// IsClosedIncluded. Limit is not a selection.
func isSelected(filter index.Filter, entry index.Entry) bool {
	isIn := func(values []string, value string) bool {
		return len(values) == 0 || slices.Contains(values, value)
	}
	switch {
	case !isIn(filter.Statuses, entry.Status),
		len(filter.Categories) > 0 && !slices.Contains(filter.Categories, entry.Category),
		!isIn(filter.Types, entry.Type),
		len(filter.Priorities) > 0 && !slices.Contains(filter.Priorities, entry.Priority),
		filter.ParentID != "" && entry.ParentID != filter.ParentID,
		filter.PlanContains != "" && !strings.Contains(entry.Plan, filter.PlanContains),
		filter.Plan == index.PlanSet && entry.Plan == "",
		filter.Plan == index.PlanUnset && entry.Plan != "",
		!filter.IsClosedIncluded && entry.Category.IsClosed():
		return false
	}
	for _, label := range filter.Labels {
		if !slices.Contains(entry.Labels, label) {
			return false
		}
	}
	for _, label := range filter.NoLabels {
		if slices.Contains(entry.Labels, label) {
			return false
		}
	}
	return true
}
