package index

import (
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/issue"
)

// parseIssue parses and validates one issue file and returns every problem that keeps it out of
// the index.
func parseIssue(file foundFile, data []byte, cfg config.Config) (issue.Issue, []diagnose.Finding) {
	parsed, problems := diagnose.ParseIssue(file.path, data)
	if len(problems) > 0 {
		return issue.Issue{}, problems
	}
	problems = diagnose.ValidateIssue(file.path, parsed, cfg)
	return parsed, append(problems, aliasProblems(file, parsed.Aliases)...)
}

// aliasProblems returns a problem for every alias of the node file that tree.CheckAliases rejects:
// one that is invalid, repeats the node's own id or is listed twice.
func aliasProblems(file foundFile, aliases []string) []diagnose.Finding {
	return diagnose.AliasFindings(file.path, file.kind, file.id, aliases)
}
