package nodestore

import (
	"slices"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
)

// Codec is what a Kind knows of its node files: how one parses, serializes and validates, which
// Aliases a parsed node records, the noun that names one node in a message, which of its fields
// reference other nodes, and the store format a write needs.
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
	// requiredFormat returns the store format (ADR 0009) that writing after, the node before
	// changed, needs config.yaml to declare: the format introducing a key value after gains, or 0
	// when it gains none. A new node is written with the zero node as before.
	requiredFormat func(before T, after T) int
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
		refs: descriptor.refs,
		requiredFormat: func(before issue.Issue, after issue.Issue) int {
			if gainsEntry(before.Related, after.Related) {
				return config.FormatRelated
			}
			return 0
		},
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
		refs: descriptor.refs,
		requiredFormat: func(before knowledge.Note, after knowledge.Note) int {
			if gainsSupersession(before, after) {
				return config.FormatSupersession
			}
			return 0
		},
	}
}

// gainsSupersession reports whether after, the edit of before, newly carries a format-3 key value:
// it is retired where before was not, or it has a superseded-by entry before lacks.
func gainsSupersession(before knowledge.Note, after knowledge.Note) bool {
	if after.IsRetired && !before.IsRetired {
		return true
	}
	return gainsEntry(before.SupersededBy, after.SupersededBy)
}

// gainsEntry reports whether after holds an entry that before lacks.
func gainsEntry(before []string, after []string) bool {
	return slices.ContainsFunc(after, func(entry string) bool {
		return !slices.Contains(before, entry)
	})
}
