package gitx

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setHermeticGitEnv isolates git from user and system configuration (ADR 0007) and stops git
// from discovering a repository above root, so tests do not call t.Parallel.
func setHermeticGitEnv(t *testing.T, root string) {
	t.Helper()
	environment := map[string]string{
		"GIT_CONFIG_GLOBAL":       os.DevNull,
		"GIT_CONFIG_NOSYSTEM":     "1",
		"GIT_CEILING_DIRECTORIES": filepath.Dir(root),
		"GIT_AUTHOR_NAME":         "Test Author",
		"GIT_AUTHOR_EMAIL":        "author@example.com",
		"GIT_COMMITTER_NAME":      "Test Committer",
		"GIT_COMMITTER_EMAIL":     "committer@example.com",
	}
	for key, value := range environment {
		t.Setenv(key, value)
	}
}

// git runs a (possibly writing) git command for test setup.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

// newRepo creates a git repository with one empty commit and returns its symlink-free root.
func newRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	setHermeticGitEnv(t, root)
	git(t, root, "init", "-q")
	git(t, root, "commit", "-q", "--allow-empty", "-m", "initial")
	return root
}

func TestDiscover_FromSubdirectory_ReturnsAbsolutePaths(t *testing.T) {
	root := newRepo(t)
	subdirectory := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(subdirectory, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	paths, err := Discover(subdirectory)

	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	gitDir := filepath.Join(root, ".git")
	if paths.WorktreeRoot != root || paths.GitDir != gitDir || paths.GitCommonDir != gitDir {
		t.Errorf("paths = %+v, want root %s and git dir %s", paths, root, gitDir)
	}
}

func TestDiscover_LinkedWorktree_SeparatesGitDirFromCommonDir(t *testing.T) {
	root := newRepo(t)
	linked := filepath.Join(filepath.Dir(root), filepath.Base(root)+"-linked")
	t.Cleanup(func() { os.RemoveAll(linked) })
	git(t, root, "worktree", "add", "-q", linked)

	paths, err := Discover(linked)

	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	commonDir := filepath.Join(root, ".git")
	if paths.WorktreeRoot != linked || paths.GitCommonDir != commonDir {
		t.Errorf("paths = %+v, want root %s common dir %s", paths, linked, commonDir)
	}
	if paths.GitDir == commonDir || !filepath.IsAbs(paths.GitDir) {
		t.Errorf("GitDir = %s, want an absolute per-worktree dir", paths.GitDir)
	}
}

func TestDiscover_OutsideRepository_FailsNotWorkTree(t *testing.T) {
	outside := t.TempDir()
	setHermeticGitEnv(t, outside)

	_, err := Discover(outside)

	if !errors.Is(err, ErrNotWorkTree) {
		t.Errorf("err = %v, want ErrNotWorkTree", err)
	}
}

func TestRun_FailingCommand_ReturnsGitStderr(t *testing.T) {
	root := newRepo(t)

	_, err := Run(root, "rev-parse", "--verify", "no-such-ref")

	var failure *Error
	if !errors.As(err, &failure) || failure.Stderr == "" {
		t.Errorf("err = %v, want *gitx.Error with git's stderr", err)
	}
}

func TestRun_DisablesOptionalLocks(t *testing.T) {
	root := newRepo(t)

	output, err := Run(root, "-c", "alias.lockmode=!echo $GIT_OPTIONAL_LOCKS", "lockmode")

	if err != nil || strings.TrimSpace(output) != "0" {
		t.Errorf("GIT_OPTIONAL_LOCKS = %q (%v), want 0", output, err)
	}
}

func TestAuthorName_Sources_FirstNonEmptyWins(t *testing.T) {
	cases := []struct {
		name       string
		authorName string
		userName   string
		user       string
		want       string
		wantErr    error
	}{
		{name: "author variable", authorName: "Env Author", userName: "Config User", user: "login", want: "Env Author"},
		{name: "git config", userName: "Config User", user: "login", want: "Config User"},
		{name: "user variable", user: "login", want: "login"},
		{name: "none", wantErr: ErrNoAuthor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRepo(t)
			if tc.userName != "" {
				git(t, root, "config", "user.name", tc.userName)
			}
			t.Setenv("GIT_AUTHOR_NAME", tc.authorName)
			t.Setenv("USER", tc.user)

			name, err := AuthorName(root)

			if name != tc.want || !errors.Is(err, tc.wantErr) {
				t.Errorf("AuthorName = %q, %v; want %q, %v", name, err, tc.want, tc.wantErr)
			}
		})
	}
}
