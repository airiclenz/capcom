package diagnose

import (
	"errors"
	"fmt"
	"slices"

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

// Verdict is what diagnose finds in one node file of kind T, issue.Issue or knowledge.Note: the
// node it parses, and every finding that keeps the file out of the index, sorted by where it comes
// from. A file that does not parse has the zero node and only its Parse finding.
type Verdict[T any] struct {
	// Parsed is the node the file parses to, or the zero node when it does not parse.
	Parsed T
	// Parse is the one Frontmatter finding that the file does not parse, or nil when it does.
	Parse []Finding
	// Fields is a Field finding for every value of Parsed that makes it no valid node, in order.
	Fields []Finding
	// Aliases is a finding for every alias of Parsed that tree.CheckAliases rejects, in order.
	Aliases []Finding
}

// IsParsed reports whether the file parses.
func (v Verdict[T]) IsParsed() bool {
	return len(v.Parse) == 0
}

// Findings returns every finding of the verdict, parse, field and alias findings in that order,
// or nil when the file is clean.
func (v Verdict[T]) Findings() []Finding {
	return slices.Concat(v.Parse, v.Fields, v.Aliases)
}

// IssueFile returns the verdict on data, the issue file at relPath whose id is id: its parse
// finding, placed at the path and line the parser blamed or at relPath with no line when it blamed
// none; otherwise a Field finding for every value cfg does not accept, and the findings of
// AliasFindings for its aliases.
func IssueFile(relPath, id string, data []byte, cfg config.Config) Verdict[issue.Issue] {
	parsed, err := issue.Parse(relPath, data)
	if err != nil {
		return Verdict[issue.Issue]{Parse: []Finding{parseFinding(relPath, err)}}
	}
	return Verdict[issue.Issue]{
		Parsed:  parsed,
		Fields:  fieldFindings(issue.Validate(relPath, parsed, cfg)),
		Aliases: AliasFindings(relPath, "issue", id, parsed.Aliases, parsed.AliasesLine()),
	}
}

// NoteFile returns the verdict on data, the Note file at relPath whose id is id, found as
// IssueFile finds it, with a Field finding for every value that makes it no usable Note.
func NoteFile(relPath, id string, data []byte) Verdict[knowledge.Note] {
	parsed, err := knowledge.Parse(relPath, data)
	if err != nil {
		return Verdict[knowledge.Note]{Parse: []Finding{parseFinding(relPath, err)}}
	}
	return Verdict[knowledge.Note]{
		Parsed:  parsed,
		Fields:  fieldFindings(knowledge.Validate(relPath, parsed)),
		Aliases: AliasFindings(relPath, "note", id, parsed.Aliases, parsed.AliasesLine()),
	}
}

// parseFinding is the Frontmatter finding of err, the failure to parse the file at relPath.
func parseFinding(relPath string, err error) Finding {
	var failure *frontmatter.Error
	if errors.As(err, &failure) {
		return Finding{Code: Frontmatter, Path: failure.Path, Line: failure.Line, Message: failure.Message}
	}
	return Finding{Code: Frontmatter, Path: relPath, Message: err.Error()}
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
