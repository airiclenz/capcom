package lock

import (
	"errors"
	"slices"

	"github.com/airiclenz/capcom/internal/proc"
)

// RenewalError reports that the holder process's renewal of the lock's heartbeat after a write
// failed (Worktree.CheckWrites). Err is TouchHolder's error, unchanged: Error gives its text and
// Unwrap returns it.
type RenewalError struct {
	Err error
}

// Error gives Err's text.
func (e *RenewalError) Error() string {
	return e.Err.Error()
}

// Unwrap returns Err.
func (e *RenewalError) Unwrap() error {
	return e.Err
}

// CheckWrites decides whether writes, each as the ids of the Issues it targets, must warn of this
// worktree's Tree lock (ADR 0017). It returns the *HeldError the writes are warned of when the lock
// is held, live or stale, a write targets no Issue whose id is the lock's purpose, and the caller
// does not descend from the record's holder process; nil when the lock is free or orphaned, or the
// writes stay silent. A caller that descends from the holder process renews the lock's heartbeat
// instead (TouchHolder), whatever the writes target; when another session has taken the lock over
// since, the writes are judged against that session's lock as any other caller's. The record is
// first read without the guard (Check), so the check creates nothing under the lock directory;
// only the holder's renewal takes the guard. chain returns the caller's process ancestry and is
// called at most once, only for a record that names a holder process. It fails with Check's error
// when the record cannot be read, and with a *RenewalError when the holder's renewal fails.
func (w Worktree) CheckWrites(chain func() []proc.Process, writes [][]string) (*HeldError, error) {
	_, err := w.Check("")
	if err == nil {
		return nil, nil
	}
	var held *HeldError
	if !errors.As(err, &held) {
		return nil, err
	}
	if held.Status.Record.Process != nil {
		ancestry := chain()
		if held.Status.HeldBy(ancestry) {
			renewed, err := w.renewForHolder(ancestry)
			if renewed == nil {
				return nil, err
			}
			held = renewed
		}
	}
	if writesOnly(writes, held.Status.Record.Purpose) {
		return nil, nil
	}
	return held, nil
}

// renewForHolder refreshes the heartbeat of this worktree's lock for the holder process, whose
// ancestry is chain. It returns nothing when the lock was renewed or has been freed since it was
// read, and a *RenewalError when the renewal failed. When another session has taken the lock over
// since, it returns that session's *HeldError.
func (w Worktree) renewForHolder(chain []proc.Process) (*HeldError, error) {
	_, err := w.TouchHolder(chain)
	var held *HeldError
	var notHeld *NotHeldError
	switch {
	case err == nil, errors.As(err, &notHeld):
		return nil, nil
	case errors.As(err, &held):
		return held, nil
	}
	return nil, &RenewalError{Err: err}
}

// writesOnly reports whether every write in writes targets the Issue whose id is purpose.
func writesOnly(writes [][]string, purpose string) bool {
	for _, ids := range writes {
		if !slices.Contains(ids, purpose) {
			return false
		}
	}
	return true
}
