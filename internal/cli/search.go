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
		"case-insensitively, best match first; closed Issues are searched too.\n\n" +
		"It prints one line per Issue with its id, status, priority and title.\n\n" +
		"When no Issue holds every word, it lists the Issues holding some of them under the line \"" +
		anyWordHeadingLine + "\", and --json sets \"match\" to \"any\" instead of \"all\".\n\n" +
		"--excerpt prints under each Issue the body lines holding a word searched for, indented four " +
		"spaces, and --json gives them as \"excerpt\".\n\n" +
		"It fails with not_found when no Issue holds any word."
	searchExample = "  capcom search refresh timeout   # issues mentioning both words, else either\n" +
		"  capcom search safari --excerpt   # with the body lines that mention safari"
)

// excerptFlagUsage is the help of the --excerpt flag shared by search and know search; it states
// the excerpt shape index.Excerpt builds.
const excerptFlagUsage = "print under each hit the body lines holding a word searched for: the first 3 such " +
	"lines with 1 line of context either side, at most 8 lines per hit, each cut at 120 characters"

// excerptIndent indents an excerpt line under its hit line.
const excerptIndent = "    "

// anyWordHeadingLine opens the text of a search no item matched on every word, above the items
// holding some of them.
const anyWordHeadingLine = "No item holds every word; these hold some of them:"

// newSearchCommand finds issues by full text: every word must appear in the title or body, as a
// whole word, case-insensitively; when no issue holds them all, the issues holding some of them
// are listed under anyWordHeadingLine. Hits are listed best match first; no hit is not_found.
// --excerpt adds under each hit the body lines it matches on.
func (a *app) newSearchCommand() *cobra.Command {
	isExcerptShown := false
	cmd := &cobra.Command{
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
			data := issueListData(entries)
			var excerpts [][]string
			if isExcerptShown {
				paths := make([]string, 0, len(entries))
				for _, entry := range entries {
					paths = append(paths, entry.Path)
				}
				if excerpts, err = a.searchExcerpts(paths, text); err != nil {
					return err
				}
				for position := range data {
					data[position].Excerpt = excerpts[position]
				}
			}
			return a.emit(cmd, output.Result{
				Kind:  "search",
				Match: match,
				Data:  data,
				Text:  searchText(match, withExcerpts(issueLines(entries, ""), excerpts)),
			})
		}),
	}
	cmd.Flags().BoolVar(&isExcerptShown, flagExcerpt, false, excerptFlagUsage)
	return cmd
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

// searchExcerpts returns, in the order of paths, the excerpt index.Excerpt builds from the indexed
// body of each hit's file for text; a hit whose body holds no word of text has none.
func (a *app) searchExcerpts(paths []string, text string) ([][]string, error) {
	excerpts := make([][]string, 0, len(paths))
	for _, path := range paths {
		body, err := a.issueIndex.Body(path)
		if err != nil {
			return nil, err
		}
		excerpts = append(excerpts, index.Excerpt(body, text))
	}
	return excerpts, nil
}

// withExcerpts puts under each line of hitLines, one line per hit, the excerpt of that hit, each
// excerpt line indented excerptIndent. Without excerpts hitLines is returned as it is.
func withExcerpts(hitLines string, excerpts [][]string) string {
	if excerpts == nil {
		return hitLines
	}
	var text strings.Builder
	for position, line := range strings.Split(strings.TrimSuffix(hitLines, "\n"), "\n") {
		text.WriteString(line + "\n")
		for _, excerptLine := range excerpts[position] {
			text.WriteString(excerptIndent + excerptLine + "\n")
		}
	}
	return text.String()
}
