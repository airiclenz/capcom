package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/airiclenz/capcom/internal/knowledge"
)

// findingData is one broken Ref in the --json data of know verify.
type findingData struct {
	Note    string `json:"note"`
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Ref     string `json:"ref"`
	Message string `json:"message"`
}

// newKnowVerifyCommand lists every Ref of every Note that no longer resolves: a missing path, a
// `path#symbol` whose file lacks the symbol text, or a path outside the working tree. Each line
// cites the Ref's line in its Note. A retired subtree is exempt. Any finding fails the command,
// after the list.
func (a *app) newKnowVerifyCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "verify",
		Short:       "List the Note refs that no longer resolve",
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
				data = append(data, findingData{finding.NoteID, finding.Path, finding.Line, finding.Ref, finding.Message})
				text.WriteString(finding.String() + "\n")
			}
			result := Result{Kind: "know-verify", Data: data, Text: text.String()}
			if len(findings) > 0 {
				result.Failure = Errorf(CodeInvalid, "%d note refs do not resolve", len(findings))
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
	var findings []knowledge.Finding
	for _, entry := range notes {
		data, err := a.readIssueFile(entry.Path)
		if err != nil {
			return nil, err
		}
		note, err := knowledge.Parse(entry.Path, data)
		if err != nil {
			return nil, a.frontmatterFailure(err)
		}
		found, err := knowledge.Verify(a.store.WorktreeRoot, entry.Path, entry.ID, note)
		if err != nil {
			return nil, asError(err)
		}
		findings = append(findings, found...)
	}
	return findings, nil
}
