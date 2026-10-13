package nodestore

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/tree"
)

// nodeFileExtension ends every node file; a node's children directory is its path without it.
const nodeFileExtension = ".md"

// hiddenNamePrefix starts the file and directory names tree.Load skips, temp files among them.
const hiddenNamePrefix = "."

// claimFailure refuses id, the id of a new node, when a node already answers to it or may answer
// to it; Create documents the order of the refusals. It is nil when id is free.
func (k *Kind[T]) claimFailure(nodes *tree.Tree, problems tree.Errors, id string) error {
	if existing, found := nodes.Lookup(id); found {
		return k.existsError(id, existing)
	}
	if err := k.orphanClaimFailure(problems, id); err != nil {
		return err
	}
	if err := k.duplicateClaimFailure(problems, id); err != nil {
		return err
	}
	return k.unparsedClaimFailure(nodes, problems, id)
}

// orphanClaimFailure refuses id when a node file anywhere under one of the orphan directories
// among problems is named id or records id among its Aliases. tree.Load leaves such a file out of
// the tree, so writing the id beside it would make a duplicate that surfaces only once the orphan
// directory is fixed.
func (k *Kind[T]) orphanClaimFailure(problems tree.Errors, id string) error {
	for _, problem := range problems {
		if !errors.Is(problem, tree.ErrOrphanDirectory) {
			continue
		}
		path, isAlias, err := findNodeFile(problem.Path, id, k.ReadAliases)
		if err != nil {
			return err
		}
		if path == "" {
			continue
		}
		claim := "taken by"
		if isAlias {
			claim = "an alias of"
		}
		return &ExistsError{
			Path:   k.relative(path),
			ID:     id,
			Holder: nodeID(path),
			Message: fmt.Sprintf(
				"id %q is already %s this file, inside the orphan directory %s",
				id,
				claim,
				k.relative(problem.Path),
			),
		}
	}
	return nil
}

// duplicateClaimFailure refuses id when a file a repeated-id problem among problems names is named
// id or records id among its Aliases, as far as ReadAliases reads them. tree.Load leaves such a
// file out of the tree, so writing the id beside it would make a second holder of the name that
// surfaces only once the duplicate is fixed.
func (k *Kind[T]) duplicateClaimFailure(problems tree.Errors, id string) error {
	for _, file := range duplicateIDFiles(problems) {
		claim, err := k.claimOf(file.path, id)
		if err != nil {
			return err
		}
		if claim == "" {
			continue
		}
		return &ExistsError{
			Path:   k.relative(file.path),
			ID:     id,
			Holder: nodeID(file.path),
			Message: fmt.Sprintf(
				"id %q is already %s this file, left out for the duplicate id %q",
				id,
				claim,
				file.id,
			),
		}
	}
	return nil
}

// claimOf words how the node file at path claims id: "taken by" when it is named id, "an alias of"
// when ReadAliases reads id among its Aliases, and "" when it does not claim id.
func (k *Kind[T]) claimOf(path string, id string) (string, error) {
	if nodeID(path) == id {
		return "taken by", nil
	}
	names, err := k.ReadAliases(path)
	if err != nil || !slices.Contains(names, id) {
		return "", err
	}
	return "an alias of", nil
}

// duplicateIDFile is a file a repeated-id problem of a tree load names, and that repeated id.
type duplicateIDFile struct {
	path string
	id   string
}

// duplicateIDFiles returns the files every repeated-id problem among problems names, its Path then
// its Other, each once, in order. A repeated-id problem wraps tree.ErrDuplicate and names no Alias.
func duplicateIDFiles(problems tree.Errors) []duplicateIDFile {
	var files []duplicateIDFile
	seen := map[string]bool{}
	for _, problem := range problems {
		if problem.Alias != "" || !errors.Is(problem, tree.ErrDuplicate) {
			continue
		}
		for _, path := range []string{problem.Path, problem.Other} {
			if !seen[path] {
				seen[path] = true
				files = append(files, duplicateIDFile{path: path, id: nodeID(problem.Path)})
			}
		}
	}
	return files
}

// unparsedClaimFailure refuses id when a node file that does not parse, of a node of nodes,
// anywhere under one of the orphan directories among problems or named by a repeated-id problem
// among them, still lists id as an alias in its raw frontmatter. Such a file cannot say which ids it holds, so writing id beside it would risk a
// duplicate that surfaces only once the file is fixed.
func (k *Kind[T]) unparsedClaimFailure(nodes *tree.Tree, problems tree.Errors, id string) error {
	path, err := k.findUnparsedAliasHolder(nodes, problems, id)
	if err != nil || path == "" {
		return err
	}
	holder := nodeID(path)
	return &ExistsError{
		Path:   k.relative(path),
		ID:     id,
		Holder: holder,
		Message: fmt.Sprintf(
			"id %q may be an alias of %s %s, whose file does not parse: fix it first",
			id,
			k.codec.Noun,
			holder,
		),
	}
}

// findUnparsedAliasHolder returns the first node file, of a node of nodes, anywhere under one of
// the orphan directories among problems or named by a repeated-id problem among them, from which
// RecoverAliases reads id, or "" when there is none.
func (k *Kind[T]) findUnparsedAliasHolder(
	nodes *tree.Tree,
	problems tree.Errors,
	id string,
) (string, error) {
	for _, node := range nodes.Nodes() {
		names, err := k.RecoverAliases(node.Path)
		if err != nil {
			return "", err
		}
		if slices.Contains(names, id) {
			return node.Path, nil
		}
	}
	for _, problem := range problems {
		if !errors.Is(problem, tree.ErrOrphanDirectory) {
			continue
		}
		// orphanClaimFailure has already refused a file named id, so a file found here holds id
		// as an alias.
		path, _, err := findNodeFile(problem.Path, id, k.RecoverAliases)
		if err != nil || path != "" {
			return path, err
		}
	}
	for _, file := range duplicateIDFiles(problems) {
		names, err := k.RecoverAliases(file.path)
		if err != nil {
			return "", err
		}
		if slices.Contains(names, id) {
			return file.path, nil
		}
	}
	return "", nil
}

// findNodeFile returns the first node file at any depth below dir that answers to id, skipping
// hidden names and entries that are not regular files as tree.Load does, or "" when there is none.
// A file answers to id when it is named id or, by isAlias, when aliases reads id among its Aliases.
func findNodeFile(
	dir string,
	id string,
	aliases tree.AliasFunc,
) (path string, isAlias bool, err error) {
	name := id + nodeFileExtension
	err = filepath.WalkDir(dir, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if current != dir && strings.HasPrefix(entry.Name(), hiddenNamePrefix) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), nodeFileExtension) {
			return nil
		}
		// A symlink, named pipe, socket or device is never a node file; ModeIrregular is not
		// refused, as tree.Walk does not refuse it: Windows reports it for some ordinary files.
		if entry.Type()&^fs.ModeIrregular != 0 {
			return nil
		}
		if entry.Name() == name {
			path = current
			return fs.SkipAll
		}
		names, err := aliases(current)
		if err != nil {
			return err
		}
		if slices.Contains(names, id) {
			path, isAlias = current, true
			return fs.SkipAll
		}
		return nil
	})
	return path, isAlias, err
}

// nodeID is the id a node file's name gives it: its base name without the extension.
func nodeID(path string) string {
	return strings.TrimSuffix(filepath.Base(path), nodeFileExtension)
}
