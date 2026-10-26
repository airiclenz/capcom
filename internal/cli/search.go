package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/index"
)

// The help text of capcom search.
const (
	searchLong = "Find the Issues whose title or body contains every word given, matched as whole words and " +
		"case-insensitively, best match first; closed Issues are searched too. It prints one line per " +
		"Issue with its id, status, priority and title. When no Issue holds every word, it lists the " +
		"Issues holding some of them under the line \"" + anyWordHeadingLine + "\", and --json sets " +
		"\"match\" to \"any\" instead of \"all\". It fails with not_found when no Issue holds any word."
	searchExample = "  capcom search refresh timeout   # issues mentioning both words, else either\n" +
		"  capcom search safari"
)

// anyWordHeadingLine opens the text of a search no item matched on every word, above the items
// holding some of them.
const anyWordHeadingLine = "No item holds every word; these hold some of them:"

// newSearchCommand finds issues by full text: every word must appear in the title or body, as a
// whole word, case-insensitively; when no issue holds them all, the issues holding some of them
// are listed under anyWordHeadingLine. Hits are listed best match first; no hit is not_found.
func (a *app) newSearchCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "search <word>...",
		Short:       "Find issues whose title or body contains every word, else some of them",
		Long:        searchLong,
		Example:     searchExample,
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			text := strings.Join(args, " ")
			entries, match, err := searchEveryWordFirst(
				func() ([]index.Entry, error) { return a.issueIndex.SearchIssues(text) },
				func() ([]index.Entry, error) { return a.issueIndex.SearchIssuesAnyWord(text) },
			)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				return output.Errorf(output.CodeNotFound, "no issue matches %q", text)
			}
			return a.emit(cmd, output.Result{
				Kind:  "search",
				Match: match,
				Data:  issueListData(entries),
				Text:  searchText(match, issueLines(entries, "")),
			})
		}),
	}
}

// searchEveryWordFirst returns the hits of everyWord with output.MatchAll when it finds any, else
// the hits of anyWord with output.MatchAny. Either search's error is returned as it is.
func searchEveryWordFirst[E any](everyWord, anyWord func() ([]E, error)) ([]E, string, error) {
	entries, err := everyWord()
	if err != nil || len(entries) > 0 {
		return entries, output.MatchAll, err
	}
	entries, err = anyWord()
	return entries, output.MatchAny, err
}

// searchText is the text of a search's hit lines: under anyWordHeadingLine when match is
// output.MatchAny, the lines alone otherwise.
func searchText(match string, lines string) string {
	if match != output.MatchAny {
		return lines
	}
	return anyWordHeadingLine + "\n" + lines
}
