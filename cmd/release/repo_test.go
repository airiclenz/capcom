package main

import (
	"context"
	"errors"
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
