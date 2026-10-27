package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/nodestore"
)

// kindFmt is the --json kind of capcom fmt.
const kindFmt = "fmt"

// fmtData is the --json data of capcom fmt: the files it rewrote, or with --check would rewrite,
// relative to the working tree root with forward slashes.
type fmtData struct {
	Paths []string `json:"paths"`
}

// formatRun collects what one capcom fmt did: the files it rewrote, or with isCheck would rewrite,
// and its warnings.
type formatRun struct {
	app      *app
	isCheck  bool
	paths    []string
	warnings []string
}

// Help text of capcom fmt.
const (
	fmtLong = "Rewrite every Issue and Note file whose bytes differ from its canonical form, listing each " +
		"one, then the count.\n\n" +
		"A file that does not parse or validate is left alone with a warning; capcom check is the " +
		"validator.\n\n" +
		"--check writes nothing, lists the files that would change and fails, exit code 1, when any " +
		"would."
	fmtExample = "  capcom fmt           # rewrite Issue and Note files in canonical form\n" +
		"  capcom fmt --check   # list what would change; exit 1 when anything would"
)

// newFmtCommand rewrites every issue and Note file whose bytes differ from its canonical form and
// lists each one, then the count. A file that does not parse or validate, and every tree problem,
// is a warning and the file is left alone; capcom check is the validator. Each kind is formatted
// by its node store, nodestore.Kind.Format, issues first. With --check it writes nothing, lists
// the files that would change and exits 1 when any would. The --json data is {paths}. Without
// --check the run is noted as a write of no Issue for the Tree lock warning (app.noteWrite).
func (a *app) newFmtCommand() *cobra.Command {
	var isCheck bool
	cmd := &cobra.Command{
		Use:         "fmt [--check]",
		Short:       "Rewrite every issue and Note file in canonical form",
		Long:        fmtLong,
		Example:     fmtExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: storeOnly},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			issues, err := nodestore.Issues(a.store, nil).Format(isCheck)
			if err != nil {
				return a.kindFailure(err, issueIDs)
			}
			notes, err := nodestore.Notes(a.store, nil).Format(isCheck)
			if err != nil {
				return a.kindFailure(err, noteIDs)
			}
			if !isCheck {
				a.noteWrite()
			}
			run := formatRun{
				app:      a,
				isCheck:  isCheck,
				paths:    append(issues.Paths, notes.Paths...),
				warnings: errorStrings(append(issues.Warnings, notes.Warnings...)),
			}
			return run.emit(cmd)
		}),
	}
	cmd.Flags().BoolVar(&isCheck, flagCheck, false, "write nothing; list the files that would change and fail when any would")
	return cmd
}

// emit renders the run: each file, then the count. Under --check any file that would change exits
// 1 after the list, which is the result itself, so --json keeps kind fmt without an error member.
func (r *formatRun) emit(cmd *cobra.Command) error {
	var text strings.Builder
	for _, path := range r.paths {
		text.WriteString(path + "\n")
	}
	summary := "formatted %d files\n"
	if r.isCheck {
		summary = "would format %d files\n"
	}
	fmt.Fprintf(&text, summary, len(r.paths))

	data := fmtData{Paths: append([]string{}, r.paths...)}
	result := output.Result{Kind: kindFmt, Data: data, Text: text.String(), Warnings: r.warnings}
	if err := r.app.emit(cmd, result); err != nil {
		return err
	}
	if !r.isCheck || len(r.paths) == 0 {
		return nil
	}
	return output.Rendered(output.Errorf(output.CodeInvalid, "%d files would change", len(r.paths)))
}

// errorStrings renders each error as its message.
func errorStrings[E error](errs []E) []string {
	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		messages = append(messages, err.Error())
	}
	return messages
}
