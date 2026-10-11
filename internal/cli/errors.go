package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

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

// hiddenNamePrefix starts the file and directory names tree.Load skips, temp files among them.
const hiddenNamePrefix = "."

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
// unless the index refresh already warned about its file, so no file is reported twice.
func (a *app) triageTreeProblems(problems tree.Errors, targets ...string) ([]string, error) {
	var blocking tree.Errors
	var warnings []string
	for _, problem := range problems {
		relative := a.relativeTreeError(problem)
		switch {
		case isBlockingTreeProblem(relative, targets):
			blocking = append(blocking, problem)
		case !a.isIndexWarnedAbout(relative.Path):
			warnings = append(warnings, relative.Error())
		}
	}
	if len(blocking) > 0 {
		return nil, a.treeLoadFailure(blocking)
	}
	return warnings, nil
}

// orphanedIDFailure refuses id, the new id of a node being written or moved, when a file of that
// id sits anywhere under one of the orphan directories among problems. tree.Load leaves such a file
// out of the tree, so writing the id beside it would make a duplicate that surfaces only once the
// orphan directory is fixed. The failure names that file relative to the working tree root; it is
// nil when there is no such file.
func (a *app) orphanedIDFailure(problems tree.Errors, id string) error {
	name := id + nodeFileExtension
	for _, problem := range problems {
		if !errors.Is(problem, tree.ErrOrphanDirectory) {
			continue
		}
		path, err := findNodeFile(problem.Path, name)
		if err != nil {
			return asError(err)
		}
		if path != "" {
			return &Error{
				Code: CodeExists,
				Path: a.worktreeRelative(path),
				Message: fmt.Sprintf(
					"id %q is already taken by this file, inside the orphan directory %s",
					id,
					a.worktreeRelative(problem.Path),
				),
			}
		}
	}
	return nil
}

// findNodeFile returns the first file named name at any depth below dir, skipping hidden names as
// tree.Load does, or "" when there is none.
func findNodeFile(dir string, name string) (string, error) {
	found := ""
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != dir && strings.HasPrefix(entry.Name(), hiddenNamePrefix) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() && entry.Name() == name {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	return found, err
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

// isIndexWarnedAbout reports whether this command's index refresh warned about the file at path,
// relative to the working tree root with forward slashes.
func (a *app) isIndexWarnedAbout(path string) bool {
	for _, warning := range a.indexReport.Warnings {
		if warning.Path == path {
			return true
		}
	}
	return false
}
