package ui

import (
	"errors"
	"sync"
	"testing"
	"time"

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

func TestSession_Holder_InsideAccess_ReportsItsPurposeAndSince(t *testing.T) {
	ts := newTestStore(t, nil)
	s := newSession(ts.store, ts.index)
	var purpose string
	var since, now time.Time
	var isHeld bool

	err := s.doFor(purposeWrite, func(*store.Store, *index.Index) error {
		purpose, since, isHeld = s.holder()
		now = time.Now()
		return nil
	})

	if err != nil {
		t.Fatalf("doFor: %v", err)
	}
	if !isHeld || purpose != purposeWrite {
		t.Errorf("holder() = %q, isHeld %v; want %q, isHeld true", purpose, isHeld, purposeWrite)
	}
	if since.IsZero() || since.After(now) {
		t.Errorf("holder() since = %v, want a time no later than %v", since, now)
	}
}

func TestSession_Holder_Do_ReportsTheReadPurpose(t *testing.T) {
	ts := newTestStore(t, nil)
	s := newSession(ts.store, ts.index)
	var purpose string

	err := s.do(func(*store.Store, *index.Index) error {
		purpose, _, _ = s.holder()
		return nil
	})

	if err != nil {
		t.Fatalf("do: %v", err)
	}
	if purpose != purposeRead {
		t.Errorf("holder() inside do = %q, want %q", purpose, purposeRead)
	}
}

func TestSession_Holder_AfterAccess_ReportsFree(t *testing.T) {
	ts := newTestStore(t, nil)
	s := newSession(ts.store, ts.index)

	err := s.doFor(purposeCheck, func(*store.Store, *index.Index) error { return nil })
	purpose, since, isHeld := s.holder()

	if err != nil {
		t.Fatalf("doFor: %v", err)
	}
	if isHeld || purpose != "" || !since.IsZero() {
		t.Errorf("holder() after access = %q, %v, isHeld %v; want free", purpose, since, isHeld)
	}
}

func TestSession_Holder_Poll_ReportsTheIndexPoll(t *testing.T) {
	ts := newTestStore(t, nil)
	s := newSession(ts.store, ts.index)
	var purpose string
	var isHeld bool
	s.onHeld = func(string) { purpose, _, isHeld = s.holder() }

	s.poll()

	if !isHeld || purpose != purposePoll {
		t.Errorf("holder() during poll = %q, isHeld %v; want %q, isHeld true", purpose, isHeld, purposePoll)
	}
	if _, _, isHeldAfter := s.holder(); isHeldAfter {
		t.Error("holder() after poll reports held, want free")
	}
}
