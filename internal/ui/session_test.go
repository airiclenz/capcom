package ui

import (
	"errors"
	"sync"
	"testing"

	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/store"
)

func TestSession_Do_ConcurrentCalls_RunOneAtATime(t *testing.T) {
	const callers = 8
	ts := newTestStore(t, map[string]string{"issues/valid-issue.md": validIssue})
	s := newSession(ts.store, ts.index)
	// Only do guards these counters, so a race detector run catches two accesses overlapping.
	var running, mostRunning, finished int
	failures := make(chan error, callers)
	var group sync.WaitGroup

	for range callers {
		group.Go(func() {
			failures <- s.do(func(_ *store.Store, issueIndex *index.Index) error {
				running++
				mostRunning = max(mostRunning, running)
				_, err := issueIndex.List(index.Filter{})
				finished++
				running--
				return err
			})
		})
	}
	group.Wait()
	close(failures)

	for err := range failures {
		if err != nil {
			t.Errorf("do: %v", err)
		}
	}
	if finished != callers {
		t.Errorf("do ran %d accesses, want %d", finished, callers)
	}
	if mostRunning != 1 {
		t.Errorf("do ran %d accesses at once, want 1", mostRunning)
	}
}

func TestSession_Do_ReturnsTheAccessError(t *testing.T) {
	ts := newTestStore(t, nil)
	s := newSession(ts.store, ts.index)
	failure := errors.New("access failed")

	err := s.do(func(*store.Store, *index.Index) error { return failure })

	if !errors.Is(err, failure) {
		t.Errorf("do error = %v, want %v", err, failure)
	}
}
