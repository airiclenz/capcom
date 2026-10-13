package cli

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestHelpText_EveryCommand_SetsLongAndExample walks the root and every available command below
// it, so a new command without help text fails here.
func TestHelpText_EveryCommand_SetsLongAndExample(t *testing.T) {
	for _, cmd := range availableCommands() {
		path := cmd.CommandPath()
		t.Run(path, func(t *testing.T) {
			if strings.TrimSpace(cmd.Long) == "" {
				t.Errorf("%s: Long is empty", path)
			}
			if strings.TrimSpace(cmd.Example) == "" {
				t.Fatalf("%s: Example is empty", path)
			}
			for _, line := range strings.Split(cmd.Example, "\n") {
				if !strings.HasPrefix(line, "  "+path+" ") {
					t.Errorf("%s: example line %q does not start with %q", path, line, "  "+path+" ")
				}
			}
		})
	}
}

// TestHelpText_CommandsWithoutKV_OmitKVFromLongAndExample checks that the help text of every
// command that does not register --kv never names it.
func TestHelpText_CommandsWithoutKV_OmitKVFromLongAndExample(t *testing.T) {
	for _, cmd := range availableCommands() {
		if cmd.Flags().Lookup(flagKV) != nil {
			continue
		}

		if strings.Contains(cmd.Long, "--"+flagKV) || strings.Contains(cmd.Example, "--"+flagKV) {
			t.Errorf("%s: help text mentions --%s", cmd.CommandPath(), flagKV)
		}
	}
}

// availableCommands returns the root of a fresh command tree and every available command below it,
// breadth first.
func availableCommands() []*cobra.Command {
	root := (&app{stdout: io.Discard, stderr: io.Discard}).newRootCommand()
	commands := []*cobra.Command{root}
	for i := 0; i < len(commands); i++ {
		for _, child := range commands[i].Commands() {
			if child.IsAvailableCommand() {
				commands = append(commands, child)
			}
		}
	}
	return commands
}
