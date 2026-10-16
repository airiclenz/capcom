package ui

import (
	"errors"
	"io/fs"
	"os"
	"sync"
	"time"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

// session owns the UI's store, with the store config it carries, and its index. Bubble Tea runs
// tea.Cmds on goroutines of their own, so every access to either goes through do, which holds
// one mutex for its duration: no two accesses ever overlap, and nothing touches the index
// outside do.
//
// A poll can swap both for a store re-opened on a changed config.yaml and an index opened on it.
// The index the caller handed in stays the caller's to close; one the session opened itself it
// closes, when it swaps it out or on close.
type session struct {
	mu    sync.Mutex
	store *store.Store
	index *index.Index
	// isIndexOwned reports an index the session opened itself, which close closes.
	isIndexOwned bool
	// configSeen is config.yaml's state on disk when the session last loaded it or tried to.
	configSeen configState
}

// configState is what a poll compares of config.yaml on disk: whether it exists, its
// modification time and size, and the error of a stat that failed for another reason.
type configState struct {
	isPresent bool
	modTime   time.Time
	size      int64
	problem   string
}

// newSession returns a session over an opened store and its index, the caller's to close, with
// config.yaml's state on disk taken as the one the store loaded.
func newSession(opened *store.Store, issueIndex *index.Index) *session {
	return &session{store: opened, index: issueIndex, configSeen: statConfig(opened.ConfigPath())}
}

// do runs access with the store and the index while no other access runs, and returns its error.
// access must not keep either past its return, and must not call do itself: the mutex is not
// reentrant.
func (s *session) do(access func(*store.Store, *index.Index) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return access(s.store, s.index)
}

// close closes the index when the session opened it itself, and leaves the caller's alone.
func (s *session) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isIndexOwned {
		return nil
	}
	s.isIndexOwned = false
	return s.index.Close()
}

// statConfig returns config.yaml's state at path. A missing file is its own state.
func statConfig(path string) configState {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return configState{}
	case err != nil:
		return configState{problem: err.Error()}
	}
	return configState{isPresent: true, modTime: info.ModTime(), size: info.Size()}
}

// reloadOutcome is what a poll did about config.yaml.
type reloadOutcome int

// The outcomes of reloadConfig.
const (
	// reloadNone: config.yaml is as the session last saw it.
	reloadNone reloadOutcome = iota
	// reloadDone: config.yaml changed and the store and index were re-opened on it.
	reloadDone
	// reloadFailed: config.yaml changed and could not be used; the old store and index stay.
	reloadFailed
)

// reloadConfig re-opens the store and its index when config.yaml's modification time, size or
// presence changed since the session last saw it, so a refresh never rebuilds the index under a
// stale config. A re-open that fails keeps the old store and index and returns its error; it is
// tried again only once config.yaml changes again. The caller holds the mutex.
func (s *session) reloadConfig() (reloadOutcome, error) {
	seen := statConfig(s.store.ConfigPath())
	if seen == s.configSeen {
		return reloadNone, nil
	}
	s.configSeen = seen
	reopened, err := store.Open(s.store.WorktreeRoot)
	if err != nil {
		return reloadFailed, err
	}
	reopenedIndex, err := index.Open(reopened, index.ModeRefresh)
	if err != nil {
		return reloadFailed, err
	}
	var closeErr error
	if s.isIndexOwned {
		closeErr = s.index.Close()
	}
	s.store, s.index, s.isIndexOwned = reopened, reopenedIndex, true
	return reloadDone, closeErr
}
