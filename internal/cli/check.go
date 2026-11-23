package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/check"
	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/store"
)

// Help text of capcom check.
const (
	checkLong = "Validate the whole store from its files, never the index: the config, every Issue and " +
		"Note file, and the trees they form.\n\n" +
		"Each problem is listed as path:line: message, and any problem fails the command, exit code " +
		"1, after the list.\n\n" +
		"A write made through a capcom command is already validated: one that leaves a problem check " +
		"reports warns with check's own words for it. Run check after a hand edit of a store file, " +
		"after a merge, and before committing hand-edited store files.\n\n" +
		"A Note that capcom know seed added, that is still as it was seeded and that this capcom " +
		"ships a newer text for is no problem: check prints a warning: line for it, naming capcom " +
		"know seed --update, and the exit code stays as it is without it.\n\n" +
		"The --json data is the list of problems as {path, line, code, message} objects; the " +
		"warnings are in the envelope's warnings member."
	checkExample = "  capcom check          # validate the whole store\n" +
		"  capcom check --json   # the problems as a JSON envelope"
)

// newCheckCommand validates the whole store, reading the files and never the index, and lists
// every problem as `path:line: message`. Any problem exits 1 after the list; the --json data is the
// list itself, as {path, line, code, message} objects. An outdated seeded Note is a warning, not a
// problem: it is in the result's Warnings, with or without problems, and changes no exit code.
func (a *app) newCheckCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "check",
		Short:   "Validate every config, issue and Note file of the store",
		Long:    checkLong,
		Example: checkExample,
		Args:    cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			problems, warnings, err := checkProblems(cwd)
			if err != nil {
				return err
			}
			data := append([]check.Problem{}, problems...)
			var text strings.Builder
			for _, problem := range problems {
				text.WriteString(problem.String() + "\n")
			}
			result := output.Result{Kind: "check", Data: data, Text: text.String(), Warnings: warnings}
			if err := a.emit(cmd, result); err != nil {
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

// checkProblems opens the store of the git working tree containing cwd and checks it, returning
// its problems and the warnings of its outdated seeded Notes (check.SeedWarnings). A config.yaml
// that cannot be used is the one problem reported, not a failure, and such a store gives no
// warning; any other open or check failure is returned as the store failure it is.
func checkProblems(cwd string) (problems []check.Problem, warnings []string, err error) {
	opened, err := store.Open(cwd)
	if err != nil {
		if problems := check.ConfigProblems(err); problems != nil {
			return problems, nil, nil
		}
		return nil, nil, output.StoreFailure(err)
	}
	problems, err = check.Run(opened)
	if err != nil {
		return nil, nil, output.StoreFailure(err)
	}
	warnings, err = check.SeedWarnings(opened)
	if err != nil {
		return nil, nil, output.StoreFailure(err)
	}
	return problems, warnings, nil
}
