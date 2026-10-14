package diagnose

import (
	"fmt"

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
