package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/agents"
	"github.com/airiclenz/capcom/internal/cli/output"
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

// agentsData is the --json data of capcom agents.
type agentsData struct {
	// Path is AGENTS.md, relative to the working tree root.
	Path string `json:"path"`
	// Block is what the command found (--check) or did.
	Block string `json:"block"`
	// Line is the begin marker's line, when the block is present.
	Line int `json:"line,omitempty"`
}

// The help text of capcom agents.
const (
	agentsLong = "Write or refresh capcom's Managed block in the AGENTS.md at the working tree root: the " +
		"section between <!-- BEGIN CAPCOM --> and <!-- END CAPCOM --> that tells agents how to use " +
		"capcom and lists the top level of the Knowledge tree. A stale block is replaced, an absent " +
		"one appended and a missing AGENTS.md created; the rest of the file is left alone. --check " +
		"writes nothing: it reports whether the block is current, and fails invalid when it is stale, " +
		"so CI catches a block that has drifted; a file without the block passes."
	agentsExample = "  capcom agents           # write or refresh the capcom block of AGENTS.md\n" +
		"  capcom agents --check   # fail when the block is out of date"
)

// newAgentsCommand writes or refreshes the capcom Managed block of AGENTS.md, or with --check
// reports whether it is current. --check fails on a stale block only: an AGENTS.md without the
// block, or no AGENTS.md, is fine.
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
	cmd.Flags().BoolVar(&isCheck, flagCheck, false, "report whether the block is current; fail when it is stale")
	return cmd
}

// writeAgentsBlock brings the block up to date and reports what it did.
func (a *app) writeAgentsBlock(cmd *cobra.Command, opened *store.Store) error {
	status, err := agents.Sync(opened.WorktreeRoot, opened.KnowledgeDir(), agents.ModeWrite)
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
	return a.emit(cmd, output.Result{Kind: "agents", Data: data, Text: text})
}

// checkAgentsBlock reports the block's state; a stale block fails invalid at its marker line.
func (a *app) checkAgentsBlock(cmd *cobra.Command, opened *store.Store) error {
	status, err := agents.Sync(opened.WorktreeRoot, opened.KnowledgeDir(), agents.ModeInspect)
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
	return a.emit(cmd, output.Result{Kind: "agents", Data: data, Text: text})
}

// agentsFailure types tangled markers as invalid at AGENTS.md and its line, and an AGENTS.md that
// cannot be read (not a regular file, or larger than 1 MiB) or that links outside the working tree
// as invalid at AGENTS.md; nil stays nil.
func agentsFailure(err error) error {
	var tangled *agents.Error
	if errors.As(err, &tangled) {
		return &output.Error{Code: output.CodeInvalid, Path: tangled.Path, Line: tangled.Line, Message: tangled.Message}
	}
	return err
}
