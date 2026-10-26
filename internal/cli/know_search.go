package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/index"
)

// Help text of capcom know search.
const (
	knowSearchLong = "Find the Notes whose title or body contains every word given, each as a whole word, and " +
		"list them best match first, one line each with the id and title. When no Note holds every word, " +
		"it lists the Notes holding some of them under the line \"" + anyWordHeadingLine + "\", and " +
		"--json sets \"match\" to \"any\" instead of \"all\". --label keeps only the Notes " +
		"carrying every label given. Retired Notes and everything below them are left out unless " +
		"--retired is given. A Note holding no word fails with not_found, exit code 3."
	knowSearchExample = "  capcom know search release                   # Notes that mention release\n" +
		"  capcom know search release tag --label release   # both words, labelled release\n" +
		"  capcom know search index --retired           # retired Notes too"
)

// newKnowSearchCommand finds Notes by full text, every word as a whole word in the title or body,
// optionally only those carrying every --label; when no Note holds them all, the Notes holding some
// of them are listed under anyWordHeadingLine. Hits are listed best match first; no hit is
// not_found. Retired subtrees are left out unless --retired is given.
func (a *app) newKnowSearchCommand() *cobra.Command {
	var labels []string
	isRetiredIncluded := false
	cmd := &cobra.Command{
		Use:         "search <word>...",
		Short:       "Find Notes whose title or body contains every word, else some of them",
		Long:        knowSearchLong,
		Example:     knowSearchExample,
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			text := strings.Join(args, " ")
			entries, match, err := searchEveryWordFirst(
				func() ([]index.NoteEntry, error) {
					return a.issueIndex.SearchNotes(text, labels, isRetiredIncluded)
				},
				func() ([]index.NoteEntry, error) {
					return a.issueIndex.SearchNotesAnyWord(text, labels, isRetiredIncluded)
				},
			)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				return output.Errorf(output.CodeNotFound, "no Note matches %q", text)
			}
			data := make([]noteData, 0, len(entries))
			var lines strings.Builder
			for _, entry := range entries {
				data = append(data, newNoteData(entry))
				lines.WriteString(entry.ID + "  " + flatten(entry.Title) + "\n")
			}
			return a.emit(cmd, output.Result{
				Kind:  "know-search",
				Match: match,
				Data:  data,
				Text:  searchText(match, lines.String()),
			})
		}),
	}
	cmd.Flags().StringSliceVar(&labels, flagLabel, nil, "only Notes with every one of these labels; repeat or separate with commas")
	cmd.Flags().BoolVar(&isRetiredIncluded, flagRetired, false, retiredFlagUsage)
	return cmd
}
