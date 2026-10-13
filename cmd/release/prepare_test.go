package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// preparedOn is the local date the tests cut their release on.
var preparedOn = time.Date(2026, time.October, 11, 23, 30, 0, 0, time.Local)

// The fixture changelog: an intro, two Unreleased bullets and the release before them, each
// release under a Keep a Changelog `## [X.Y.Z] - YYYY-MM-DD` heading.
const (
	unreleasedChangelog = "# Changelog\n" +
		"\n" +
		"All notable changes.\n" +
		"\n" +
		"## [Unreleased]\n" +
		"\n" +
		"- **First.** One.\n" +
		"\n" +
		"- **Second.** Two.\n" +
		"\n" +
		"## [0.24.11] - 2026-10-08\n" +
		"\n" +
		"- **Old.** Three.\n"
	rolledUpChangelog = "# Changelog\n" +
		"\n" +
		"All notable changes.\n" +
		"\n" +
		"## [Unreleased]\n" +
		"\n" +
		"## [0.25.0] - 2026-10-11\n" +
		"\n" +
		"- **First.** One.\n" +
		"\n" +
		"- **Second.** Two.\n" +
		"\n" +
		"## [0.24.11] - 2026-10-08\n" +
		"\n" +
		"- **Old.** Three.\n"
)

// The tags and VERSION of the fixture checkout: v0.24.11 is the highest release tag, and VERSION
// holds it bare.
var preparedTags = []string{"v0.24.10", "v0.24.11"}

const preparedVersion = "0.24.11\n"

// prepareCheckout is a checkout prepare runs over: CHANGELOG.md and VERSION on disk in dir, where
// prepare reads them, and the repo's state in a memRepo that records every commit it is asked to
// make into commits.
type prepareCheckout struct {
	dir     string
	repo    memRepo
	commits []memCommit
}

// newPrepareCheckout writes a checkout with the given CHANGELOG.md and VERSION, its tree clean and
// its tags preparedTags.
func newPrepareCheckout(t *testing.T, changelog, version string) *prepareCheckout {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, changelogFile), changelog)
	writeTestFile(t, filepath.Join(dir, versionFile), version)
	checkout := &prepareCheckout{dir: dir}
	checkout.repo = memRepo{tags: preparedTags, madeCommits: &checkout.commits}
	return checkout
}

// writeTestFile writes content to path, failing the test when it cannot.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readTestFile returns path's content, failing the test when it cannot.
func readTestFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// opener opens the checkout's memRepo whatever directory it is asked for.
func (c *prepareCheckout) opener() repoOpener {
	return func(string) repo { return c.repo }
}

// prepare runs the prepare verb's body over the checkout, as `release prepare <version>` would.
func (c *prepareCheckout) prepare(t *testing.T, version string) (string, error) {
	t.Helper()
	return c.prepareWith(t, c.opener(), version)
}

// prepareWith runs the prepare verb's body over the checkout's files, its repo opened with
// openRepo, over a runner that refuses every command: prepare asks the repo, never the runner.
func (c *prepareCheckout) prepareWith(t *testing.T, openRepo repoOpener, version string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	options := prepareOptions{dir: c.dir, version: version, now: func() time.Time { return preparedOn }, openRepo: openRepo}
	err := runPrepare(t.Context(), refusingRunner(), &output, options)
	return output.String(), err
}

// refusingRunner is a recording runner whose every command fails.
func refusingRunner() *fakeRunner {
	return &fakeRunner{respond: func(call recordedCall) (string, error) {
		return "", errors.New(call.name + ": unavailable")
	}}
}

// TestPrepareMakesTheRollupThenTheBumpCommit pins the verb's whole effect: two commits with the
// exact subjects, each of its one path — the rollup recording the new heading, the bump a fresh
// bare `X.Y.Z\n` in VERSION — with nothing pushed. A bare version reads as its `v` form.
func TestPrepareMakesTheRollupThenTheBumpCommit(t *testing.T) {
	t.Parallel()

	for _, typed := range []string{"v0.25.0", "0.25.0"} {
		t.Run(typed, func(t *testing.T) {
			t.Parallel()
			checkout := newPrepareCheckout(t, unreleasedChangelog, preparedVersion)

			output, err := checkout.prepare(t, typed)

			if err != nil {
				t.Fatalf("prepare %s: %v", typed, err)
			}
			wantCommits := []memCommit{
				{subject: "chore(release): cut v0.25.0", path: changelogFile, content: rolledUpChangelog},
				{subject: "chore(version): bump version to v0.25.0", path: versionFile, content: "0.25.0\n"},
			}
			if !slices.Equal(checkout.commits, wantCommits) {
				t.Errorf("commits =\n%+v\nwant\n%+v", checkout.commits, wantCommits)
			}
			if !strings.Contains(output, "nothing was pushed") {
				t.Errorf("output does not say nothing was pushed:\n%s", output)
			}
		})
	}
}

// The fixture of capcom's own CHANGELOG.md before its first release: the Keep a Changelog
// header, an `### Added` group under Unreleased, and no release heading yet.
const (
	firstReleaseChangelog = "# Changelog\n" +
		"\n" +
		"All notable changes to this project are documented in this file.\n" +
		"\n" +
		"The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),\n" +
		"and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).\n" +
		"\n" +
		"## [Unreleased]\n" +
		"\n" +
		"### Added\n" +
		"\n" +
		"- `capcom` binary with a `version` command.\n" +
		"- `capcom init` creates the store.\n"
	firstReleaseRolledUp = "# Changelog\n" +
		"\n" +
		"All notable changes to this project are documented in this file.\n" +
		"\n" +
		"The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),\n" +
		"and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).\n" +
		"\n" +
		"## [Unreleased]\n" +
		"\n" +
		"## [0.1.0] - 2026-10-11\n" +
		"\n" +
		"### Added\n" +
		"\n" +
		"- `capcom` binary with a `version` command.\n" +
		"- `capcom init` creates the store.\n"
)

// TestPrepareCutsTheFirstReleaseOfAKeepAChangelogFile pins the cut capcom's own checkout makes
// first: no `v*` tag exists, so any version is above the highest one; VERSION reads `0.0.0`; and
// the `### Added` group moves under the new release heading with Unreleased left empty above it.
func TestPrepareCutsTheFirstReleaseOfAKeepAChangelogFile(t *testing.T) {
	t.Parallel()
	checkout := newPrepareCheckout(t, firstReleaseChangelog, "0.0.0\n")
	checkout.repo.tags = nil

	_, err := checkout.prepare(t, "v0.1.0")

	if err != nil {
		t.Fatalf("prepare v0.1.0: %v", err)
	}
	wantCommits := []memCommit{
		{subject: "chore(release): cut v0.1.0", path: changelogFile, content: firstReleaseRolledUp},
		{subject: "chore(version): bump version to v0.1.0", path: versionFile, content: "0.1.0\n"},
	}
	if !slices.Equal(checkout.commits, wantCommits) {
		t.Errorf("commits =\n%+v\nwant\n%+v", checkout.commits, wantCommits)
	}
}

// TestRollUpChangelog pins the rollup's bytes: the heading goes directly under Unreleased with
// one blank line on each side however many blank lines stood there, and a CRLF changelog keeps
// its line ending.
func TestRollUpChangelog(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		changelog string
		want      string
	}{
		"the house layout": {changelog: unreleasedChangelog, want: rolledUpChangelog},
		"no blank line under Unreleased": {
			changelog: "## [Unreleased]\n- **First.** One.\n",
			want:      "## [Unreleased]\n\n## [0.25.0] - 2026-10-11\n\n- **First.** One.\n",
		},
		"several blank lines under Unreleased": {
			changelog: "## [Unreleased]\n\n\n\n- **First.** One.\n",
			want:      "## [Unreleased]\n\n## [0.25.0] - 2026-10-11\n\n- **First.** One.\n",
		},
		"a CRLF changelog": {
			changelog: "# Changelog\r\n\r\n## [Unreleased]\r\n\r\n- **First.** One.\r\n",
			want:      "# Changelog\r\n\r\n## [Unreleased]\r\n\r\n## [0.25.0] - 2026-10-11\r\n\r\n- **First.** One.\r\n",
		},
		"a newer patch beside an older one": {
			changelog: "## [Unreleased]\n\n- **First.** One.\n\n## [0.25.1] - 2026-10-09\n",
			want:      "## [Unreleased]\n\n## [0.25.0] - 2026-10-11\n\n- **First.** One.\n\n## [0.25.1] - 2026-10-09\n",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := rollUpChangelog([]byte(tc.changelog), releaseVersion{minor: 25}, preparedOn)

			if err != nil {
				t.Fatalf("rollUpChangelog: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("rollup =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// TestPrepareRefuses pins every refusal: the run fails, the error names the reason, and nothing
// is written, staged or committed.
func TestPrepareRefuses(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		version   string
		changelog string
		versionIn string
		isDirty   bool
		failing   string
		wantError string
	}{
		"a malformed version": {version: "v0.25", wantError: "is not a release version"},
		"a pre-release version": {
			version: "v0.25.0-rc.1", wantError: "is not a release version",
		},
		"the highest tag itself": {
			version: "v0.24.11", wantError: "v0.24.11 is not above the highest release tag v0.24.11",
		},
		"below the highest tag": {
			version: "v0.24.9", wantError: "v0.24.9 is not above the highest release tag v0.24.11",
		},
		"a dirty working tree": {isDirty: true, wantError: "the working tree is not clean"},
		"the tags cannot be listed": {
			failing: methodReleaseTags, wantError: "list the release tags",
		},
		"origin cannot be fetched": {
			failing: methodFetchMain, wantError: "fetch origin's main and its tags: FetchMain: cannot run",
		},
		"the tree's status cannot be read": {
			failing: methodIsClean, wantError: "read the working tree's status",
		},
		"VERSION already reads the version": {
			versionIn: "0.25.0\n",
			wantError: "VERSION already reads 0.25.0, an unpushed version bump: cut the next patch, v0.25.1, instead",
		},
		"VERSION already reads the version with its v": {
			versionIn: "v0.25.0\n",
			wantError: "VERSION already reads 0.25.0, an unpushed version bump: cut the next patch, v0.25.1, instead",
		},
		"no Unreleased heading": {
			changelog: "# Changelog\n\n## [0.24.11] - 2026-10-08\n\n- **Old.** Three.\n",
			wantError: "CHANGELOG.md has no ## [Unreleased] heading",
		},
		"no bullet under Unreleased": {
			changelog: "## [Unreleased]\n\nProse, not a bullet.\n\n## [0.24.11] - 2026-10-08\n\n- **Old.** Three.\n",
			wantError: "CHANGELOG.md:1: no bullet under ## [Unreleased] to release",
		},
		"an empty Unreleased at the end of the file": {
			changelog: "## [0.24.11] - 2026-10-08\n\n- **Old.** Three.\n\n## [Unreleased]\n\n",
			wantError: "CHANGELOG.md:5: no bullet under ## [Unreleased] to release",
		},
		"a heading for the version already": {
			changelog: "## [Unreleased]\n\n- **First.** One.\n\n## [0.25.0] - 2026-10-01\n\n- **Old.** Three.\n",
			wantError: "CHANGELOG.md:5: already has a ## [0.25.0] heading",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			changelog := tc.changelog
			if changelog == "" {
				changelog = unreleasedChangelog
			}
			versionIn := tc.versionIn
			if versionIn == "" {
				versionIn = preparedVersion
			}
			version := tc.version
			if version == "" {
				version = "v0.25.0"
			}
			checkout := newPrepareCheckout(t, changelog, versionIn)
			checkout.repo.isDirty = tc.isDirty
			checkout.repo.failing = tc.failing

			_, err := checkout.prepare(t, version)

			if err == nil {
				t.Fatalf("prepare %s succeeded", version)
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("error = %v, want it to contain %q", err, tc.wantError)
			}
			if len(checkout.commits) != 0 {
				t.Errorf("commits = %+v, want nothing committed", checkout.commits)
			}
			if got := readTestFile(t, filepath.Join(checkout.dir, changelogFile)); got != changelog {
				t.Errorf("CHANGELOG.md was rewritten:\n%s", got)
			}
			if got := readTestFile(t, filepath.Join(checkout.dir, versionFile)); got != versionIn {
				t.Errorf("VERSION was rewritten: %q", got)
			}
		})
	}
}

// TestPrepareNamesEveryRefusal pins that one run reports every refusal it finds, not only the
// first, so the owner fixes them in one pass.
func TestPrepareNamesEveryRefusal(t *testing.T) {
	t.Parallel()
	checkout := newPrepareCheckout(t, "# Changelog\n", "0.24.9\n")
	checkout.repo.isDirty = true

	_, err := checkout.prepare(t, "v0.24.9")

	for _, want := range []string{
		"the working tree is not clean",
		"not above the highest release tag",
		"VERSION already reads 0.24.9",
		"has no ## [Unreleased] heading",
	} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to contain %q", err, want)
		}
	}
}

// bumpRefusingRepo is a memRepo whose bump commit fails, as a commit hook that refuses it would.
type bumpRefusingRepo struct {
	memRepo
}

// CommitFile refuses the bump commit and makes any other.
func (r bumpRefusingRepo) CommitFile(ctx context.Context, path string, content []byte, subject string) error {
	if strings.HasPrefix(subject, "chore(version)") {
		return errors.New("the commit hook refused it")
	}
	return r.memRepo.CommitFile(ctx, path, content, subject)
}

// tagFetchingRepo is a memRepo whose origin holds release tags the checkout lacks: ReleaseTags
// lists them only once FetchMain has fetched them, as git lists a tag only once it is fetched.
type tagFetchingRepo struct {
	memRepo
	originTags []string
	// isFetched is a pointer so every copy of the repo shares whether FetchMain ran.
	isFetched *bool
}

// FetchMain fetches origin's tags into the checkout.
func (r tagFetchingRepo) FetchMain(ctx context.Context) error {
	if err := r.memRepo.FetchMain(ctx); err != nil {
		return err
	}
	*r.isFetched = true
	return nil
}

// ReleaseTags lists the checkout's release tags, origin's among them once they are fetched.
func (r tagFetchingRepo) ReleaseTags(ctx context.Context) ([]string, error) {
	tags, err := r.memRepo.ReleaseTags(ctx)
	if err != nil || !*r.isFetched {
		return tags, err
	}
	return append(tags, r.originTags...), nil
}

// TestPrepareJudgesTheVersionAgainstOriginsTags pins that prepare fetches before it reads the
// tags: a version origin already released refuses even while the checkout lacks its tag.
func TestPrepareJudgesTheVersionAgainstOriginsTags(t *testing.T) {
	t.Parallel()
	checkout := newPrepareCheckout(t, unreleasedChangelog, preparedVersion)
	var isFetched bool
	openRepo := func(string) repo {
		return tagFetchingRepo{memRepo: checkout.repo, originTags: []string{"v0.25.0"}, isFetched: &isFetched}
	}

	_, err := checkout.prepareWith(t, openRepo, "v0.25.0")

	if err == nil || !strings.Contains(err.Error(), "v0.25.0 is not above the highest release tag v0.25.0") {
		t.Errorf("error = %v, want it to judge v0.25.0 against origin's tag v0.25.0", err)
	}
	if len(checkout.commits) != 0 {
		t.Errorf("commits = %+v, want nothing committed", checkout.commits)
	}
}

// TestPrepareReportsAFailedBumpCommit pins that a bump commit that fails is a run failure naming
// the rollup commit already made, so the owner knows the checkout holds one of the two.
func TestPrepareReportsAFailedBumpCommit(t *testing.T) {
	t.Parallel()
	checkout := newPrepareCheckout(t, unreleasedChangelog, preparedVersion)
	openRepo := func(string) repo { return bumpRefusingRepo{memRepo: checkout.repo} }

	_, err := checkout.prepareWith(t, openRepo, "v0.25.0")

	if err == nil || !strings.Contains(err.Error(), `the rollup commit "chore(release): cut v0.25.0" was made`) {
		t.Errorf("error = %v, want it to name the rollup commit already made", err)
	}
	if len(checkout.commits) != 1 || checkout.commits[0].path != changelogFile {
		t.Errorf("commits = %+v, want only the rollup commit", checkout.commits)
	}
}

// TestPrepareRefusalIsARunFailure pins the exit status through the command tree: a refused
// version — malformed included — exits 1, while a missing version is a usage error (exit 2).
// The repo is the one the command tree was handed and the runner refuses every command, so a
// run that reached git other than through the repo would show. It changes the working
// directory, so it cannot run in parallel.
func TestPrepareRefusalIsARunFailure(t *testing.T) {
	checkout := newPrepareCheckout(t, unreleasedChangelog, preparedVersion)
	t.Chdir(checkout.dir)

	for _, version := range []string{"v0.25", "v0.24.11"} {
		commands := refusingRunner()
		_, err := executeWith(t, commands, checkout.opener(), nil, "prepare", version)
		if code := exitCodeFor(err); err == nil || code != exitRunFailed {
			t.Errorf("prepare %s = %v (exit %d), want exit %d", version, err, code, exitRunFailed)
		}
		if len(commands.calls) != 0 {
			t.Errorf("prepare %s ran %+v, want every git call through the repo", version, commands.calls)
		}
	}

	_, err := executeWith(t, refusingRunner(), checkout.opener(), nil, "prepare")
	if code := exitCodeFor(err); err == nil || code != exitBadUsage {
		t.Errorf("prepare with no version = %v (exit %d), want exit %d", err, code, exitBadUsage)
	}
}
