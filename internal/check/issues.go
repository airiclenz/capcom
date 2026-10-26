package check

import (
	"fmt"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/tree"
)

// parsedIssue is one issue file of the tree with what parsing it gave.
type parsedIssue struct {
	node     tree.Node
	issue    issue.Issue
	isParsed bool
	// fields is the Field findings of the file's Reading.
	fields []diagnose.Finding
}

// issues checks the issue tree: its load problems, each file's frontmatter and fields, its plan,
// its deps and the dependency cycles. The dependency graph holds the issue files Load left out for
// a repeated id too, so a dep on one of them is no missing dep and a cycle through one is reported,
// but only the loaded issues are checked one by one.
func (c *checker) issues() error {
	parsed := map[string]issue.Issue{}
	fields := map[string][]diagnose.Finding{}
	reader := nodestore.IssueReader(c.store.Config)
	// Only the reading's Field findings are check's own: its parse finding goes to addParseFailure,
	// and loadTree reports the alias problems.
	readAliases := func(path string) ([]string, error) {
		data, isRead, err := c.readFile(path)
		if !isRead {
			return nil, err
		}
		c.checkConflictMarkers(path, data)
		reading := reader.Decode(c.store.Relative(path), data)
		if !reading.IsParsed() {
			c.addParseFailure(path, reading.Parse[0])
			return nil, nil
		}
		parsed[path] = reading.Node
		fields[path] = reading.Fields
		return reading.Aliases, nil
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

// checkIssue checks one parsed issue: its fields against the config, as its Reading found them, its
// plan, its standing mark, its deps and its related links, and that each key line of the issue
// tree's format table, whatever it holds, sits in a store declaring the key's format.
func (c *checker) checkIssue(entry parsedIssue, graph issue.DepGraph) {
	path := entry.node.Path
	for _, finding := range entry.fields {
		c.add(path, finding.Line, kindCode(finding.Code), finding.Message)
	}
	if plan := entry.issue.Plan; plan != "" {
		if _, isFound := c.store.Config.PlanPath(c.store.WorktreeRoot, plan); !isFound {
			c.add(path, entry.issue.PlanLine(), diagnose.MissingPlan, fmt.Sprintf(
				"plan %q is in none of the plans directories %v",
				plan,
				c.store.Config.Plans,
			))
		}
	}
	c.checkStanding(entry)
	for _, dep := range entry.issue.Deps {
		if _, isFound := graph.Resolve(dep); !isFound {
			c.add(path, entry.issue.DepsLine(), diagnose.MissingDep, fmt.Sprintf(
				"dependency %q names no issue by id or by an alias only one issue has",
				dep,
			))
		}
	}
	c.checkRelated(entry, graph)
	formats := nodestore.IssueDescriptor(c.store.Config).Formats
	addKeysFormatTooOld(c, path, entry.issue, formats, c.store.Config.Format)
}

// checkStanding reports a standing issue whose status is in a closed category or the doing
// category, at the standing key's line. A status the config does not declare is a Field problem
// already, so it is left alone here.
func (c *checker) checkStanding(entry parsedIssue) {
	if !entry.issue.IsStanding {
		return
	}
	status := entry.issue.Status
	category, isDeclared := c.store.Config.CategoryOf(status)
	if !isDeclared || (!category.IsClosed() && category != config.CategoryDoing) {
		return
	}
	c.add(entry.node.Path, entry.issue.StandingLine(), diagnose.Standing, fmt.Sprintf(
		"standing issue has status %q in the %s category; a standing issue only groups its "+
			"children and is never claimed or closed: move it out of that category, or run "+
			"capcom update %s --no-standing",
		status,
		category,
		entry.node.ID,
	))
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
