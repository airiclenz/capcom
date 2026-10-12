package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// The commits of the test history, oldest first. Each id is longer than a short id, so a test
// can tell the full id in the report from the short one in a message.
const (
	rootCommit    = "1000000000000000000000000000000000000000"
	featureCommit = "2000000000000000000000000000000000000000"
	bumpCommit    = "3000000000000000000000000000000000000000"
	rollupCommit  = "4000000000000000000000000000000000000000"
	sideCommit    = "5000000000000000000000000000000000000000"
	mergeCommit   = "6000000000000000000000000000000000000000"
)

// The paths the test commits change against their first parent.
var (
	bumpPaths    = []string{"VERSION"}
	rollupPaths  = []string{"CHANGELOG.md", "VERSION"}
	featurePaths = []string{"cmd/release/dist.go", "cmd/release/dist_test.go"}
	mixedPaths   = []string{"CHANGELOG.md", "README.md"}
)

// fakeCommit is one commit of a test history: its parents, and the paths it changes against its
// first parent.
type fakeCommit struct {
	parents []string
	changed []string
}

// linearHistory is root ← feature ← bump ← rollup, each commit with its own path set.
func linearHistory() map[string]fakeCommit {
	return map[string]fakeCommit{
		rootCommit:    {},
		featureCommit: {parents: []string{rootCommit}, changed: featurePaths},
		bumpCommit:    {parents: []string{featureCommit}, changed: bumpPaths},
		rollupCommit:  {parents: []string{bumpCommit}, changed: rollupPaths},
	}
}

// exitStatus is a failed command that ran and exited with this status, as exec.ExitError
// reports one.
type exitStatus int

func (e exitStatus) Error() string { return fmt.Sprintf("exit status %d", int(e)) }
func (e exitStatus) ExitCode() int { return int(e) }

// TestGatePeelsOnlyTrailingChangelogAndVersionCommits pins how the gate commit is found: HEAD's
// trailing first-parent commits that touch only CHANGELOG.md and/or VERSION are walked past, the
// walk stops at the first commit touching anything else, and a merge is never walked past — nor
// asked for its changed paths, which memRepo refuses.
func TestGatePeelsOnlyTrailingChangelogAndVersionCommits(t *testing.T) {
	t.Parallel()

	mixedHistory := linearHistory()
	mixedHistory[sideCommit] = fakeCommit{parents: []string{bumpCommit}, changed: mixedPaths}
	mixedHistory[rollupCommit] = fakeCommit{parents: []string{sideCommit}, changed: rollupPaths}

	mergeHistory := linearHistory()
	mergeHistory[sideCommit] = fakeCommit{parents: []string{featureCommit}, changed: featurePaths}
	mergeHistory[mergeCommit] = fakeCommit{parents: []string{bumpCommit, sideCommit}, changed: bumpPaths}

	cases := map[string]struct {
		commits map[string]fakeCommit
		head    string
		want    string
	}{
		"a feature commit at HEAD is the gate":   {commits: linearHistory(), head: featureCommit, want: featureCommit},
		"a VERSION-only bump is peeled":          {commits: linearHistory(), head: bumpCommit, want: featureCommit},
		"a rollup and a bump are both peeled":    {commits: linearHistory(), head: rollupCommit, want: featureCommit},
		"the peel stops at another path":         {commits: mixedHistory, head: rollupCommit, want: sideCommit},
		"a merge at HEAD is never peeled":        {commits: mergeHistory, head: mergeCommit, want: mergeCommit},
		"a root commit at HEAD is the gate":      {commits: linearHistory(), head: rootCommit, want: rootCommit},
		"a bump onto the root stops at the root": {commits: map[string]fakeCommit{rootCommit: {}, bumpCommit: {parents: []string{rootCommit}, changed: bumpPaths}}, head: bumpCommit, want: rootCommit},
		"an empty commit is not peeled":          {commits: map[string]fakeCommit{rootCommit: {}, bumpCommit: {parents: []string{rootCommit}}}, head: bumpCommit, want: bumpCommit},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := memRepo{commits: tc.commits, head: tc.head, originMain: tc.head, greenRuns: []string{tc.want}}

			report := checkGate(t.Context(), repo)

			if report.Commit != tc.want {
				t.Errorf("gate commit = %s, want %s", report.Commit, tc.want)
			}
			if !report.IsOK {
				t.Errorf("gate blocked: %v", report.Failures)
			}
		})
	}
}

// TestGateNamesEachFailingCondition pins every way the gate blocks — each named in its own
// failure — and the CI condition's widening: a green run on a peeled commit origin/main contains
// clears it in place of one on the gate commit, since a push runs CI on its head alone.
func TestGateNamesEachFailingCondition(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo         memRepo
		wantCommit   string
		wantFailures []string
	}{
		"green on the gate commit clears": {
			repo:       memRepo{head: bumpCommit, originMain: bumpCommit, greenRuns: []string{featureCommit}},
			wantCommit: featureCommit,
		},
		"green run on the peeled bump, none on G clears": {
			repo:       memRepo{head: bumpCommit, originMain: bumpCommit, greenRuns: []string{bumpCommit}},
			wantCommit: featureCommit,
		},
		"green run on a peeled commit origin/main lacks does not clear": {
			repo:         memRepo{head: rollupCommit, originMain: bumpCommit, greenRuns: []string{rollupCommit}},
			wantCommit:   featureCommit,
			wantFailures: []string{"ci.yml has no successful run for 200000000000 or 300000000000"},
		},
		"dirty tree": {
			repo:         memRepo{head: featureCommit, originMain: featureCommit, isDirty: true, greenRuns: []string{featureCommit}},
			wantCommit:   featureCommit,
			wantFailures: []string{"the working tree is not clean"},
		},
		"gate commit not pushed": {
			repo:       memRepo{head: bumpCommit, originMain: rootCommit, greenRuns: []string{featureCommit}},
			wantCommit: featureCommit,
			wantFailures: []string{
				"gate commit 200000000000 is not on origin/main: push it first",
			},
		},
		"origin/main ahead of HEAD": {
			repo:         memRepo{head: featureCommit, originMain: bumpCommit, greenRuns: []string{featureCommit}},
			wantCommit:   featureCommit,
			wantFailures: []string{"origin/main has commits HEAD does not: pull first"},
		},
		"no green run": {
			repo:         memRepo{head: featureCommit, originMain: featureCommit},
			wantCommit:   featureCommit,
			wantFailures: []string{"ci.yml has no successful run for 200000000000"},
		},
		"every condition at once": {
			repo:       memRepo{head: featureCommit, originMain: rootCommit, isDirty: true},
			wantCommit: featureCommit,
			wantFailures: []string{
				"the working tree is not clean",
				"gate commit 200000000000 is not on origin/main: push it first",
				"ci.yml has no successful run for 200000000000",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := tc.repo
			repo.commits = linearHistory()

			report := checkGate(t.Context(), repo)

			if report.Commit != tc.wantCommit {
				t.Errorf("gate commit = %s, want %s", report.Commit, tc.wantCommit)
			}
			wantFailures := tc.wantFailures
			if wantFailures == nil {
				wantFailures = []string{}
			}
			if !slices.Equal(report.Failures, wantFailures) {
				t.Errorf("failures = %q, want %q", report.Failures, wantFailures)
			}
			if report.IsOK != (len(wantFailures) == 0) {
				t.Errorf("ok = %t with failures %q", report.IsOK, report.Failures)
			}
		})
	}
}

// TestGateNamesACheckThatCannotRun pins that a check the gate cannot run blocks it with the
// cause named, never passes it by default.
func TestGateNamesACheckThatCannotRun(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		failing string
		want    string
	}{
		"fetch":         {failing: methodFetchMain, want: "could not fetch origin"},
		"status":        {failing: methodIsClean, want: "could not read the working tree's status"},
		"head":          {failing: methodResolve, want: "could not resolve HEAD"},
		"peel":          {failing: methodParents, want: "could not find the gate commit"},
		"changed paths": {failing: methodChangedPaths, want: "could not find the gate commit"},
		"ancestry":      {failing: methodIsAncestor, want: "could not check whether 200000000000 is on origin/main"},
		"ci runs":       {failing: methodHasGreenRun, want: "could not read the ci.yml runs for 200000000000"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := memRepo{commits: linearHistory(), head: bumpCommit, originMain: bumpCommit,
				greenRuns: []string{featureCommit}, failing: tc.failing}

			report := checkGate(t.Context(), repo)

			if report.IsOK {
				t.Fatal("gate cleared although a check could not run")
			}
			if !slices.ContainsFunc(report.Failures, func(failure string) bool {
				return strings.HasPrefix(failure, tc.want)
			}) {
				t.Errorf("failures = %q, want one starting %q", report.Failures, tc.want)
			}
		})
	}
}
