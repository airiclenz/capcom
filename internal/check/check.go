// Package check validates a whole store at once, for `capcom check`: the config, the issue tree
// and the Knowledge tree, read straight from the files and never from the index. It owns no rule
// of its own beyond resolving deps and plans; it collects what the config parser, the tree loader,
// the issue and Note parsers and validators, the dependency cycle search and the Ref verifier
// report, each as a Problem citing the file and, when known, the line (ADR 0006).
package check

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// The closed set of Problem codes.
const (
	// CodeConfig is a config.yaml that cannot be used.
	CodeConfig = "config"
	// CodeInvalidID is a node file whose name is not a valid id.
	CodeInvalidID = "invalid-id"
	// CodeInvalidAlias is an alias outside the allowed characters.
	CodeInvalidAlias = "invalid-alias"
	// CodeDuplicate is an id or alias used twice in one tree.
	CodeDuplicate = "duplicate"
	// CodeOrphanDirectory is a directory without the node file of the same name.
	CodeOrphanDirectory = "orphan-directory"
	// CodeFrontmatter is an issue or Note file that does not parse: a malformed frontmatter, an
	// unknown or repeated key, a value of the wrong shape.
	CodeFrontmatter = "frontmatter"
	// CodeField is a parsed issue or Note whose values are not usable: a missing title, an
	// undeclared status or type, a closed issue without closed or reason, and the like.
	CodeField = "field"
	// CodeMissingDep is a dependency that names no issue.
	CodeMissingDep = "missing-dep"
	// CodeCycle is a dependency cycle.
	CodeCycle = "cycle"
	// CodeMissingPlan is a plan found in none of the configured plans directories.
	CodeMissingPlan = "missing-plan"
	// CodeBrokenRef is a Note Ref that does not resolve in the working tree.
	CodeBrokenRef = "broken-ref"
)

// refSymbolSeparator splits a Note Ref into its path and symbol.
const refSymbolSeparator = "#"

// planExtension is the extension a plan basename is looked up with, besides the bare name.
const planExtension = ".md"

// Problem is one thing wrong with the store. Path is relative to the working tree root with
// forward slashes; Line is 1-based, or 0 when no single line is to blame.
type Problem struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// String renders "path:line: message", or "path: message" when the line is unknown.
func (p Problem) String() string {
	if p.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", p.Path, p.Line, p.Message)
	}
	return fmt.Sprintf("%s: %s", p.Path, p.Message)
}

// Run validates the store of the git working tree containing cwd and returns every problem it
// finds, ordered by path and line, or nil for a valid store. A config.yaml that cannot be used is
// the only problem reported, since every issue rule reads the config.
//
// It fails, as store.Open does, outside a git working tree and without a store, and when a store
// file or directory cannot be read.
func Run(cwd string) ([]Problem, error) {
	opened, err := store.Open(cwd)
	var configFailure *config.Error
	if errors.As(err, &configFailure) {
		return []Problem{{
			Path:    configFailure.Path,
			Line:    configFailure.Line,
			Code:    CodeConfig,
			Message: configFailure.Message,
		}}, nil
	}
	if err != nil {
		return nil, err
	}

	run := checker{store: opened}
	if err := run.issues(); err != nil {
		return nil, err
	}
	if err := run.notes(); err != nil {
		return nil, err
	}
	slices.SortStableFunc(run.problems, func(a, b Problem) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line))
	})
	return run.problems, nil
}

// checker collects the problems of one store.
type checker struct {
	store    *store.Store
	problems []Problem
}

// add records one problem at the absolute or worktree-relative path.
func (c *checker) add(path string, line int, code string, message string) {
	c.problems = append(c.problems, Problem{
		Path:    c.relative(path),
		Line:    line,
		Code:    code,
		Message: message,
	})
}

// relative names path relative to the working tree root with forward slashes; a path already
// relative is kept.
func (c *checker) relative(path string) string {
	if !filepath.IsAbs(path) {
		return filepath.ToSlash(path)
	}
	relative, err := filepath.Rel(c.store.WorktreeRoot, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

// loadTree loads the tree in dir, recording each of its load problems, and returns what could be
// placed.
func (c *checker) loadTree(dir string, aliases tree.AliasFunc) (*tree.Tree, error) {
	loaded, err := tree.Load(dir, aliases)
	var problems tree.Errors
	if !errors.As(err, &problems) {
		return loaded, err
	}
	for _, problem := range problems {
		c.add(problem.Path, 0, treeCode(problem.Err), problem.Err.Error())
	}
	return loaded, nil
}

// treeCode is the Problem code of a tree load problem, by the sentinel it wraps.
func treeCode(err error) string {
	switch {
	case errors.Is(err, tree.ErrInvalidID):
		return CodeInvalidID
	case errors.Is(err, tree.ErrInvalidAlias):
		return CodeInvalidAlias
	case errors.Is(err, tree.ErrOrphanDirectory):
		return CodeOrphanDirectory
	default:
		return CodeDuplicate
	}
}

// readFile reads a store file, naming it in the error.
func (c *checker) readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.relative(path), err)
	}
	return data, nil
}

// parsedIssue is one issue file of the tree with what parsing it gave.
type parsedIssue struct {
	node     tree.Node
	issue    issue.Issue
	isParsed bool
}

// issues checks the issue tree: its load problems, each file's frontmatter and fields, its plan,
// its deps and the dependency cycles.
func (c *checker) issues() error {
	parsed := map[string]issue.Issue{}
	readAliases := func(path string) ([]string, error) {
		data, err := c.readFile(path)
		if err != nil {
			return nil, err
		}
		found, err := issue.Parse(c.relative(path), data)
		if err != nil {
			c.addParseFailure(path, err)
			return nil, nil
		}
		parsed[path] = found
		return found.Aliases, nil
	}
	loaded, err := c.loadTree(c.store.IssuesDir(), readAliases)
	if err != nil {
		return err
	}

	entries := make([]parsedIssue, 0, len(loaded.Nodes()))
	for _, node := range loaded.Nodes() {
		found, isParsed := parsed[node.Path]
		entries = append(entries, parsedIssue{node: node, issue: found, isParsed: isParsed})
	}
	graph := issue.NewDepGraph(graphNodes(entries))
	for _, entry := range entries {
		if entry.isParsed {
			c.checkIssue(entry, graph)
		}
	}
	c.checkCycles(entries, graph)
	return nil
}

// addParseFailure records a file that does not parse, at the line its parser blamed.
func (c *checker) addParseFailure(path string, err error) {
	var issueFailure *issue.Error
	var noteFailure *knowledge.Error
	switch {
	case errors.As(err, &issueFailure):
		c.add(path, issueFailure.Line, CodeFrontmatter, issueFailure.Message)
	case errors.As(err, &noteFailure):
		c.add(path, noteFailure.Line, CodeFrontmatter, noteFailure.Message)
	default:
		c.add(path, 0, CodeFrontmatter, err.Error())
	}
}

// graphNodes turns the tree's issues into dependency graph nodes. A file that does not parse is a
// node without deps, so a dependency on it still resolves by id.
func graphNodes(entries []parsedIssue) []issue.Node {
	nodes := make([]issue.Node, 0, len(entries))
	for _, entry := range entries {
		nodes = append(nodes, issue.Node{
			ID:       entry.node.ID,
			ParentID: entry.node.ParentID,
			Deps:     entry.issue.Deps,
			Aliases:  entry.node.Aliases,
		})
	}
	return nodes
}

// checkIssue checks one parsed issue: its fields against the config, its plan and its deps.
func (c *checker) checkIssue(entry parsedIssue, graph issue.DepGraph) {
	path := entry.node.Path
	for _, problem := range issue.Validate(c.relative(path), entry.issue, c.store.Config) {
		c.add(path, problem.Line, CodeField, problem.Message)
	}
	if plan := entry.issue.Plan; plan != "" && !c.isPlanFound(plan) {
		c.add(path, entry.issue.PlanLine(), CodeMissingPlan, fmt.Sprintf(
			"plan %q is in none of the plans directories %v",
			plan,
			c.store.Config.Plans,
		))
	}
	for _, dep := range entry.issue.Deps {
		if _, isFound := graph.Resolve(dep); !isFound {
			c.add(path, entry.issue.DepsLine(), CodeMissingDep, fmt.Sprintf(
				"dependency %q names no issue by id or by an alias only one issue has",
				dep,
			))
		}
	}
}

// isPlanFound reports whether plan, a basename, names a file in one of the configured plans
// directories, as written or with the .md extension.
func (c *checker) isPlanFound(plan string) bool {
	if filepath.Base(filepath.FromSlash(plan)) != plan {
		return false
	}
	for _, dir := range c.store.Config.Plans {
		base := filepath.Join(c.store.WorktreeRoot, filepath.FromSlash(dir), plan)
		for _, candidate := range []string{base + planExtension, base} {
			info, err := os.Stat(candidate)
			if err == nil && !info.IsDir() {
				return true
			}
		}
	}
	return false
}

// checkCycles records each dependency cycle once, at the deps line of the issue it starts with.
func (c *checker) checkCycles(entries []parsedIssue, graph issue.DepGraph) {
	byID := make(map[string]parsedIssue, len(entries))
	for _, entry := range entries {
		byID[entry.node.ID] = entry
	}
	for _, cycle := range graph.Cycles() {
		first := byID[cycle[0]]
		c.add(first.node.Path, first.issue.DepsLine(), CodeCycle, fmt.Sprintf(
			"dependency cycle: %s",
			strings.Join(cycle, " -> "),
		))
	}
}

// notes checks the Knowledge tree: its load problems, each Note's frontmatter and fields, and its
// Refs.
func (c *checker) notes() error {
	parsed := map[string]knowledge.Note{}
	readAliases := func(path string) ([]string, error) {
		data, err := c.readFile(path)
		if err != nil {
			return nil, err
		}
		found, err := knowledge.Parse(c.relative(path), data)
		if err != nil {
			c.addParseFailure(path, err)
			return nil, nil
		}
		parsed[path] = found
		return found.Aliases, nil
	}
	loaded, err := c.loadTree(c.store.KnowledgeDir(), readAliases)
	if err != nil {
		return err
	}

	for _, node := range loaded.Nodes() {
		note, isParsed := parsed[node.Path]
		if !isParsed {
			continue
		}
		if err := c.checkNote(node, note); err != nil {
			return err
		}
	}
	return nil
}

// checkNote checks one parsed Note: its fields, then whether each Ref resolves.
func (c *checker) checkNote(node tree.Node, note knowledge.Note) error {
	path := c.relative(node.Path)
	for _, problem := range knowledge.Validate(path, note) {
		c.add(path, problem.Line, CodeField, problem.Message)
	}
	findings, err := knowledge.Verify(c.store.WorktreeRoot, path, node.ID, note)
	if err != nil {
		return err
	}
	for _, finding := range findings {
		if refPath, _, _ := strings.Cut(finding.Ref, refSymbolSeparator); refPath == "" {
			continue // Validate already reported a Ref without a path
		}
		c.add(path, finding.Line, CodeBrokenRef, fmt.Sprintf("ref %q: %s", finding.Ref, finding.Message))
	}
	return nil
}
