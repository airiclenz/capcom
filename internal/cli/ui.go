package cli

import (
	"os"

	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom"
	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/ui"
)

// uiMaxArgs is the most arguments capcom ui takes: a tab, then an id.
const uiMaxArgs = 2

// noColorVariable names the environment variable that, set to anything non-empty, turns the
// terminal UI's colours off (https://no-color.org).
const noColorVariable = "NO_COLOR"

// The help text of capcom ui.
const (
	uiLong = "Open the terminal UI over this store: the Ready, Issues and Knowledge tabs. With no " +
		"argument it opens the Ready tab; a tab name opens that tab, and an id after it selects that " +
		"Issue or Note there. It needs a terminal: when stdin or stdout is not one, it fails. q or " +
		"ctrl+c quits. --json does not apply and is refused."
	uiExample = "  capcom ui                           # the Ready tab\n" +
		"  capcom ui issues                    # the Issues tab\n" +
		"  capcom ui issues login-timeout-race # the Issues tab with that Issue selected\n" +
		"  capcom ui knowledge store-layout    # the Knowledge tab with that Note selected"
)

// newUICommand runs the terminal UI on the store and index prepare opened. The arguments and
// --json are checked in the Args validator, so a malformed invocation fails before the index is
// refreshed.
func (a *app) newUICommand() *cobra.Command {
	return &cobra.Command{
		Use:     "ui [issues|ready|knowledge] [id]",
		Short:   "Open the terminal UI",
		Long:    uiLong,
		Example: uiExample,
		Args: func(cmd *cobra.Command, args []string) error {
			if a.isJSON {
				return output.UsageErrorf("%s does not support --%s", cmd.CommandPath(), flagJSON)
			}
			_, _, _, err := uiTarget(args)
			return err
		},
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			tab, id, isTabGiven, err := uiTarget(args)
			if err != nil {
				return err
			}
			in, out, err := a.terminal(cmd)
			if err != nil {
				return err
			}
			return ui.Run(ui.Options{
				Store:      a.store,
				Index:      a.issueIndex,
				Tab:        tab,
				ID:         id,
				IsTabGiven: isTabGiven,
				In:         in,
				Out:        out,
				NoColor:    os.Getenv(noColorVariable) != "",
				Version:    capcom.Version(),
			})
		}),
	}
}

// uiTarget reads the tab and the id to select from capcom ui's arguments, and whether a tab was
// given: none opens the Ready tab with no tab given, and an id is taken only after a tab name.
// Anything else is a usage error.
func uiTarget(args []string) (ui.Tab, string, bool, error) {
	if len(args) > uiMaxArgs {
		return ui.TabReady, "", false, output.UsageErrorf("accepts at most %d args (a tab, then an id), received %d", uiMaxArgs, len(args))
	}
	if len(args) == 0 {
		return ui.TabReady, "", false, nil
	}
	tab, err := ui.ParseTab(args[0])
	if err != nil {
		return ui.TabReady, "", false, output.UsageErrorf("%s", err.Error())
	}
	var id string
	if len(args) == uiMaxArgs {
		id = args[1]
	}
	return tab, id, true, nil
}

// terminal returns the command's stdin and stdout when both are terminals, and otherwise fails
// naming the stream that is not. It checks the streams the command was given, never the
// process's own, so a test's buffers are refused like a pipe.
func (a *app) terminal(cmd *cobra.Command) (*os.File, *os.File, error) {
	in, isInFile := cmd.InOrStdin().(*os.File)
	if !isInFile || !term.IsTerminal(in.Fd()) {
		return nil, nil, output.Errorf(output.CodeInvalid, "capcom ui needs a terminal: stdin is not a terminal")
	}
	out, isOutFile := a.stdout.(*os.File)
	if !isOutFile || !term.IsTerminal(out.Fd()) {
		return nil, nil, output.Errorf(output.CodeInvalid, "capcom ui needs a terminal: stdout is not a terminal")
	}
	return in, out, nil
}
