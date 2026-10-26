package gitx

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The committer times of the commits datedRepo makes, in Unix seconds.
const (
	firstCommitTime  = 1_700_000_000
	secondCommitTime = 1_700_100_000
)

// newUnbornRepo creates a git repository with no commit yet and returns its symlink-free root.
func newUnbornRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	setHermeticGitEnv(t, root)
	git(t, root, "init", "-q")
	return root
}

// commitFileAt is commitFile with the commit's author and committer time set to unixTime.
func commitFileAt(t *testing.T, root, name, content string, unixTime int64) {
	t.Helper()
	date := fmt.Sprintf("@%d +0000", unixTime)
	t.Setenv("GIT_AUTHOR_DATE", date)
	t.Setenv("GIT_COMMITTER_DATE", date)
	commitFile(t, root, name, content)
}

// revParse returns the object name revision names.
func revParse(t *testing.T, root, revision string) string {
	t.Helper()
	output, err := run(root, "rev-parse", revision)
	if err != nil {
		t.Fatalf("rev-parse %s: %v", revision, err)
	}
	return strings.TrimSpace(output)
}

// datedRepo creates a repository with two commits on HEAD, at firstCommitTime and
// secondCommitTime, and returns its root with the two commits' object names.
func datedRepo(t *testing.T) (root, first, second string) {
	t.Helper()
	root = newUnbornRepo(t)
	commitFileAt(t, root, ".capcom/issues/a.md", "a\n", firstCommitTime)
	first = revParse(t, root, "HEAD")
	commitFileAt(t, root, ".capcom/issues/b.md", "b\n", secondCommitTime)
	second = revParse(t, root, "HEAD")
	return root, first, second
}

func TestResolveCommit_RefsAndUnknownNames(t *testing.T) {
	root, first, second := datedRepo(t)
	git(t, root, "tag", "start", first)
	tests := []struct {
		name     string
		revision string
		want     string
	}{
		{name: "HEAD", revision: "HEAD", want: second},
		{name: "tag", revision: "start", want: first},
		{name: "relative", revision: "HEAD~1", want: first},
		{name: "unknown ref", revision: "no-such-ref", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			commit, err := ResolveCommit(root, test.revision)

			if commit != test.want || err != nil {
				t.Errorf("ResolveCommit(%q) = %q, %v; want %q", test.revision, commit, err, test.want)
			}
		})
	}
}

func TestResolveCommit_UnbornHead_IsEmptyWithoutError(t *testing.T) {
	root := newUnbornRepo(t)

	commit, err := ResolveCommit(root, "HEAD")

	if commit != "" || err != nil {
		t.Errorf("ResolveCommit on an unborn HEAD = %q, %v; want empty and no error", commit, err)
	}
}

func TestCommitBefore_PicksTheNewestCommitAtOrBeforeTheTime(t *testing.T) {
	root, first, second := datedRepo(t)
	cases := []struct {
		name     string
		unixTime int64
		want     string
	}{
		{name: "before the first commit", unixTime: firstCommitTime - 1, want: ""},
		{name: "at the first commit", unixTime: firstCommitTime, want: first},
		{name: "between the commits", unixTime: secondCommitTime - 1, want: first},
		{name: "after the last commit", unixTime: secondCommitTime + 1, want: second},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			commit, err := CommitBefore(root, time.Unix(testCase.unixTime, 0))

			if err != nil || commit != testCase.want {
				t.Errorf("CommitBefore = %q, %v; want %q and no error", commit, err, testCase.want)
			}
		})
	}
}

func TestCommitBefore_UnbornHead_IsEmptyWithoutError(t *testing.T) {
	root := newUnbornRepo(t)

	commit, err := CommitBefore(root, time.Now())

	if commit != "" || err != nil {
		t.Errorf("CommitBefore on an unborn HEAD = %q, %v; want empty and no error", commit, err)
	}
}

func TestCommitBefore_NotARepository_ReturnsTheGitError(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	setHermeticGitEnv(t, root)

	commit, err := CommitBefore(root, time.Now())

	var gitFailure *Error
	if commit != "" || !errors.As(err, &gitFailure) {
		t.Errorf("CommitBefore outside a repository = %q, %v; want empty and the git error", commit, err)
	}
}

func TestTreeFiles_ListsThePathspecAtEachRevision(t *testing.T) {
	root, first, second := datedRepo(t)
	commitFile(t, root, "README.md", "readme\n")
	blobA := revParse(t, root, first+":.capcom/issues/a.md")
	blobB := revParse(t, root, second+":.capcom/issues/b.md")
	cases := []struct {
		name     string
		revision string
		want     []TreeFile
	}{
		{name: "first", revision: first, want: []TreeFile{{Path: ".capcom/issues/a.md", Blob: blobA}}},
		{name: "second", revision: second, want: []TreeFile{
			{Path: ".capcom/issues/a.md", Blob: blobA},
			{Path: ".capcom/issues/b.md", Blob: blobB},
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			files, err := TreeFiles(filepath.Join(root, ".capcom"), testCase.revision, ".capcom")

			if err != nil {
				t.Fatalf("TreeFiles: %v", err)
			}
			if !reflect.DeepEqual(files, testCase.want) {
				t.Errorf("TreeFiles = %+v, want %+v", files, testCase.want)
			}
		})
	}
}

func TestTreeFiles_UnbornHead_IsEmptyWithoutError(t *testing.T) {
	root := newUnbornRepo(t)

	files, err := TreeFiles(root, "HEAD", ".capcom")

	if files != nil || err != nil {
		t.Errorf("TreeFiles at an unborn HEAD = %+v, %v; want none and no error", files, err)
	}
}
