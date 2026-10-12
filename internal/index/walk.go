package index

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/tree"
)

// Node layout of the issue tree and the Knowledge tree (ADR 0001), as the tree package reads it.
const (
	nodeExtension = ".md"
	hiddenPrefix  = "."
)

// walkIssues lists the node files of the tree in issuesDir (the issue tree, or the Knowledge
// tree), in path order, with paths relative to worktreeRoot. A missing issuesDir is an empty tree. A file whose name is not a valid
// id and a directory without a valid node file are warned about through r and skipped, with the
// directory's whole subtree.
func walkIssues(worktreeRoot, issuesDir string, r *refresher) ([]foundFile, error) {
	if _, err := os.Stat(issuesDir); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	walker := treeWalker{worktreeRoot: worktreeRoot, refresher: r}
	if err := walker.walk(issuesDir, ""); err != nil {
		return nil, err
	}
	slices.SortFunc(walker.found, func(a, b foundFile) int {
		return cmp.Compare(a.path, b.path)
	})
	return walker.found, nil
}

// treeWalker collects the node files below one directory.
type treeWalker struct {
	worktreeRoot string
	refresher    *refresher
	found        []foundFile
}

// walk collects the node files of dir, all children of parentID, and recurses into the
// directories that belong to a valid node.
func (w *treeWalker) walk(dir string, parentID string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("%s: %w", w.relative(dir), err)
	}

	validIDs := map[string]bool{}
	invalidIDs := map[string]bool{}
	var childDirs []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, hiddenPrefix) {
			continue
		}
		if entry.IsDir() {
			childDirs = append(childDirs, name)
			continue
		}
		id, isNode := strings.CutSuffix(name, nodeExtension)
		if !isNode {
			continue
		}
		absolute := filepath.Join(dir, name)
		if err := tree.ValidateID(id); err != nil {
			invalidIDs[id] = true
			w.refresher.warn(WarningInvalidID, w.relative(absolute), 0, err.Error())
			continue
		}
		info, err := entry.Info()
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("%s: %w", w.relative(absolute), err)
		}
		validIDs[id] = true
		w.found = append(w.found, foundFile{
			path:     w.relative(absolute),
			absolute: absolute,
			id:       id,
			parentID: parentID,
			size:     info.Size(),
			mtime:    info.ModTime().UnixNano(),
		})
	}

	for _, name := range childDirs {
		path := filepath.Join(dir, name)
		switch {
		case validIDs[name]:
			if err := w.walk(path, name); err != nil {
				return err
			}
		case invalidIDs[name]:
			w.refresher.warn(WarningOrphanDirectory, w.relative(path), 0, fmt.Sprintf(
				"directory left out: its node file %s%s has an invalid id", name, nodeExtension,
			))
		default:
			w.refresher.warn(WarningOrphanDirectory, w.relative(path), 0, fmt.Sprintf(
				"%v: no %s%s beside it; left out", tree.ErrOrphanDirectory, name, nodeExtension,
			))
		}
	}
	return nil
}

// relative names a path below the working tree root with forward slashes, for rows and warnings.
func (w *treeWalker) relative(path string) string {
	relative, err := filepath.Rel(w.worktreeRoot, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}
