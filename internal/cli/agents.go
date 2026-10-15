package cli

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/agents"
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
	// Path is AGENTS.md, relative to the working tree root.
	Path string `json:"path"`
	// Block is what the command found (--check) or did.
	Block string `json:"block"`
	// Line is the begin marker's line, when the block is present.
	Line int `json:"line,omitempty"`
	// Guides are the agent guides config.yaml lists under agents.guides, when AGENTS.md carries
	// the block.
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
		"capcom and lists the top level of the Knowledge tree. A stale block is replaced, an absent " +
		"one appended and a missing AGENTS.md created; the rest of the file is left alone. Each agent " +
		"guide config.yaml lists under agents.guides (CLAUDE.md and GEMINI.md by default) that " +
		"exists but neither links to AGENTS.md nor imports it gets an @AGENTS.md import line " +
		"appended; a missing guide is not created, and one that links outside the working tree is " +
		"skipped. --check writes nothing: it reports whether the block is current, and fails " +
		"invalid when it is stale or a guide does not reach it, so CI catches a block that has " +
		"drifted; a file without the block passes."
	agentsExample = "  capcom agents           # write or refresh the capcom block of AGENTS.md, import it into the guides\n" +
		"  capcom agents --check   # fail when the block is out of date or a guide does not reach it"
)

// newAgentsCommand writes or refreshes the capcom Managed block of AGENTS.md and imports it into
// the agent guides, or with --check reports whether the block is current and the guides reach it.
// --check fails on a stale block or an unreached guide only: an AGENTS.md without the block, or no
// AGENTS.md, is fine.
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
			return a.writeAgentsBlock(cmd, a.store)
		}),
	}
	cmd.Flags().BoolVar(&isCheck, flagCheck, false, "report whether the block is current; fail when it is stale or a guide does not reach it")
	return cmd
}

// writeAgentsBlock brings the block up to date, appends the import line to each agent guide that
// does not reach it, and reports what it did.
func (a *app) writeAgentsBlock(cmd *cobra.Command, opened *store.Store) error {
	status, err := syncAgentsBlock(opened, agents.ModeWrite)
	if err != nil {
		return agentsFailure(err)
	}
	data := agentsData{Path: agents.FileName, Block: blockWritten}
	text := "wrote the capcom block to " + agents.FileName + "\n"
	switch status.State {
	case agents.StateCurrent:
		data.Block = blockCurrent
		text = "the capcom block of " + agents.FileName + " is current\n"
	case agents.StateStale:
		data.Block = blockRefreshed
		text = "refreshed the capcom block of " + agents.FileName + "\n"
	}
	guides, err := agents.Guides(opened.WorktreeRoot, opened.Config.Agents.Guides, agents.ModeWrite)
	if err != nil {
		return agentsFailure(err)
	}
	data.Guides = newAgentsGuideData(guides)
	return a.emit(cmd, output.Result{Kind: "agents", Data: data, Text: text + guidesText(guides)})
}

// syncAgentsBlock reconciles the capcom block of the AGENTS.md of opened with its Knowledge top level
// and its configured doing status, as mode says; see agents.Sync.
func syncAgentsBlock(opened *store.Store, mode agents.Mode) (agents.Status, error) {
	// No doing status configured is the empty string, which leaves the claim line out.
	doingStatus, _ := opened.Config.DefaultStatus(config.CategoryDoing)
	return agents.Sync(opened.WorktreeRoot, opened.KnowledgeDir(), doingStatus, mode)
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
// marker line, and, the block current, the first guide that does not reach it fails invalid at
// that guide.
func (a *app) checkAgentsBlock(cmd *cobra.Command, opened *store.Store) error {
	status, err := syncAgentsBlock(opened, agents.ModeInspect)
	if err != nil {
		return agentsFailure(err)
	}
	data := agentsData{Path: agents.FileName, Block: blockAbsent, Line: status.Line}
	text := agents.FileName + " has no capcom block\n"
	switch status.State {
	case agents.StateCurrent:
		data.Block = blockCurrent
		text = "the capcom block of " + agents.FileName + " is current\n"
	case agents.StateStale:
		return &output.Error{
			Code:    output.CodeInvalid,
			Path:    agents.FileName,
			Line:    status.Line,
			Message: "the capcom block is out of date; run capcom agents to refresh it",
		}
	}
	guides, err := agents.Guides(opened.WorktreeRoot, opened.Config.Agents.Guides, agents.ModeInspect)
	if err != nil {
		return agentsFailure(err)
	}
	for _, guide := range guides {
		if guide.State == agents.GuideUnreached {
			return &output.Error{Code: output.CodeInvalid, Path: guide.Path, Message: guide.Problem()}
		}
	}
	data.Guides = newAgentsGuideData(guides)
	return a.emit(cmd, output.Result{Kind: "agents", Data: data, Text: text})
}

// agentsFailure types tangled markers as invalid at AGENTS.md and its line, an AGENTS.md that
// cannot be read (not a regular file, or larger than 1 MiB) or that links outside the working tree
// as invalid at AGENTS.md, and an agent guide that cannot be read as invalid at the guide; nil
// stays nil.
func agentsFailure(err error) error {
	var tangled *agents.Error
	if errors.As(err, &tangled) {
		return &output.Error{Code: output.CodeInvalid, Path: tangled.Path, Line: tangled.Line, Message: tangled.Message}
	}
	return err
}
