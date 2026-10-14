package nodestore

import (
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
)

// keyRetired is a Note's retired key, which needs the supersession store format.
const keyRetired = "retired"

// Descriptor is what a Tree kind is, apart from its files on disk: the noun that names one of its
// nodes in a message, how a parsed node is validated, the fields that reference other nodes and
// the store format (ADR 0009) each format-gated key needs. It is built without a store, so a
// reader that holds no Kind describes the kind exactly as the Kind's Codec does.
type Descriptor[T any] struct {
	// Noun names one node of the kind in a message: "issue" or "Note".
	Noun string
	// NounPhrase names one node of the kind with its article, as the errors of a move or a removal
	// do: "an issue" or "a Note".
	NounPhrase string
	// Validate returns every problem of node, or nil when there is none; path names the file in
	// each problem.
	Validate func(path string, node T) []*frontmatter.Error
	// Formats is the kind's format table: every key a file may carry only from a later store
	// format on, in file order. A kind with no such key has none.
	Formats []KeyFormat[T]
	// refs are the kind's reference fields, whose entries name other nodes of the tree by id.
	refs []refField[T]
}

// KeyFormat is one row of a kind's format table: a frontmatter key, the first store format whose
// files may carry it, and the accessor of the key's line in a parsed node.
type KeyFormat[T any] struct {
	// Key is the frontmatter key.
	Key string
	// Format is the first store format (config.Format*) whose files may carry Key.
	Format int
	// Line returns the 1-based line of Key in the file node was parsed from, or 0 when the file
	// has none or node was built in memory.
	Line func(node T) int
}

// RefKeys returns the keys of the kind's reference fields, in the order the kind lists them.
func (d Descriptor[T]) RefKeys() []string {
	keys := make([]string, 0, len(d.refs))
	for _, field := range d.refs {
		keys = append(keys, field.key)
	}
	return keys
}

// IssueDescriptor is the Descriptor of the issue tree; cfg declares the statuses and types its
// Validate accepts. Only Validate depends on cfg: the nouns, format table and ref keys are the same
// for every cfg, so a reader with no store may take them from IssueDescriptor(config.Config{}).
func IssueDescriptor(cfg config.Config) Descriptor[issue.Issue] {
	return Descriptor[issue.Issue]{
		Noun:       "issue",
		NounPhrase: "an issue",
		Validate: func(path string, node issue.Issue) []*frontmatter.Error {
			return issue.Validate(path, node, cfg)
		},
		Formats: []KeyFormat[issue.Issue]{
			{Key: RefRelated, Format: config.FormatRelated, Line: issue.Issue.RelatedLine},
		},
		refs: []refField[issue.Issue]{
			{
				key: RefDeps,
				get: func(node issue.Issue) []string { return node.Deps },
				set: func(node issue.Issue, entries []string) issue.Issue {
					node.Deps = entries
					return node
				},
				blocksRemoval: true,
			},
			{
				key: RefRelated,
				get: func(node issue.Issue) []string { return node.Related },
				set: func(node issue.Issue, entries []string) issue.Issue {
					node.Related = entries
					return node
				},
			},
		},
	}
}

// NoteDescriptor is the Descriptor of the Knowledge tree.
func NoteDescriptor() Descriptor[knowledge.Note] {
	return Descriptor[knowledge.Note]{
		Noun:       "Note",
		NounPhrase: "a Note",
		Validate:   knowledge.Validate,
		Formats: []KeyFormat[knowledge.Note]{
			{Key: keyRetired, Format: config.FormatSupersession, Line: knowledge.Note.RetiredLine},
			{Key: RefSupersededBy, Format: config.FormatSupersession, Line: knowledge.Note.SupersededByKeyLine},
		},
		refs: []refField[knowledge.Note]{
			{
				key: RefSupersededBy,
				get: func(node knowledge.Note) []string { return node.SupersededBy },
				set: func(node knowledge.Note, entries []string) knowledge.Note {
					node.SupersededBy = entries
					return node
				},
				isSelfRefused: true,
			},
		},
	}
}
