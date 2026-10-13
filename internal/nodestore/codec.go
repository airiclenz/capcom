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
	// nounPhrase names one node of the kind with its article, as the errors of a move or a removal
	// do: "an issue" or "a note".
	nounPhrase string
	// setAliases returns node with its Aliases replaced by aliases.
	setAliases func(node T, aliases []string) T
	// refs are the node's reference fields, whose entries name other nodes of the tree by id; a
	// rename rewrites them, and a removal drops them, through one engine.
	refs []refField[T]
	// requiredFormat returns the store format (ADR 0009) that writing after, the node before
	// changed, needs config.yaml to declare: the format introducing a key value after gains, or 0
	// when it gains none. A new node is written with the zero node as before.
	requiredFormat func(before T, after T) int
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
		requiredFormat: func(before issue.Issue, after issue.Issue) int {
			if gainsEntry(before.Related, after.Related) {
				return config.FormatRelated
			}
			return 0
		},
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
				key: RefSupersededBy,
				get: func(node knowledge.Note) []string { return node.SupersededBy },
				set: func(node knowledge.Note, entries []string) knowledge.Note {
					node.SupersededBy = entries
					return node
				},
				isSelfRefused: true,
			},
		},
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
