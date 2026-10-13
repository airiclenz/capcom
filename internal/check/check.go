// Package check validates a whole store at once, for `capcom check`: the config, the issue tree
// and the Knowledge tree, read straight from the files and never from the index. It owns no rule
// of its own beyond resolving deps, related links, plans and superseded-by links, and checking that
// a Note's format-3 keys sit in a store declaring format 3; it collects what the config parser,
// the tree loader, the issue and Note parsers and validators, the git conflict-marker scan of
// package rawfm, the dependency cycle search and the Ref verifier report, each as a
// Problem citing the file and, when known, the line (ADR 0006). It also reports an AGENTS.md whose
// capcom Managed block is out of date or whose markers are tangled, and one that cannot be read,
// being not a regular file or larger than 1 MiB; an AGENTS.md without the block, or no AGENTS.md
// at all, is no problem.
package check

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/agents"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/rawfm"
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
	// CodeNotRegular is an entry left out unread because it is not a regular file (a symlink, a
	// named pipe, a device) or is a node file larger than 1 MiB.
	CodeNotRegular = "not-regular"
	// CodeFrontmatter is an issue or Note file that does not parse: a malformed frontmatter, an
	// unknown or repeated key, a value of the wrong shape.
	CodeFrontmatter = "frontmatter"
	// CodeConflict is a git conflict marker left in an issue or Note file by a merge.
	CodeConflict = "conflict"
	// CodeField is a parsed issue or Note whose values are not usable: a missing title, an
	// undeclared status or type, a closed issue without closed or reason, and the like.
	CodeField = "field"
	// CodeMissingDep is a dependency that names no issue.
	CodeMissingDep = "missing-dep"
	// CodeMissingRelated is a related link that names no issue.
	CodeMissingRelated = "missing-related"
	// CodeMissingSupersededBy is a superseded-by link that names no Note.
	CodeMissingSupersededBy = "missing-superseded-by"
	// CodeFormatTooOld is a Note key the store's declared format does not hold yet: retired or
	// superseded-by in a store below format 3.
	CodeFormatTooOld = "format-too-old"
	// CodeCycle is a dependency cycle.
	CodeCycle = "cycle"
	// CodeMissingPlan is a plan found in none of the configured plans directories.
	CodeMissingPlan = "missing-plan"
	// CodeBrokenRef is a Note Ref that does not resolve in the working tree.
	CodeBrokenRef = "broken-ref"
	// CodeAgentsBlock is an AGENTS.md whose Managed block is stale or whose markers are tangled,
	// or one that is not a regular file or is larger than 1 MiB.
	CodeAgentsBlock = "agents-block"
)

// refSymbolSeparator splits a Note Ref into its path and symbol.
const refSymbolSeparator = "#"

// Store formats of the supersession check (ADR 0009).
const (
	// supersessionFormatVersion is the first store format whose Note files may carry the retired
	// and superseded-by keys.
	supersessionFormatVersion = 3
	// missingConfigFormat is the format of a store without config.yaml: a missing format key means
	// format 1, and config.Default stands in for the file at the current format only so that the
	// other config defaults apply.
	missingConfigFormat = 1
)

// The Note keys a store needs format 3 for, as Problems name them.
const (
	keyRetired      = "retired"
	keySupersededBy = "superseded-by"
)

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
	if err := run.agentsBlock(); err != nil {
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

// loadTree loads the tree in dir, recording each of its load problems with every file it names
// relative to the working tree root, and returns what could be placed. aliasesLine returns the
// line of the aliases key in the file at a path aliases parsed, or 0; an alias problem cites it.
func (c *checker) loadTree(
	dir string,
	aliases tree.AliasFunc,
	aliasesLine func(path string) int,
) (*tree.Tree, error) {
	loaded, err := tree.Load(dir, aliases)
	var problems tree.Errors
	if !errors.As(err, &problems) {
		return loaded, err
	}
	for _, problem := range problems {
		relative := *problem
		relative.Other = c.relative(problem.Other)
		line := 0
		if problem.Alias != "" {
			line = aliasesLine(problem.Path)
		}
		c.add(problem.Path, line, treeCode(problem.Err), relative.Message())
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
	case errors.Is(err, tree.ErrNotRegular), errors.Is(err, tree.ErrTooLarge):
		return CodeNotRegular
	default:
		return CodeDuplicate
	}
}

// readFile reads the issue or Note file at path through nodefile.ReadNodeFile. A file it refuses
// unread, one no longer a regular file or grown past 1 MiB since the walk, is recorded as a
// not-regular problem, as the walk records one, and isRead is false with no error. Any other
// failed read is an error naming the file.
func (c *checker) readFile(path string) (data []byte, isRead bool, err error) {
	data, err = nodefile.ReadNodeFile(path)
	for _, refusal := range []error{tree.ErrNotRegular, tree.ErrTooLarge} {
		if errors.Is(err, refusal) {
			c.add(path, 0, CodeNotRegular, refusal.Error())
			return nil, false, nil
		}
	}
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", c.relative(path), err)
	}
	return data, true, nil
}

// parsedIssue is one issue file of the tree with what parsing it gave.
type parsedIssue struct {
	node     tree.Node
	issue    issue.Issue
	isParsed bool
}

// issues checks the issue tree: its load problems, each file's frontmatter and fields, its plan,
// its deps and the dependency cycles. The dependency graph holds the issue files Load left out for
// a repeated id too, so a dep on one of them is no missing dep and a cycle through one is reported,
// but only the loaded issues are checked one by one.
func (c *checker) issues() error {
	parsed := map[string]issue.Issue{}
	readAliases := func(path string) ([]string, error) {
		data, isRead, err := c.readFile(path)
		if !isRead {
			return nil, err
		}
		c.checkConflictMarkers(path, data)
		found, err := issue.Parse(c.relative(path), data)
		if err != nil {
			c.addParseFailure(path, err)
			return nil, nil
		}
		parsed[path] = found
		return found.Aliases, nil
	}
	aliasesLine := func(path string) int { return parsed[path].AliasesLine() }
	loaded, err := c.loadTree(c.store.IssuesDir(), readAliases, aliasesLine)
	if err != nil {
		return err
	}

	loadedCount := len(loaded.Nodes())
	nodes := slices.Concat(loaded.Nodes(), loaded.LeftOut())
	entries := make([]parsedIssue, 0, len(nodes))
	for _, node := range nodes {
		found, isParsed := parsed[node.Path]
		entries = append(entries, parsedIssue{node: node, issue: found, isParsed: isParsed})
	}
	graph := issue.NewDepGraph(graphNodes(entries))
	for _, entry := range entries[:loadedCount] {
		if entry.isParsed {
			c.checkIssue(entry, graph)
		}
	}
	c.checkCycles(entries, graph)
	return nil
}

// checkConflictMarkers records every git conflict marker in the issue or Note file at path.
func (c *checker) checkConflictMarkers(path string, data []byte) {
	for _, marker := range rawfm.ConflictMarkers(data) {
		c.add(path, marker.Line, CodeConflict, rawfm.ConflictMessage(marker))
	}
}

// addParseFailure records a file that does not parse, at the line its parser blamed. A parser
// blames a conflict marker in the frontmatter with the conflict problem's own words, which
// checkConflictMarkers has already recorded, so that failure is not recorded twice.
func (c *checker) addParseFailure(path string, err error) {
	line, message := 0, err.Error()
	var failure *frontmatter.Error
	if errors.As(err, &failure) {
		line, message = failure.Line, failure.Message
	}
	conflict := Problem{Path: c.relative(path), Line: line, Code: CodeConflict, Message: message}
	if slices.Contains(c.problems, conflict) {
		return
	}
	c.add(path, line, CodeFrontmatter, message)
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

// checkIssue checks one parsed issue: its fields against the config, its plan, its deps and its
// related links.
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
	c.checkRelated(entry, graph)
}

// checkRelated checks the related links of one parsed issue: each must resolve to an issue other
// than the issue itself, by id or by an alias. Both problems cite the related key's line.
func (c *checker) checkRelated(entry parsedIssue, graph issue.DepGraph) {
	path := entry.node.Path
	for _, related := range entry.issue.Related {
		id, isFound := graph.Resolve(related)
		switch {
		case !isFound:
			c.add(path, entry.issue.RelatedLine(), CodeMissingRelated, fmt.Sprintf(
				"related %q names no issue by id or by an alias only one issue has",
				related,
			))
		case id == entry.node.ID:
			c.add(path, entry.issue.RelatedLine(), CodeField, fmt.Sprintf(
				"related %q names the issue itself",
				related,
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

// checkCycles records each dependency cycle once, at the deps line of the issue it starts with:
// the first of entries with that id.
func (c *checker) checkCycles(entries []parsedIssue, graph issue.DepGraph) {
	byID := make(map[string]parsedIssue, len(entries))
	for _, entry := range entries {
		if _, isSeen := byID[entry.node.ID]; !isSeen {
			byID[entry.node.ID] = entry
		}
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
		data, isRead, err := c.readFile(path)
		if !isRead {
			return nil, err
		}
		c.checkConflictMarkers(path, data)
		found, err := knowledge.Parse(c.relative(path), data)
		if err != nil {
			c.addParseFailure(path, err)
			return nil, nil
		}
		parsed[path] = found
		return found.Aliases, nil
	}
	aliasesLine := func(path string) int { return parsed[path].AliasesLine() }
	loaded, err := c.loadTree(c.store.KnowledgeDir(), readAliases, aliasesLine)
	if err != nil {
		return err
	}

	declaredFormat, err := c.declaredFormat()
	if err != nil {
		return err
	}
	nodes := loaded.Nodes()
	isRetired := retiredSubtrees(nodes, parsed)
	leftOut := leftOutNames(loaded)
	for _, node := range nodes {
		note, isParsed := parsed[node.Path]
		if !isParsed {
			continue
		}
		c.checkSupersession(node, note, loaded, leftOut, declaredFormat)
		if err := c.checkNote(node, note, isRetired[node.ID]); err != nil {
			return err
		}
	}
	return nil
}

// declaredFormat returns the store format config.yaml declares, or format 1 when the store has no
// config.yaml (ADR 0009).
func (c *checker) declaredFormat() (int, error) {
	path := c.store.ConfigPath()
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return missingConfigFormat, nil
	}
	if err != nil {
		return 0, fmt.Errorf("%s: %w", c.relative(path), err)
	}
	return c.store.Config.Format, nil
}

// leftOutNames returns the ids and aliases of the node files Load left out of loaded for a
// repeated id.
func leftOutNames(loaded *tree.Tree) map[string]bool {
	names := map[string]bool{}
	for _, node := range loaded.LeftOut() {
		names[node.ID] = true
		for _, alias := range node.Aliases {
			names[alias] = true
		}
	}
	return names
}

// checkSupersession checks the supersession keys of one parsed Note: each superseded-by entry must
// name a Note by id or by an alias, or one of leftOut, the names of the Note files Load left out
// for a repeated id, and the retired and superseded-by keys need a store declaring format 3,
// however they are set. An entry Validate rejects as no id is left to Validate.
func (c *checker) checkSupersession(
	node tree.Node,
	note knowledge.Note,
	notes *tree.Tree,
	leftOut map[string]bool,
	declaredFormat int,
) {
	path := node.Path
	for i, successor := range note.SupersededBy {
		if tree.ValidateAlias(successor) != nil || leftOut[successor] {
			continue
		}
		if _, isFound := notes.Lookup(successor); !isFound {
			c.add(path, note.SupersededByLine(i), CodeMissingSupersededBy, fmt.Sprintf(
				"superseded-by %q names no note by id or by an alias only one note has",
				successor,
			))
		}
	}
	if declaredFormat >= supersessionFormatVersion {
		return
	}
	keyLines := []struct {
		key  string
		line int
	}{
		{key: keyRetired, line: note.RetiredLine()},
		{key: keySupersededBy, line: note.SupersededByKeyLine()},
	}
	for _, keyLine := range keyLines {
		if keyLine.line == 0 {
			continue
		}
		c.add(path, keyLine.line, CodeFormatTooOld, fmt.Sprintf(
			"%s needs store format %d, but the store declares format %d: set format: %d in %s, "+
				"or run a capcom write verb that raises it",
			keyLine.key,
			supersessionFormatVersion,
			declaredFormat,
			supersessionFormatVersion,
			c.relative(c.store.ConfigPath()),
		))
	}
}

// retiredSubtrees returns the ids of the nodes that are retired or sit below a retired ancestor,
// read from the parsed Note files (by path) and never from the index. A Note that did not parse
// counts as not retired.
func retiredSubtrees(nodes []tree.Node, parsed map[string]knowledge.Note) map[string]bool {
	byID := make(map[string]tree.Node, len(nodes))
	for _, node := range nodes {
		byID[node.ID] = node
	}
	isRetired := map[string]bool{}
	for _, node := range nodes {
		for current, isFound := node, true; isFound; current, isFound = byID[current.ParentID] {
			if parsed[current.Path].IsRetired {
				isRetired[node.ID] = true
				break
			}
		}
	}
	return isRetired
}

// checkNote checks one parsed Note: its fields, then, unless isRetired says it sits in a retired
// subtree, whether each Ref resolves.
func (c *checker) checkNote(node tree.Node, note knowledge.Note, isRetired bool) error {
	path := c.relative(node.Path)
	for _, problem := range knowledge.Validate(path, note) {
		c.add(path, problem.Line, CodeField, problem.Message)
	}
	if isRetired {
		return nil
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

// agentsBlock checks the Managed block of AGENTS.md against the Knowledge tree's top level.
func (c *checker) agentsBlock() error {
	status, err := agents.Sync(c.store.WorktreeRoot, c.store.KnowledgeDir(), agents.ModeInspect)
	var tangled *agents.Error
	if errors.As(err, &tangled) {
		c.add(tangled.Path, tangled.Line, CodeAgentsBlock, tangled.Message)
		return nil
	}
	if err != nil {
		return err
	}
	if status.State == agents.StateStale {
		c.add(agents.FileName, status.Line, CodeAgentsBlock,
			"the capcom block is out of date; run capcom agents to refresh it")
	}
	return nil
}
