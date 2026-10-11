package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// newSearchCommand finds issues by full text: every word must appear in the title or body, as a
// whole word, case-insensitively. Hits are listed best match first; no hit is not_found.
func (a *app) newSearchCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "search <word>...",
		Short:       "Find issues whose title or body contains every word",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			text := strings.Join(args, " ")
			entries, err := a.issueIndex.SearchIssues(text)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				return Errorf(CodeNotFound, "no issue matches %q", text)
			}
			return a.emit(cmd, Result{
				Kind: "search",
				Data: issueListData(entries),
				Text: issueLines(entries, ""),
			})
		}),
	}
}
