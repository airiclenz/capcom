package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// repo is the semantic seam over the capcom checkout and its GitHub repository: what the release
// verbs ask of them, in their own terms rather than as git or gh argv. execRepo is the production
// adapter; the tests hold the same answers as plain state in memRepo. Every method fails with an
// error naming the operation when its answer cannot be read; a "no" answer is never an error.
type repo interface {
	// FetchMain fetches origin's main branch and its tags into the checkout.
	FetchMain(ctx context.Context) error
	// IsClean reports whether the working tree has no change and no untracked file.
	IsClean(ctx context.Context) (bool, error)
	// Resolve returns the full commit id a revision — a ref, a tag, a commit id — names.
	Resolve(ctx context.Context, revision string) (string, error)
	// Parents returns commit's parents, first parent first: none for a root commit, two or more
	// for a merge.
	Parents(ctx context.Context, commit string) ([]string, error)
	// ChangedPaths returns the paths commit changes against parent.
	ChangedPaths(ctx context.Context, parent, commit string) ([]string, error)
	// IsAncestor reports whether ancestor is descendant or one of its ancestors.
	IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error)
	// HasGreenRun reports whether ci.yml has at least one successful run for commit.
	HasGreenRun(ctx context.Context, commit string) (bool, error)
}

// repoOpener opens the repo for the checkout at dir. newRootCommand hands one to every verb's run
// options, so a test can put any repo behind a whole command.
type repoOpener func(dir string) repo

// execRepos opens execRepo adapters that run their commands through commands.
func execRepos(commands runner) repoOpener {
	return func(dir string) repo {
		return execRepo{commands: commands, dir: dir}
	}
}

// open opens the repo for dir: through o, or, when o is nil — a verb's run options built without
// one, as a test driving a run function directly builds them — the execRepo over commands.
func (o repoOpener) open(commands runner, dir string) repo {
	if o == nil {
		return execRepo{commands: commands, dir: dir}
	}
	return o(dir)
}

// execRepo is the production repo: it owns the git and gh argv for the checkout at dir and for
// its GitHub repository, and turns their output and exit statuses into answers. Every call that
// reaches a remote — a fetch, a push, an ls-remote, any gh call — runs with nonInteractiveEnv, so
// a missing credential fails the call instead of prompting.
type execRepo struct {
	commands runner
	dir      string
}

// git runs a local git command in the checkout.
func (r execRepo) git(ctx context.Context, args ...string) (string, error) {
	return r.commands.run(ctx, r.dir, nil, "git", args...)
}

// remote runs a git or gh command that reaches a remote, never prompting.
func (r execRepo) remote(ctx context.Context, name string, args ...string) (string, error) {
	return r.commands.run(ctx, r.dir, nonInteractiveEnv, name, args...)
}

// FetchMain fetches origin's main and every tag.
func (r execRepo) FetchMain(ctx context.Context) error {
	_, err := r.remote(ctx, "git", "fetch", "--quiet", remoteName, mainBranch, "--tags")
	return err
}

// IsClean reads `git status --porcelain`: a clean tree prints nothing.
func (r execRepo) IsClean(ctx context.Context) (bool, error) {
	status, err := r.git(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(status) == "", nil
}

// Resolve peels revision to a commit with `git rev-parse --verify`.
func (r execRepo) Resolve(ctx context.Context, revision string) (string, error) {
	output, err := r.git(ctx, "rev-parse", "--verify", revision+"^{commit}")
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(output)
	if commit == "" {
		return "", fmt.Errorf("git rev-parse printed no commit for %s", revision)
	}
	return commit, nil
}

// Parents reads `git rev-list --parents -n1`, which prints the commit followed by each of its
// parents.
func (r execRepo) Parents(ctx context.Context, commit string) ([]string, error) {
	output, err := r.git(ctx, "rev-list", "--parents", "-n1", commit)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return nil, fmt.Errorf("git rev-list printed nothing for %s", commit)
	}
	return fields[1:], nil
}

// ChangedPaths lists the paths with the two-tree `git diff-tree -r --name-only`: the one-commit
// form would print the commit id first, and nothing at all for a merge.
func (r execRepo) ChangedPaths(ctx context.Context, parent, commit string) ([]string, error) {
	output, err := r.git(ctx, "diff-tree", "-r", "--name-only", parent, commit)
	if err != nil {
		return nil, err
	}
	return strings.Fields(output), nil
}

// exitCoder is what a failed command's error carries when the command ran and exited non-zero
// (exec.ExitError has it), so a caller can tell a "no" answer from a command that broke.
type exitCoder interface {
	ExitCode() int
}

// IsAncestor asks `git merge-base --is-ancestor`, which answers with its exit status: 0 yes, 1
// no, anything else a failure.
func (r execRepo) IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	_, err := r.git(ctx, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	var exit exitCoder
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// HasGreenRun lists ci.yml's successful runs for commit with `gh run list`.
func (r execRepo) HasGreenRun(ctx context.Context, commit string) (bool, error) {
	output, err := r.remote(ctx, "gh", "run", "list",
		"--workflow", ciWorkflow, "--commit", commit, "--status", "success", "--json", "databaseId")
	if err != nil {
		return false, err
	}
	var runs []struct {
		DatabaseID int64 `json:"databaseId"`
	}
	if err := json.Unmarshal([]byte(output), &runs); err != nil {
		return false, fmt.Errorf("parse gh run list output: %w", err)
	}
	return len(runs) > 0, nil
}
