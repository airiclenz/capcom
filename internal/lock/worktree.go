package lock

import "github.com/airiclenz/capcom/internal/store"

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
