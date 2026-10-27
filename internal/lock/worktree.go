package lock

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/store"
)

// Layout of a linked worktree's identity: git keeps the worktree's git dir at
// `<git-common-dir>/worktrees/<name>`, and capcom keeps the worktree's id in
// `<git-dir>/capcom/worktree-id` there, which git deletes with the worktree.
const (
	linkedWorktreesDir = "worktrees"
	worktreeIDFile     = "worktree-id"
)

// Worktree is the Tree lock of one worktree: the repository's lock directory paired with that
// worktree's key, so no caller pairs Open and WorktreeKey by hand. Each method behaves as the
// Locks method of the same name called with Key.
type Worktree struct {
	locks *Locks
	key   string
}

// ForWorktree returns the lock of s's worktree, kept under s's git common dir with s's configured
// TTL. Nothing is touched on disk until the lock is read or written.
func ForWorktree(s *store.Store) Worktree {
	return Worktree{
		locks: Open(s.GitCommonDir, s.Config.Lock.TTL),
		key:   WorktreeKey(s.GitDir, s.GitCommonDir),
	}
}

// Key returns this worktree's lock key: MainWorktreeKey or `wt-<linked worktree name>`.
func (w Worktree) Key() string {
	return w.key
}

// RecordPath returns the file that holds this worktree's lock record.
func (w Worktree) RecordPath() string {
	return w.locks.RecordPath(w.key)
}

// Acquire takes this worktree's lock for claim, as Locks.Acquire does.
func (w Worktree) Acquire(claim Claim, token string, isTakeStale bool) (Status, bool, error) {
	return w.locks.Acquire(w.key, claim, token, isTakeStale)
}

// SessionToken returns the Lock token a session acquiring this worktree's lock for purpose holds,
// as Locks.SessionToken does.
func (w Worktree) SessionToken(token, purpose string) (string, error) {
	return w.locks.SessionToken(w.key, token, purpose)
}

// Touch refreshes the heartbeat of this worktree's lock for the holder of token, as Locks.Touch
// does.
func (w Worktree) Touch(token string) (Status, error) {
	return w.locks.Touch(w.key, token)
}

// Release frees this worktree's lock for the holder of token, as Locks.Release does.
func (w Worktree) Release(token string) (Status, error) {
	return w.locks.Release(w.key, token)
}

// Break frees this worktree's lock whoever holds it, as Locks.Break does.
func (w Worktree) Break() (Status, error) {
	return w.locks.Break(w.key)
}

// Check returns this worktree's lock when it is free or held by token, as Locks.Check does.
func (w Worktree) Check(token string) (Status, error) {
	return w.locks.Check(w.key, token)
}

// Read returns the state of this worktree's lock.
func (w Worktree) Read() (Status, error) {
	return w.locks.Read(w.key)
}

// Overview returns the Lock overview of the repository with this worktree's state, as
// Locks.Overview does.
func (w Worktree) Overview() (Overview, error) {
	return w.locks.Overview(w.key)
}

// linkedGitDir returns the git dir of the linked worktree key names, `<common>/worktrees/<name>`,
// and false when key is the primary worktree's, or names no linked worktree.
func (l *Locks) linkedGitDir(key string) (string, bool) {
	name, isLinked := strings.CutPrefix(key, linkedWorktreePrefix)
	if !isLinked {
		return "", false
	}
	return filepath.Join(l.commonDir, linkedWorktreesDir, name), true
}

// isOrphaned reports whether record, read from key's valid record, belongs to a linked worktree
// that is gone: its git dir is missing, or the record names a worktree id other than the one kept
// there, a missing id included. A record an older capcom wrote names no id and is orphaned only
// when the git dir is missing; the primary worktree's is never orphaned. Creates nothing.
func (l *Locks) isOrphaned(key string, record Record) (bool, error) {
	gitDir, isLinked := l.linkedGitDir(key)
	if !isLinked {
		return false, nil
	}
	if _, err := os.Stat(gitDir); errors.Is(err, fs.ErrNotExist) {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("%s: %w", gitDir, err)
	}
	if record.Worktree == "" {
		return false, nil
	}

	id, err := readWorktreeID(gitDir)
	if err != nil {
		return false, err
	}
	return id != record.Worktree, nil
}

// ensureWorktreeID returns the id of the linked worktree key names, writing a new random one to
// its git dir when none is kept there; the primary worktree has none and gets "". A missing git
// dir fails naming it and is never created. The caller holds the guard.
func (l *Locks) ensureWorktreeID(key string) (string, error) {
	gitDir, isLinked := l.linkedGitDir(key)
	if !isLinked {
		return "", nil
	}
	id, err := readWorktreeID(gitDir)
	if err != nil || id != "" {
		return id, err
	}

	id, err = randomHex()
	if err != nil {
		return "", fmt.Errorf("generating worktree id: %w", err)
	}
	idDir := filepath.Join(gitDir, capcomDir)
	if err := os.Mkdir(idDir, dirPermissions); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", err // a *fs.PathError, which names idDir
	}
	path := filepath.Join(idDir, worktreeIDFile)
	if err := nodefile.WriteFileAtomic(path, strings.NewReader(id+"\n")); err != nil {
		return "", err
	}
	return id, nil
}

// readWorktreeID returns the worktree id kept under gitDir, or "" when none is; only an id file
// that cannot be read fails, naming its path.
func readWorktreeID(gitDir string) (string, error) {
	path := filepath.Join(gitDir, capcomDir, worktreeIDFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}
