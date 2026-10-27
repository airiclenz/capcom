package check

import (
	"fmt"
	"strings"

	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/tree"
)

// refSymbolSeparator splits a Note Ref into its path and symbol.
const refSymbolSeparator = "#"

// notes checks the Knowledge tree: its load problems, each Note's frontmatter and fields, and its
// Refs.
func (c *checker) notes() error {
	parsed := map[string]knowledge.Note{}
	fields := map[string][]diagnose.Finding{}
	reader := nodestore.NoteReader()
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
	loaded, err := c.loadTree(c.store.KnowledgeDir(), "Note", readAliases, aliasesLine)
	if err != nil {
		return err
	}

	nodes := loaded.Nodes()
	isRetired := retiredSubtrees(nodes, parsed)
	leftOut := leftOutNames(loaded)
	var live []knowledge.NoteFile
	for _, node := range nodes {
		note, isParsed := parsed[node.Path]
		if !isParsed {
			continue
		}
		c.checkSupersession(node, note, loaded, leftOut, c.store.Config.Format)
		c.checkFields(node, fields[node.Path])
		if !isRetired[node.ID] {
			live = append(live, knowledge.NoteFile{ID: node.ID, Path: c.store.Relative(node.Path), Note: note})
		}
	}
	return c.checkRefs(live)
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
// for a repeated id, and each key line of the Knowledge tree's format table (retired and
// superseded-by), whatever it holds, needs a store declaring the key's format. An entry Validate rejects as no id is left to Validate.
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
			c.add(path, note.SupersededByLine(i), diagnose.MissingSupersededBy, fmt.Sprintf(
				"superseded-by %q names no Note by id or by an alias only one Note has",
				successor,
			))
		}
	}
	addKeysFormatTooOld(c, path, note, nodestore.NoteDescriptor().Formats, declaredFormat)
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

// checkFields reports fieldFindings, the Field findings of the Reading of the Note file at node.
func (c *checker) checkFields(node tree.Node, fieldFindings []diagnose.Finding) {
	for _, finding := range fieldFindings {
		c.add(node.Path, finding.Line, kindCode(finding.Code), finding.Message)
	}
}

// checkRefs checks the Refs of notes, the parsed Notes outside a retired subtree, in one pass:
// whether each resolves, and whether a clone of the repository would have it.
func (c *checker) checkRefs(notes []knowledge.NoteFile) error {
	findings, err := knowledge.Verify(nodestore.RefRepository(c.store), notes)
	if err != nil {
		return err
	}
	for _, finding := range findings {
		if refPath, _, _ := strings.Cut(finding.Ref, refSymbolSeparator); refPath == "" {
			continue // Validate already reported a Ref without a path
		}
		c.add(finding.Path, finding.Line, finding.Code, fmt.Sprintf("ref %q: %s", finding.Ref, finding.Message))
	}
	return nil
}
