package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/nodestore"
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

// nodeFileExtension ends every issue and Note file; a node's children directory is its path
// without it.
const nodeFileExtension = ".md"

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

// kindFailure types a failure of a nodestore Kind: a node file that cannot be used is invalid, a
// stale --if-match a conflict and an id another node answers to exists, each at the file the
// failure names, relative to the working tree root, and its line when known. An argument that
// resolves to no node or to several goes through resolveFailure; any other failure through asError.
func kindFailure(err error) *Error {
	var file *nodestore.FileError
	if errors.As(err, &file) {
		return &Error{Code: CodeInvalid, Path: file.Path, Line: file.Line, Message: file.Message}
	}
	var conflict *nodestore.ConflictError
	if errors.As(err, &conflict) {
		return &Error{Code: CodeConflict, Path: conflict.Path, Message: conflict.Message}
	}
	var exists *nodestore.ExistsError
	if errors.As(err, &exists) {
		return &Error{Code: CodeExists, Path: exists.Path, Message: exists.Message}
	}
	if errors.Is(err, tree.ErrNotFound) || errors.Is(err, tree.ErrAmbiguous) {
		failure, _ := resolveFailure(err) // an argument failure: the command stops here
		return failure
	}
	return asError(err)
}

// worktreeRelative names path relative to the working tree root with forward slashes, or returns
// it unchanged when it is empty or outside the working tree.
func (a *app) worktreeRelative(path string) string {
	if relative := relativeSlashPath(a.store.WorktreeRoot, path); relative != "" {
		return relative
	}
	return path
}

// worktreeRelativeText returns text with every path inside the working tree named relative to its
// root, so a message built from an OS error never carries the worktree's absolute path.
func (a *app) worktreeRelativeText(text string) string {
	return strings.ReplaceAll(text, a.store.WorktreeRoot+string(filepath.Separator), "")
}

// treeProblemWarnings renders each problem as a warning, "path: message".
func treeProblemWarnings(problems tree.Errors) []string {
	var warnings []string
	for _, problem := range problems {
		warnings = append(warnings, problem.Error())
	}
	return warnings
}

// unwarnedTreeProblems renders each of problems, their paths relative to the working tree root
// with forward slashes, as a warning, "path: message", leaving out every problem the index refresh
// already warned about, so no problem is reported twice.
func (a *app) unwarnedTreeProblems(problems tree.Errors) []string {
	var unwarned tree.Errors
	for _, problem := range problems {
		if !a.isIndexWarnedAbout(problem) {
			unwarned = append(unwarned, problem)
		}
	}
	return treeProblemWarnings(unwarned)
}

// isIndexWarnedAbout reports whether this command's index refresh warned about problem, its path
// relative to the working tree root with forward slashes: a warning names the same file and has the
// same kind. A different problem on a file the index warned about is not covered.
func (a *app) isIndexWarnedAbout(problem *tree.Error) bool {
	kind, isIndexed := treeProblemKindOf(problem)
	if !isIndexed {
		return false
	}
	for _, warning := range a.indexReport.Warnings {
		if warning.Path == problem.Path && warning.Kind == kind {
			return true
		}
	}
	return false
}

// treeProblemKindOf returns the index warning kind of a tree.Load problem, by the sentinel it
// wraps, and false for a problem the index never reports. A repeated alias also wraps ErrDuplicate,
// so it is matched first.
func treeProblemKindOf(problem *tree.Error) (index.WarningKind, bool) {
	switch {
	case errors.Is(problem, tree.ErrInvalidID):
		return index.WarningInvalidID, true
	case errors.Is(problem, tree.ErrInvalidAlias):
		return index.WarningInvalidAlias, true
	case errors.Is(problem, tree.ErrRepeatedAlias):
		return index.WarningRepeatedAlias, true
	case errors.Is(problem, tree.ErrDuplicate):
		return index.WarningDuplicate, true
	case errors.Is(problem, tree.ErrOrphanDirectory):
		return index.WarningOrphanDirectory, true
	}
	return "", false
}
