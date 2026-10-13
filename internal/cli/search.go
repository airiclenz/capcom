package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
)

// The help text of capcom search.
const (
	searchLong = "Find the Issues whose title or body contains every word given, matched as whole words and " +
		"case-insensitively, best match first; closed Issues are searched too. It prints one line per " +
		"Issue with its id, status, priority and title, and fails with not_found when nothing matches."
	searchExample = "  capcom search refresh timeout   # issues mentioning both words\n" +
		"  capcom search safari"
)

// newSearchCommand finds issues by full text: every word must appear in the title or body, as a
// whole word, case-insensitively. Hits are listed best match first; no hit is not_found.
func (a *app) newSearchCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "search <word>...",
		Short:       "Find issues whose title or body contains every word",
		Long:        searchLong,
		Example:     searchExample,
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			text := strings.Join(args, " ")
			entries, err := a.issueIndex.SearchIssues(text)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				return output.Errorf(output.CodeNotFound, "no issue matches %q", text)
			}
			return a.emit(cmd, output.Result{
				Kind: "search",
				Data: issueListData(entries),
				Text: issueLines(entries, ""),
			})
		}),
	}
}
