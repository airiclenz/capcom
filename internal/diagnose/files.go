package diagnose

import (
	"errors"
	"fmt"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/tree"
)

// Finding is one problem with one issue or Note file, or with a file or directory of its tree.
// Path is relative to the working tree root with forward slashes; Line is 1-based, or 0 when no
// single line is to blame; Code names the kind of problem. check reports it, JSON-encoded in this
// field order with every key always present, and the index stores it, JSON-encoded, with the file
// it warns about.
type Finding struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Code    Kind   `json:"code"`
	Message string `json:"message"`
}

// String renders "path:line: message", or "path: message" when the line is unknown.
func (f Finding) String() string {
	if f.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", f.Path, f.Line, f.Message)
	}
	return fmt.Sprintf("%s: %s", f.Path, f.Message)
}

// ParseIssue parses data, the issue file at relPath, and returns the issue, or no issue and the
// one Frontmatter finding that the file does not parse: at the path and line the parser blamed, or
// at relPath with no line when it blamed none.
func ParseIssue(relPath string, data []byte) (issue.Issue, []Finding) {
	parsed, err := issue.Parse(relPath, data)
	if err != nil {
		return issue.Issue{}, []Finding{parseFinding(relPath, err)}
	}
	return parsed, nil
}

// ParseNote parses data, the Note file at relPath, and returns the Note, or no Note and the one
// Frontmatter finding that the file does not parse, placed as ParseIssue places it.
func ParseNote(relPath string, data []byte) (knowledge.Note, []Finding) {
	parsed, err := knowledge.Parse(relPath, data)
	if err != nil {
		return knowledge.Note{}, []Finding{parseFinding(relPath, err)}
	}
	return parsed, nil
}

// parseFinding is the Frontmatter finding of err, the failure to parse the file at relPath.
func parseFinding(relPath string, err error) Finding {
	var failure *frontmatter.Error
	if errors.As(err, &failure) {
		return Finding{Code: Frontmatter, Path: failure.Path, Line: failure.Line, Message: failure.Message}
	}
	return Finding{Code: Frontmatter, Path: relPath, Message: err.Error()}
}

// ValidateIssue returns a Field finding for every value of parsed, the issue file at relPath, that
// cfg does not accept, or nil when there is none.
func ValidateIssue(relPath string, parsed issue.Issue, cfg config.Config) []Finding {
	return fieldFindings(issue.Validate(relPath, parsed, cfg))
}

// ValidateNote returns a Field finding for every value of parsed, the Note file at relPath, that
// makes it no usable Note, or nil when there is none.
func ValidateNote(relPath string, parsed knowledge.Note) []Finding {
	return fieldFindings(knowledge.Validate(relPath, parsed))
}

// fieldFindings turns the problems a validator reports into Field findings, in order.
func fieldFindings(problems []*frontmatter.Error) []Finding {
	var findings []Finding
	for _, problem := range problems {
		findings = append(findings, Finding{Code: Field, Path: problem.Path, Line: problem.Line, Message: problem.Message})
	}
	return findings
}

// AliasFindings returns a finding for every alias of the node file at relPath that
// tree.CheckAliases rejects: an InvalidAlias finding for one that is not a valid name, and a
// RepeatedAlias finding for one that repeats id, the node's own, or is listed twice. kind names
// the node in a message, "issue" or "note", as CheckAliases words it. Every finding cites line,
// the line of the file's aliases key, as check cites it; 0 when it is unknown.
func AliasFindings(relPath, kind, id string, aliases []string, line int) []Finding {
	_, rejected := tree.CheckAliases(relPath, kind, id, aliases)
	var findings []Finding
	for _, problem := range rejected {
		// CheckAliases rejects an alias only as invalid or repeated, both sentinels TreeKind knows.
		problemKind, _ := TreeKind(problem)
		findings = append(findings, Finding{
			Code:    problemKind,
			Path:    problem.Path,
			Line:    line,
			Message: problem.Message(),
		})
	}
	return findings
}
