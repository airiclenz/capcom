package ui

import (
	"sync"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

// session owns the UI's store, with the store config it carries, and its index. Bubble Tea runs
// tea.Cmds on goroutines of their own, so every access to either goes through do, which holds
// one mutex for its duration: no two accesses ever overlap, and nothing touches the index
// outside do.
type session struct {
	mu    sync.Mutex
	store *store.Store
	index *index.Index
}

// newSession returns a session over an opened store and its index.
func newSession(opened *store.Store, issueIndex *index.Index) *session {
	return &session{store: opened, index: issueIndex}
}

// do runs access with the store and the index while no other access runs, and returns its error.
// access must not keep either past its return, and must not call do itself: the mutex is not
// reentrant.
func (s *session) do(access func(*store.Store, *index.Index) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return access(s.store, s.index)
}
