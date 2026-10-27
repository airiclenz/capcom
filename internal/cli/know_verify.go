package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodestore"
)

// findingData is one Ref that fails in the --json data of know verify; Code is broken-ref or
// unshared-ref.
type findingData struct {
	Code    string `json:"code"`
	Note    string `json:"note"`
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Ref     string `json:"ref"`
	Message string `json:"message"`
}

// Help text of capcom know verify.
const (
	knowVerifyLong = "List every Ref of every Note that no longer resolves: a missing path, a path#symbol whose " +
		"file lacks the symbol text, or a path outside the working tree.\n\n" +
		"It also lists a Ref a clone would lack although it resolves here: one inside the git " +
		"directory, or one git ignores.\n\n" +
		"Each line cites the Ref's line in its Note as path:line.\n\n" +
		"Retired subtrees are exempt.\n\n" +
		"It takes no arguments, and any finding fails the command, exit code 1, after the list."
	knowVerifyExample = "  capcom know verify          # Note refs that no longer resolve\n" +
		"  capcom know verify --json   # the findings as a JSON envelope"
)

// newKnowVerifyCommand lists every Ref of every Note that no longer resolves: a missing path, a
// `path#symbol` whose file lacks the symbol text, or a path outside the working tree; and every
// Ref a clone would lack: one inside the git directory, or one git ignores. Each line cites the
// Ref's line in its Note. A retired subtree is exempt. Any finding fails the command, after the
// list.
func (a *app) newKnowVerifyCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "verify",
		Short:       "List the Note refs that no longer resolve",
		Long:        knowVerifyLong,
		Example:     knowVerifyExample,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationIndex: indexRefresh},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			findings, err := a.verifyNotes()
			if err != nil {
				return err
			}
			data := make([]findingData, 0, len(findings))
			var text strings.Builder
			for _, finding := range findings {
				data = append(data, findingData{
					string(finding.Code), finding.NoteID, finding.Path, finding.Line, finding.Ref, finding.Message,
				})
				text.WriteString(finding.String() + "\n")
			}
			result := output.Result{Kind: "know-verify", Data: data, Text: text.String()}
			if len(findings) > 0 {
				result.Failure = output.Errorf(output.CodeInvalid, "%s", verifyFailure(findings))
			}
			return a.emit(cmd, result)
		}),
	}
}

// verifyNotes reads every usable Note file outside a retired subtree and checks its Refs, in tree
// order.
func (a *app) verifyNotes() ([]knowledge.Finding, error) {
	notes, err := a.issueIndex.Notes(false)
	if err != nil {
		return nil, err
	}
	files := make([]knowledge.NoteFile, 0, len(notes))
	reader := nodestore.NoteReader()
	for _, entry := range notes {
		reading, err := parseNodeFile(a, reader, entry.Path)
		if err != nil {
			return nil, err
		}
		files = append(files, knowledge.NoteFile{ID: entry.ID, Path: entry.Path, Note: reading.Node})
	}
	findings, err := knowledge.Verify(nodestore.RefRepository(a.store), files)
	if err != nil {
		return nil, output.AsError(err)
	}
	return findings, nil
}

// verifyFailure summarises findings for the error know verify fails with: how many Refs do not
// resolve, and how many name a path a clone lacks, leaving out a count of zero.
func verifyFailure(findings []knowledge.Finding) string {
	var broken, unshared int
	for _, finding := range findings {
		if finding.Code == diagnose.UnsharedRef {
			unshared++
		} else {
			broken++
		}
	}
	var parts []string
	if broken > 0 {
		parts = append(parts, fmt.Sprintf("%d Note refs do not resolve", broken))
	}
	if unshared > 0 {
		parts = append(parts, fmt.Sprintf("%d Note refs name a path a clone lacks", unshared))
	}
	return strings.Join(parts, "; ")
}
