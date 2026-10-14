package check

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// planExtension is the extension a plan basename is looked up with, besides the bare name.
const planExtension = ".md"

// keyRelated is the issue key a store needs format 2 for, as a Problem names it.
const keyRelated = "related"

// parsedIssue is one issue file of the tree with what parsing it gave.
type parsedIssue struct {
	node     tree.Node
	issue    issue.Issue
	isParsed bool
	// fields is the Field findings of the file's verdict.
	fields []diagnose.Finding
}

// issues checks the issue tree: its load problems, each file's frontmatter and fields, its plan,
// its deps and the dependency cycles. The dependency graph holds the issue files Load left out for
// a repeated id too, so a dep on one of them is no missing dep and a cycle through one is reported,
// but only the loaded issues are checked one by one.
func (c *checker) issues() error {
	parsed := map[string]issue.Issue{}
	fields := map[string][]diagnose.Finding{}
	readAliases := func(path string) ([]string, error) {
		data, isRead, err := c.readFile(path)
		if !isRead {
			return nil, err
		}
		c.checkConflictMarkers(path, data)
		id := tree.IDOf(filepath.Base(path))
		verdict := diagnose.IssueFile(c.relative(path), id, data, c.store.Config)
		if !verdict.IsParsed() {
			c.addParseFailure(path, verdict.Parse[0])
			return nil, nil
		}
		parsed[path] = verdict.Parsed
		fields[path] = verdict.Fields
		return verdict.Parsed.Aliases, nil
	}
	aliasesLine := func(path string) int { return parsed[path].AliasesLine() }
	loaded, err := c.loadTree(c.store.IssuesDir(), "issue", readAliases, aliasesLine)
	if err != nil {
		return err
	}

	loadedCount := len(loaded.Nodes())
	nodes := slices.Concat(loaded.Nodes(), loaded.LeftOut())
	entries := make([]parsedIssue, 0, len(nodes))
	for _, node := range nodes {
		found, isParsed := parsed[node.Path]
		entries = append(entries, parsedIssue{
			node:     node,
			issue:    found,
			isParsed: isParsed,
			fields:   fields[node.Path],
		})
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

// checkIssue checks one parsed issue: its fields against the config, as its verdict found them, its
// plan, its deps and its related links, and that its related key, however it is set, sits in a
// store declaring format 2.
func (c *checker) checkIssue(entry parsedIssue, graph issue.DepGraph) {
	path := entry.node.Path
	for _, finding := range entry.fields {
		c.add(path, finding.Line, kindCode(finding.Code), finding.Message)
	}
	if plan := entry.issue.Plan; plan != "" && !c.isPlanFound(plan) {
		c.add(path, entry.issue.PlanLine(), diagnose.MissingPlan, fmt.Sprintf(
			"plan %q is in none of the plans directories %v",
			plan,
			c.store.Config.Plans,
		))
	}
	for _, dep := range entry.issue.Deps {
		if _, isFound := graph.Resolve(dep); !isFound {
			c.add(path, entry.issue.DepsLine(), diagnose.MissingDep, fmt.Sprintf(
				"dependency %q names no issue by id or by an alias only one issue has",
				dep,
			))
		}
	}
	c.checkRelated(entry, graph)
	if line := entry.issue.RelatedLine(); line > 0 && c.store.Config.Format < config.FormatRelated {
		c.addFormatTooOld(path, line, keyRelated, config.FormatRelated, c.store.Config.Format)
	}
}

// checkRelated checks the related links of one parsed issue: each must resolve to an issue other
// than the issue itself, by id or by an alias. Both problems cite the related key's line.
func (c *checker) checkRelated(entry parsedIssue, graph issue.DepGraph) {
	path := entry.node.Path
	for _, related := range entry.issue.Related {
		id, isFound := graph.Resolve(related)
		switch {
		case !isFound:
			c.add(path, entry.issue.RelatedLine(), diagnose.MissingRelated, fmt.Sprintf(
				"related %q names no issue by id or by an alias only one issue has",
				related,
			))
		case id == entry.node.ID:
			c.add(path, entry.issue.RelatedLine(), diagnose.Field, fmt.Sprintf(
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
		c.add(first.node.Path, first.issue.DepsLine(), diagnose.Cycle, fmt.Sprintf(
			"dependency cycle: %s",
			strings.Join(cycle, " -> "),
		))
	}
}
