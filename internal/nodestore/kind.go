package nodestore

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/config"
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
	// raiseFormat raises config.yaml to declare at least store format version and reports whether
	// it changed the file, as store.Store.RaiseFormat does.
	raiseFormat func(version int) (bool, error)
	// configPath is the store's config.yaml, which a failure of raiseFormat names.
	configPath string
	// sync is the hook WithSync sets, run once after each write that lands; nil runs none.
	sync SyncFunc
}

// SyncFunc is a Kind's sync hook: it brings what the caller keeps in step with the tree up to date
// after a write has landed, and reports whether it changed anything. Its failure never fails the
// write; the write's Effects carry it.
type SyncFunc func() (bool, error)

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
	// RaisedFormat is the store format (ADR 0009) the write raised config.yaml to, so that the store
	// declares the format of every key line the written file carries, whatever the line holds; 0
	// when config.yaml was left alone.
	RaisedFormat int
	// IsSynced reports whether the Kind's sync hook, run once the write had landed, changed what it
	// keeps in step with the tree; false when the Kind has no hook, it ran none or it failed.
	IsSynced bool
	// SyncFailure is the failure of the Kind's sync hook; the write itself has landed. It is nil
	// when the hook succeeded or did not run.
	SyncFailure error
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
		raiseFormat:  s.RaiseFormat,
		configPath:   s.ConfigPath(),
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
		raiseFormat:  s.RaiseFormat,
		configPath:   s.ConfigPath(),
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
// that cannot be read otherwise fails with a *tree.Error naming path as it was given.
func (k *Kind[T]) ReadContent(path string) (tree.Content, error) {
	reading, err := k.reader.Read(path, k.relative(path))
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
// was given.
func (k *Kind[T]) RecoverAliases(path string) ([]string, error) {
	data, err := nodefile.ReadRegularFile(path)
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

// raise raises config.yaml to the store format the node file data, the bytes about to be written to
// path, needs (Codec.requiredFormat): a key line data carries needs its format whatever the line
// holds. It returns the format it raised the file to: 0 when data needs no format or the store
// already declares it or above. data that does not parse fails with a *FileError at path; any other
// failure is a *FileError at config.yaml: a config.yaml that is not a regular file or is larger than
// 1 MiB is named by the refusal alone, and a *config.Error, which already names config.yaml, keeps
// its line and message.
func (k *Kind[T]) raise(path string, data []byte) (int, error) {
	written, err := k.codec.Parse(k.relative(path), data)
	if err != nil {
		return 0, k.fileError(path, err)
	}
	version := k.codec.requiredFormat(written)
	if version == 0 {
		return 0, nil
	}
	isRaised, err := k.raiseFormat(version)
	if err != nil {
		path := k.relative(k.configPath)
		var configErr *config.Error
		if errors.As(err, &configErr) {
			return 0, &FileError{Path: path, Line: configErr.Line, Message: configErr.Message}
		}
		message := k.relativeText(err.Error())
		for _, refusal := range []error{tree.ErrNotRegular, tree.ErrTooLarge} {
			if errors.Is(err, refusal) {
				// The error already names config.yaml, which the FileError's path names.
				message = refusal.Error()
			}
		}
		return 0, &FileError{Path: path, Message: message}
	}
	if !isRaised {
		return 0, nil
	}
	return version, nil
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
