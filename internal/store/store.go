// Package store locates a repository's capcom store: the `.capcom` directory at the root of the
// git working tree that holds config.yaml, issues/ and knowledge/. The store root is resolved at
// startup from the working directory and carried as a field, never hard-coded elsewhere
// (ADR 0004), so a different store layout stays possible later.
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/gitx"
)

// Layout of a store, relative to the working tree root.
const (
	dirName      = ".capcom"
	configFile   = "config.yaml"
	issuesDir    = "issues"
	knowledgeDir = "knowledge"
)

const (
	dirPermissions  = 0o755
	filePermissions = 0o644
)

// ErrNotInitialized reports a git working tree without a store; `capcom init` creates one.
var ErrNotInitialized = errors.New("no capcom store here; run capcom init")

// ErrNotGitRepository reports a directory outside any git working tree; capcom requires one.
var ErrNotGitRepository = errors.New("capcom requires a git repository")

// Store is an opened capcom store. Every path is absolute.
type Store struct {
	// Root is the store directory (`.capcom` in the working tree root).
	Root string
	// WorktreeRoot is the top directory of the git working tree, with symlinks resolved.
	WorktreeRoot string
	// GitDir is this worktree's git directory.
	GitDir string
	// GitCommonDir is the git directory shared by every worktree of the repository.
	GitCommonDir string
	// Config is the store's parsed config.yaml.
	Config config.Config
}

// Open finds the store of the git working tree containing cwd, from any directory inside it, and
// loads its config. It fails with ErrNotGitRepository outside a working tree, ErrNotInitialized
// when the tree has no store, and a *config.Error when config.yaml is invalid.
func Open(cwd string) (*Store, error) {
	store, err := locate(cwd)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(store.Root); errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", store.Root, ErrNotInitialized)
	}
	if err := store.loadConfig(); err != nil {
		return nil, err
	}
	return store, nil
}

// Init creates the store of the git working tree containing cwd: config.yaml with the default
// configuration, issues/ and knowledge/. It creates only what is missing and never overwrites, so
// a re-run is a no-op. It returns the opened store and the paths it created, relative to the
// working tree root with forward slashes, in creation order.
func Init(cwd string) (*Store, []string, error) {
	store, err := locate(cwd)
	if err != nil {
		return nil, nil, err
	}

	var created []string
	for _, dir := range []string{"", issuesDir, knowledgeDir} {
		isNew, err := store.createDir(dir)
		if err != nil {
			return nil, nil, err
		}
		if isNew {
			created = append(created, store.relative(dir))
		}
	}
	isNew, err := store.createConfig()
	if err != nil {
		return nil, nil, err
	}
	if isNew {
		created = append(created, store.relative(configFile))
	}

	if err := store.loadConfig(); err != nil {
		return nil, nil, err
	}
	return store, created, nil
}

// IssuesDir returns the directory that holds the issue tree.
func (s *Store) IssuesDir() string {
	return filepath.Join(s.Root, issuesDir)
}

// locate resolves the store paths for cwd without touching the store itself.
func locate(cwd string) (*Store, error) {
	paths, err := gitx.Discover(cwd)
	if errors.Is(err, gitx.ErrNotWorkTree) {
		return nil, fmt.Errorf("%s: %w", cwd, ErrNotGitRepository)
	}
	if err != nil {
		return nil, err
	}
	return &Store{
		Root:         filepath.Join(paths.WorktreeRoot, dirName),
		WorktreeRoot: paths.WorktreeRoot,
		GitDir:       paths.GitDir,
		GitCommonDir: paths.GitCommonDir,
	}, nil
}

// loadConfig reads and parses config.yaml. A missing file means the default configuration.
func (s *Store) loadConfig() error {
	data, err := os.ReadFile(filepath.Join(s.Root, configFile))
	if errors.Is(err, fs.ErrNotExist) {
		s.Config = config.Default()
		return nil
	}
	displayPath := s.relative(configFile)
	if err != nil {
		return fmt.Errorf("%s: %w", displayPath, err)
	}
	parsed, err := config.Parse(displayPath, data)
	if err != nil {
		return err
	}
	s.Config = parsed
	return nil
}

// createDir creates the store subdirectory name ("" for the store root) and reports whether it
// was missing.
func (s *Store) createDir(name string) (bool, error) {
	path := filepath.Join(s.Root, name)
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return false, fmt.Errorf("%s: exists and is not a directory", s.relative(name))
		}
		return false, nil
	}
	if err := os.Mkdir(path, dirPermissions); err != nil {
		return false, fmt.Errorf("%s: %w", s.relative(name), err)
	}
	return true, nil
}

// createConfig writes the default config.yaml unless one exists, and reports whether it wrote it.
func (s *Store) createConfig() (bool, error) {
	path := filepath.Join(s.Root, configFile)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePermissions)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%s: %w", s.relative(configFile), err)
	}
	_, writeErr := file.WriteString(config.DefaultFile)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return false, fmt.Errorf("%s: %w", s.relative(configFile), err)
	}
	return true, nil
}

// relative names a path inside the store relative to the working tree root, with forward
// slashes, for messages and output.
func (s *Store) relative(name string) string {
	return filepath.ToSlash(filepath.Join(dirName, name))
}
