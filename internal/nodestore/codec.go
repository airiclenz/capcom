package nodestore

import (
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
)

// Codec is what a Kind knows of its node files: how one parses, serializes and validates, which
// Aliases a parsed node records, and the noun that names one node in a message.
type Codec[T any] struct {
	// Noun names one node of the kind in a message: "issue" or "note".
	Noun string
	// Parse reads the file content data; path names the file in a failure, a *frontmatter.Error
	// citing the line when it is known.
	Parse func(path string, data []byte) (T, error)
	// Serialize writes node in the kind's canonical form, which Parse reads back unchanged.
	Serialize func(node T) []byte
	// Validate returns every problem of node, or nil when there is none; path names the file in
	// each problem.
	Validate func(path string, node T) []*frontmatter.Error
	// Aliases returns the former ids node records.
	Aliases func(node T) []string
}

// issueCodec is the Codec of the issue tree; cfg declares the statuses and types Validate accepts.
func issueCodec(cfg config.Config) Codec[issue.Issue] {
	return Codec[issue.Issue]{
		Noun:      "issue",
		Parse:     issue.Parse,
		Serialize: issue.Issue.Serialize,
		Validate: func(path string, node issue.Issue) []*frontmatter.Error {
			return issue.Validate(path, node, cfg)
		},
		Aliases: func(node issue.Issue) []string { return node.Aliases },
	}
}

// noteCodec is the Codec of the Knowledge tree.
func noteCodec() Codec[knowledge.Note] {
	return Codec[knowledge.Note]{
		Noun:      "note",
		Parse:     knowledge.Parse,
		Serialize: knowledge.Note.Serialize,
		Validate:  knowledge.Validate,
		Aliases:   func(node knowledge.Note) []string { return node.Aliases },
	}
}
