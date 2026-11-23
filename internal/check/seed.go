package check

import (
	"fmt"

	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/seed"
	"github.com/airiclenz/capcom/internal/store"
)

// outdatedSeedWarning is the warning for one outdated seeded Note: its file relative to the
// working tree root, its id, and its id again for the command that prints the new text.
const outdatedSeedWarning = "%s: seeded Note %s is outdated: capcom know seed --update replaces it, " +
	"capcom know seed --show %s prints the new text"

// SeedWarnings returns one warning for every Note of the opened store s that is an outdated
// seeded Note: its text is still the text capcom seeded it with, and the base set this binary
// ships holds another text for its id (seed.Outdated). Each names the Note's file and the command
// that replaces it; they are ordered by id. A Note that is retired or sits below a retired Note
// gives none, and neither does one that does not parse or that the tree loader left out.
//
// A warning is no Problem: Run does not report it, and a store whose only finding is a warning
// is valid. SeedWarnings reads the files as Run does and fails where Run fails, when a store file
// or directory cannot be read.
func SeedWarnings(s *store.Store) ([]string, error) {
	set, err := seed.Load()
	if err != nil {
		return nil, err
	}

	// The checker is used for its reads alone: the problems it records are Run's to report.
	run := checker{store: s}
	parsed := map[string]knowledge.Note{}
	reader := nodestore.NoteReader()
	// loadTree calls readAliases concurrently: the file is read and decoded unlocked, and parsed
	// is written holding run.mu.
	readAliases := func(path string) ([]string, error) {
		data, isRead, err := run.readFile(path)
		if !isRead {
			return nil, err
		}
		reading := reader.Decode(s.Relative(path), data)
		if !reading.IsParsed() {
			return nil, nil
		}
		run.mu.Lock()
		defer run.mu.Unlock()
		parsed[path] = reading.Node
		return reading.Aliases, nil
	}
	aliasesLine := func(path string) int { return parsed[path].AliasesLine() }
	loaded, err := run.loadTree(s.KnowledgeDir(), "Note", readAliases, aliasesLine)
	if err != nil {
		return nil, err
	}

	nodes := loaded.Nodes()
	isRetired := retiredSubtrees(nodes, parsed)
	var warnings []string
	for _, node := range nodes {
		note, isParsed := parsed[node.Path]
		if !isParsed || isRetired[node.ID] {
			continue
		}
		if set.State(node.ID, note) == seed.Outdated {
			warnings = append(warnings, fmt.Sprintf(outdatedSeedWarning, s.Relative(node.Path), node.ID, node.ID))
		}
	}
	return warnings, nil
}
