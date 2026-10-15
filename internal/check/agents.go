package check

import (
	"errors"

	"github.com/airiclenz/capcom/internal/agents"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
)

// agentsBlock checks the Managed block of AGENTS.md against the Knowledge tree's top level and the
// configured doing status, and, when AGENTS.md carries the block, that each configured agent guide
// reaches it.
func (c *checker) agentsBlock() error {
	// No doing status configured is the empty string, which leaves the claim line out.
	doingStatus, _ := c.store.Config.DefaultStatus(config.CategoryDoing)
	status, err := agents.Sync(c.store.WorktreeRoot, c.store.KnowledgeDir(), doingStatus, agents.ModeInspect)
	var tangled *agents.Error
	if errors.As(err, &tangled) {
		c.add(tangled.Path, tangled.Line, diagnose.AgentsBlock, tangled.Message)
		return nil
	}
	if err != nil {
		return err
	}
	if status.State == agents.StateStale {
		c.add(agents.FileName, status.Line, diagnose.AgentsBlock,
			"the capcom block is out of date; run capcom agents to refresh it")
	}
	if status.State != agents.StateCurrent && status.State != agents.StateStale {
		return nil
	}
	return c.agentsGuides()
}

// agentsGuides reports each configured agent guide that does not reach the Managed block, and one
// that cannot be read.
func (c *checker) agentsGuides() error {
	guides, err := agents.Guides(c.store.WorktreeRoot, c.store.Config.Agents.Guides, agents.ModeInspect)
	var unreadable *agents.Error
	if errors.As(err, &unreadable) {
		c.add(unreadable.Path, unreadable.Line, diagnose.AgentsGuide, unreadable.Message)
		return nil
	}
	if err != nil {
		return err
	}
	for _, guide := range guides {
		if guide.State == agents.GuideUnreached {
			c.add(guide.Path, 0, diagnose.AgentsGuide, guide.Problem())
		}
	}
	return nil
}
