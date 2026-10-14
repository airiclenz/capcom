package nodestore

import (
	"errors"
	"slices"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/tree"
)

// errUnparsed is the failure of a tree.AliasParseFunc built on a Reader when the file does not
// parse; tree.ReadAliases only tells it from success.
var errUnparsed = errors.New("node file does not parse")

// Reader is the Kind reader of one Tree kind: the one place a node file of the kind is read and
// decoded. It is built without a store, from what builds the kind's Descriptor, so every reader of
// node files judges a file exactly as the Kind does.
type Reader[T any] struct {
	// verdict decodes data, the node file at relPath whose id is id, as the kind's Descriptor
	// describes it.
	verdict func(relPath, id string, data []byte) diagnose.Verdict[T]
	// aliases returns the Aliases a parsed node records.
	aliases func(node T) []string
}

// Reading is what a Reader finds in one node file: the node, the file's etag and Aliases, and
// every finding that keeps the file out of the index, sorted by where it comes from. Every
// finding names the file relative to the working tree root with forward slashes.
type Reading[T any] struct {
	// Node is the node the file parses to, or the zero node when it was not read or does not parse.
	Node T
	// ETag is nodefile.ETag of the file's content, or "" when the read was refused.
	ETag string
	// Aliases are what tree.ReadAliases returns for the file: the node's Aliases when it parses, and
	// the aliases still recovered from its raw frontmatter when it does not; none when the read was
	// refused.
	Aliases []string
	// Refusal is the one NotRegular finding of a file nodefile.ReadRegularFile refused unread (not
	// a regular file, or larger than 1 MiB), or nil when the file was read.
	Refusal []diagnose.Finding
	// Parse is the one Frontmatter finding that the file does not parse, or nil when it does or
	// was not read.
	Parse []diagnose.Finding
	// Fields is a Field finding for every value of Node that makes it no valid node, in order.
	Fields []diagnose.Finding
	// AliasFindings is a finding for every alias of Node that tree.CheckAliases rejects, in order.
	AliasFindings []diagnose.Finding
}

// IsRead reports whether the file was read, not refused unread.
func (r Reading[T]) IsRead() bool {
	return len(r.Refusal) == 0
}

// IsParsed reports whether the file was read and parses.
func (r Reading[T]) IsParsed() bool {
	return r.IsRead() && len(r.Parse) == 0
}

// Findings returns every finding of the reading, refusal, parse, field and alias findings in that
// order, or nil when the file is clean.
func (r Reading[T]) Findings() []diagnose.Finding {
	return slices.Concat(r.Refusal, r.Parse, r.Fields, r.AliasFindings)
}

// IsUsable reports whether the file is usable: it has no finding at all, as the index keeps only
// such a file.
func (r Reading[T]) IsUsable() bool {
	return len(r.Findings()) == 0
}

// IssueReader is the Kind reader of the issue tree, whose files IssueDescriptor(cfg) describes;
// cfg declares the statuses and types a usable issue may carry.
func IssueReader(cfg config.Config) Reader[issue.Issue] {
	return Reader[issue.Issue]{
		verdict: func(relPath, id string, data []byte) diagnose.Verdict[issue.Issue] {
			return diagnose.IssueFile(relPath, id, data, cfg)
		},
		aliases: func(node issue.Issue) []string { return node.Aliases },
	}
}

// NoteReader is the Kind reader of the Knowledge tree, whose files NoteDescriptor describes.
func NoteReader() Reader[knowledge.Note] {
	return Reader[knowledge.Note]{
		verdict: diagnose.NoteFile,
		aliases: func(node knowledge.Note) []string { return node.Aliases },
	}
}

// Read reads and decodes the node file at path, which relPath names relative to the working tree
// root with forward slashes, as every finding names it. A file nodefile.ReadRegularFile refuses
// (not a regular file, or larger than 1 MiB) is no failure: the Reading carries its NotRegular
// finding, worded as the walk words it. A file that cannot be read otherwise fails with the read's
// error.
func (r Reader[T]) Read(path, relPath string) (Reading[T], error) {
	data, err := nodefile.ReadRegularFile(path)
	if refusal := diagnose.ReadRefusal(err); refusal != nil {
		return Reading[T]{
			Refusal: []diagnose.Finding{{Code: diagnose.NotRegular, Path: relPath, Message: refusal.Error()}},
		}, nil
	}
	if err != nil {
		return Reading[T]{}, err
	}
	return r.Decode(relPath, data), nil
}

// Decode decodes data, the content already read of the node file relPath names relative to the
// working tree root with forward slashes; the node's id is the file's name.
func (r Reader[T]) Decode(relPath string, data []byte) Reading[T] {
	verdict := r.verdict(relPath, tree.IDOf(relPath), data)
	aliases, _ := tree.ReadAliases(data, func([]byte) ([]string, error) {
		if !verdict.IsParsed() {
			return nil, errUnparsed
		}
		return r.aliases(verdict.Parsed), nil
	})
	return Reading[T]{
		Node:          verdict.Parsed,
		ETag:          nodefile.ETag(data),
		Aliases:       aliases,
		Parse:         verdict.Parse,
		Fields:        verdict.Fields,
		AliasFindings: verdict.Aliases,
	}
}
