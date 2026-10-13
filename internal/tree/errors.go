package tree

import (
	"errors"
	"fmt"
	"strings"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// Sentinel errors, reachable through errors.Is on every error this package returns.
var (
	// ErrInvalidID reports an id that is not a 1-4 word kebab slug of at most 40 characters.
	ErrInvalidID = errors.New("invalid id")
	// ErrInvalidAlias reports an alias outside [a-z0-9.-]+.
	ErrInvalidAlias = errors.New("invalid alias")
	// ErrDuplicate reports an id or alias used twice anywhere in one tree.
	ErrDuplicate = errors.New("duplicate")
	// ErrRepeatedAlias reports an alias that repeats its own node's id or is listed twice in its
	// own file. Every error that wraps it also wraps ErrDuplicate.
	ErrRepeatedAlias = errors.New("repeated alias")
	// ErrOrphanDirectory reports a child directory without the node file of the same name, or
	// whose node file is left out: named by an invalid id, not a regular file, larger than 1 MiB,
	// or of an id another node file also has.
	ErrOrphanDirectory = errors.New("directory without its node file")
	// ErrNotFound reports an argument that matches no id, alias or id prefix.
	ErrNotFound = errors.New("not found")
	// ErrAmbiguous reports an id prefix that matches more than one node; see AmbiguousError.
	ErrAmbiguous = errors.New("ambiguous id prefix")
	// ErrNotRegular is nodefile.ErrNotRegular, re-exported so errors.Is matches either name.
	ErrNotRegular = nodefile.ErrNotRegular
	// ErrTooLarge is nodefile.ErrTooLarge, re-exported so errors.Is matches either name.
	ErrTooLarge = nodefile.ErrTooLarge
	// ErrChanged reports a file a Plan rewrites that no longer holds the bytes the rewrite was
	// worked out from: it changed after it was read, and the rewrite is not written over it.
	ErrChanged = errors.New("changed since it was read")
)

// Error is a problem with one file or directory of a tree. Err wraps one of the sentinel errors.
// Other is the file a duplicate id or alias also belongs to, kept out of Err so a caller can name
// it relative to a root of its own; it is "" for every other problem. Alias is the alias an invalid
// or duplicate alias problem is about, and "" for a problem with an id or a directory.
type Error struct {
	Path  string
	Other string
	Alias string
	Err   error
}

// Error renders "path: message".
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Message())
}

// Message renders the problem without its path: Err, followed by Other when there is one.
func (e *Error) Message() string {
	if e.Other == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%v %s", e.Err, e.Other)
}

// Unwrap returns the underlying error, so errors.Is matches the sentinels.
func (e *Error) Unwrap() error {
	return e.Err
}

// Errors is every problem Load found in one tree, ordered by path. errors.Is and errors.As reach
// each problem, and through it the sentinel it wraps.
type Errors []*Error

// Error renders one "path: message" line per problem.
func (e Errors) Error() string {
	lines := make([]string, 0, len(e))
	for _, problem := range e {
		lines = append(lines, problem.Error())
	}
	return strings.Join(lines, "\n")
}

// Unwrap returns the problems, so errors.Is and errors.As look into each.
func (e Errors) Unwrap() []error {
	unwrapped := make([]error, 0, len(e))
	for _, problem := range e {
		unwrapped = append(unwrapped, problem)
	}
	return unwrapped
}

// invalidIDDirectoryError is the problem of a child directory whose node file is there but left
// out: named by an invalid id, not a regular file, larger than 1 MiB, or of a repeated id. It wraps
// ErrOrphanDirectory, since the directory is left out like one without its node file, while its
// text names the node file and why it was left out.
type invalidIDDirectoryError struct {
	name   string
	reason string
}

// Error renders the problem without its path.
func (e *invalidIDDirectoryError) Error() string {
	return fmt.Sprintf("directory left out: its node file %s%s %s", e.name, nodeExtension, e.reason)
}

// Unwrap returns ErrOrphanDirectory.
func (e *invalidIDDirectoryError) Unwrap() error {
	return ErrOrphanDirectory
}

// repeatedAliasError is an alias that repeats its own node's id or is listed twice. It renders as
// its message alone and wraps both ErrRepeatedAlias and ErrDuplicate.
type repeatedAliasError struct {
	message string
}

// Error returns the message.
func (e *repeatedAliasError) Error() string {
	return e.message
}

// Unwrap returns the sentinels, so errors.Is matches ErrRepeatedAlias and ErrDuplicate.
func (e *repeatedAliasError) Unwrap() []error {
	return []error{ErrRepeatedAlias, ErrDuplicate}
}
