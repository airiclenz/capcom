package cli

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/seed"
)

// codeSpan matches one backtick span of a Note's text.
var codeSpan = regexp.MustCompile("`([^`]+)`")

// listItemStart matches the first line of a list item.
var listItemStart = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s`)

// commandSpanPrefix is how a backtick span that names a capcom command starts.
const commandSpanPrefix = "capcom "

// noteBlocks splits a Note's text into its blocks, each on one line: a list item with the lines
// that continue it, or a paragraph. A backtick span that wraps over a line break stays whole.
func noteBlocks(text string) []string {
	var blocks []string
	var current []string
	flush := func() {
		if len(current) > 0 {
			blocks = append(blocks, strings.Join(current, " "))
			current = nil
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if listItemStart.MatchString(line) {
			flush()
		}
		current = append(current, strings.TrimSpace(line))
	}
	flush()
	return blocks
}

// isPlaceholder reports whether word stands for a value the reader fills in: <id>, <word>...,
// [flags] or a bare "...".
func isPlaceholder(word string) bool {
	return strings.HasPrefix(word, "<") || strings.HasPrefix(word, "[") || strings.HasPrefix(word, "...")
}

// resolveSpanCommand returns the command that words, the words of a span after "capcom", name
// below root. The command path ends at the first flag or placeholder. cobra's Find returns a
// group and no error for a subcommand the group lacks, so a word Find leaves over is a missing
// command on a command with subcommands, and an argument on any other.
func resolveSpanCommand(root *cobra.Command, words []string) (*cobra.Command, error) {
	var path []string
	for _, word := range words {
		if strings.HasPrefix(word, "-") || isPlaceholder(word) {
			break
		}
		path = append(path, word)
	}
	cmd, rest, err := root.Find(path)
	if err != nil {
		return nil, err
	}
	if len(rest) > 0 && cmd.HasSubCommands() {
		return nil, fmt.Errorf("unknown command %q for %q", rest[0], cmd.CommandPath())
	}
	return cmd, nil
}

// missingFlags returns each --flag among words that cmd neither defines nor inherits. --help is
// a flag of every command; cobra adds it only when the command runs.
func missingFlags(cmd *cobra.Command, words []string) []string {
	inherited := cmd.InheritedFlags()
	own := cmd.Flags()
	var missing []string
	for _, word := range words {
		if !strings.HasPrefix(word, "--") {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimPrefix(word, "--"), "=")
		if name == "" || name == "help" {
			continue
		}
		if own.Lookup(name) == nil && inherited.Lookup(name) == nil {
			missing = append(missing, "--"+name)
		}
	}
	return missing
}

// commandSpanProblems returns one line for each capcom command or flag that a backtick span of
// text names and the command tree below root lacks. A span that starts with "capcom " is
// resolved as a command with its flags. A span that starts with a flag is checked against the
// command the span before it in the same block named, and not at all when there is none: such a
// flag belongs to another program.
func commandSpanProblems(root *cobra.Command, text string) []string {
	var problems []string
	for _, block := range noteBlocks(text) {
		var named *cobra.Command
		for _, match := range codeSpan.FindAllStringSubmatch(block, -1) {
			span := strings.TrimSpace(match[1])
			words := strings.Fields(span)
			switch {
			case strings.HasPrefix(span, commandSpanPrefix):
				cmd, err := resolveSpanCommand(root, words[1:])
				if err != nil {
					problems = append(problems, fmt.Sprintf("`%s`: %v", span, err))
					named = nil
					continue
				}
				named = cmd
			case strings.HasPrefix(span, "--") && named != nil:
			default:
				continue
			}
			for _, flag := range missingFlags(named, words) {
				problems = append(problems, fmt.Sprintf("`%s`: %q has no flag %s", span, named.CommandPath(), flag))
			}
		}
	}
	return problems
}

// newSpanCheckRoot returns the root command tree the spans are resolved against.
func newSpanCheckRoot() *cobra.Command {
	return (&app{stdout: io.Discard, stderr: io.Discard}).newRootCommand()
}

// TestKnowSeedNotes_CommandSpans_NameCommandsAndFlagsThatExist reads every Note of the base set
// and fails for a capcom command or flag its text names that this binary does not have.
func TestKnowSeedNotes_CommandSpans_NameCommandsAndFlagsThatExist(t *testing.T) {
	set, err := seed.Load()
	if err != nil {
		t.Fatalf("seed.Load: %v", err)
	}
	root := newSpanCheckRoot()

	for _, note := range set.Notes() {
		for _, problem := range commandSpanProblems(root, note.Title+"\n\n"+note.Body) {
			t.Errorf("base Note %s: %s", note.ID, problem)
		}
	}
}

// TestKnowSeedNotes_CommandSpanProblems_MadeUpText pins what the span check reports, on Note
// text made up for the test.
func TestKnowSeedNotes_CommandSpanProblems_MadeUpText(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string // a part of each problem line, in order; none when the text passes
	}{
		{"a command that exists", "Run `capcom ready` first.", nil},
		{"a missing command", "Run `capcom frobnicate` first.", []string{`unknown command "frobnicate" for "capcom"`}},
		{"a missing subcommand of a group", "Run `capcom know frobnicate`.", []string{`unknown command "frobnicate" for "capcom know"`}},
		{"a literal argument on a leaf command", "Run `capcom know tree some-note`.", nil},
		{"a flag that exists", "Run `capcom list --tree --depth 2`.", nil},
		{"a missing flag", "Run `capcom show an-issue --no-such-flag`.", []string{`"capcom show" has no flag --no-such-flag`}},
		{"a missing flag with a value", "Run `capcom show an-issue --no-such-flag=1`.", []string{`has no flag --no-such-flag`}},
		{"an inherited flag", "Run `capcom know tree --json`.", nil},
		{"--help", "Run `capcom know tree --help` and `capcom --help`.", nil},
		{"a placeholder after the command", "Run `capcom show <id> --section acceptance`.", nil},
		{"a missing flag after a placeholder", "Run `capcom show <id> --no-such-flag`.", []string{`"capcom show" has no flag --no-such-flag`}},
		{"a placeholder in place of the command", "Each command's flags are in `capcom <cmd> --help`, as in `capcom ...`.", nil},
		{"a bare flag after its command", "- Read one part with `capcom show <id>`,\n  and `--section <name>` picks it.", nil},
		{"a missing bare flag after its command", "- Read it with `capcom show <id>`,\n  with `--no-such-flag <name>` for one part.", []string{`"capcom show" has no flag --no-such-flag`}},
		{"a bare flag no command precedes", "Never push with `--no-such-flag`, nor with `git push --force`.", nil},
		{"a bare flag in the next list item", "- Read it with `capcom show <id>`.\n- Never pass `--no-such-flag`.", nil},
		{"a bare flag in the next paragraph", "Read it with `capcom show <id>`.\n\nNever pass `--no-such-flag`.", nil},
		{"a bare flag after a command that is missing", "- Run `capcom frobnicate`, with `--no-such-flag`.", []string{`unknown command "frobnicate"`}},
		{"a span wrapped over a line break", "- Run `capcom know\n  frobnicate` first.", []string{`unknown command "frobnicate" for "capcom know"`}},
		{"another program's command", "Run `git reset --hard` or `capcomx --no-such-flag`.", nil},
	}
	root := newSpanCheckRoot()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := commandSpanProblems(root, tc.text)

			if len(got) != len(tc.want) {
				t.Fatalf("commandSpanProblems(%q) = %q, want %d problem(s) %q", tc.text, got, len(tc.want), tc.want)
			}
			for i, want := range tc.want {
				if !strings.Contains(got[i], want) {
					t.Errorf("problem %d = %q, want it to contain %q", i, got[i], want)
				}
			}
		})
	}
}
