package nodestore

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/tree"
)

// FileError is a node file that cannot be read or used: Path is the file relative to the working
// tree root with forward slashes, Line its 1-based line at fault (0 when no single line is to
// blame), and Message the problem, naming any other file relative to the working tree root too.
// IsValidation marks a node that does not validate and so was not written: the file on disk is
// not at fault.
type FileError struct {
	Path         string
	Line         int
	Message      string
	IsValidation bool
}

// Error renders "path:line: message", or "path: message" when the line is unknown.
func (e *FileError) Error() string {
	return locatedMessage(e.Path, e.Line, e.Message)
}

// ConflictError refuses a change because a node file changed since it was read: either the
// --if-match etag is not the node file's, so the file changed since the caller read it, and
// Message names both etags and tells the caller to show the node again; or a file the command
// rewrites changed between the command's own read and its write, and IfMatch and ETag are empty
// and Message tells the caller to run the command again. Path is the file relative to the working
// tree root with forward slashes.
type ConflictError struct {
	Path    string
	IfMatch string
	ETag    string
	Message string
}

// Error renders "path: message".
func (e *ConflictError) Error() string {
	return locatedMessage(e.Path, 0, e.Message)
}

// ExistsError refuses an id another node already answers to, as its id or as one of its Aliases.
// Path is that node's file relative to the working tree root with forward slashes, Holder its id,
// and Message says whether ID is its id or an alias.
type ExistsError struct {
	Path    string
	ID      string
	Holder  string
	Message string
}

// Error renders "path: message".
func (e *ExistsError) Error() string {
	return locatedMessage(e.Path, 0, e.Message)
}

// locatedMessage renders message prefixed by "path:" or "path:line:".
func locatedMessage(path string, line int, message string) string {
	if line > 0 {
		return fmt.Sprintf("%s:%d: %s", path, line, message)
	}
	return fmt.Sprintf("%s: %s", path, message)
}

// fileError types err, a failure to read or parse the node file at path, as a *FileError at that
// file: a *frontmatter.Error keeps its own file and line. Both the path and the message name files
// relative to the working tree root.
func (k *Kind[T]) fileError(path string, err error) *FileError {
	var failure *frontmatter.Error
	if errors.As(err, &failure) {
		return &FileError{
			Path:    k.relative(failure.Path),
			Line:    failure.Line,
			Message: k.relativeText(failure.Message),
		}
	}
	return &FileError{Path: k.relative(path), Message: k.relativeText(err.Error())}
}

// conflictError refuses a change of the node file at path, whose etag is etag, asked for at the
// stale etag ifMatch.
func (k *Kind[T]) conflictError(path string, ifMatch string, etag string) *ConflictError {
	return &ConflictError{
		Path:    k.relative(path),
		IfMatch: ifMatch,
		ETag:    etag,
		Message: fmt.Sprintf(
			"etag %s does not match the file's etag %s; show the %s again",
			ifMatch,
			etag,
			k.codec.Noun,
		),
	}
}

// changedMessage is the Message of a *ConflictError at a file that changed between a command's
// read and its write.
const changedMessage = "changed since this command read it; run the command again"

// changedError refuses the write of the node file at path, which changed since the command read
// it, appending each of rollbackFailures, the changes that could not be taken back, after "; ",
// each on one line with the paths inside the working tree relative to its root.
func (k *Kind[T]) changedError(path string, rollbackFailures []error) *ConflictError {
	messages := []string{changedMessage}
	for _, failure := range rollbackFailures {
		messages = append(messages, k.relativeText(strings.ReplaceAll(failure.Error(), "\n", " ")))
	}
	return &ConflictError{Path: k.relative(path), Message: strings.Join(messages, problemSeparator)}
}

// applyChangedFailure returns the *ConflictError for err, the failure of a planned move or
// removal's Apply, when its cause is a rewrite refused with tree.ErrChanged, naming the rewritten
// file and the changes that could not be taken back; otherwise it returns nil.
func (k *Kind[T]) applyChangedFailure(err error) *ConflictError {
	failures := []error{err}
	if joined, isJoined := err.(interface{ Unwrap() []error }); isJoined {
		failures = joined.Unwrap()
	}
	var changed *fs.PathError
	if !errors.Is(failures[0], tree.ErrChanged) || !errors.As(failures[0], &changed) {
		return nil
	}
	return k.changedError(changed.Path, failures[1:])
}

// existsError refuses id, which is the id or an Alias of the existing node.
func (k *Kind[T]) existsError(id string, existing tree.Node) *ExistsError {
	message := fmt.Sprintf("id %q is already taken by %s %s", id, k.codec.Noun, existing.ID)
	if existing.ID != id {
		message = fmt.Sprintf("id %q is already an alias of %s %s", id, k.codec.Noun, existing.ID)
	}
	return &ExistsError{
		Path:    k.relative(existing.Path),
		ID:      id,
		Holder:  existing.ID,
		Message: message,
	}
}
