package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// describePackageFile says what a step that publishes one generated file to a channel repository
// does.
func describePackageFile(run *publishRun, repo, path string) string {
	return fmt.Sprintf("render %s from release %s's %s, then commit %q to %s and push it when the file changes",
		path, run.version, checksumsFile, packageCommitSubject(run.version), repo)
}

// publishPackageFile clones repo into a temporary directory, writes content to path there and,
// only when that changes the file, commits it as `capcom X.Y.Z` and pushes the commit straight to
// the repository's branch — no pull request. A repository with no commits yet has
// no branch to push to, so its first commit is pushed to main.
func publishPackageFile(ctx context.Context, run *publishRun, repo, path, content string) error {
	workDir, err := os.MkdirTemp("", "capcom-channel-")
	if err != nil {
		return fmt.Errorf("create the %s clone directory: %w", repo, err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	cloneDir := filepath.Join(workDir, "repo")
	if _, err := run.commands.run(ctx, workDir, nonInteractiveEnv, "gh", "repo", "clone", repo, cloneDir, "--", "--quiet"); err != nil {
		return fmt.Errorf("clone %s: %w", repo, err)
	}
	history, err := run.commands.run(ctx, cloneDir, nil, "git", "rev-list", "--all", "--max-count=1")
	if err != nil {
		return fmt.Errorf("read %s's history: %w", repo, err)
	}

	filePath := filepath.Join(cloneDir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(filePath), directoryMode); err != nil {
		return fmt.Errorf("create %s's directory in the %s clone: %w", path, repo, err)
	}
	if err := os.WriteFile(filePath, []byte(content), documentMode); err != nil {
		return fmt.Errorf("write %s in the %s clone: %w", path, repo, err)
	}
	if _, err := run.commands.run(ctx, cloneDir, nil, "git", "add", "--", path); err != nil {
		return fmt.Errorf("stage %s in the %s clone: %w", path, repo, err)
	}
	staged, err := run.commands.run(ctx, cloneDir, nil, "git", "diff", "--cached", "--name-only")
	if err != nil {
		return fmt.Errorf("compare %s with %s's: %w", path, repo, err)
	}
	if strings.TrimSpace(staged) == "" {
		_, _ = fmt.Fprintf(run.output, "  %s already holds this %s: nothing to push\n", repo, path)
		return nil
	}

	if _, err := run.commands.run(ctx, cloneDir, nil, "git", "commit", "--quiet", "-m", packageCommitSubject(run.version)); err != nil {
		return fmt.Errorf("commit %s to %s: %w", path, repo, err)
	}
	pushRef := "HEAD"
	if strings.TrimSpace(history) == "" {
		pushRef = "HEAD:" + mainBranch
	}
	if _, err := run.commands.run(ctx, cloneDir, nonInteractiveEnv, "git", "push", "--quiet", remoteName, pushRef); err != nil {
		return fmt.Errorf("push %s to %s: %w", path, repo, err)
	}
	return nil
}

// packageCommitSubject is the subject of the commit a tap or bucket takes for a release.
func packageCommitSubject(version releaseVersion) string {
	return binaryName + " " + bareRelease(version)
}

// pinnedVersionStep reports a channel file done when the version it pins is the release's bare
// version or a newer release — publish never moves a channel back onto an older version — and
// pending when it pins an older version or one that does not parse.
func pinnedVersionStep(name, file, pinned string, version releaseVersion) stepReport {
	bare := bareRelease(version)
	if pinned == bare {
		return stepReport{Name: name, State: stateDone, Detail: fmt.Sprintf("%s pins %s", file, bare)}
	}
	if pinnedVersion, err := parseReleaseVersion(pinned); err == nil && pinnedVersion.compare(version) > 0 {
		return stepReport{Name: name, State: stateDone, Detail: fmt.Sprintf("%s pins newer %s", file, pinned)}
	}
	return stepReport{Name: name, State: statePending, Detail: fmt.Sprintf("%s pins %q, not %s", file, pinned, bare)}
}

// readRepoFile reads one file of a GitHub repository through the contents API, a 404 failing
// like any other failed call: the smoke check's read of the bucket, which the probes make through
// channelRepos instead.
func readRepoFile(ctx context.Context, commands runner, dir, repo, path string) (string, error) {
	return execChannelRepos{commands: commands, dir: dir}.readContents(ctx, repo, path)
}

// isNotFound reports whether a gh api failure is GitHub answering 404 — the path or repository
// does not exist, or the repository is empty — as opposed to a call that could not be made.
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "(HTTP 404)")
}

// setupPendingError is a step's converge reporting, without failing, that the step cannot finish
// until a human does a one-time setup: publish records the step setup-pending with detail and
// goes on to the next step.
type setupPendingError struct {
	detail string
}

func (e setupPendingError) Error() string { return e.detail }
