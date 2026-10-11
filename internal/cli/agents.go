package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/agents"
	"github.com/airiclenz/capcom/internal/store"
)

// flagCheck makes capcom agents report the block's state instead of writing it.
const flagCheck = "check"

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

// newAgentsCommand writes or refreshes the capcom Managed block of AGENTS.md, or with --check
// reports whether it is current. --check fails on a stale block only: an AGENTS.md without the
// block, or no AGENTS.md, is fine.
func (a *app) newAgentsCommand() *cobra.Command {
	isCheck := false
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Write or refresh the capcom block of AGENTS.md",
		Args:  cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			opened, err := store.Open(cwd)
			if err != nil {
				return storeFailure(err)
			}
			if isCheck {
				return a.checkAgentsBlock(cmd, opened)
			}
			return a.writeAgentsBlock(cmd, opened)
		}),
	}
	cmd.Flags().BoolVar(&isCheck, flagCheck, false, "report whether the block is current; fail when it is stale")
	return cmd
}

// writeAgentsBlock brings the block up to date and reports what it did.
func (a *app) writeAgentsBlock(cmd *cobra.Command, opened *store.Store) error {
	status, err := refreshAgentsBlock(opened, false)
	if err != nil {
		return err
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
	return a.emit(cmd, Result{Kind: "agents", Data: data, Text: text})
}

// checkAgentsBlock reports the block's state; a stale block fails invalid at its marker line.
func (a *app) checkAgentsBlock(cmd *cobra.Command, opened *store.Store) error {
	notes, err := agents.TopNotes(opened.KnowledgeDir())
	if err != nil {
		return err
	}
	status, err := agents.Inspect(opened.WorktreeRoot, notes)
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
		return &Error{
			Code:    CodeInvalid,
			Path:    agents.FileName,
			Line:    status.Line,
			Message: "the capcom block is out of date; run capcom agents to refresh it",
		}
	}
	return a.emit(cmd, Result{Kind: "agents", Data: data, Text: text})
}

// refreshAgentsBlock writes the block for the store's Knowledge top level and returns the Status
// found before writing. With isPresentOnly it writes only over a block already there, leaving an
// AGENTS.md without one, or a missing AGENTS.md, as it is.
func refreshAgentsBlock(opened *store.Store, isPresentOnly bool) (agents.Status, error) {
	notes, err := agents.TopNotes(opened.KnowledgeDir())
	if err != nil {
		return agents.Status{}, err
	}
	if isPresentOnly {
		status, err := agents.Inspect(opened.WorktreeRoot, notes)
		if err != nil || status.State != agents.StateStale {
			return status, agentsFailure(err)
		}
	}
	status, err := agents.Write(opened.WorktreeRoot, notes)
	return status, agentsFailure(err)
}

// agentsFailure types tangled markers as invalid at AGENTS.md and its line; nil stays nil.
func agentsFailure(err error) error {
	var tangled *agents.Error
	if errors.As(err, &tangled) {
		return &Error{Code: CodeInvalid, Path: tangled.Path, Line: tangled.Line, Message: tangled.Message}
	}
	return err
}
