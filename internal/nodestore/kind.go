package nodestore

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/gitx"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// Kind is the node store of one Tree kind. Every path it reports, in a Written or a typed failure,
// is relative to the working tree root with forward slashes.
type Kind[T any] struct {
	codec Codec[T]
	// reader is the Kind reader of the kind, through which every node file is read and decoded.
	reader Reader[T]
	// dir is the directory that holds the kind's tree.
	dir string
	// knowledgeDir is the directory that holds the store's Knowledge tree, whose Notes' Refs a move
	// rewrites when they name the moved node file or its children directory.
	knowledgeDir string
	// worktreeRoot is the top directory of the git working tree.
	worktreeRoot string
	// relative names a path relative to the working tree root with forward slashes, as
	// store.Store.Relative does; a path outside the working tree is kept, with forward slashes.
	relative func(path string) string
	// resolver resolves an argument naming an existing node; it may be nil for a caller that
	// never resolves one.
	resolver Resolver
	// applyMove carries out a planned move, returning its cleanup failures as warnings; a test
	// swaps it to inject a failure of the renames and rewrites Apply makes.
	applyMove func(move *tree.Move) ([]error, error)
	// applyRemoval carries out a planned removal, returning its cleanup failures as warnings; a
	// test swaps it to inject a failure of the rewrites and removals Apply makes.
	applyRemoval func(removal *tree.Removal) ([]error, error)
	// formatGate refuses a write of a file carrying the line of a key that needs a newer store
	// format than the store declares, as store.Store.RequireFormat does; WithFormatGate swaps it.
	formatGate FormatGate
	// fsys is the file system the load, ReadContent, RecoverAliases, Create and Edit read and write
	// the tree through: nodefile.Disk unless WithFS swaps it.
	fsys nodefile.FS
	// sync is the hook WithSync sets, run once after each write that lands; nil runs none.
	sync SyncFunc
}

// SyncFunc is a Kind's sync hook: it brings what the caller keeps in step with the tree up to date
// after a write has landed, and reports whether it changed anything. Its failure never fails the
// write; the write's Effects carry it.
type SyncFunc func() (bool, error)

// FormatGate refuses a write of a node file carrying the line of key, which needs store format
// version (ADR 0019), with an error, typically a *store.FormatError; it returns nil to let the write
// go ahead. It never raises the format.
type FormatGate func(key string, version int) error

// Resolver resolves an argument naming an existing node, its id or one of its Aliases, to that
// node. It fails with an error the caller types itself when no node answers to the argument.
type Resolver interface {
	Resolve(arg string) (Target, error)
}

// Target is a node a Resolver resolved: its id and its file, relative to the working tree root
// with forward slashes.
type Target struct {
	ID   string
	Path string
}

// Written is the outcome of a write: the node's id, its file relative to the working tree root
// with forward slashes, the file's new etag, and the Effects of the write beyond the node file.
type Written struct {
	ID   string
	Path string
	ETag string
	Effects
}

// Effects are what a write changed besides the node file.
type Effects struct {
	// IsSynced reports whether the Kind's sync hook, run once the write had landed, changed what it
	// keeps in step with the tree; false when the Kind has no hook, it ran none or it failed.
	IsSynced bool
	// SyncFailure is the failure of the Kind's sync hook; the write itself has landed. It is nil
	// when the hook succeeded or did not run.
	SyncFailure error
	// DroppedComments lists the files the write rewrote whose frontmatter held a YAML comment,
	// which the canonical form it writes does not keep; empty when it dropped none.
	DroppedComments CommentDrops
}

// CommentDrops lists the files, relative to the working tree root with forward slashes, whose
// frontmatter comments a write dropped, in the order it rewrote them. It is a string, the paths
// separated by newlines, so that Effects stays comparable with ==; Paths lists them.
type CommentDrops string

// Paths returns the files d lists, in order; nil when it lists none.
func (d CommentDrops) Paths() []string {
	if d == "" {
		return nil
	}
	return strings.Split(string(d), "\n")
}

// add appends path to the files d lists.
func (d *CommentDrops) add(path string) {
	if *d != "" {
		*d += "\n"
	}
	*d += CommentDrops(path)
}

// Issues returns the node store of the issue tree of s, whose config validates every issue;
// resolver resolves an argument naming an issue.
func Issues(s *store.Store, resolver Resolver) *Kind[issue.Issue] {
	return &Kind[issue.Issue]{
		codec:        issueCodec(s.Config),
		reader:       IssueReader(s.Config),
		dir:          s.IssuesDir(),
		knowledgeDir: s.KnowledgeDir(),
		worktreeRoot: s.WorktreeRoot,
		relative:     s.Relative,
		resolver:     resolver,
		applyMove:    applyMove,
		applyRemoval: (*tree.Removal).Apply,
		formatGate:   s.RequireFormat,
		fsys:         nodefile.Disk{},
	}
}

// Notes returns the node store of the Knowledge tree of s; resolver resolves an argument naming a
// Note.
func Notes(s *store.Store, resolver Resolver) *Kind[knowledge.Note] {
	return &Kind[knowledge.Note]{
		codec:        noteCodec(),
		reader:       NoteReader(),
		dir:          s.KnowledgeDir(),
		knowledgeDir: s.KnowledgeDir(),
		worktreeRoot: s.WorktreeRoot,
		relative:     s.Relative,
		resolver:     resolver,
		applyMove:    applyMove,
		applyRemoval: (*tree.Removal).Apply,
		formatGate:   s.RequireFormat,
		fsys:         nodefile.Disk{},
	}
}

// RefRepository returns the git working tree of s that knowledge.Verify checks Refs against, with
// git deciding which paths are ignored.
func RefRepository(s *store.Store) knowledge.Repository {
	return knowledge.Repository{
		WorktreeRoot: s.WorktreeRoot,
		GitDir:       s.GitDir,
		GitCommonDir: s.GitCommonDir,
		IgnoredPaths: gitx.IgnoredPaths,
	}
}

// WithSync returns a copy of the Kind that runs sync once after each write that lands: a Create at
// the top of the tree, and every Edit, Move and Remove. A Create below a parent and a failed write
// run none. The outcome is reported on the write's Effects.
func (k *Kind[T]) WithSync(sync SyncFunc) *Kind[T] {
	synced := *k
	synced.sync = sync
	return &synced
}

// WithFS returns a copy of the Kind that reads and writes the tree through fsys: the Tree load, with
// its directory listings and ReadContent's reads, RecoverAliases' reads, Create's parent check and
// write, and Edit's read, re-check and write. Format, Move and Remove still read, and Move and
// Remove still write, the node files on the disk, so a Kind given an FS other than nodefile.Disk is
// for Create and Edit only.
func (k *Kind[T]) WithFS(fsys nodefile.FS) *Kind[T] {
	routed := *k
	routed.fsys = fsys
	return &routed
}

// WithFormatGate returns a copy of the Kind whose Create and Edit ask gate, instead of
// store.Store.RequireFormat, whether a file they are about to write may carry a key line of a later
// store format: gate is handed the key and the format it needs, and a refusal fails the write.
func (k *Kind[T]) WithFormatGate(gate FormatGate) *Kind[T] {
	gated := *k
	gated.formatGate = gate
	return &gated
}

// runSync runs the Kind's sync hook, when it has one, recording its outcome on effects.
func (k *Kind[T]) runSync(effects *Effects) {
	if k.sync == nil {
		return
	}
	isSynced, err := k.sync()
	if err != nil {
		effects.SyncFailure = err
		return
	}
	effects.IsSynced = isSynced
}

// applyMove carries out move through its Plan, returning the failures of its cleanups, the
// removal of the emptied old parent directory, as warnings.
func applyMove(move *tree.Move) ([]error, error) {
	return move.Plan.Apply()
}

// ReadContent is the tree.ContentFunc of the kind's tree: what the node file at path records, as
// the kind's Reader reads it. A file that parses gives its Aliases, and is usable when the Reader
// finds nothing wrong with it. A file that does not parse gives the aliases tree.ReadAliases still
// recovers from its raw frontmatter; the index reports it. A file nodefile.ReadRegularFile refuses
// (not a regular file, or larger than 1 MiB) is left out unread, as the walk leaves one out. A file
// that cannot be read otherwise fails with a *tree.Error naming path as it was given. The file is
// read through the Kind's FS (WithFS).
func (k *Kind[T]) ReadContent(path string) (tree.Content, error) {
	reading, err := k.reader.ReadFS(k.fsys, path, k.relative(path))
	if err != nil {
		return tree.Content{}, &tree.Error{Path: path, Err: err}
	}
	if !reading.IsRead() {
		return tree.Content{Parse: tree.UnreadFile}, nil
	}
	if !reading.IsParsed() {
		// The index refresh before every command already warned about this file.
		return tree.Content{Aliases: reading.Aliases, Parse: tree.UnparsedFile}, nil
	}
	return tree.Content{Aliases: reading.Aliases, Parse: tree.ParsedFile, IsUsable: reading.IsUsable()}, nil
}

// RecoverAliases is the recovering lookup of the kind's tree: the aliases tree.ReadAliases still
// recovers from the raw frontmatter of a node file at path that does not parse, and none for one
// that parses, whose Aliases ReadContent returns. A file nodefile.ReadRegularFile refuses gives none, as
// for ReadContent. A file that cannot be read otherwise fails with a *tree.Error naming path as it
// was given. The file is read through the Kind's FS (WithFS).
func (k *Kind[T]) RecoverAliases(path string) ([]string, error) {
	data, err := k.fsys.ReadRegularFile(path)
	if isRefusedRead(err) {
		return nil, nil
	}
	if err != nil {
		return nil, &tree.Error{Path: path, Err: err}
	}
	aliases, isParsed := tree.ReadAliases(data, k.aliasParser(path))
	if isParsed {
		return nil, nil
	}
	return aliases, nil
}

// aliasParser returns the tree.AliasParseFunc of the node file at path: the Aliases of the node
// the kind's Reader decodes from it, or errUnparsed when it does not parse.
func (k *Kind[T]) aliasParser(path string) tree.AliasParseFunc {
	return func(data []byte) ([]string, error) {
		reading := k.reader.Decode(k.relative(path), data)
		if !reading.IsParsed() {
			return nil, errUnparsed
		}
		return reading.Aliases, nil
	}
}

// gateFormat asks the Kind's FormatGate whether the node file data, the bytes about to be written
// to path, may be written: of the key lines data carries, the one needing the latest store format
// (Codec.requiredFormat) is handed to the gate, whatever the line holds. data that needs no format
// passes without asking. data that does not parse fails with a *FileError at path; a refusal that is
// a *store.FormatError comes back naming path, and any other refusal comes back as it is.
func (k *Kind[T]) gateFormat(path string, data []byte) error {
	written, err := k.codec.Parse(k.relative(path), data)
	if err != nil {
		return k.fileError(path, err)
	}
	key, version := k.codec.requiredFormat(written)
	if version == 0 {
		return nil
	}
	err = k.formatGate(key, version)
	var refused *store.FormatError
	if errors.As(err, &refused) {
		located := *refused
		located.Path = k.relative(path)
		return &located
	}
	return err
}

// isRefusedRead reports whether err is nodefile.ReadRegularFile refusing a file unread: one that is
// not a regular file, or is larger than 1 MiB.
func isRefusedRead(err error) bool {
	return errors.Is(err, tree.ErrNotRegular) || errors.Is(err, tree.ErrTooLarge)
}

// relativeText returns text with every path inside the working tree named relative to its root, so
// a message built from an OS error never carries the worktree's absolute path.
func (k *Kind[T]) relativeText(text string) string {
	return strings.ReplaceAll(text, k.worktreeRoot+string(filepath.Separator), "")
}
