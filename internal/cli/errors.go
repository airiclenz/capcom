package cli

import (
	"errors"
	"fmt"

	"github.com/airiclenz/capcom/internal/tree"
)

// Code is the machine-readable class of a failed command. It appears as error.code in the
// --json envelope and as the STATUS value in --kv output.
type Code string

// The closed set of error codes capcom reports.
const (
	CodeNotFound       Code = "not_found"
	CodeExists         Code = "exists"
	CodeInvalid        Code = "invalid"
	CodeConflict       Code = "conflict"
	CodeLocked         Code = "locked"
	CodeNotInitialized Code = "not_initialized"
)

// Process exit codes.
const (
	ExitOK       = 0
	ExitError    = 1
	ExitUsage    = 2
	ExitNotFound = 3
)

// Error is a typed command failure. Path and Line, when set, locate the file (and line within it)
// the failure is about; Line is meaningful only together with Path.
type Error struct {
	Code    Code
	Message string
	Path    string
	Line    int
	isUsage bool
	// isRendered marks a failure the output layer already wrote with its partial result; Run
	// only maps it to the exit code.
	isRendered bool
}

// Errorf builds an Error with the given code and a formatted message.
func Errorf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// UsageErrorf builds an invalid-code Error for a malformed invocation (unknown command, bad flag,
// wrong argument count); it exits with ExitUsage instead of ExitError.
func UsageErrorf(format string, args ...any) *Error {
	return &Error{Code: CodeInvalid, Message: fmt.Sprintf(format, args...), isUsage: true}
}

// Error renders the message prefixed by "path:" or "path:line:" when a location is known.
func (e *Error) Error() string {
	switch {
	case e.Path != "" && e.Line > 0:
		return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Message)
	case e.Path != "":
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	default:
		return e.Message
	}
}

// ExitCode maps the error to the process exit code: usage errors 2, not_found and locked 3,
// everything else 1.
func (e *Error) ExitCode() int {
	if e.isUsage {
		return ExitUsage
	}
	if e.Code == CodeNotFound || e.Code == CodeLocked {
		return ExitNotFound
	}
	return ExitError
}

// asError returns err as an *Error, giving an untyped failure the invalid code so every failure
// reaches the output layer with a code from the closed set.
func asError(err error) *Error {
	var typed *Error
	if errors.As(err, &typed) {
		return typed
	}
	return &Error{Code: CodeInvalid, Message: err.Error()}
}

// treeLoadFailure returns a tree.Load failure as an *Error, naming every file of its tree
// problems, the file at fault and the other file of a duplicate alike, relative to the working
// tree root with forward slashes. Any other failure goes through asError.
func (a *app) treeLoadFailure(err error) *Error {
	var problems tree.Errors
	if errors.As(err, &problems) {
		relative := make(tree.Errors, 0, len(problems))
		for _, problem := range problems {
			relative = append(relative, a.relativeTreeError(problem))
		}
		return &Error{Code: CodeInvalid, Message: relative.Error()}
	}
	var problem *tree.Error
	if errors.As(err, &problem) {
		return &Error{Code: CodeInvalid, Message: a.relativeTreeError(problem).Error()}
	}
	return asError(err)
}

// relativeTreeError returns a copy of problem whose Path and Other are relative to the working
// tree root with forward slashes; a path outside it is kept as it is.
func (a *app) relativeTreeError(problem *tree.Error) *tree.Error {
	relative := *problem
	relative.Path = a.worktreeRelative(problem.Path)
	relative.Other = a.worktreeRelative(problem.Other)
	return &relative
}

// worktreeRelative names path relative to the working tree root with forward slashes, or returns
// it unchanged when it is empty or outside the working tree.
func (a *app) worktreeRelative(path string) string {
	if relative := relativeSlashPath(a.store.WorktreeRoot, path); relative != "" {
		return relative
	}
	return path
}
