package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/agents"
	"github.com/airiclenz/capcom/internal/check"
	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/store"
)

// What capcom agents found or did, as the block value of its --json data.
const (
	blockAbsent    = "absent"
	blockCurrent   = "current"
	blockStale     = "stale"
	blockWritten   = "written"
	blockRefreshed = "refreshed"
)

// guideStates names each agents.GuideState as the state value of a guide in the --json data.
var guideStates = map[agents.GuideState]string{
	agents.GuideMissing:   "missing",
	agents.GuideReaches:   "reaches",
	agents.GuideUnreached: "unreached",
	agents.GuideImported:  "imported",
	agents.GuideOutside:   "outside",
}

// agentsData is the --json data of capcom agents.
type agentsData struct {
	// Path is the block file config.yaml names under agents.file (AGENTS.md by default), relative
	// to the working tree root.
	Path string `json:"path"`
	// Block is what the command found (--check) or did.
	Block string `json:"block"`
	// Line is the begin marker's line, when the block is present.
	Line int `json:"line,omitempty"`
	// Guides are the agent guides config.yaml lists under agents.guides, when the block file
	// carries the block, led by AGENTS.md itself when the block file is another file.
	Guides []agentsGuideData `json:"guides,omitempty"`
}

// agentsGuideData is one agent guide in the --json data of capcom agents.
type agentsGuideData struct {
	// Path is the guide, relative to the working tree root.
	Path string `json:"path"`
	// State is what the command found (--check) or did: missing, reaches, unreached, imported or
	// outside.
	State string `json:"state"`
}

// newAgentsGuideData returns the --json data of guides.
func newAgentsGuideData(guides []agents.Guide) []agentsGuideData {
	data := make([]agentsGuideData, 0, len(guides))
	for _, guide := range guides {
		data = append(data, agentsGuideData{Path: guide.Path, State: guideStates[guide.State]})
	}
	return data
}

// The help text of capcom agents.
const (
	agentsLong = "Write or refresh capcom's Managed block in the AGENTS.md at the working tree root: the " +
		"section between <!-- BEGIN CAPCOM --> and <!-- END CAPCOM --> that tells agents how to use " +
		"capcom and lists the top level of the Knowledge tree.\n\n" +
		"A stale block is replaced, an absent one appended and a missing AGENTS.md created; the rest " +
		"of the file is left alone.\n\n" +
		"Each agent guide config.yaml lists under agents.guides (CLAUDE.md and GEMINI.md by default) " +
		"that exists but neither links to AGENTS.md nor imports it gets an @AGENTS.md import line " +
		"appended; a missing guide is not created, and one that links outside the working tree is " +
		"skipped.\n\n" +
		"--check writes nothing: it reports whether the block is current, and fails " +
		"invalid when it is stale or a guide does not reach it, so CI catches a block that has " +
		"drifted; a file without the block passes."
	agentsExample = "  capcom agents           # write or refresh the capcom block of AGENTS.md, import it into the guides\n" +
		"  capcom agents --check   # fail when the block is out of date or a guide does not reach it"
)

// newAgentsCommand writes or refreshes the capcom Managed block of the configured block file
// (agents.file, AGENTS.md by default) and makes AGENTS.md and the agent guides reach it, or with
// --check reports whether the block is current and they reach it. --check fails on a stale block,
// an unreached guide or capcom markers left in an AGENTS.md that is not the block file only: a
// block file without the block, or no block file, is fine. Without --check the run is noted as a write of no Issue for the Tree lock
// warning (app.noteWrite).
func (a *app) newAgentsCommand() *cobra.Command {
	isCheck := false
	cmd := &cobra.Command{
		Use:         "agents",
		Short:       "Write or refresh the capcom block of AGENTS.md",
		Long:        agentsLong,
		Example:     agentsExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: storeOnly},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			if isCheck {
				return a.checkAgentsBlock(cmd, a.store)
			}
			a.noteWrite()
			return a.writeAgentsBlock(cmd, a.store)
		}),
	}
	cmd.Flags().BoolVar(&isCheck, flagCheck, false, "report whether the block is current; fail when it is stale or a guide does not reach it")
	return cmd
}

// writeAgentsBlock brings the block of the configured block file up to date, appends the import
// line to AGENTS.md when it is not the block file and does not reach it, and to each agent guide
// that does not reach AGENTS.md, and reports what it did.
func (a *app) writeAgentsBlock(cmd *cobra.Command, opened *store.Store) error {
	blockFile := opened.Config.Agents.File
	status, err := syncAgentsBlock(opened, agents.ModeWrite)
	if err != nil {
		return agentsFailure(err)
	}
	data := agentsData{Path: blockFile, Block: blockWritten}
	text := "wrote the capcom block to " + blockFile + "\n"
	switch status.State {
	case agents.StateCurrent:
		data.Block = blockCurrent
		text = "the capcom block of " + blockFile + " is current\n"
	case agents.StateStale:
		data.Block = blockRefreshed
		text = "refreshed the capcom block of " + blockFile + "\n"
	}
	guides, err := reachingGuides(opened, agents.ModeWrite)
	if err != nil {
		return agentsFailure(err)
	}
	data.Guides = newAgentsGuideData(guides)
	return a.emit(cmd, output.Result{Kind: "agents", Data: data, Text: text + guidesText(guides)})
}

// syncAgentsBlock reconciles the capcom block of the block file opened configures (agents.file,
// AGENTS.md by default) with its Knowledge top level and its configured doing status, as mode says;
// see agents.Sync.
func syncAgentsBlock(opened *store.Store, mode agents.Mode) (agents.Status, error) {
	// No doing status configured is the empty string, which leaves the claim line out.
	doingStatus, _ := opened.Config.DefaultStatus(config.CategoryDoing)
	return agents.Sync(opened.WorktreeRoot, opened.Config.Agents.File, opened.KnowledgeDir(), doingStatus, mode)
}

// reachingGuides reports, as mode says, whether each file that must reach the block does: AGENTS.md
// first when the block file opened configures is another file (see agents.AgentsGuide), then each
// agent guide config.yaml lists under agents.guides (see agents.Guides). Callers run it only once
// the block file carries the block.
func reachingGuides(opened *store.Store, mode agents.Mode) ([]agents.Guide, error) {
	agentsGuide, err := agents.AgentsGuide(opened.WorktreeRoot, opened.Config.Agents.File, mode)
	if err != nil {
		return nil, err
	}
	guides, err := agents.Guides(opened.WorktreeRoot, opened.Config.Agents.File, opened.Config.Agents.Guides, mode)
	if err != nil {
		return nil, err
	}
	if agentsGuide.Path == "" {
		return guides, nil
	}
	return append([]agents.Guide{agentsGuide}, guides...), nil
}

// guidesText reports each guide capcom agents added the import line to, and each it skipped
// because it links outside the working tree, one line each.
func guidesText(guides []agents.Guide) string {
	var text strings.Builder
	for _, guide := range guides {
		switch guide.State {
		case agents.GuideImported:
			text.WriteString("added " + guide.ImportLine + " to " + guide.Path + "\n")
		case agents.GuideOutside:
			text.WriteString("skipped " + guide.Path + ": links outside the working tree\n")
		}
	}
	return text.String()
}

// checkAgentsBlock reports the block's state and the guides'; a stale block fails invalid at its
// marker line, and, the block current, the first file that does not reach it fails invalid at that
// file: AGENTS.md, when the block file is another file, then the agent guides. Each failure hints
// capcom agents, which refreshes the block and adds the import lines. The block current, an
// AGENTS.md that is not the block file but still holds capcom markers fails invalid too, with a
// hint to remove them.
func (a *app) checkAgentsBlock(cmd *cobra.Command, opened *store.Store) error {
	blockFile := opened.Config.Agents.File
	status, err := syncAgentsBlock(opened, agents.ModeInspect)
	if err != nil {
		return agentsFailure(err)
	}
	data := agentsData{Path: blockFile, Block: blockAbsent, Line: status.Line}
	text := blockFile + " has no capcom block\n"
	switch status.State {
	case agents.StateCurrent:
		data.Block = blockCurrent
		text = "the capcom block of " + blockFile + " is current\n"
	case agents.StateStale:
		return &output.Error{
			Code:    output.CodeInvalid,
			Path:    blockFile,
			Line:    status.Line,
			Message: "the capcom block is out of date",
			Hint:    agentsCommand,
		}
	}
	if status.State != agents.StateCurrent {
		return a.emit(cmd, output.Result{Kind: "agents", Data: data, Text: text})
	}
	guides, err := reachingGuides(opened, agents.ModeInspect)
	if err != nil {
		return agentsFailure(err)
	}
	if err := guidesFailure(guides, blockFile); err != nil {
		return err
	}
	data.Guides = newAgentsGuideData(guides)
	return a.emit(cmd, output.Result{Kind: "agents", Data: data, Text: text})
}

// guidesFailure is the failure of capcom agents --check for guides, as reachingGuides reports
// them against blockFile: the first one that does not reach the block, invalid at it and naming
// the file it must reach (blockFile for AGENTS.md, AGENTS.md for an agent guide), or else an
// AGENTS.md that still holds capcom markers, invalid at AGENTS.md. It is nil when every guide
// reaches the block.
func guidesFailure(guides []agents.Guide, blockFile string) error {
	for _, guide := range guides {
		if guide.State != agents.GuideUnreached {
			continue
		}
		target := agents.FileName
		if guide.Path == agents.FileName {
			target = blockFile
		}
		return &output.Error{
			Code:    output.CodeInvalid,
			Path:    guide.Path,
			Message: "does not reach the capcom block in " + target + "; it needs the line " + guide.ImportLine,
			Hint:    agentsCommand,
		}
	}
	for _, guide := range guides {
		if guide.HasMarkers {
			return &output.Error{
				Code:    output.CodeInvalid,
				Path:    guide.Path,
				Message: check.StrayMarkersMessage(blockFile),
				Hint:    check.StrayMarkersHint,
			}
		}
	}
	return nil
}

// agentsCommand is the command that refreshes the capcom block and makes every agent guide reach
// it.
const agentsCommand = "capcom agents"

// agentsFailure types tangled markers as invalid at the block file and its line, a block file that
// cannot be read (not a regular file, or larger than 1 MiB) or that links outside the working tree
// as invalid at the block file, and an agent guide, AGENTS.md among them, that cannot be read as
// invalid at the guide; nil stays nil.
func agentsFailure(err error) error {
	var tangled *agents.Error
	if errors.As(err, &tangled) {
		return &output.Error{Code: output.CodeInvalid, Path: tangled.Path, Line: tangled.Line, Message: tangled.Message}
	}
	return err
}
