package index

import (
	"errors"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/tree"
)

// parseIssue parses and validates one issue file and returns every problem that keeps it out of
// the index.
func parseIssue(file foundFile, data []byte, cfg config.Config) (issue.Issue, []Warning) {
	parsed, err := issue.Parse(file.path, data)
	if err != nil {
		var failure *issue.Error
		if errors.As(err, &failure) {
			return issue.Issue{}, []Warning{{Kind: WarningFrontmatter, Path: failure.Path, Line: failure.Line, Message: failure.Message}}
		}
		return issue.Issue{}, []Warning{{Kind: WarningFrontmatter, Path: file.path, Message: err.Error()}}
	}

	var problems []Warning
	for _, failure := range issue.Validate(file.path, parsed, cfg) {
		problems = append(problems, Warning{Kind: WarningField, Path: failure.Path, Line: failure.Line, Message: failure.Message})
	}
	return parsed, append(problems, aliasProblems(file, parsed.Aliases)...)
}

// aliasProblems returns a problem for every alias of the node file that tree.CheckAliases rejects:
// one that is invalid, repeats the node's own id or is listed twice.
func aliasProblems(file foundFile, aliases []string) []Warning {
	_, rejected := tree.CheckAliases(file.path, file.kind, file.id, aliases)
	var problems []Warning
	for _, problem := range rejected {
		// CheckAliases rejects an alias only as invalid or repeated, both sentinels TreeKind knows.
		kind, _ := diagnose.TreeKind(problem)
		problems = append(problems, Warning{Kind: kind, Path: problem.Path, Message: problem.Message()})
	}
	return problems
}
