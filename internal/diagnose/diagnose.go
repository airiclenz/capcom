// Package diagnose classifies the problems capcom finds in a store's files, for check, the index
// refresh and the cli alike: one Kind per problem, the one mapping from a tree sentinel error to
// its Kind, the Findings of parsing, validating and alias-checking one issue or Note file, and the
// names two or more files of one tree claim. It never reads the index, so check, which never does
// either, may import it.
package diagnose

import (
	"errors"

	"github.com/airiclenz/capcom/internal/tree"
)

// Kind classifies a problem with one file or directory, so a caller tells problems apart without
// reading the message. Its values are the kinds the index stores with each warning.
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
