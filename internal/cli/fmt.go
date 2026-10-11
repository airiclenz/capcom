package cli

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// kindFmt is the --json kind of capcom fmt.
const kindFmt = "fmt"

// fmtData is the --json data of capcom fmt: the files it rewrote, or with --check would rewrite,
// relative to the working tree root with forward slashes.
type fmtData struct {
	Paths []string `json:"paths"`
}

// canonicalForm is the canonical content of one store file and the write that puts it in place.
type canonicalForm struct {
	content []byte
	write   func() error
}

// canonicalizer parses and validates the store file at path, holding data, and returns its
// canonical form, or the warnings that keep the file from being formatted.
type canonicalizer func(path string, data []byte) (canonicalForm, []string)

// formatRun collects what one capcom fmt did: the files it rewrote, or with isCheck would rewrite,
// and its warnings.
type formatRun struct {
	app      *app
	isCheck  bool
	paths    []string
	warnings []string
}

// newFmtCommand rewrites every issue and Note file whose bytes differ from its canonical form and
// lists each one, then the count. A file that does not parse or validate, and every tree problem,
// is a warning and the file is left alone; capcom check is the validator. With --check it writes
// nothing, lists the files that would change and exits 1 when any would. The --json data is
// {paths}.
func (a *app) newFmtCommand() *cobra.Command {
	var isCheck bool
	cmd := &cobra.Command{
		Use:   "fmt [--check]",
		Short: "Rewrite every issue and Note file in canonical form",
		Args:  cobra.NoArgs,
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			opened, err := store.Open(cwd)
			if err != nil {
				return storeFailure(err)
			}
			a.store = opened
			run := formatRun{app: a, isCheck: isCheck}
			if err := run.formatTree(opened.IssuesDir(), readAliases, a.canonicalIssue); err != nil {
				return err
			}
			if err := run.formatTree(opened.KnowledgeDir(), readNoteAliases, a.canonicalNote); err != nil {
				return err
			}
			return run.emit(cmd)
		}),
	}
	cmd.Flags().BoolVar(&isCheck, flagCheck, false, "write nothing; list the files that would change and fail when any would")
	return cmd
}

// formatTree brings every file the tree in dir places to its canonical form. Each tree problem is
// a warning, as are the warnings canonicalize returns for a file, which is then left alone. A file
// that cannot be read or written fails the run.
func (r *formatRun) formatTree(dir string, aliases tree.AliasFunc, canonicalize canonicalizer) error {
	loaded, err := tree.Load(dir, aliases)
	problems, err := r.app.treeProblems(err)
	if err != nil {
		return err
	}
	for _, problem := range problems {
		r.warnings = append(r.warnings, r.app.relativeTreeError(problem).Error())
	}

	for _, node := range loaded.Nodes() {
		path := r.app.worktreeRelative(node.Path)
		data, err := r.app.readIssueFile(path)
		if err != nil {
			return err
		}
		form, warnings := canonicalize(path, data)
		if len(warnings) > 0 {
			r.warnings = append(r.warnings, warnings...)
			continue
		}
		if bytes.Equal(data, form.content) {
			continue
		}
		if !r.isCheck {
			if err := form.write(); err != nil {
				return err
			}
		}
		r.paths = append(r.paths, path)
	}
	return nil
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
	result := Result{Kind: kindFmt, Data: data, Text: text.String(), Warnings: r.warnings}
	if err := r.app.emit(cmd, result); err != nil {
		return err
	}
	if !r.isCheck || len(r.paths) == 0 {
		return nil
	}
	failure := Errorf(CodeInvalid, "%d files would change", len(r.paths))
	failure.isRendered = true
	return failure
}

// canonicalIssue is the canonicalizer of issue files: it parses and validates against the config
// before anything is written, so writeIssueFile never refuses the issue.
func (a *app) canonicalIssue(path string, data []byte) (canonicalForm, []string) {
	parsed, err := issue.Parse(path, data)
	if err != nil {
		return canonicalForm{}, []string{err.Error()}
	}
	if problems := issue.Validate(path, parsed, a.store.Config); len(problems) > 0 {
		return canonicalForm{}, errorStrings(problems)
	}
	write := func() error {
		_, err := a.writeIssueFile(path, parsed)
		return err
	}
	return canonicalForm{content: parsed.Serialize(), write: write}, nil
}

// canonicalNote is the canonicalizer of Note files: it parses and validates before anything is
// written, so writeNoteFile never refuses the Note.
func (a *app) canonicalNote(path string, data []byte) (canonicalForm, []string) {
	parsed, err := knowledge.Parse(path, data)
	if err != nil {
		return canonicalForm{}, []string{err.Error()}
	}
	if problems := knowledge.Validate(path, parsed); len(problems) > 0 {
		return canonicalForm{}, errorStrings(problems)
	}
	write := func() error {
		_, err := a.writeNoteFile(path, parsed)
		return err
	}
	return canonicalForm{content: parsed.Serialize(), write: write}, nil
}

// errorStrings renders each error as its message.
func errorStrings[E error](errs []E) []string {
	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		messages = append(messages, err.Error())
	}
	return messages
}
