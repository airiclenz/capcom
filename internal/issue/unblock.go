package issue

import (
	"slices"

	"github.com/airiclenz/capcom/internal/config"
)

// NewlyReady returns the ids of the todo issues that become Ready once the issues closed names
// take the categories it gives them, in node order. closed maps an issue id (never an alias) to
// its new category, normally done or dropped; an id that names no node is ignored. An issue
// already Ready before is not returned, nor is one the config's ready.hide hides. nodes are the
// whole tree, as Ready needs them, and are left unchanged. It returns nil when no issue becomes
// Ready.
func NewlyReady(nodes []Node, cfg config.Config, closed map[string]config.Category) []string {
	before := map[string]bool{}
	for _, verdict := range Ready(nodes, cfg) {
		before[verdict.ID] = verdict.IsReady
	}

	after := slices.Clone(nodes)
	for index := range after {
		if category, isClosing := closed[after[index].ID]; isClosing {
			after[index].Category = category
		}
	}

	var ready []string
	for _, verdict := range Ready(after, cfg) {
		if verdict.IsReady && !verdict.IsHidden && !before[verdict.ID] {
			ready = append(ready, verdict.ID)
		}
	}
	return ready
}
