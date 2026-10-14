package nodestore

import (
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
)

// Codec is what a Kind knows of its node files: how one parses, serializes and validates, which
// Aliases a parsed node records, the noun that names one node in a message, which of its fields
// reference other nodes, and which of its key lines need a store format.
type Codec[T any] struct {
	// Noun names one node of the kind in a message, as the kind's Descriptor names it.
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
	// nounPhrase names one node of the kind with its article, as the errors of a move or a removal
	// do; it is the kind's Descriptor's NounPhrase.
	nounPhrase string
	// setAliases returns node with its Aliases replaced by aliases.
	setAliases func(node T, aliases []string) T
	// refs are the node's reference fields, as the kind's Descriptor lists them, whose entries name
	// other nodes of the tree by id; a rename rewrites them, and a removal drops them, through one
	// engine.
	refs []refField[T]
	// formats is the kind's format table, as the kind's Descriptor lists it: every key whose line
	// a file may carry only from a later store format on.
	formats []KeyFormat[T]
}

// issueCodec is the Codec of the issue tree, filled from IssueDescriptor; cfg declares the
// statuses and types Validate accepts.
func issueCodec(cfg config.Config) Codec[issue.Issue] {
	descriptor := IssueDescriptor(cfg)
	return Codec[issue.Issue]{
		Noun:       descriptor.Noun,
		Parse:      issue.Parse,
		Serialize:  issue.Issue.Serialize,
		Validate:   descriptor.Validate,
		Aliases:    func(node issue.Issue) []string { return node.Aliases },
		nounPhrase: descriptor.NounPhrase,
		setAliases: func(node issue.Issue, aliases []string) issue.Issue {
			node.Aliases = aliases
			return node
		},
		refs:    descriptor.refs,
		formats: descriptor.Formats,
	}
}

// noteCodec is the Codec of the Knowledge tree, filled from NoteDescriptor.
func noteCodec() Codec[knowledge.Note] {
	descriptor := NoteDescriptor()
	return Codec[knowledge.Note]{
		Noun:       descriptor.Noun,
		Parse:      knowledge.Parse,
		Serialize:  knowledge.Note.Serialize,
		Validate:   descriptor.Validate,
		Aliases:    func(node knowledge.Note) []string { return node.Aliases },
		nounPhrase: descriptor.NounPhrase,
		setAliases: func(node knowledge.Note, aliases []string) knowledge.Note {
			node.Aliases = aliases
			return node
		},
		refs:    descriptor.refs,
		formats: descriptor.Formats,
	}
}

// requiredFormat returns the store format (ADR 0009) a file holding node, parsed from that file,
// needs config.yaml to declare: the latest format of the format table whose key has a line in the
// file, whatever the line holds, or 0 when the file carries none of them. A key line needs its
// format even when empty, because a release older than the key rejects the file for the key alone.
func (c Codec[T]) requiredFormat(node T) int {
	required := 0
	for _, row := range c.formats {
		if row.Line(node) > 0 {
			required = max(required, row.Format)
		}
	}
	return required
}
