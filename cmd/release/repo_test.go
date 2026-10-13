package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestExecRepoRunsEachMethodsArgv pins the git and gh argv behind every execRepo method, run in
// the checkout's directory, and what each makes of the output: every call that reaches a remote
// runs with nonInteractiveEnv, every local one with no environment of its own.
func TestExecRepoRunsEachMethodsArgv(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		call     func(ctx context.Context, checkout execRepo) (any, error)
		output   string
		want     any
		wantCall recordedCall
	}{
		"FetchMain": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return nil, checkout.FetchMain(ctx)
			},
			wantCall: recordedCall{env: nonInteractiveEnv, name: "git",
				args: []string{"fetch", "--quiet", "origin", "main", "--tags"}},
		},
		"IsClean on a clean tree": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.IsClean(ctx)
			},
			want:     true,
			wantCall: recordedCall{name: "git", args: []string{"status", "--porcelain"}},
		},
		"IsClean on a dirty tree": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.IsClean(ctx)
			},
			output:   "?? notes.txt\n",
			want:     false,
			wantCall: recordedCall{name: "git", args: []string{"status", "--porcelain"}},
		},
		"Resolve": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.Resolve(ctx, "v0.24.11")
			},
			output:   featureCommit + "\n",
			want:     featureCommit,
			wantCall: recordedCall{name: "git", args: []string{"rev-parse", "--verify", "v0.24.11^{commit}"}},
		},
		"Parents of an ordinary commit": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.Parents(ctx, bumpCommit)
			},
			output:   bumpCommit + " " + featureCommit + "\n",
			want:     []string{featureCommit},
			wantCall: recordedCall{name: "git", args: []string{"rev-list", "--parents", "-n1", bumpCommit}},
		},
		"Parents of a merge": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.Parents(ctx, mergeCommit)
			},
			output:   mergeCommit + " " + bumpCommit + " " + sideCommit + "\n",
			want:     []string{bumpCommit, sideCommit},
			wantCall: recordedCall{name: "git", args: []string{"rev-list", "--parents", "-n1", mergeCommit}},
		},
		"Parents of a root commit": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.Parents(ctx, rootCommit)
			},
			output:   rootCommit + "\n",
			want:     []string{},
			wantCall: recordedCall{name: "git", args: []string{"rev-list", "--parents", "-n1", rootCommit}},
		},
		"ChangedPaths uses the two-tree form": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.ChangedPaths(ctx, bumpCommit, rollupCommit)
			},
			output:   "CHANGELOG.md\nVERSION\n",
			want:     rollupPaths,
			wantCall: recordedCall{name: "git", args: []string{"diff-tree", "-r", "--name-only", bumpCommit, rollupCommit}},
		},
		"IsAncestor yes": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.IsAncestor(ctx, featureCommit, remoteMain)
			},
			want:     true,
			wantCall: recordedCall{name: "git", args: []string{"merge-base", "--is-ancestor", featureCommit, "origin/main"}},
		},
		"HasGreenRun with a run": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.HasGreenRun(ctx, featureCommit)
			},
			output: `[{"databaseId":38131086119}]` + "\n",
			want:   true,
			wantCall: recordedCall{env: nonInteractiveEnv, name: "gh", args: []string{"run", "list",
				"--workflow", "ci.yml", "--commit", featureCommit, "--status", "success", "--json", "databaseId"}},
		},
		"HasGreenRun with none": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.HasGreenRun(ctx, featureCommit)
			},
			output: "[]\n",
			want:   false,
			wantCall: recordedCall{env: nonInteractiveEnv, name: "gh", args: []string{"run", "list",
				"--workflow", "ci.yml", "--commit", featureCommit, "--status", "success", "--json", "databaseId"}},
		},
		"ReleaseTags": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.ReleaseTags(ctx)
			},
			output:   "v0.24.10\nv0.24.11\n",
			want:     []string{"v0.24.10", "v0.24.11"},
			wantCall: recordedCall{name: "git", args: []string{"tag", "--list", "v*"}},
		},
		"ReleaseTags with none": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.ReleaseTags(ctx)
			},
			want:     []string{},
			wantCall: recordedCall{name: "git", args: []string{"tag", "--list", "v*"}},
		},
		"CountCommits": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.CountCommits(ctx, "v0.24.11..HEAD")
			},
			output:   "7\n",
			want:     7,
			wantCall: recordedCall{name: "git", args: []string{"rev-list", "--count", "v0.24.11..HEAD"}},
		},
		"LatestRelease": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				release, isPublished, err := checkout.LatestRelease(ctx)
				return []any{release, isPublished}, err
			},
			output: `{"publishedAt":"2026-10-08T19:24:24Z","tagName":"v0.24.11"}` + "\n",
			want:   []any{githubRelease{Tag: "v0.24.11", Published: "2026-10-08T19:24:24Z"}, true},
			wantCall: recordedCall{env: nonInteractiveEnv, name: "gh",
				args: []string{"release", "view", "--repo", "airiclenz/capcom", "--json", "tagName,publishedAt"}},
		},
		"RemoteHasTag with the tag": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.RemoteHasTag(ctx, "v0.24.11")
			},
			output: "81eec1b3ac7d27f7ef38fe2368b59b89cbd8832b\trefs/tags/v0.24.11\n",
			want:   true,
			wantCall: recordedCall{env: nonInteractiveEnv, name: "git",
				args: []string{"ls-remote", "--tags", "origin", "refs/tags/v0.24.11"}},
		},
		"RemoteHasTag without it": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.RemoteHasTag(ctx, "v0.24.11")
			},
			want: false,
			wantCall: recordedCall{env: nonInteractiveEnv, name: "git",
				args: []string{"ls-remote", "--tags", "origin", "refs/tags/v0.24.11"}},
		},
		"RemoteTagType": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.RemoteTagType(ctx, "v0.24.11")
			},
			output: "tag\n",
			want:   "tag",
			wantCall: recordedCall{env: nonInteractiveEnv, name: "gh",
				args: []string{"api", "repos/airiclenz/capcom/git/ref/tags/v0.24.11", "--jq", ".object.type"}},
		},
		"ReleaseAssets keeps only the uploaded ones": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				assets, isFound, err := checkout.ReleaseAssets(ctx, "v0.24.11")
				return []any{assets, isFound}, err
			},
			output: `{"assets":[{"name":"capcom_0.24.11_linux_amd64.tar.gz","state":"uploaded"},` +
				`{"name":"SHA256SUMS","state":"starter"}],"isDraft":false}` + "\n",
			want: []any{githubReleaseAssets{names: []string{"capcom_0.24.11_linux_amd64.tar.gz"}}, true},
			wantCall: recordedCall{env: nonInteractiveEnv, name: "gh",
				args: []string{"release", "view", "v0.24.11", "--repo", "airiclenz/capcom", "--json", "assets,isDraft"}},
		},
		"ReleaseAssets reads a draft": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				assets, isFound, err := checkout.ReleaseAssets(ctx, "v0.24.11")
				return []any{assets, isFound}, err
			},
			output: `{"assets":[{"name":"SHA256SUMS","state":"uploaded"}],"isDraft":true}` + "\n",
			want:   []any{githubReleaseAssets{names: []string{"SHA256SUMS"}, isDraft: true}, true},
			wantCall: recordedCall{env: nonInteractiveEnv, name: "gh",
				args: []string{"release", "view", "v0.24.11", "--repo", "airiclenz/capcom", "--json", "assets,isDraft"}},
		},
		"PushMain": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return nil, checkout.PushMain(ctx)
			},
			wantCall: recordedCall{env: nonInteractiveEnv, name: "git", args: []string{"push", "origin", "HEAD:main"}},
		},
		"FetchTag": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return nil, checkout.FetchTag(ctx, "v0.24.11")
			},
			wantCall: recordedCall{env: nonInteractiveEnv, name: "git",
				args: []string{"fetch", "--quiet", "origin", "refs/tags/v0.24.11:refs/tags/v0.24.11"}},
		},
		"IsAnnotated on an annotated tag": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.IsAnnotated(ctx, "v0.24.11")
			},
			output:   "tag\n",
			want:     true,
			wantCall: recordedCall{name: "git", args: []string{"cat-file", "-t", "refs/tags/v0.24.11"}},
		},
		"IsAnnotated on a lightweight tag": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.IsAnnotated(ctx, "v0.24.11")
			},
			output:   "commit\n",
			want:     false,
			wantCall: recordedCall{name: "git", args: []string{"cat-file", "-t", "refs/tags/v0.24.11"}},
		},
		"ShowFile": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.ShowFile(ctx, "HEAD", "VERSION")
			},
			output:   "0.24.11\n",
			want:     "0.24.11\n",
			wantCall: recordedCall{name: "git", args: []string{"show", "HEAD:VERSION"}},
		},
		"LastCommitTouching": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.LastCommitTouching(ctx, "HEAD", "VERSION")
			},
			output:   bumpCommit + "\n",
			want:     bumpCommit,
			wantCall: recordedCall{name: "git", args: []string{"log", "-1", "--format=%H", "HEAD", "--", "VERSION"}},
		},
		// CreateRelease verifies the tag exists on the remote rather than making one, titles the
		// release with it and marks it latest; the assets follow in the order they were given.
		"CreateRelease": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return nil, checkout.CreateRelease(ctx, "v0.24.11", "/notes.md", []string{"/dist/a.tar.gz", "/dist/SHA256SUMS"})
			},
			wantCall: recordedCall{env: nonInteractiveEnv, name: "gh", args: []string{"release", "create", "v0.24.11",
				"--repo", "airiclenz/capcom", "--verify-tag", "--title", "v0.24.11", "--notes-file", "/notes.md", "--latest",
				"/dist/a.tar.gz", "/dist/SHA256SUMS"}},
		},
		// UploadAssets' --clobber replaces an asset left in a failed upload state.
		"UploadAssets": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return nil, checkout.UploadAssets(ctx, "v0.24.11", []string{"/dist/a.zip", "/dist/b.zip"})
			},
			wantCall: recordedCall{env: nonInteractiveEnv, name: "gh", args: []string{"release", "upload", "v0.24.11",
				"--repo", "airiclenz/capcom", "--clobber", "/dist/a.zip", "/dist/b.zip"}},
		},
		// UndraftRelease publishes the draft and marks it latest, as CreateRelease does.
		"UndraftRelease": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return nil, checkout.UndraftRelease(ctx, "v0.24.11")
			},
			wantCall: recordedCall{env: nonInteractiveEnv, name: "gh", args: []string{"release", "edit", "v0.24.11",
				"--repo", "airiclenz/capcom", "--draft=false", "--latest"}},
		},
		// ReadAsset downloads to stdout, --output -, never to a file in the checkout, whose clean
		// tree the next gate checks.
		"ReadAsset": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return checkout.ReadAsset(ctx, "v0.24.11", checksumsFile)
			},
			output: "abc  capcom_0.24.11_linux_amd64.tar.gz\n",
			want:   "abc  capcom_0.24.11_linux_amd64.tar.gz\n",
			wantCall: recordedCall{env: nonInteractiveEnv, name: "gh", args: []string{"release", "download", "v0.24.11",
				"--repo", "airiclenz/capcom", "--pattern", "SHA256SUMS", "--output", "-"}},
		},
		// DownloadAsset writes the one asset into the directory it is given, outside the checkout.
		"DownloadAsset": {
			call: func(ctx context.Context, checkout execRepo) (any, error) {
				return nil, checkout.DownloadAsset(ctx, "v0.24.11", "capcom_0.24.11_linux_amd64.tar.gz", "/downloads")
			},
			wantCall: recordedCall{env: nonInteractiveEnv, name: "gh", args: []string{"release", "download", "v0.24.11",
				"--repo", "airiclenz/capcom", "--pattern", "capcom_0.24.11_linux_amd64.tar.gz", "--dir", "/downloads"}},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			commands := &fakeRunner{respond: func(recordedCall) (string, error) { return tc.output, nil }}
			checkout := execRepo{commands: commands, dir: "/checkout"}

			got, err := tc.call(t.Context(), checkout)

			if err != nil {
				t.Fatalf("%s failed: %v", name, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s = %#v, want %#v", name, got, tc.want)
			}
			wantCall := tc.wantCall
			wantCall.dir = "/checkout"
			if len(commands.calls) != 1 || !reflect.DeepEqual(commands.calls[0], wantCall) {
				t.Errorf("calls = %+v, want exactly %+v", commands.calls, wantCall)
			}
		})
	}
}

// TestExecRepoTellsANoFromABrokenCommand pins execRepo's classification: merge-base's exit
// status 1 is a "no", while any other failure, and output that answers nothing, is an error
// naming what broke.
func TestExecRepoTellsANoFromABrokenCommand(t *testing.T) {
	t.Parallel()

	t.Run("IsAncestor exit 1 is no", func(t *testing.T) {
		t.Parallel()
		checkout := execRepo{commands: answering("", exitStatus(1)), dir: "."}

		isAncestor, err := checkout.IsAncestor(t.Context(), bumpCommit, featureCommit)

		if err != nil || isAncestor {
			t.Errorf("IsAncestor = %t, %v; want false, nil", isAncestor, err)
		}
	})

	t.Run("LatestRelease release not found is none published", func(t *testing.T) {
		t.Parallel()
		checkout := execRepo{commands: answering("", errors.New(ghNoRelease)), dir: "."}

		release, isPublished, err := checkout.LatestRelease(t.Context())

		if err != nil || isPublished || release != (githubRelease{}) {
			t.Errorf("LatestRelease = %+v, %t, %v; want none, false, nil", release, isPublished, err)
		}
	})

	t.Run("ReleaseAssets release not found is no release", func(t *testing.T) {
		t.Parallel()
		checkout := execRepo{commands: answering("", errors.New(ghNoRelease)), dir: "."}

		assets, isFound, err := checkout.ReleaseAssets(t.Context(), "v0.24.11")

		if err != nil || isFound || !reflect.DeepEqual(assets, githubReleaseAssets{}) {
			t.Errorf("ReleaseAssets = %+v, %t, %v; want none, false, nil", assets, isFound, err)
		}
	})

	cases := map[string]struct {
		call      func(ctx context.Context, checkout execRepo) error
		output    string
		err       error
		wantError string
	}{
		"IsAncestor on another exit status": {
			call: func(ctx context.Context, checkout execRepo) error {
				_, err := checkout.IsAncestor(ctx, "no-such-commit", featureCommit)
				return err
			},
			err:       exitStatus(128),
			wantError: "exit status 128",
		},
		"Resolve printing no commit": {
			call: func(ctx context.Context, checkout execRepo) error {
				_, err := checkout.Resolve(ctx, "HEAD")
				return err
			},
			output:    "\n",
			wantError: "git rev-parse printed no commit for HEAD",
		},
		"Parents printing nothing": {
			call: func(ctx context.Context, checkout execRepo) error {
				_, err := checkout.Parents(ctx, bumpCommit)
				return err
			},
			wantError: "git rev-list printed nothing for " + bumpCommit,
		},
		"HasGreenRun with output that is not JSON": {
			call: func(ctx context.Context, checkout execRepo) error {
				_, err := checkout.HasGreenRun(ctx, featureCommit)
				return err
			},
			output:    "not json",
			wantError: "parse gh run list output",
		},
		"CountCommits printing no number": {
			call: func(ctx context.Context, checkout execRepo) error {
				_, err := checkout.CountCommits(ctx, "HEAD")
				return err
			},
			output:    "\n",
			wantError: "invalid syntax",
		},
		"LatestRelease when gh fails otherwise": {
			call: func(ctx context.Context, checkout execRepo) error {
				_, _, err := checkout.LatestRelease(ctx)
				return err
			},
			err:       errors.New(ghNotFound),
			wantError: "(HTTP 404)",
		},
		"LatestRelease with output that is not JSON": {
			call: func(ctx context.Context, checkout execRepo) error {
				_, _, err := checkout.LatestRelease(ctx)
				return err
			},
			output:    "not json",
			wantError: "parse gh release view output",
		},
		"LatestRelease naming no tag": {
			call: func(ctx context.Context, checkout execRepo) error {
				_, _, err := checkout.LatestRelease(ctx)
				return err
			},
			output:    "{}\n",
			wantError: "gh release view named no latest release",
		},
		"ReleaseAssets with output that is not JSON": {
			call: func(ctx context.Context, checkout execRepo) error {
				_, _, err := checkout.ReleaseAssets(ctx, "v0.24.11")
				return err
			},
			output:    "not json",
			wantError: "parse gh release view output",
		},
		"LastCommitTouching finding no commit": {
			call: func(ctx context.Context, checkout execRepo) error {
				_, err := checkout.LastCommitTouching(ctx, "HEAD", "VERSION")
				return err
			},
			output:    "\n",
			wantError: "git log found no commit changing VERSION at HEAD",
		},
		"IsClean when git fails": {
			call: func(ctx context.Context, checkout execRepo) error {
				_, err := checkout.IsClean(ctx)
				return err
			},
			err:       errors.New("git status: not a git repository"),
			wantError: "not a git repository",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkout := execRepo{commands: answering(tc.output, tc.err), dir: "."}

			err := tc.call(t.Context(), checkout)

			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("error = %v, want one containing %q", err, tc.wantError)
			}
		})
	}
}

// TestExecRepoKeepsAMissingToolDetectable pins that the methods smoke reads through hand back a
// runner error wrapped, never flattened, so isMissingTool still tells a gh that is not installed
// from one that ran and failed.
func TestExecRepoKeepsAMissingToolDetectable(t *testing.T) {
	t.Parallel()
	notFound := fmt.Errorf("gh: %w", &exec.Error{Name: "gh", Err: exec.ErrNotFound})

	cases := map[string]func(ctx context.Context, checkout execRepo) error{
		"RemoteHasTag": func(ctx context.Context, checkout execRepo) error {
			_, err := checkout.RemoteHasTag(ctx, "v0.24.11")
			return err
		},
		"RemoteTagType": func(ctx context.Context, checkout execRepo) error {
			_, err := checkout.RemoteTagType(ctx, "v0.24.11")
			return err
		},
		"Resolve": func(ctx context.Context, checkout execRepo) error {
			_, err := checkout.Resolve(ctx, "v0.24.11")
			return err
		},
		"LatestRelease": func(ctx context.Context, checkout execRepo) error {
			_, _, err := checkout.LatestRelease(ctx)
			return err
		},
		"ReadAsset": func(ctx context.Context, checkout execRepo) error {
			_, err := checkout.ReadAsset(ctx, "v0.24.11", checksumsFile)
			return err
		},
		"DownloadAsset": func(ctx context.Context, checkout execRepo) error {
			return checkout.DownloadAsset(ctx, "v0.24.11", checksumsFile, t.TempDir())
		},
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkout := execRepo{commands: answering("", notFound), dir: "."}

			err := call(t.Context(), checkout)

			if !isMissingTool(err) {
				t.Errorf("%s error = %v, want one a missing tool is told by", name, err)
			}
		})
	}
}

// TestExecRepoCloneAtFetchesThenClonesTheTag pins CloneAt's four calls: it fetches origin's
// tags, never prompting, finds the checkout's top level, clones that from the clone's parent
// directory and checks the tag out in the clone, the last three local.
func TestExecRepoCloneAtFetchesThenClonesTheTag(t *testing.T) {
	t.Parallel()
	commands := &fakeRunner{respond: func(call recordedCall) (string, error) {
		if call.args[0] == "rev-parse" {
			return publishToplevel + "\n", nil
		}
		return "", nil
	}}
	checkout := execRepo{commands: commands, dir: "/checkout"}
	build := t.TempDir()
	clone := filepath.Join(build, "src")

	err := checkout.CloneAt(t.Context(), "v0.24.11", clone)

	if err != nil {
		t.Fatalf("CloneAt: %v", err)
	}
	want := []recordedCall{
		{dir: "/checkout", env: nonInteractiveEnv, name: "git", args: []string{"fetch", "--quiet", "--tags", "origin"}},
		{dir: "/checkout", name: "git", args: []string{"rev-parse", "--show-toplevel"}},
		{dir: build, name: "git", args: []string{"clone", "--quiet", publishToplevel, clone}},
		{dir: clone, name: "git", args: []string{"checkout", "--quiet", "v0.24.11"}},
	}
	if !reflect.DeepEqual(commands.calls, want) {
		t.Errorf("calls = %+v, want %+v", commands.calls, want)
	}
}

// TestExecRepoCloneAtNamesTheStepThatFailed pins CloneAt's failures: each names its step, and a
// failed step runs nothing after it.
func TestExecRepoCloneAtNamesTheStepThatFailed(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		failing   string
		wantError string
		wantCalls int
	}{
		"the fetch":     {failing: "fetch", wantError: "fetch origin's tags", wantCalls: 1},
		"the top level": {failing: "rev-parse", wantError: "find the checkout's top level", wantCalls: 2},
		"the clone":     {failing: "clone", wantError: "clone the checkout to build v0.24.11", wantCalls: 3},
		"the checkout":  {failing: "checkout", wantError: "check out v0.24.11 in the clone", wantCalls: 4},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			commands := &fakeRunner{respond: func(call recordedCall) (string, error) {
				if call.args[0] == tc.failing {
					return "", errors.New("git " + tc.failing + ": cannot run")
				}
				return publishToplevel + "\n", nil
			}}
			checkout := execRepo{commands: commands, dir: "/checkout"}

			err := checkout.CloneAt(t.Context(), "v0.24.11", "/build/src")

			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("error = %v, want one containing %q", err, tc.wantError)
			}
			if len(commands.calls) != tc.wantCalls {
				t.Errorf("calls = %+v, want %d", commands.calls, tc.wantCalls)
			}
		})
	}
}

// TestExecRepoCommitFileStagesOnlyItsPath pins CommitFile: it writes the file — an existing one
// or a new one — then stages that one path by name and commits it with the subject, both local
// git calls in the checkout.
func TestExecRepoCommitFileStagesOnlyItsPath(t *testing.T) {
	t.Parallel()
	const subject = "chore(version): bump version to v0.25.0"

	for _, path := range []string{versionFile, "NEW.md"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeTestFile(t, filepath.Join(dir, versionFile), "0.24.11\n")
			commands := answering("", nil)
			checkout := execRepo{commands: commands, dir: dir}

			err := checkout.CommitFile(t.Context(), path, []byte("0.25.0\n"), subject)

			if err != nil {
				t.Fatalf("CommitFile: %v", err)
			}
			if got := readTestFile(t, filepath.Join(dir, path)); got != "0.25.0\n" {
				t.Errorf("%s = %q, want %q", path, got, "0.25.0\n")
			}
			want := []recordedCall{
				{dir: dir, name: "git", args: []string{"add", path}},
				{dir: dir, name: "git", args: []string{"commit", "--quiet", "-m", subject}},
			}
			if !reflect.DeepEqual(commands.calls, want) {
				t.Errorf("calls = %+v, want %+v", commands.calls, want)
			}
		})
	}
}

// TestExecRepoCommitFileNamesTheStepThatFailed pins CommitFile's failures: each names its step
// and what it worked on, and a failed stage commits nothing.
func TestExecRepoCommitFileNamesTheStepThatFailed(t *testing.T) {
	t.Parallel()
	const subject = "chore(release): cut v0.25.0"

	cases := map[string]struct {
		failing   string
		wantError string
		wantCalls int
	}{
		"the stage":  {failing: "add", wantError: "stage CHANGELOG.md", wantCalls: 1},
		"the commit": {failing: "commit", wantError: `commit "chore(release): cut v0.25.0"`, wantCalls: 2},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			commands := &fakeRunner{respond: func(call recordedCall) (string, error) {
				if call.args[0] == tc.failing {
					return "", errors.New("git " + tc.failing + ": cannot run")
				}
				return "", nil
			}}
			checkout := execRepo{commands: commands, dir: t.TempDir()}

			err := checkout.CommitFile(t.Context(), changelogFile, []byte("# Changelog\n"), subject)

			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("error = %v, want one containing %q", err, tc.wantError)
			}
			if len(commands.calls) != tc.wantCalls {
				t.Errorf("calls = %+v, want %d", commands.calls, tc.wantCalls)
			}
		})
	}

	t.Run("the write", func(t *testing.T) {
		t.Parallel()
		commands := answering("", nil)
		checkout := execRepo{commands: commands, dir: filepath.Join(t.TempDir(), "missing")}

		err := checkout.CommitFile(t.Context(), changelogFile, []byte("# Changelog\n"), subject)

		if err == nil || !strings.Contains(err.Error(), "write CHANGELOG.md") {
			t.Errorf("error = %v, want one naming the write of CHANGELOG.md", err)
		}
		if len(commands.calls) != 0 {
			t.Errorf("calls = %+v, want none after a failed write", commands.calls)
		}
	})
}

// TestRepoOpenerFallsBackToExecRepo pins what a verb's run options built without an opener open:
// the execRepo over the run's runner, at the options' directory.
func TestRepoOpenerFallsBackToExecRepo(t *testing.T) {
	t.Parallel()
	commands := answering("", nil)

	opened := repoOpener(nil).open(commands, "/checkout")

	if !reflect.DeepEqual(opened, execRepo{commands: commands, dir: "/checkout"}) {
		t.Errorf("opened %#v, want the execRepo over the runner at /checkout", opened)
	}
}

// answering is a recording runner that answers every command with output and err.
func answering(output string, err error) *fakeRunner {
	return &fakeRunner{respond: func(recordedCall) (string, error) {
		return output, err
	}}
}

// TestExecReposOpensAtTheDirectory pins the production opener: one execRepo per directory, over
// the runner it was built with.
func TestExecReposOpensAtTheDirectory(t *testing.T) {
	t.Parallel()
	commands := answering("", nil)

	opened := []repo{execRepos(commands)("a"), execRepos(commands)("b")}

	want := []repo{execRepo{commands: commands, dir: "a"}, execRepo{commands: commands, dir: "b"}}
	if !slices.Equal(opened, want) {
		t.Errorf("opened %#v, want %#v", opened, want)
	}
}
