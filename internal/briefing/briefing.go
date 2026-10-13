package briefing

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/airiclenz/capcom/internal/check"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/elsewhere"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/lock"
	"github.com/airiclenz/capcom/internal/store"
)

// runCheck is the check readHealth runs, a seam so a test can make a source panic.
var runCheck = check.Run

// Section names the source of one Failure of the Briefing.
type Section string

// The sections a Failure names, in the order Compose reads their sources.
const (
	SectionStore     Section = "store"
	SectionIndex     Section = "index"
	SectionReady     Section = "ready"
	SectionDoing     Section = "doing"
	SectionLocks     Section = "locks"
	SectionCheck     Section = "check"
	SectionElsewhere Section = "elsewhere"
	// SectionInternal is a defect in Compose itself, recovered from a panic.
	SectionInternal Section = "prime"
)

// Failure is one source of the Briefing that could not be read.
type Failure struct {
	Section Section
	Err     error
}

// Briefing is what capcom tells an agent at the start of a session, as data. A section is nil when
// its source failed; the failure is then one of Failures.
type Briefing struct {
	// Work is the Ready work and the issues in progress, nil when the store, the index, the
	// Ready-list or the doing query failed.
	Work *Work
	// Locks is every worktree's held Tree lock, ordered by key: nil when they could not be listed,
	// and never nil otherwise.
	Locks []lock.Status
	// Health is what check found, nil when check failed.
	Health *Health
	// Elsewhere is the issue changes on other branches, nil when there are none.
	Elsewhere []elsewhere.Entry
	// Failures are in the order Compose reads the sources: store, index, ready, doing, locks (one
	// per invalid lock record when the rest were listed), check, then one per Elsewhere scan
	// warning, worded as the scan wrote it.
	Failures []Failure
}

// Work is the Briefing's issue sections, each in Ready-list order.
type Work struct {
	// ReadyPlanned is the Ready issues that carry a plan.
	ReadyPlanned []index.Entry
	// ReadyUnplanned is the Ready issues without one.
	ReadyUnplanned []index.Entry
	// Doing is the issues whose status is in the doing category.
	Doing []index.Entry
}

// Health is the store's check result as the Briefing shows it.
type Health struct {
	// BrokenRefs is the check problems of Note refs that name no file, never nil.
	BrokenRefs []check.Problem
	// ProblemCount counts every check problem, the broken refs included.
	ProblemCount int
}

// Compose reads the Briefing of the store cwd belongs to, with lock ages and the Elsewhere window
// measured back from now. It never fails: a source that cannot be read leaves its section out and
// becomes one Failure, and a store that cannot be opened leaves every section out. Even a defect
// becomes a Failure of SectionInternal, so a Briefing never costs a session its start.
func Compose(cwd string, now time.Time) (composed Briefing) {
	defer func() {
		if recovered := recover(); recovered != nil {
			composed = Briefing{Failures: append(composed.Failures, Failure{
				Section: SectionInternal,
				Err:     fmt.Errorf("internal error: %v", recovered),
			})}
		}
	}()

	s, err := store.Open(cwd)
	if err != nil {
		composed.fail(SectionStore, err)
		return composed
	}
	composed.readWork(s)
	composed.readLocks(s)
	composed.readHealth(s)
	composed.scanElsewhere(s, now)
	return composed
}

// fail records that section's source failed with err.
func (b *Briefing) fail(section Section, err error) {
	b.Failures = append(b.Failures, Failure{Section: section, Err: err})
}

// readWork sets Work from the refreshed index: the Ready-list split planned and unplanned, and the
// issues in a doing status.
func (b *Briefing) readWork(s *store.Store) {
	// The refresh report Open leaves on Report is never read on purpose: its warnings name files
	// the index left out, and readHealth already counts each of them in ProblemCount. Repeating
	// them would flood the session start with one line per broken file.
	opened, err := index.Open(s, index.ModeRefresh)
	if err != nil {
		b.fail(SectionIndex, fmt.Errorf("%w; run capcom reindex", err))
		return
	}
	defer func() {
		if err := opened.Close(); err != nil {
			b.fail(SectionIndex, err)
		}
	}()

	ready, err := ReadyList(opened, s.Config, ReadyOptions{})
	if err != nil {
		b.fail(SectionReady, err)
		return
	}
	doing, err := opened.List(index.Filter{Categories: []config.Category{config.CategoryDoing}})
	if err != nil {
		b.fail(SectionDoing, err)
		return
	}
	work := &Work{Doing: doing}
	for _, item := range ready {
		if item.Entry.Plan != "" {
			work.ReadyPlanned = append(work.ReadyPlanned, item.Entry)
		} else {
			work.ReadyUnplanned = append(work.ReadyUnplanned, item.Entry)
		}
	}
	b.Work = work
}

// readLocks sets Locks to the Tree lock of every worktree that has one; each lock record that
// holds no valid lock becomes one Failure of SectionLocks.
func (b *Briefing) readLocks(s *store.Store) {
	held, warnings, err := lock.Open(s.GitCommonDir, s.Config.Lock.TTL).List()
	if err != nil {
		b.fail(SectionLocks, err)
		return
	}
	b.Locks = append([]lock.Status{}, held...)
	for _, warning := range warnings {
		b.fail(SectionLocks, warning)
	}
}

// readHealth sets Health from a check of the opened store s.
func (b *Briefing) readHealth(s *store.Store) {
	problems, err := runCheck(s)
	if err != nil {
		b.fail(SectionCheck, err)
		return
	}
	health := &Health{BrokenRefs: []check.Problem{}, ProblemCount: len(problems)}
	for _, problem := range problems {
		if problem.Code == check.CodeBrokenRef {
			health.BrokenRefs = append(health.BrokenRefs, problem)
		}
	}
	b.Health = health
}

// scanElsewhere sets Elsewhere from a scan of the issue tree on other branches, within the
// store's Elsewhere window. A store outside its worktree has no pathspec, so nothing is scanned.
func (b *Briefing) scanElsewhere(s *store.Store, now time.Time) {
	relative, err := filepath.Rel(s.WorktreeRoot, s.IssuesDir())
	if err != nil || strings.HasPrefix(relative, "..") {
		return
	}
	report := elsewhere.Scan(elsewhere.Options{
		WorktreeRoot: s.WorktreeRoot,
		Pathspecs:    []string{filepath.ToSlash(relative)},
		Days:         s.Config.Elsewhere.Days,
		Now:          now,
	})
	b.Elsewhere = report.Entries
	for _, warning := range report.Warnings {
		b.fail(SectionElsewhere, errors.New(warning))
	}
}
