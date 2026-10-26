// Package diagnose classifies the problems capcom finds in a store's files, for check, the index
// refresh and the cli alike: one Kind per problem, the one mapping from a tree sentinel error to
// its Kind, the Findings of alias-checking one issue or Note file, and the names two or more files
// of one tree claim; nodestore's Kind reader finds the rest of a node file's Findings. It never
// reads the index, so check, which never does either, may import it.
package diagnose

import (
	"errors"

	"github.com/airiclenz/capcom/internal/tree"
)

// Kind classifies a problem with one file or directory, so a caller tells problems apart without
// reading the message. Its values are the codes check reports and the kinds the index stores with
// each warning; the index stores only the kinds a file's walk, parse or validation finds.
type Kind string

// The kinds of problem.
const (
	// InvalidID is a node file whose name is not a valid id.
	InvalidID Kind = "invalid-id"
	// InvalidAlias is an alias that is not a valid name.
	InvalidAlias Kind = "invalid-alias"
	// Duplicate is an id or alias another file of the same tree also has.
	Duplicate Kind = "duplicate"
	// RepeatedAlias is an alias that repeats its own node's id or is listed twice.
	RepeatedAlias Kind = "repeated-alias"
	// OrphanDirectory is a directory left out because it has no valid node file beside it.
	OrphanDirectory Kind = "orphan-directory"
	// NotRegular is an entry left out unread because it is not a regular file (a symlink, a named
	// pipe, a device) or is a node file larger than 1 MiB.
	NotRegular Kind = "not-regular"
	// Frontmatter is a file whose frontmatter cannot be read.
	Frontmatter Kind = "frontmatter"
	// Field is a field value the file's kind does not accept.
	Field Kind = "field"
	// Config is a config.yaml that cannot be used.
	Config Kind = "config"
	// Conflict is a git conflict marker left in an issue or Note file by a merge.
	Conflict Kind = "conflict"
	// MissingDep is a dependency that names no issue.
	MissingDep Kind = "missing-dep"
	// MissingRelated is a related link that names no issue.
	MissingRelated Kind = "missing-related"
	// MissingSupersededBy is a superseded-by link that names no Note.
	MissingSupersededBy Kind = "missing-superseded-by"
	// FormatTooOld is an issue or Note key the store's declared format does not hold yet: related
	// in a store below format 2, retired or superseded-by in a store below format 3.
	FormatTooOld Kind = "format-too-old"
	// Cycle is a dependency cycle.
	Cycle Kind = "cycle"
	// MissingPlan is a plan found in none of the configured plans directories.
	MissingPlan Kind = "missing-plan"
	// Standing is a standing issue whose status is in a closed category or the doing category: a
	// standing Parent only groups its children and is never claimed or closed.
	Standing Kind = "standing"
	// BrokenRef is a Note Ref that does not resolve in the working tree.
	BrokenRef Kind = "broken-ref"
	// UnsharedRef is a Note Ref that resolves in this working tree but names a path a clone lacks:
	// one inside the git directory, or one git ignores.
	UnsharedRef Kind = "unshared-ref"
	// AgentsBlock is an AGENTS.md whose Managed block is stale or whose markers are tangled, or
	// one that is not a regular file or is larger than 1 MiB.
	AgentsBlock Kind = "agents-block"
	// AgentsGuide is an agent guide that does not reach the Managed block of AGENTS.md, or one that
	// is not a regular file or is larger than 1 MiB.
	AgentsGuide Kind = "agents-guide"
)

// TreeKind returns the Kind of a problem tree.Walk, tree.Load or tree.CheckAliases reports, by the
// sentinel it wraps, and false for a problem that wraps none of them. A repeated alias also wraps
// tree.ErrDuplicate, so it is matched first; tree.ErrTooLarge is a NotRegular problem.
func TreeKind(problem *tree.Error) (Kind, bool) {
	switch {
	case errors.Is(problem, tree.ErrInvalidID):
		return InvalidID, true
	case errors.Is(problem, tree.ErrInvalidAlias):
		return InvalidAlias, true
	case errors.Is(problem, tree.ErrRepeatedAlias):
		return RepeatedAlias, true
	case errors.Is(problem, tree.ErrDuplicate):
		return Duplicate, true
	case errors.Is(problem, tree.ErrOrphanDirectory):
		return OrphanDirectory, true
	case errors.Is(problem, tree.ErrNotRegular), errors.Is(problem, tree.ErrTooLarge):
		return NotRegular, true
	}
	return "", false
}

// ReadRefusal returns the tree sentinel err wraps when nodefile.ReadRegularFile refused a file
// unread, tree.ErrNotRegular or tree.ErrTooLarge, or nil for any other err. The sentinel's text is
// the message a NotRegular problem carries, worded as the walk words the same problem.
func ReadRefusal(err error) error {
	for _, sentinel := range []error{tree.ErrNotRegular, tree.ErrTooLarge} {
		if errors.Is(err, sentinel) {
			return sentinel
		}
	}
	return nil
}
