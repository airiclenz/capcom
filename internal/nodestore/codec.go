package nodestore

import (
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/tree"
)

// Codec is what a Kind knows of its node files: how one parses, serializes and validates, which
// Aliases a parsed node records, the noun that names one node in a message, and how a move of one
// node is worked out.
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
	// planMove works out the move of the node with exact id id in nodes to the destination to,
	// reading the node files but changing nothing, failing as the kind's PlanMove does.
	planMove func(nodes *tree.Tree, id string, to tree.Destination) (*plannedMove, error)
	// planRemove works out the removal of the node with exact id id in nodes, reading the node
	// files but changing nothing: it refuses a node with children first, then an ifMatch that is
	// not the node file's etag, as a *ConflictError at the absolute file with no message, then,
	// on the issue kind, dependents unless shouldDropDeps, failing otherwise as the kind's
	// PlanRemove does.
	planRemove func(nodes *tree.Tree, id string, ifMatch string, shouldDropDeps bool) (*plannedRemoval, error)
}

// plannedMove is a move a Codec worked out: the tree move, with the kind's rewrites added, and
// the moved node file's content before and after it.
type plannedMove struct {
	*tree.Move
	// original is the moved node file's content before the move, as read from disk.
	original []byte
	// content is the moved node file's content after the move.
	content []byte
	// unread are the files of other nodes that do not parse, so the references in them to the
	// moved node could not be checked and were left as they are.
	unread []string
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
		planMove:   planIssueMove,
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
		planMove:   planNoteMove,
		planRemove: planNoteRemove,
	}
}

// planIssueMove is the move planner of the issue tree: issue.PlanMove, which also rewrites the deps
// and related entries that name the moved issue.
func planIssueMove(nodes *tree.Tree, id string, to tree.Destination) (*plannedMove, error) {
	move, err := issue.PlanMove(nodes, id, to)
	if err != nil {
		return nil, err
	}
	return &plannedMove{Move: move.Move, original: move.Original, content: move.Content, unread: move.Unread}, nil
}

// planNoteMove is the move planner of the Knowledge tree: knowledge.PlanMove.
func planNoteMove(nodes *tree.Tree, id string, to tree.Destination) (*plannedMove, error) {
	move, err := knowledge.PlanMove(nodes, id, to)
	if err != nil {
		return nil, err
	}
	return &plannedMove{Move: move.Move, original: move.Original, content: move.Content}, nil
}
