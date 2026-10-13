package check

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/tree"
)

// refSymbolSeparator splits a Note Ref into its path and symbol.
const refSymbolSeparator = "#"

// The Note keys a store needs format 3 for, as Problems name them.
const (
	keyRetired      = "retired"
	keySupersededBy = "superseded-by"
)

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
		return config.MissingFormat, nil
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
	if declaredFormat >= config.FormatSupersession {
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
			config.FormatSupersession,
			declaredFormat,
			config.FormatSupersession,
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
