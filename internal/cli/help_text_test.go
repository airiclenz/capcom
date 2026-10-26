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

// helpRuleColumns and helpRuleMaxLines bound one rule of a Long: word-wrapped at helpRuleColumns, a
// paragraph or bullet runs to at most helpRuleMaxLines lines.
const (
	helpRuleColumns  = 100
	helpRuleMaxLines = 4
)

// shortRuleCommands are the commands whose Long is already split into short rules.
var shortRuleCommands = []string{
	"capcom", "capcom version", "capcom agents", "capcom check", "capcom close", "capcom create",
	"capcom dep", "capcom dep add", "capcom dep rm", "capcom dep list", "capcom dep graph",
	"capcom finish", "capcom fmt", "capcom init",
	"capcom know", "capcom know add", "capcom know update", "capcom know verify", "capcom know tree",
	"capcom know show", "capcom know search", "capcom list", "capcom lock", "capcom lock acquire",
	"capcom lock touch", "capcom lock release", "capcom lock break", "capcom lock check",
	"capcom lock status", "capcom log", "capcom merge-driver",
}

// TestHelpText_ShortRuleCommands_LongRulesFitFourLines checks that every paragraph or bullet of the
// Long of each command in shortRuleCommands fits in helpRuleMaxLines lines at helpRuleColumns.
func TestHelpText_ShortRuleCommands_LongRulesFitFourLines(t *testing.T) {
	commands := map[string]*cobra.Command{}
	for _, cmd := range availableCommands() {
		commands[cmd.CommandPath()] = cmd
	}
	for _, path := range shortRuleCommands {
		t.Run(path, func(t *testing.T) {
			cmd, ok := commands[path]
			if !ok {
				t.Fatalf("%s: no such available command", path)
			}

			for _, rule := range helpRules(cmd.Long) {
				if lines := wrappedLineCount(rule, helpRuleColumns); lines > helpRuleMaxLines {
					t.Errorf("%s: rule runs to %d lines at %d columns, want at most %d: %q",
						path, lines, helpRuleColumns, helpRuleMaxLines, rule)
				}
			}
		})
	}
}

func TestHelpRules_SplitsParagraphsAndBullets(t *testing.T) {
	long := "First rule\ncontinued.\n\nSecond rule.\n\n- one bullet\n- another bullet"

	got := helpRules(long)

	want := []string{"First rule continued.", "Second rule.", "- one bullet", "- another bullet"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("helpRules(%q) = %q, want %q", long, got, want)
	}
}

func TestWrappedLineCount_CountsGreedyWrappedLines(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "fits one line", text: "aaaa bbbb", want: 1},
		{name: "exactly the width", text: "aaaa bbbbb", want: 1},
		{name: "one over the width", text: "aaaa bbbbbb", want: 2},
		{name: "word longer than the width", text: "aaaaaaaaaaaa bb", want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := wrappedLineCount(test.text, 10); got != test.want {
				t.Errorf("wrappedLineCount(%q, 10) = %d, want %d", test.text, got, test.want)
			}
		})
	}
}

// helpRules splits a Long into its rules: blank lines separate paragraphs, a line starting with
// "- " starts a bullet, and any other line continues the rule before it.
func helpRules(long string) []string {
	var rules []string
	current := ""
	flush := func() {
		if current != "" {
			rules = append(rules, current)
			current = ""
		}
	}
	for _, line := range strings.Split(long, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "- "):
			flush()
			current = line
		case current == "":
			current = line
		default:
			current += " " + line
		}
	}
	flush()
	return rules
}

// wrappedLineCount returns how many lines text takes when its words are wrapped greedily at width
// columns; a word longer than width takes a line of its own.
func wrappedLineCount(text string, width int) int {
	lines, column := 0, 0
	for _, word := range strings.Fields(text) {
		length := len([]rune(word))
		switch {
		case column == 0:
			lines++
			column = length
		case column+1+length <= width:
			column += 1 + length
		default:
			lines++
			column = length
		}
	}
	return lines
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
