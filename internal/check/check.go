// Package check validates a whole store at once, for `capcom check`: the config, the issue tree
// and the Knowledge tree, read straight from the files and never from the index. It owns no rule
// of its own beyond resolving deps, related links, plans and superseded-by links, and checking that
// a Note's format-3 keys sit in a store declaring format 3; it collects what the config parser,
// the tree loader, the issue and Note parsers and validators, the git conflict-marker scan of
// package rawfm, the dependency cycle search and the Ref verifier report, each as a
// Problem citing the file and, when known, the line (ADR 0006). It also reports an AGENTS.md whose
// capcom Managed block is out of date or whose markers are tangled, and one that cannot be read,
// being not a regular file or larger than 1 MiB; an AGENTS.md without the block, or no AGENTS.md
// at all, is no problem. When AGENTS.md carries the block, it reports each agent guide config.yaml
// lists under agents.guides that neither resolves to AGENTS.md nor imports it, and one that cannot
// be read; a missing guide, or one that links outside the working tree, is no problem.
package check

import (
	"cmp"
	"errors"
	"slices"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/rawfm"
	"github.com/airiclenz/capcom/internal/store"
)

// Run validates the opened store s and returns every problem it finds, ordered by path and line,
// or nil for a valid store. It fails when a store file or directory cannot be read.
func Run(s *store.Store) ([]Problem, error) {
	run := checker{store: s}
	if err := run.issues(); err != nil {
		return nil, err
	}
	if err := run.notes(); err != nil {
		return nil, err
	}
	if err := run.agentsBlock(); err != nil {
		return nil, err
	}
	slices.SortStableFunc(run.problems, func(a, b Problem) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line))
	})
	return run.problems, nil
}

// ConfigProblems turns err, a store.Open failure, into the problems check reports for it: the one
// config problem when config.yaml cannot be used, since every issue rule reads the config, and nil
// for any other failure.
func ConfigProblems(err error) []Problem {
	var configFailure *config.Error
	if !errors.As(err, &configFailure) {
		return nil
	}
	return []Problem{{
		Path:    configFailure.Path,
		Line:    configFailure.Line,
		Code:    diagnose.Config,
		Message: configFailure.Message,
	}}
}

// checker collects the problems of one store.
type checker struct {
	store    *store.Store
	problems []Problem
}

// add records one problem at the absolute or worktree-relative path.
func (c *checker) add(path string, line int, code diagnose.Kind, message string) {
	c.problems = append(c.problems, Problem{
		Path:    c.store.Relative(path),
		Line:    line,
		Code:    code,
		Message: message,
	})
}

// checkConflictMarkers records every git conflict marker in the issue or Note file at path.
func (c *checker) checkConflictMarkers(path string, data []byte) {
	for _, marker := range rawfm.ConflictMarkers(data) {
		c.add(path, marker.Line, diagnose.Conflict, rawfm.ConflictMessage(marker))
	}
}

// addParseFailure records failure, the finding that the file at path does not parse, at the line
// its parser blamed. A parser blames a conflict marker in the frontmatter with the conflict
// problem's own words, which checkConflictMarkers has already recorded, so that failure is not
// recorded twice.
func (c *checker) addParseFailure(path string, failure diagnose.Finding) {
	conflict := Problem{Path: c.store.Relative(path), Line: failure.Line, Code: diagnose.Conflict, Message: failure.Message}
	if slices.Contains(c.problems, conflict) {
		return
	}
	c.add(path, failure.Line, diagnose.Frontmatter, failure.Message)
}
