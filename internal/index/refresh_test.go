package index

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// writeDuplicateIDWithChildren writes a.md, a/x.md, b.md, b/x.md and b/x/c2.md, which lists the
// alias old-c: x is a repeated id, so c2 lies under the directory of one of its files. a.md and
// b.md keep a/ and b/ from being orphan directories.
func writeDuplicateIDWithChildren(t *testing.T, s *store.Store) {
	t.Helper()
	writeIssue(t, s, "a.md", issueText("A", "open", ""))
	writeIssue(t, s, "a/x.md", issueText("X one", "open", ""))
	writeIssue(t, s, "b.md", issueText("B", "open", ""))
	writeIssue(t, s, "b/x.md", issueText("X two", "open", ""))
	writeIssue(t, s, "b/x/c2.md", issueWith("C2", "open", "aliases: [old-c]\n"))
}

// assertRepeatedIDProblem fails the test unless err is the duplicate problem of the repeated id x,
// naming both of its files.
func assertRepeatedIDProblem(t *testing.T, arg string, err error) {
	t.Helper()
	var problem *tree.Error
	if !errors.As(err, &problem) || !errors.Is(err, tree.ErrDuplicate) || errors.Is(err, tree.ErrNotFound) {
		t.Fatalf("Resolve(%q) err = %v, want a *tree.Error wrapping ErrDuplicate", arg, err)
	}
	for _, path := range []string{".capcom/issues/a/x.md", ".capcom/issues/b/x.md"} {
		if !strings.Contains(err.Error(), path) {
			t.Errorf("Resolve(%q) err = %v, want it to name %s", arg, err, path)
		}
	}
}

func TestRefresh_DuplicateIDWithChildren_LeavesTheSubtreeOut(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeDuplicateIDWithChildren(t, s)
	x := openIndex(t, s)

	report := refresh(t, x)

	if isIndexed(t, x, "c2") {
		t.Errorf("c2 is indexed, want it left out with the directory of the repeated id x")
	}
	want := Warning{
		Kind:    WarningOrphanDirectory,
		Path:    ".capcom/issues/b/x",
		Message: "directory left out: its node file x.md has a duplicate id",
	}
	if !slices.Contains(report.Warnings, want) {
		t.Errorf("warnings = %q, want %q", warningStrings(report), want.String())
	}
	for _, arg := range []string{"c2", "old-c"} {
		_, err := x.Resolve(arg)
		assertRepeatedIDProblem(t, arg, err)
	}
}

func TestResolve_PrefixOfANodeBelowARepeatedID_CountsItAsACandidate(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeDuplicateIDWithChildren(t, s)
	writeIssue(t, s, "c3.md", issueText("C3", "open", ""))
	x := openIndex(t, s)
	refresh(t, x)

	_, sharedErr := x.Resolve("c")
	_, ownErr := x.Resolve("c2")

	var ambiguous *tree.AmbiguousError
	if !errors.As(sharedErr, &ambiguous) || !slices.Equal(ambiguous.Candidates, []string{"c2", "c3"}) {
		t.Errorf("Resolve(c) err = %v, want ambiguous between c2 and c3", sharedErr)
	}
	assertRepeatedIDProblem(t, "c2", ownErr)
}

func TestResolve_PrefixOfOnlyANodeBelowARepeatedID_FailsWithItsDuplicateProblem(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeDuplicateIDWithChildren(t, s)
	x := openIndex(t, s)
	refresh(t, x)

	_, err := x.Resolve("c")

	assertRepeatedIDProblem(t, "c", err)
}

func TestRefresh_RepeatedIDFixed_IndexesTheSubtreeAgain(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeDuplicateIDWithChildren(t, s)
	x := openIndex(t, s)
	refresh(t, x)
	if err := os.Remove(filepath.Join(s.IssuesDir(), "a", "x.md")); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	refresh(t, x)

	if entry, err := x.Resolve("old-c"); err != nil || entry.ID != "c2" {
		t.Errorf("Resolve(old-c) = %q, %v; want c2", entry.ID, err)
	}
}

func TestRefresh_RepeatedIDBelowARepeatedID_StaysADuplicate(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeDuplicateIDWithChildren(t, s)
	writeIssue(t, s, "b/x/yak.md", issueText("Yak one", "open", ""))
	writeIssue(t, s, "c.md", issueText("C", "open", ""))
	writeIssue(t, s, "c/yak.md", issueText("Yak two", "open", ""))
	x := openIndex(t, s)
	refresh(t, x)

	refresh(t, x)
	_, err := x.Resolve("yak")

	if !errors.Is(err, tree.ErrDuplicate) {
		t.Fatalf("Resolve(yak) err = %v, want ErrDuplicate", err)
	}
	for _, path := range []string{".capcom/issues/b/x/yak.md", ".capcom/issues/c/yak.md"} {
		if !strings.Contains(err.Error(), path) {
			t.Errorf("Resolve(yak) err = %v, want it to name %s", err, path)
		}
	}
}
