package index

import (
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/issue"
)

// parseIssue parses and validates one issue file and returns the issue with every problem of its
// verdict, each of which keeps it out of the index.
func parseIssue(file foundFile, data []byte, cfg config.Config) (issue.Issue, []diagnose.Finding) {
	verdict := diagnose.IssueFile(file.path, file.id, data, cfg)
	return verdict.Parsed, verdict.Findings()
}
