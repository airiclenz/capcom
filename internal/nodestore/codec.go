package nodestore

import (
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/tree"
)

// Codec is what a Kind knows of its node files: how one parses, serializes and validates, which
// Aliases a parsed node records, the noun that names one node in a message, which of its fields
// reference other nodes, and how a removal of one node is worked out.
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
	// nounPhrase names one node of the kind with its article, as the errors of a move do: "an
	// issue" or "a note".
	nounPhrase string
	// setAliases returns node with its Aliases replaced by aliases.
	setAliases func(node T, aliases []string) T
	// refs are the node's reference fields, whose entries name other nodes of the tree by id; a
	// rename rewrites them through one engine.
	refs []refField[T]
	// planRemove works out the removal of the node with exact id id in nodes, reading the node
	// files but changing nothing: it refuses a node with children first, then an ifMatch that is
	// not the node file's etag, as a *ConflictError at the absolute file with no message, then,
	// on the issue kind, dependents unless shouldDropDeps, failing otherwise as the kind's
	// PlanRemove does.
	planRemove func(nodes *tree.Tree, id string, ifMatch string, shouldDropDeps bool) (*plannedRemoval, error)
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
		Aliases:    func(node issue.Issue) []string { return node.Aliases },
		nounPhrase: "an issue",
		setAliases: func(node issue.Issue, aliases []string) issue.Issue {
			node.Aliases = aliases
			return node
		},
		refs: []refField[issue.Issue]{
			{
				key: "deps",
				get: func(node issue.Issue) []string { return node.Deps },
				set: func(node issue.Issue, entries []string) issue.Issue {
					node.Deps = entries
					return node
				},
			},
			{
				key: "related",
				get: func(node issue.Issue) []string { return node.Related },
				set: func(node issue.Issue, entries []string) issue.Issue {
					node.Related = entries
					return node
				},
			},
		},
		planRemove: planIssueRemove,
	}
}

// noteCodec is the Codec of the Knowledge tree.
func noteCodec() Codec[knowledge.Note] {
	return Codec[knowledge.Note]{
		Noun:       "note",
		Parse:      knowledge.Parse,
		Serialize:  knowledge.Note.Serialize,
		Validate:   knowledge.Validate,
		Aliases:    func(node knowledge.Note) []string { return node.Aliases },
		nounPhrase: "a note",
		setAliases: func(node knowledge.Note, aliases []string) knowledge.Note {
			node.Aliases = aliases
			return node
		},
		refs: []refField[knowledge.Note]{
			{
				key: "superseded-by",
				get: func(node knowledge.Note) []string { return node.SupersededBy },
				set: func(node knowledge.Note, entries []string) knowledge.Note {
					node.SupersededBy = entries
					return node
				},
				isSelfRefused: true,
			},
		},
		planRemove: planNoteRemove,
	}
}
