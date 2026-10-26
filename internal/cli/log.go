package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
)

// The help text of capcom log.
const (
	logLong = "Append an entry to an Issue's Log, dated today and attributed to the author git would record " +
		"for a commit.\n\n" +
		"The Log is append-only: it carries progress, findings and remarks, while the body stays the " +
		"statement of the work.\n\n" +
		"The text is one argument, so quote it.\n\n" +
		"It prints the Issue's id, path and new etag."
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
			appended, err := a.ops().AppendLog(args[0], "", text, time.Now())
			written, err := a.written(issueIDs, appended, err)
			if err != nil {
				return err
			}
			return a.emit(cmd, writeResult(written.ID, written.Path, written.ETag))
		}),
	})
}
