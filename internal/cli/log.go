package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/gitx"
	"github.com/airiclenz/capcom/internal/issue"
)

// newLogCommand appends a dated, attributed entry to an issue's Log section, creating the section
// when the issue has none.
func (a *app) newLogCommand() *cobra.Command {
	return a.withKV(&cobra.Command{
		Use:         "log <id|alias|prefix> <text>",
		Short:       "Append an entry to an issue's Log",
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			text := args[1]
			if strings.TrimSpace(text) == "" {
				return Errorf(CodeInvalid, "the log text is empty")
			}
			result, err := a.editIssue(args[0], "", func(target issue.Issue) (issue.Issue, error) {
				return a.appendLog(target, text)
			})
			if err != nil {
				return err
			}
			return a.emit(cmd, result)
		}),
	})
}

// appendLog adds a Log entry dated today and attributed to the author git would record (see
// gitx.AuthorName). Every command that writes a Log entry goes through it.
func (a *app) appendLog(target issue.Issue, text string) (issue.Issue, error) {
	author, err := gitx.AuthorName(a.store.WorktreeRoot)
	if err != nil {
		return issue.Issue{}, Errorf(CodeInvalid, "%s", err.Error())
	}
	return target.AppendLog(time.Now(), author, text), nil
}
