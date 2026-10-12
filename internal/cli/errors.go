package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

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

// worktreeRelativeText returns text with every path inside the working tree named relative to its
// root, so a message built from an OS error never carries the worktree's absolute path.
func (a *app) worktreeRelativeText(text string) string {
	return strings.ReplaceAll(text, a.store.WorktreeRoot+string(filepath.Separator), "")
}

// treeProblems returns the problems of a tree.Load failure that still returned the tree of what it
// could place, for a command that goes on with that tree; it returns nil for a nil err. Any other
// failure, which returns no tree, comes back as treeLoadFailure types it.
func (a *app) treeProblems(err error) (tree.Errors, error) {
	if err == nil {
		return nil, nil
	}
	var problems tree.Errors
	if errors.As(err, &problems) {
		return problems, nil
	}
	return nil, a.treeLoadFailure(err)
}

// triageTreeProblems splits the tree problems of a command that writes by the files it touches,
// targets, given relative to the working tree root with forward slashes ("" entries are ignored).
// A problem blocks the command when it names a target as its file or as the other file of a
// duplicate, or is an orphan directory where a target's children go; the blocking problems come
// back as the command's failure. Every other problem is returned as a warning, "path: message",
// unless the index refresh already warned about the same problem, so no problem is reported twice.
func (a *app) triageTreeProblems(problems tree.Errors, targets ...string) ([]string, error) {
	warned, err := a.splitTreeProblems(problems, targets...)
	if err != nil {
		return nil, err
	}
	return treeProblemWarnings(warned), nil
}

// splitTreeProblems splits problems as triageTreeProblems does, but returns the problems to warn
// about as tree errors, their paths relative to the working tree root with forward slashes, for
// triageTreeProblems to render.
func (a *app) splitTreeProblems(problems tree.Errors, targets ...string) (tree.Errors, error) {
	var blocking tree.Errors
	var warned tree.Errors
	for _, problem := range problems {
		relative := a.relativeTreeError(problem)
		switch {
		case isBlockingTreeProblem(relative, targets):
			blocking = append(blocking, problem)
		case !a.isIndexWarnedAbout(relative):
			warned = append(warned, relative)
		}
	}
	if len(blocking) > 0 {
		return nil, a.treeLoadFailure(blocking)
	}
	return warned, nil
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

// isBlockingTreeProblem reports whether problem, its paths relative like targets, concerns one of
// targets: it is the target's file, the other file of the target's duplicate, or an orphan
// directory at the target's children directory.
func isBlockingTreeProblem(problem *tree.Error, targets []string) bool {
	for _, target := range targets {
		if target == "" {
			continue
		}
		if problem.Path == target || problem.Other == target {
			return true
		}
		childrenDir := strings.TrimSuffix(target, nodeFileExtension)
		if errors.Is(problem, tree.ErrOrphanDirectory) && problem.Path == childrenDir {
			return true
		}
	}
	return false
}

// isIndexWarnedAbout reports whether this command's index refresh warned about problem, its path
// relative to the working tree root with forward slashes: a warning names the same file and the
// same kind of problem. A different problem on a file the index warned about is not covered.
func (a *app) isIndexWarnedAbout(problem *tree.Error) bool {
	kind := treeProblemKindOf(problem)
	if kind == problemKindOther {
		return false
	}
	for _, warning := range a.indexReport.Warnings {
		if warning.Path == problem.Path && indexWarningKindOf(warning.Message) == kind {
			return true
		}
	}
	return false
}

// problemKind is the class of a tree problem or an index warning, which is all
// isIndexWarnedAbout compares: a tree problem and an index warning word the same problem
// differently.
type problemKind int

// The problem kinds the tree and the index both report.
const (
	// problemKindOther is any problem the other side never reports, such as a bad status.
	problemKindOther problemKind = iota
	problemKindInvalidID
	problemKindInvalidAlias
	// problemKindDuplicate is an id or alias another file also has.
	problemKindDuplicate
	// problemKindRepeatedAlias is an alias that repeats its own node's id or is listed twice.
	problemKindRepeatedAlias
	problemKindOrphanDirectory
)

// The leading and trailing text of the index warnings that have no tree sentinel of their own:
// the walk's directory beside an invalid node file, and aliasProblems' repeated aliases.
const (
	leftOutDirectoryWarningPrefix = "directory left out:"
	aliasWarningPrefix            = "alias "
	ownIDAliasWarningSuffix       = "'s own id"
	listedTwiceAliasWarningSuffix = " is listed twice"
)

// treeProblemKindOf returns the kind of a tree.Load problem, by the sentinel it wraps. A duplicate
// alias whose other file is its own is a repeated alias.
func treeProblemKindOf(problem *tree.Error) problemKind {
	switch {
	case errors.Is(problem, tree.ErrInvalidID):
		return problemKindInvalidID
	case errors.Is(problem, tree.ErrInvalidAlias):
		return problemKindInvalidAlias
	case errors.Is(problem, tree.ErrDuplicate) && problem.Alias != "" && problem.Other == problem.Path:
		return problemKindRepeatedAlias
	case errors.Is(problem, tree.ErrDuplicate):
		return problemKindDuplicate
	case errors.Is(problem, tree.ErrOrphanDirectory):
		return problemKindOrphanDirectory
	}
	return problemKindOther
}

// indexWarningKindOf returns the kind of an index warning, by its leading text: the text of the
// tree sentinel it starts with, or one of the index's own wordings of the same problem.
func indexWarningKindOf(message string) problemKind {
	switch {
	case strings.HasPrefix(message, tree.ErrInvalidID.Error()):
		return problemKindInvalidID
	case strings.HasPrefix(message, tree.ErrInvalidAlias.Error()):
		return problemKindInvalidAlias
	case strings.HasPrefix(message, tree.ErrDuplicate.Error()):
		return problemKindDuplicate
	case strings.HasPrefix(message, aliasWarningPrefix) &&
		(strings.HasSuffix(message, ownIDAliasWarningSuffix) ||
			strings.HasSuffix(message, listedTwiceAliasWarningSuffix)):
		return problemKindRepeatedAlias
	case strings.HasPrefix(message, tree.ErrOrphanDirectory.Error()),
		strings.HasPrefix(message, leftOutDirectoryWarningPrefix):
		return problemKindOrphanDirectory
	}
	return problemKindOther
}
