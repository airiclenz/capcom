// Package nodestore is the node store of capcom's two Tree kinds, the issue tree and the Knowledge
// tree: a Kind binds the kind's Codec to its directory in the store, so the verbs of both kinds
// read, change and write node files through one module, report one set of typed failures and look
// up Aliases the same way. It reads the files itself and never consults the index: where a verb
// needs the index's resolution of an argument, the caller injects it as a Resolver.
package nodestore

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/knowledge"
	"github.com/airiclenz/capcom/internal/rawfm"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// Kind is the node store of one Tree kind. Every path it reports, in a Written or a typed failure,
// is relative to the working tree root with forward slashes.
type Kind[T any] struct {
	codec Codec[T]
	// dir is the directory that holds the kind's tree.
	dir string
	// worktreeRoot is the top directory of the git working tree.
	worktreeRoot string
	// resolver resolves an argument naming an existing node; it may be nil for a caller that
	// never resolves one.
	resolver Resolver
	// applyMove carries out a planned move; a test swaps it to inject a failure of the renames and
	// rewrites Apply makes.
	applyMove func(move *tree.Move) error
	// applyRemoval carries out a planned removal, returning its cleanup failures as warnings; a
	// test swaps it to inject a failure of the rewrites and removals Apply makes.
	applyRemoval func(removal *tree.Removal) ([]error, error)
}

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
// with forward slashes, and the file's new etag.
type Written struct {
	ID   string
	Path string
	ETag string
}

// Issues returns the node store of the issue tree of s, whose config validates every issue;
// resolver resolves an argument naming an issue.
func Issues(s *store.Store, resolver Resolver) *Kind[issue.Issue] {
	return &Kind[issue.Issue]{
		codec:        issueCodec(s.Config),
		dir:          s.IssuesDir(),
		worktreeRoot: s.WorktreeRoot,
		resolver:     resolver,
		applyMove:    (*tree.Move).Apply,
		applyRemoval: (*tree.Removal).Apply,
	}
}

// Notes returns the node store of the Knowledge tree of s; resolver resolves an argument naming a
// Note.
func Notes(s *store.Store, resolver Resolver) *Kind[knowledge.Note] {
	return &Kind[knowledge.Note]{
		codec:        noteCodec(),
		dir:          s.KnowledgeDir(),
		worktreeRoot: s.WorktreeRoot,
		resolver:     resolver,
		applyMove:    (*tree.Move).Apply,
		applyRemoval: (*tree.Removal).Apply,
	}
}

// ReadAliases is the tree.AliasFunc of the kind's tree: the Aliases the node file at path records.
// A file that does not parse contributes none; the index reports it. A file that cannot be read
// fails with a *tree.Error naming path as it was given.
func (k *Kind[T]) ReadAliases(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &tree.Error{Path: path, Err: err}
	}
	node, err := k.codec.Parse(path, data)
	if err != nil {
		// The index refresh before every command already warned about this file.
		return nil, nil
	}
	return k.codec.Aliases(node), nil
}

// RecoverAliases is the recovering lookup of the kind's tree: the aliases rawfm.Aliases still reads
// from the raw frontmatter of a node file at path that does not parse, and none for one that
// parses, whose Aliases ReadAliases returns. A file that cannot be read fails with a *tree.Error
// naming path as it was given.
func (k *Kind[T]) RecoverAliases(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &tree.Error{Path: path, Err: err}
	}
	if _, err := k.codec.Parse(path, data); err == nil {
		return nil, nil
	}
	return rawfm.Aliases(data), nil
}

// relative names path relative to the working tree root with forward slashes. A path that is
// already relative, or lies outside the working tree, is kept as it is, with forward slashes.
func (k *Kind[T]) relative(path string) string {
	if !filepath.IsAbs(path) {
		return filepath.ToSlash(path)
	}
	relative, err := filepath.Rel(k.worktreeRoot, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return path
	}
	return filepath.ToSlash(relative)
}

// relativeText returns text with every path inside the working tree named relative to its root, so
// a message built from an OS error never carries the worktree's absolute path.
func (k *Kind[T]) relativeText(text string) string {
	return strings.ReplaceAll(text, k.worktreeRoot+string(filepath.Separator), "")
}
