package cli

import (
	"io"
	"strings"
	"testing"
)

// issueCommandPaths are the issue commands whose help text carries a Long and an Example.
var issueCommandPaths = []string{
	"create",
	"update",
	"close",
	"reopen",
	"rm",
	"mv",
	"log",
	"show",
	"list",
	"ready",
	"search",
}

// graphAndAgentCommandPaths are the graph, coordination and agent commands whose help text carries
// a Long and an Example.
var graphAndAgentCommandPaths = []string{
	"dep",
	"dep add",
	"dep rm",
	"dep list",
	"dep graph",
	"related",
	"related add",
	"related rm",
	"lock",
	"lock acquire",
	"lock touch",
	"lock release",
	"lock break",
	"lock check",
	"lock status",
	"prime",
	"agents",
}

// commandsWithoutKV are the commands that never register --kv, so their help never names it.
var commandsWithoutKV = []string{"list", "ready", "search", "dep", "related", "lock", "prime", "agents"}

func TestHelpText_IssueCommands_SetLongAndExample(t *testing.T) {
	assertLongAndExample(t, issueCommandPaths)
}

func TestHelpText_GraphAndAgentCommands_SetLongAndExample(t *testing.T) {
	assertLongAndExample(t, graphAndAgentCommandPaths)
}

func TestHelpText_CommandsWithoutKV_OmitKVFromLongAndExample(t *testing.T) {
	root := (&app{stdout: io.Discard, stderr: io.Discard}).newRootCommand()

	for _, path := range commandsWithoutKV {
		cmd, _, err := root.Find(strings.Fields(path))
		if err != nil {
			t.Fatalf("find %q: %v", path, err)
		}

		if strings.Contains(cmd.Long, "--"+flagKV) || strings.Contains(cmd.Example, "--"+flagKV) {
			t.Errorf("%s: help text mentions --%s", path, flagKV)
		}
	}
}

// assertLongAndExample checks that each command path sets a non-empty Long and an Example whose
// every line starts with "  capcom <path> ".
func assertLongAndExample(t *testing.T, paths []string) {
	t.Helper()
	root := (&app{stdout: io.Discard, stderr: io.Discard}).newRootCommand()

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			cmd, _, err := root.Find(strings.Fields(path))
			if err != nil || cmd.CommandPath() != "capcom "+path {
				t.Fatalf("find %q: command %q, err %v", path, cmd.CommandPath(), err)
			}

			if strings.TrimSpace(cmd.Long) == "" {
				t.Errorf("%s: Long is empty", path)
			}
			if strings.TrimSpace(cmd.Example) == "" {
				t.Fatalf("%s: Example is empty", path)
			}
			for _, line := range strings.Split(cmd.Example, "\n") {
				if !strings.HasPrefix(line, "  capcom "+path+" ") {
					t.Errorf("%s: example line %q does not start with %q", path, line, "  capcom "+path+" ")
				}
			}
		})
	}
}
