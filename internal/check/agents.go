package check

import (
	"errors"

	"github.com/airiclenz/capcom/internal/agents"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
)

// agentsBlock checks the Managed block of the configured block file (agents.file, AGENTS.md by
// default) against the Knowledge tree's top level and the configured doing status, and, when the
// block file carries the block, that AGENTS.md, when it is not the block file, and each configured
// agent guide reach it.
func (c *checker) agentsBlock() error {
	blockFile := c.store.Config.Agents.File
	// No doing status configured is the empty string, which leaves the claim line out.
	doingStatus, _ := c.store.Config.DefaultStatus(config.CategoryDoing)
	status, err := agents.Sync(c.store.WorktreeRoot, blockFile, c.store.KnowledgeDir(), doingStatus, agents.ModeInspect)
	var tangled *agents.Error
	if errors.As(err, &tangled) {
		c.add(tangled.Path, tangled.Line, diagnose.AgentsBlock, tangled.Message)
		return nil
	}
	if err != nil {
		return err
	}
	if status.State == agents.StateStale {
		c.add(blockFile, status.Line, diagnose.AgentsBlock,
			"the capcom block is out of date; run capcom agents to refresh it")
	}
	if status.State != agents.StateCurrent && status.State != agents.StateStale {
		return nil
	}
	if err := c.agentsFile(); err != nil {
		return err
	}
	return c.agentsGuides()
}

// agentsFile reports an AGENTS.md that is not the configured block file and does not reach it, one
// that is missing, which leaves the block reached by no agent guide, one that still holds capcom
// markers, and one that cannot be read. With AGENTS.md as the block file it reports nothing.
func (c *checker) agentsFile() error {
	blockFile := c.store.Config.Agents.File
	guide, err := agents.AgentsGuide(c.store.WorktreeRoot, blockFile, agents.ModeInspect)
	var unreadable *agents.Error
	if errors.As(err, &unreadable) {
		c.add(unreadable.Path, unreadable.Line, diagnose.AgentsGuide, unreadable.Message)
		return nil
	}
	if err != nil {
		return err
	}
	if guide.State == agents.GuideUnreached {
		c.add(guide.Path, 0, diagnose.AgentsGuide, guide.Problem())
	}
	if guide.IsMissingAGENTSMd() {
		c.add(guide.Path, 0, diagnose.AgentsGuide,
			MissingAGENTSMdMessage(blockFile)+"; "+MissingAGENTSMdHint(guide.ImportLine))
	}
	if guide.HasMarkers {
		c.add(guide.Path, 0, diagnose.AgentsGuide, StrayMarkersMessage(blockFile)+"; "+StrayMarkersHint)
	}
	return nil
}

// MissingAGENTSMdMessage reports a missing AGENTS.md while blockFile, another file, holds the
// Managed block, so that no agent guide reaches it, without AGENTS.md's path: callers put it before
// the message.
func MissingAGENTSMdMessage(blockFile string) string {
	return "is missing, so no agent guide reaches the capcom block in " + blockFile
}

// MissingAGENTSMdHint is what makes the block MissingAGENTSMdMessage reports reachable again:
// AGENTS.md created holding importLine. capcom never creates AGENTS.md while agents.file names
// another file, which may live there because AGENTS.md cannot be written.
func MissingAGENTSMdHint(importLine string) string {
	return "create " + agents.FileName + " holding the line " + importLine
}

// StrayMarkersMessage reports capcom markers left in AGENTS.md while blockFile, another file, holds
// the Managed block, without AGENTS.md's path: callers put it before the message.
func StrayMarkersMessage(blockFile string) string {
	return "holds capcom markers, but the capcom block lives in " + blockFile
}

// StrayMarkersHint is what clears capcom markers StrayMarkersMessage reports: capcom never removes
// them.
const StrayMarkersHint = "remove the capcom markers and what lies between them from " + agents.FileName

// agentsGuides reports each configured agent guide that does not reach the Managed block, and one
// that cannot be read.
func (c *checker) agentsGuides() error {
	guides, err := agents.Guides(c.store.WorktreeRoot, c.store.Config.Agents.File, c.store.Config.Agents.Guides, agents.ModeInspect)
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
