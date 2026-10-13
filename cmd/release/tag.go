package main

import (
	"context"
	"fmt"
	"time"
)

// probeTag reports the tag done only when origin holds an annotated tag for the version: a
// lightweight tag carries no message and is not what a release is cut from, so it is unknown,
// never done.
func probeTag(ctx context.Context, env probeEnv, version releaseVersion) stepReport {
	tag := version.String()
	hasTag, err := env.repo.RemoteHasTag(ctx, tag)
	if err != nil {
		return unknownStep(stepTag, "could not list %s's tags: %v", remoteName, err)
	}
	if !hasTag {
		return stepReport{Name: stepTag, State: statePending, Detail: fmt.Sprintf("%s has no tag %s", remoteName, tag)}
	}

	kind, err := env.repo.RemoteTagType(ctx, tag)
	if err != nil {
		return unknownStep(stepTag, "could not read the type of tag %s: %v", tag, err)
	}
	switch kind {
	case "tag":
		return stepReport{Name: stepTag, State: stateDone, Detail: fmt.Sprintf("annotated tag %s on %s", tag, remoteName)}
	case "commit":
		return unknownStep(stepTag, "%s on %s is a lightweight tag; a release tag must be annotated", tag, remoteName)
	default:
		return unknownStep(stepTag, "tag %s points at a %q object, not an annotated tag", tag, kind)
	}
}

// describeTag says what the tag step does.
func describeTag(run *publishRun) string {
	return fmt.Sprintf("push HEAD to %s when it is behind, then wait up to %s for the CI-made annotated tag %s at the %s bump",
		remoteMain, run.options.tagTimeout, run.version, versionFile)
}

// convergeTag pushes the commit the gate judged to main when origin/main is behind it, waits for
// the tag-on-version-bump workflow to tag the bump, and then requires that tag annotated and on
// the commit that set VERSION. HEAD resolving to another commit than the gated one refuses the
// step before anything is pushed: a commit made after the gate cleared was never judged. It never
// creates a tag: the workflow is the one tagger.
func convergeTag(ctx context.Context, run *publishRun) error {
	current, err := run.repo.Resolve(ctx, "HEAD")
	if err != nil {
		return fmt.Errorf("resolve HEAD: %w", err)
	}
	if current != run.head {
		return fmt.Errorf("HEAD moved from %s to %s after the gate cleared it: run publish again",
			short(run.head), short(current))
	}

	isPushed, err := run.repo.IsAncestor(ctx, run.head, remoteMain)
	if err != nil {
		return fmt.Errorf("check whether %s holds %s: %w", remoteMain, short(run.head), err)
	}
	if !isPushed {
		if err := run.repo.PushMain(ctx, run.head); err != nil {
			return fmt.Errorf("push %s to %s: %w", short(run.head), remoteMain, err)
		}
	}
	if err := waitForTag(ctx, run); err != nil {
		return err
	}
	return verifyTag(ctx, run)
}

// waitForTag polls origin for the version's tag until it appears or the timeout passes.
func waitForTag(ctx context.Context, run *publishRun) error {
	tag := run.version.String()
	for waited := time.Duration(0); ; waited += run.options.pollInterval {
		hasTag, err := run.repo.RemoteHasTag(ctx, tag)
		if err != nil {
			return fmt.Errorf("list %s's tags: %w", remoteName, err)
		}
		if hasTag {
			return nil
		}
		if waited >= run.options.tagTimeout {
			return fmt.Errorf("%s had no tag %s after %s: check the tag-on-version-bump workflow's run for the push, then run publish again",
				remoteName, run.version, run.options.tagTimeout)
		}
		if err := run.options.sleep(ctx, run.options.pollInterval); err != nil {
			return fmt.Errorf("wait for tag %s: %w", run.version, err)
		}
	}
}

// verifyTag fetches the tag — on a first run the workflow made it after the gate's fetch, so it
// is not local yet — and requires it annotated and on the last commit at or before the gated
// commit that changed VERSION.
func verifyTag(ctx context.Context, run *publishRun) error {
	tag := run.version.String()
	if err := run.repo.FetchTag(ctx, tag); err != nil {
		return fmt.Errorf("fetch tag %s: %w", tag, err)
	}

	isAnnotated, err := run.repo.IsAnnotated(ctx, tag)
	if err != nil {
		return fmt.Errorf("read the type of tag %s: %w", tag, err)
	}
	if !isAnnotated {
		return fmt.Errorf("%s on %s is a lightweight tag; a release tag must be annotated", tag, remoteName)
	}

	tagged, err := run.repo.Resolve(ctx, tag)
	if err != nil {
		return fmt.Errorf("resolve tag %s: %w", tag, err)
	}
	bump, err := run.repo.LastCommitTouching(ctx, run.head, versionFile)
	if err != nil {
		return fmt.Errorf("find the %s bump commit: %w", versionFile, err)
	}
	if tagged != bump {
		return fmt.Errorf("tag %s is on %s, not on the %s bump %s", tag, short(tagged), versionFile, short(bump))
	}
	return nil
}
