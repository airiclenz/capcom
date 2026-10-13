package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/gitx"
	"github.com/airiclenz/capcom/internal/issue"
)

// The help text of capcom log.
const (
	logLong = "Append an entry to an Issue's Log, dated today and attributed to the author git would record " +
		"for a commit. The Log is append-only: it carries progress, findings and remarks, while the " +
		"body stays the statement of the work. The text is one argument, so quote it. It prints the " +
		"Issue's id, path and new etag."
	logExample = "  capcom log login-timeout-race \"reproduced with two tabs\"\n" +
		"  capcom log login-timeout-race \"root cause: the refresh timer survives logout\""
)

// newLogCommand appends a dated, attributed entry to an issue's Log section, creating the section
// when the issue has none.
func (a *app) newLogCommand() *cobra.Command {
	return a.withKV(&cobra.Command{
		Use:         "log <id|alias|prefix> <text>",
		Short:       "Append an entry to an issue's Log",
		Long:        logLong,
		Example:     logExample,
		Args:        cobra.ExactArgs(2),
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			text := args[1]
			if strings.TrimSpace(text) == "" {
				return output.Errorf(output.CodeInvalid, "the log text is empty")
			}
			written, err := a.editIssue(args[0], "", func(target issue.Issue) (issue.Issue, error) {
				return a.appendLog(target, text)
			})
			if err != nil {
				return err
			}
			return a.emit(cmd, writeResult(written.ID, written.Path, written.ETag))
		}),
	})
}

// appendLog adds a Log entry dated today and attributed to the author git would record (see
// gitx.AuthorName). Every command that writes a Log entry goes through it.
func (a *app) appendLog(target issue.Issue, text string) (issue.Issue, error) {
	author, err := gitx.AuthorName(a.store.WorktreeRoot)
	if err != nil {
		return issue.Issue{}, output.Errorf(output.CodeInvalid, "%s", err.Error())
	}
	return target.AppendLog(time.Now(), author, text), nil
}
