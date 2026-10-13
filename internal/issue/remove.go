package issue

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/rawfm"
	"github.com/airiclenz/capcom/internal/tree"
)

// removeKind is the noun phrase, with its article, that the refusals of an issue removal name an
// issue by.
const removeKind = moveKind

// Removal is the removal of one issue: the tree removal of its file and its empty children
// directory, with the files of the issues whose deps or related entries naming it are dropped
// added as rewrites, which Apply writes before it removes anything.
type Removal struct {
	*tree.Removal
	// DepsDropped are the ids of the issues whose deps entries naming the removed issue are
	// dropped, in tree order.
	DepsDropped []string
	// RelatedDropped are the ids of the issues whose related entries naming the removed issue are
	// dropped, in tree order.
	RelatedDropped []string
	// Unread are the files of other issues that do not parse, or that tree.ReadNodeFile refuses
	// unread, so their deps and related entries could not be checked for the removed issue and were
	// left as they are.
	Unread []string
}

// RemoveOptions are the choices of one issue removal.
type RemoveOptions struct {
	// IfMatch, when not empty, is the etag (see edit.ETag) the issue file must still have.
	IfMatch string
	// ShouldDropDeps drops the deps entries of the issues that depend on the removed issue instead
	// of refusing its removal.
	ShouldDropDeps bool
}

// ETagMismatchError refuses a removal whose RemoveOptions.IfMatch is not the issue file's etag.
type ETagMismatchError struct {
	// Path is the issue file.
	Path string
	// IfMatch is the etag the removal asked for.
	IfMatch string
	// ETag is the file's etag.
	ETag string
}

// Error names both etags.
func (e *ETagMismatchError) Error() string {
	return fmt.Sprintf("etag %s does not match the file's etag %s", e.IfMatch, e.ETag)
}

// DependentsError refuses the removal of an issue other issues depend on.
type DependentsError struct {
	// Node is the refused issue.
	Node tree.Node
	// Dependents are the ids of the issues whose deps name it, in the order of their files' paths.
	Dependents []string
}

// Error names the issue and its dependents.
func (e *DependentsError) Error() string {
	return fmt.Sprintf("issue %s is a dependency of %s", e.Node.ID, strings.Join(e.Dependents, ", "))
}

// PlanRemove works out the removal of the issue with exact id id in issues, reading the issue
// files but changing nothing. Every other issue's deps and related entries that name the issue,
// by its id or one of its Aliases, are dropped; related entries never refuse the removal. The
// issue's own file is read for its etag but never parsed, so an issue whose file does not parse
// is removed like any other: its Aliases are the node's, or those rawfm still reads from the file
// when the node has none. A dependency of the issue on itself is no dependent.
//
// PlanRemove fails as tree.PlanRemove does, naming an issue, then with the *fs.PathError of a file
// it cannot read, an *ETagMismatchError when options.IfMatch is not the issue file's etag, and a
// *DependentsError when other issues depend on it and options do not drop those deps.
func PlanRemove(issues *tree.Tree, id string, options RemoveOptions) (*Removal, error) {
	planned, err := issues.PlanRemove(id, removeKind)
	if err != nil {
		return nil, err
	}
	data, err := tree.ReadNodeFile(planned.Node.Path)
	if err != nil {
		return nil, err
	}
	if current := edit.ETag(data); options.IfMatch != "" && options.IfMatch != current {
		return nil, &ETagMismatchError{Path: planned.Node.Path, IfMatch: options.IfMatch, ETag: current}
	}

	removal := &Removal{Removal: planned}
	aliases := planned.Node.Aliases
	if len(aliases) == 0 {
		aliases = rawfm.Aliases(data)
	}
	rewrites, dependents, err := removal.planDrops(issues, append([]string{planned.Node.ID}, aliases...))
	if err != nil {
		return nil, err
	}
	if len(dependents) > 0 && !options.ShouldDropDeps {
		return nil, &DependentsError{Node: planned.Node, Dependents: dependents}
	}
	for _, rewrite := range rewrites {
		planned.AddRewrite(rewrite)
	}
	return removal, nil
}

// planDrops reads every other issue file of issues, those tree.Load left out for a repeated id
// included, through tree.ReadNodeFile and returns the files whose content the removal changes,
// without the deps and related entries that are one of names, in tree order, and the ids of the
// dependents, in the order of their files' paths. It sets r.DepsDropped, r.RelatedDropped and
// r.Unread.
func (r *Removal) planDrops(issues *tree.Tree, names []string) ([]tree.Rewrite, []string, error) {
	isDropped := func(entry string) bool { return slices.Contains(names, entry) }
	var rewrites []tree.Rewrite
	var dependents []tree.Node
	for _, other := range slices.Concat(issues.Nodes(), issues.LeftOut()) {
		if other.ID == r.Node.ID {
			continue
		}
		data, err := tree.ReadNodeFile(other.Path)
		if isRefusedRead(err) {
			r.Unread = append(r.Unread, other.Path)
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		parsed, err := Parse(other.Path, data)
		if err != nil {
			r.Unread = append(r.Unread, other.Path)
			continue
		}

		edited := parsed
		edited.Deps = dropEntries(parsed.Deps, isDropped)
		edited.Related = dropEntries(parsed.Related, isDropped)
		isDependent := len(edited.Deps) != len(parsed.Deps)
		isRelatedDropped := len(edited.Related) != len(parsed.Related)
		if isDependent {
			dependents = append(dependents, other)
			r.DepsDropped = append(r.DepsDropped, other.ID)
		}
		if isRelatedDropped {
			r.RelatedDropped = append(r.RelatedDropped, other.ID)
		}
		if isDependent || isRelatedDropped {
			rewrites = append(rewrites, tree.Rewrite{Path: other.Path, Original: data, Content: edited.Serialize()})
		}
	}
	slices.SortStableFunc(dependents, func(first tree.Node, second tree.Node) int {
		return strings.Compare(filepath.ToSlash(first.Path), filepath.ToSlash(second.Path))
	})
	ids := make([]string, 0, len(dependents))
	for _, dependent := range dependents {
		ids = append(ids, dependent.ID)
	}
	return rewrites, ids, nil
}

// dropEntries returns a copy of entries, a deps or related list, without the entries isDropped
// matches, or nil when none is left.
func dropEntries(entries []string, isDropped func(string) bool) []string {
	kept := slices.DeleteFunc(slices.Clone(entries), isDropped)
	if len(kept) == 0 {
		return nil
	}
	return kept
}

// isRefusedRead reports whether err is tree.ReadNodeFile refusing a file unread: one that is not a
// regular file, or is larger than 1 MiB.
func isRefusedRead(err error) bool {
	return errors.Is(err, tree.ErrNotRegular) || errors.Is(err, tree.ErrTooLarge)
}
