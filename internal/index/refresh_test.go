package index

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/diagnose"
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
	want := diagnose.Finding{
		Code:    diagnose.OrphanDirectory,
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

func TestRefresh_RepeatedIDDirectoryWithoutNodeFiles_WarnsItLeftOut(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "a.md", issueText("A", "open", ""))
	writeIssue(t, s, "a/x.md", issueText("X one", "open", ""))
	writeIssue(t, s, "b.md", issueText("B", "open", ""))
	writeIssue(t, s, "b/x.md", issueText("X two", "open", ""))
	if err := os.MkdirAll(filepath.Join(s.IssuesDir(), "b", "x"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	x := openIndex(t, s)

	report := refresh(t, x)

	want := diagnose.Finding{
		Code:    diagnose.OrphanDirectory,
		Path:    ".capcom/issues/b/x",
		Message: "directory left out: its node file x.md has a duplicate id",
	}
	if !slices.Contains(report.Warnings, want) {
		t.Errorf("warnings = %q, want %q", warningStrings(report), want.String())
	}
	for _, warning := range report.Warnings {
		if warning.Path == ".capcom/issues/a/x" {
			t.Errorf("warnings = %q, want none for the absent directory a/x", warningStrings(report))
		}
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

func TestRefresh_InvalidFileListsAnAliasFirst_WarnsNamingTheFirstCleanClaimant(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeIssue(t, s, "aaa.md", issueWith("A", "bogus", "aliases: [s]\n"))
	writeIssue(t, s, "mmm.md", issueWith("M", "open", "aliases: [s]\n"))
	writeIssue(t, s, "zzz.md", issueWith("Z", "open", "aliases: [s]\n"))
	x := openIndex(t, s)

	report := refresh(t, x)

	var duplicates []string
	for _, warning := range report.Warnings {
		if warning.Code == diagnose.Duplicate {
			duplicates = append(duplicates, warning.String())
		}
	}
	want := []string{`.capcom/issues/zzz.md: duplicate alias "s": also an alias of .capcom/issues/mmm.md`}
	if !slices.Equal(duplicates, want) {
		t.Errorf("duplicate warnings = %q, want only %q", duplicates, want)
	}
}

func TestRefresh_NodesBelowARepeatedID_LeftOutWithTheDirectoryWarningAlone(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	writeDuplicateIDWithChildren(t, s)
	writeIssue(t, s, "b/x/bad.md", issueWith("Bad", "bogus", ""))
	writeIssue(t, s, "kept.md", issueWith("Kept", "open", "aliases: [old-c]\n"))
	writeNote(t, s, "a.md", "---\ntitle: A\n---\nBody.\n")
	writeNote(t, s, "a/x.md", "---\ntitle: X one\n---\nBody.\n")
	writeNote(t, s, "b.md", "---\ntitle: B\n---\nBody.\n")
	writeNote(t, s, "b/x.md", "---\ntitle: X two\n---\nBody.\n")
	writeNote(t, s, "b/x/bad.md", "---\ntitle: \"\"\n---\nBody.\n")
	x := openIndex(t, s)

	report := refresh(t, x)

	for _, id := range []string{"c2", "bad"} {
		if isIndexed(t, x, id) {
			t.Errorf("issue %s is indexed, want it left out with the directory of the repeated id x", id)
		}
	}
	if _, found, err := x.Note("bad"); err != nil || found {
		t.Errorf("Note(bad) = %t, %v; want it left out with the directory of the repeated id x", found, err)
	}
	for _, dir := range []string{".capcom/issues/b/x", ".capcom/knowledge/b/x"} {
		want := diagnose.Finding{
			Code:    diagnose.OrphanDirectory,
			Path:    dir,
			Message: "directory left out: its node file x.md has a duplicate id",
		}
		if !slices.Contains(report.Warnings, want) {
			t.Errorf("warnings = %q, want %q", warningStrings(report), want.String())
		}
	}
	for _, warning := range report.Warnings {
		if strings.HasPrefix(warning.Path, ".capcom/issues/b/x/") || strings.HasPrefix(warning.Path, ".capcom/knowledge/b/x/") {
			t.Errorf("warnings = %q, want none of a file below the repeated id x", warningStrings(report))
		}
	}
}

// rewriteKeepingMtime writes content over the file at path and sets its mtime back to mtime. It
// writes again, at most for two seconds, until the change time and identity changeStamp reads
// move off the ones before the first write, so a file system whose change time ticks coarsely
// still shows the edit.
func rewriteKeepingMtime(t *testing.T, path, content string, mtime time.Time) {
	t.Helper()
	before := stampOf(t, path)
	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatalf("Chtimes: %v", err)
		}
		if stampOf(t, path) != before {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the change time of %s did not move in two seconds of rewrites", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// stampOf returns the change time and identity changeStamp reads for the file at path, joined.
func stampOf(t *testing.T, path string) string {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	ctime, identity := changeStamp(path, info)
	return fmt.Sprintf("%d %s", ctime, identity)
}

func TestRefresh_SameSizeEditWithMtimeRestored_IsSeen(t *testing.T) {
	t.Parallel()
	if !slices.Contains([]string{"linux", "darwin", "windows"}, runtime.GOOS) {
		t.Skipf("changeStamp reads no change time on %s", runtime.GOOS)
	}
	s := newStore(t)
	then := time.Now().Add(-time.Hour).Truncate(time.Second)
	path := writeIssue(t, s, "kept-mtime.md", issueText("Alpha title", "open", ""))
	if err := os.Chtimes(path, then, then); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
	x := openIndex(t, s, laterClock())
	rewriteKeepingMtime(t, path, issueText("Bravo title", "open", ""), then)

	report := refresh(t, x)

	if want := []string{".capcom/issues/kept-mtime.md"}; !slices.Equal(report.parsed, want) {
		t.Errorf("parsed = %v, want %v: the same-size edit with its mtime restored", report.parsed, want)
	}
	if title := mustIssue(t, x, "kept-mtime").Title; title != "Bravo title" {
		t.Errorf("title = %q, want the edit seen", title)
	}
	if found, err := x.Search("bravo"); err != nil || !slices.Equal(found, []string{"kept-mtime"}) {
		t.Errorf("Search(bravo) = %v, %v; want [kept-mtime]", found, err)
	}
}
