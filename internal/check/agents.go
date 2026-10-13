package check

import (
	"errors"

	"github.com/airiclenz/capcom/internal/agents"
)

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
