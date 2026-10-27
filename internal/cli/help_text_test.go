package cli

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
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

// TestHelpText_EveryCommand_PrintsExamplesBeforeShortRules renders the --help of every available
// command and checks that each example line comes before the first line of Long, and that every
// paragraph or bullet of Long fits in helpRuleMaxLines lines at helpRuleColumns.
func TestHelpText_EveryCommand_PrintsExamplesBeforeShortRules(t *testing.T) {
	for _, cmd := range availableCommands() {
		path := cmd.CommandPath()
		t.Run(path, func(t *testing.T) {
			stdout, stderr, exitCode := runCLI(t, "", helpArgs(path)...)
			if exitCode != ExitOK || stderr != "" {
				t.Fatalf("%s --help: exit = %d stderr = %q, want 0 and empty", path, exitCode, stderr)
			}

			longStart := strings.LastIndex(stdout, firstLine(cmd.Long))
			if longStart < 0 {
				t.Fatalf("%s --help does not print the first line of Long:\n%s", path, stdout)
			}
			for _, line := range strings.Split(cmd.Example, "\n") {
				if at := strings.Index(stdout, line); at < 0 || at > longStart {
					t.Errorf("%s --help: example line %q is missing or comes after Long", path, line)
				}
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

// TestHelpText_Help_PrintsSectionsInOrder checks the section order of --help once for a command
// (create) and once for a group (lock), whose subcommands come before its examples.
func TestHelpText_Help_PrintsSectionsInOrder(t *testing.T) {
	tests := []struct {
		path     string
		sections []string
	}{
		{
			path:     "capcom create",
			sections: []string{"Usage:\n", "\nExamples:\n", "\nFlags:\n", "\nGlobal Flags:\n"},
		},
		{
			path: "capcom lock",
			sections: []string{
				"Usage:\n", "\nAvailable Commands:\n", "\nExamples:\n", "\nFlags:\n", "\nGlobal Flags:\n",
			},
		},
	}
	commands := map[string]*cobra.Command{}
	for _, cmd := range availableCommands() {
		commands[cmd.CommandPath()] = cmd
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			cmd := commands[test.path]
			markers := append([]string{cmd.Short + "\n"}, test.sections...)
			markers = append(markers, firstLine(cmd.Long))

			stdout := mustRun(t, "", helpArgs(test.path)...)

			rest := stdout
			for _, marker := range markers {
				at := strings.Index(rest, marker)
				if at < 0 {
					t.Fatalf("%s --help: %q missing or out of order:\n%s", test.path, marker, stdout)
				}
				rest = rest[at+len(marker):]
			}
		})
	}
}

// helpArgs returns the arguments that print the help of the command at path, a command path
// starting with capcom.
func helpArgs(path string) []string {
	return append(strings.Fields(path)[1:], "--help")
}

// firstLine returns the first line of text.
func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
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

// refFlagValue matches a --ref or --add-ref flag with its value, quoted or bare.
var refFlagValue = regexp.MustCompile(`--(?:add-)?ref[ =]("[^"]*"|\S+)`)

// refPaths returns the repository paths that --ref and --add-ref values in text name, each
// without its #symbol. A value holding neither a / nor a . is prose ("--ref set"), not a path.
func refPaths(text string) []string {
	var paths []string
	for _, match := range refFlagValue.FindAllStringSubmatch(text, -1) {
		value := strings.TrimRight(strings.Trim(match[1], `"`), ".,;:)`")
		if !strings.ContainsAny(value, "/.") {
			continue
		}
		path, _, _ := strings.Cut(value, "#")
		paths = append(paths, path)
	}
	return paths
}

// TestRefPaths_ProseAndExamples_KeepsOnlyPaths pins which --ref values count as paths, so the
// prose in know add's and know update's Long text is never stat'ed.
func TestRefPaths_ProseAndExamples_KeepsOnlyPaths(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{"prose", "--label and --ref set its Labels; A --ref inside the git directory; --add-ref and --remove-ref", nil},
		{"bare path", `capcom know add --id x --title "X" --ref docs/release.md   # a Note`, []string{"docs/release.md"}},
		{"symbol and quotes", `--add-ref "internal/cli/root.go#Run" --ref=README.md`, []string{"internal/cli/root.go", "README.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := refPaths(tc.text)

			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("refPaths(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

// TestHelpText_RefExamples_ResolveInRepo fails when README.md or a command's Long or Example text
// shows a --ref or --add-ref naming a path missing from this repository, since following such an
// example writes a Note that capcom check fails on.
func TestHelpText_RefExamples_ResolveInRepo(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	readme, err := os.ReadFile(filepath.Join(repoRoot, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	sources := map[string]string{"README.md": string(readme)}
	for _, cmd := range availableCommands() {
		sources[cmd.CommandPath()+" Long"] = cmd.Long
		sources[cmd.CommandPath()+" Example"] = cmd.Example
	}

	for source, text := range sources {
		for _, path := range refPaths(text) {
			if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(path))); err != nil {
				t.Errorf("%s: --ref %s does not resolve in this repository: %v", source, path, err)
			}
		}
	}
}
