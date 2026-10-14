package nodestore

import (
	"errors"
	"slices"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/diagnose"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/tree"
)

// errUnparsed is the failure of a tree.AliasParseFunc built on a Reader when the file does not
// parse; tree.ReadAliases only tells it from success.
var errUnparsed = errors.New("node file does not parse")

// Reader is the Kind reader of one Tree kind: the one place a node file of the kind is read and
// decoded. It is built without a store, from the kind's Descriptor, so every reader of node files
// judges a file exactly as the Kind does.
type Reader[T any] struct {
	// descriptor describes the kind: the noun its alias findings name a node with, and how a parsed
	// node is validated.
	descriptor Descriptor[T]
	// parse parses data, the node file at relPath.
	parse func(relPath string, data []byte) (T, error)
	// aliases returns the Aliases a parsed node records.
	aliases func(node T) []string
	// aliasesLine returns the 1-based line of a parsed node's aliases key, or 0 when it has none.
	aliasesLine func(node T) int
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
		descriptor:  IssueDescriptor(cfg),
		parse:       issue.Parse,
		aliases:     func(node issue.Issue) []string { return node.Aliases },
		aliasesLine: issue.Issue.AliasesLine,
	}
}

// NoteReader is the Kind reader of the Knowledge tree, whose files NoteDescriptor describes.
func NoteReader() Reader[knowledge.Note] {
	return Reader[knowledge.Note]{
		descriptor:  NoteDescriptor(),
		parse:       knowledge.Parse,
		aliases:     func(node knowledge.Note) []string { return node.Aliases },
		aliasesLine: knowledge.Note.AliasesLine,
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
// working tree root with forward slashes; the node's id is the file's name. A file that does not
// parse has the zero node and only its Parse finding, placed at the path and line the parser
// blamed, or at relPath with no line when it blamed none. One that parses has a Field finding for
// every value the Descriptor's Validate rejects, and the findings of diagnose.AliasFindings for
// its Aliases.
func (r Reader[T]) Decode(relPath string, data []byte) Reading[T] {
	reading := Reading[T]{ETag: nodefile.ETag(data)}
	parsed, err := r.parse(relPath, data)
	if err != nil {
		reading.Parse = []diagnose.Finding{parseFinding(relPath, err)}
	} else {
		reading.Node = parsed
		reading.Fields = fieldFindings(r.descriptor.Validate(relPath, parsed))
		reading.AliasFindings = diagnose.AliasFindings(
			relPath,
			r.descriptor.Noun,
			tree.IDOf(relPath),
			r.aliases(parsed),
			r.aliasesLine(parsed),
		)
	}
	reading.Aliases, _ = tree.ReadAliases(data, func([]byte) ([]string, error) {
		if err != nil {
			return nil, errUnparsed
		}
		return r.aliases(parsed), nil
	})
	return reading
}

// parseFinding is the Frontmatter finding of err, the failure to parse the file at relPath.
func parseFinding(relPath string, err error) diagnose.Finding {
	var failure *frontmatter.Error
	if errors.As(err, &failure) {
		return diagnose.Finding{
			Code:    diagnose.Frontmatter,
			Path:    failure.Path,
			Line:    failure.Line,
			Message: failure.Message,
		}
	}
	return diagnose.Finding{Code: diagnose.Frontmatter, Path: relPath, Message: err.Error()}
}

// fieldFindings turns the problems a validator reports into Field findings, in order.
func fieldFindings(problems []*frontmatter.Error) []diagnose.Finding {
	var findings []diagnose.Finding
	for _, problem := range problems {
		findings = append(findings, diagnose.Finding{
			Code:    diagnose.Field,
			Path:    problem.Path,
			Line:    problem.Line,
			Message: problem.Message,
		})
	}
	return findings
}
