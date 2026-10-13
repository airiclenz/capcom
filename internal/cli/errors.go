package cli

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/tree"
)

// kindFailure types a failure of a nodestore Kind: a node file that cannot be used is invalid, a
// stale --if-match, or a file changed between the command's read and its write, a conflict and an
// id another node answers to exists, each at the file the failure names, relative to the working
// tree root, and its line when known. An argument that resolves to no node or to several goes
// through resolveFailure; any other failure through output.AsError.
func kindFailure(err error) *output.Error {
	var file *nodestore.FileError
	if errors.As(err, &file) {
		return &output.Error{Code: output.CodeInvalid, Path: file.Path, Line: file.Line, Message: file.Message}
	}
	var conflict *nodestore.ConflictError
	if errors.As(err, &conflict) {
		return &output.Error{Code: output.CodeConflict, Path: conflict.Path, Message: conflict.Message}
	}
	var exists *nodestore.ExistsError
	if errors.As(err, &exists) {
		return &output.Error{Code: output.CodeExists, Path: exists.Path, Message: exists.Message}
	}
	if errors.Is(err, tree.ErrNotFound) || errors.Is(err, tree.ErrAmbiguous) {
		failure, _ := resolveFailure(err) // an argument failure: the command stops here
		return failure
	}
	return output.AsError(err)
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
	kind, isIndexed := diagnose.TreeKind(problem)
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
