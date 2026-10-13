package main

import (
	"context"
	"fmt"
	"strings"
)

// describePackageFile says what a step that publishes one generated file to a channel repository
// does.
func describePackageFile(run *publishRun, repo, path string) string {
	return fmt.Sprintf("render %s from release %s's %s, then commit %q to %s and push it when the file changes",
		path, run.version, checksumsFile, packageCommitSubject(run.version), repo)
}

// publishPackageFile publishes content to path in repo through the run's channels, committed as
// `capcom X.Y.Z`, and says so when the repository already holds it.
func publishPackageFile(ctx context.Context, run *publishRun, repo, path, content string) error {
	isPushed, err := run.channels.PublishFile(ctx, repo, path, content, packageCommitSubject(run.version))
	if err != nil {
		return err
	}
	if !isPushed {
		_, _ = fmt.Fprintf(run.output, "  %s already holds this %s: nothing to push\n", repo, path)
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
