package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/check"
	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/store"
)

// newCheckCommand validates the whole store, reading the files and never the index, and lists
// every problem as `path:line: message`. Any problem exits 1 after the list; the --json data is the
// list itself, as {path, line, code, message} objects.
func (a *app) newCheckCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Validate every config, issue and Note file of the store",
		Args:  cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			problems, err := checkProblems(cwd)
			if err != nil {
				return err
			}
			data := append([]check.Problem{}, problems...)
			var text strings.Builder
			for _, problem := range problems {
				text.WriteString(problem.String() + "\n")
			}
			if err := a.emit(cmd, output.Result{Kind: "check", Data: data, Text: text.String()}); err != nil {
				return err
			}
			if len(problems) == 0 {
				return nil
			}
			// The problems are the result, so the failure only sets the exit code: --json keeps
			// kind check without an error member, and text mode writes nothing to stderr.
			return output.Rendered(output.Errorf(output.CodeInvalid, "%d problems found", len(problems)))
		}),
	}
}

// checkProblems opens the store of the git working tree containing cwd and checks it. A config.yaml
// that cannot be used is the one problem reported, not a failure; any other open or check failure
// is returned as the store failure it is.
func checkProblems(cwd string) ([]check.Problem, error) {
	opened, err := store.Open(cwd)
	if err != nil {
		if problems := check.ConfigProblems(err); problems != nil {
			return problems, nil
		}
		return nil, output.StoreFailure(err)
	}
	problems, err := check.Run(opened)
	if err != nil {
		return nil, output.StoreFailure(err)
	}
	return problems, nil
}
