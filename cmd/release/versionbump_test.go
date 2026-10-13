package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The one test of this package that runs real git (adr-0008-amend-bumptest): it drives
// .github/scripts/version-bump.sh, which decides the tags the tag-on-version-bump workflow
// makes, against a throwaway local repository. Nothing here reaches the network or gh.

// zeroSHA is the before sha GitHub sends for a newly created branch.
const zeroSHA = "0000000000000000000000000000000000000000"

// bumpRepo is a hermetic throwaway git repository: env isolates every git call from the
// user's and the system's configuration and from any repository above the temp directory.
type bumpRepo struct {
	dir string
	env []string
}

// newBumpRepo creates an empty repository in a temp directory, skipping the test where the
// script cannot run.
func newBumpRepo(t *testing.T) bumpRepo {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("version-bump.sh runs only on the Linux tag-on-version-bump runner")
	}
	for _, tool := range []string{"bash", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not found: %v", tool, err)
		}
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	repo := bumpRepo{
		dir: dir,
		env: []string{
			"GIT_CONFIG_GLOBAL=" + os.DevNull,
			"GIT_CONFIG_NOSYSTEM=1",
			"GIT_CEILING_DIRECTORIES=" + filepath.Dir(dir),
			"GIT_AUTHOR_NAME=Test Author",
			"GIT_AUTHOR_EMAIL=author@example.com",
			"GIT_COMMITTER_NAME=Test Committer",
			"GIT_COMMITTER_EMAIL=committer@example.com",
		},
	}
	repo.git(t, "init", "-q")
	return repo
}

// git runs git in the repository and returns its trimmed stdout.
func (repo bumpRepo) git(t *testing.T, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = repo.dir
	command.Env = append(os.Environ(), repo.env...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(string(out))
}

// commit writes content to the named file, commits it and returns the commit's sha.
func (repo bumpRepo) commit(t *testing.T, name, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo.dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	repo.git(t, "add", name)
	repo.git(t, "commit", "-q", "-m", "set "+name)
	return repo.git(t, "rev-parse", "HEAD")
}

// runVersionBump runs the script on the range before..after, requires it to succeed, and
// returns what it wrote to $GITHUB_OUTPUT and to stderr.
func (repo bumpRepo) runVersionBump(t *testing.T, before, after string) (output, stderr string) {
	t.Helper()
	output, stderr, exitCode := repo.runVersionBumpStatus(t, before, after)
	if exitCode != 0 {
		t.Fatalf("version-bump.sh %s %s: exit status %d: %s", before, after, exitCode, stderr)
	}
	return output, stderr
}

// runVersionBumpStatus runs the script on the range before..after and returns what it wrote
// to $GITHUB_OUTPUT and to stderr, and its exit status.
func (repo bumpRepo) runVersionBumpStatus(t *testing.T, before, after string) (output, stderr string, exitCode int) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", ".github", "scripts", "version-bump.sh"))
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	outputFile := filepath.Join(t.TempDir(), "github_output")
	command := exec.Command("bash", script, before, after)
	command.Dir = repo.dir
	command.Env = append(os.Environ(), append(repo.env, "GITHUB_OUTPUT="+outputFile)...)
	var errBuffer bytes.Buffer
	command.Stderr = &errBuffer
	if err := command.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("version-bump.sh %s %s: %v: %s", before, after, err, errBuffer.String())
		}
		exitCode = exitErr.ExitCode()
	}
	written, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", outputFile, err)
	}
	return string(written), errBuffer.String(), exitCode
}

func TestVersionBumpTagsTheBumpCommitNotThePushHead(t *testing.T) {
	t.Parallel()
	repo := newBumpRepo(t)
	before := repo.commit(t, "VERSION", "1.2.2\n")
	bump := repo.commit(t, "VERSION", "1.2.3\n")
	after := repo.commit(t, "README.md", "later work\n")

	output, _ := repo.runVersionBump(t, before, after)

	if want := "bumps=" + bump + "=v1.2.3\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}

func TestVersionBumpTagsTwoBumpsInOneRangeOldestFirst(t *testing.T) {
	t.Parallel()
	repo := newBumpRepo(t)
	before := repo.commit(t, "VERSION", "1.0.0\n")
	first := repo.commit(t, "VERSION", "1.1.0\n")
	repo.commit(t, "README.md", "between the bumps\n")
	second := repo.commit(t, "VERSION", "1.2.0\n")
	after := repo.commit(t, "README.md", "after the bumps\n")

	output, _ := repo.runVersionBump(t, before, after)

	if want := "bumps=" + first + "=v1.1.0 " + second + "=v1.2.0\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}

func TestVersionBumpTagsNothingWithoutAVersionChange(t *testing.T) {
	t.Parallel()
	repo := newBumpRepo(t)
	before := repo.commit(t, "VERSION", "1.2.3\n")
	after := repo.commit(t, "README.md", "no bump\n")

	output, _ := repo.runVersionBump(t, before, after)

	if want := "bumps=\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}

func TestVersionBumpRefusesAVersionThatIsNotBareXYZ(t *testing.T) {
	t.Parallel()
	repo := newBumpRepo(t)
	before := repo.commit(t, "VERSION", "1.0.0\n")
	after := repo.commit(t, "VERSION", "1.2\n")

	output, stderr, exitCode := repo.runVersionBumpStatus(t, before, after)

	if exitCode == 0 {
		t.Errorf("exit status = 0, want non-zero for a refused bump")
	}
	if want := "bumps=\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
	if !strings.Contains(stderr, "refusing to tag") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "refusing to tag")
	}
}

// TestVersionBumpExitsNonZeroOnALeadingV pins VERSION's one spelling at the tagger: a
// "v0.6.0" bump is refused and fails the step, after the valid bump is still listed.
func TestVersionBumpExitsNonZeroOnALeadingV(t *testing.T) {
	t.Parallel()
	repo := newBumpRepo(t)
	before := repo.commit(t, "VERSION", "0.5.0\n")
	valid := repo.commit(t, "VERSION", "0.5.1\n")
	after := repo.commit(t, "VERSION", "v0.6.0\n")

	output, stderr, exitCode := repo.runVersionBumpStatus(t, before, after)

	if exitCode == 0 {
		t.Errorf("exit status = 0, want non-zero for VERSION v0.6.0")
	}
	if want := "bumps=" + valid + "=v0.5.1\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
	if want := "reads 'v0.6.0', not a bare X.Y.Z value"; !strings.Contains(stderr, want) {
		t.Errorf("stderr = %q, want it to contain %q", stderr, want)
	}
}

// TestVersionBumpFallsBackToTheNewestReleaseTag pins the fallback for a `before` the clone
// does not hold: the range starts at the newest release tag reachable from `after`, so a bump
// buried under a later commit is still tagged.
func TestVersionBumpFallsBackToTheNewestReleaseTag(t *testing.T) {
	t.Parallel()
	repo := newBumpRepo(t)
	repo.commit(t, "VERSION", "1.0.0\n")
	repo.git(t, "tag", "v1.0.0")
	repo.commit(t, "README.md", "work after the release\n")
	bump := repo.commit(t, "VERSION", "1.1.0\n")
	after := repo.commit(t, "README.md", "work after the bump\n")
	missing := strings.Repeat("ab", 20)

	output, _ := repo.runVersionBump(t, missing, after)

	if want := "bumps=" + bump + "=v1.1.0\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}

func TestVersionBumpFallsBackToTheParentOfAfterForAZeroBefore(t *testing.T) {
	t.Parallel()
	repo := newBumpRepo(t)
	repo.commit(t, "VERSION", "1.0.0\n")
	repo.commit(t, "VERSION", "1.2.2\n")
	after := repo.commit(t, "VERSION", "1.2.3\n")

	output, _ := repo.runVersionBump(t, zeroSHA, after)

	if want := "bumps=" + after + "=v1.2.3\n"; output != want {
		t.Errorf("output = %q, want %q", output, want)
	}
}
