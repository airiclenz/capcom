package main

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// The tap repository and formula path the homebrew step writes to, and so the ones its probe reads.
const (
	tapRepo     = "airiclenz/homebrew-tap"
	formulaPath = "Formula/capcom.rb"
)

// formulaVersionPattern finds the version a Homebrew formula pins: its `version "X.Y.Z"` line.
var formulaVersionPattern = regexp.MustCompile(`(?m)^\s*version\s+"([^"]*)"`)

// probeHomebrew reports the tap setup-pending while its repository does not exist, and done when
// its formula pins the version or a newer one. A tap without the formula is pending: the
// repository is the one-time setup, the formula is what publish writes.
func probeHomebrew(ctx context.Context, env probeEnv, version releaseVersion) stepReport {
	hasTap, err := env.channels.Exists(ctx, tapRepo)
	if err != nil {
		return unknownStep(stepHomebrew, "could not read the %s repository: %v", tapRepo, err)
	}
	if !hasTap {
		return stepReport{Name: stepHomebrew, State: stateSetupPending, Detail: fmt.Sprintf("no %s repository yet", tapRepo)}
	}

	formula, hasFormula, err := env.channels.ReadFile(ctx, tapRepo, formulaPath)
	if err != nil {
		return unknownStep(stepHomebrew, "could not read %s from %s: %v", formulaPath, tapRepo, err)
	}
	if !hasFormula {
		return stepReport{Name: stepHomebrew, State: statePending, Detail: fmt.Sprintf("%s has no %s", tapRepo, formulaPath)}
	}

	fields := formulaVersionPattern.FindStringSubmatch(formula)
	if fields == nil {
		return stepReport{Name: stepHomebrew, State: statePending, Detail: fmt.Sprintf("%s pins no version", formulaPath)}
	}
	return pinnedVersionStep(stepHomebrew, "formula", fields[1], version)
}

// describeHomebrew says what the homebrew step does.
func describeHomebrew(run *publishRun) string {
	return describePackageFile(run, tapRepo, formulaPath)
}

// convergeHomebrew renders the formula from the release's SHA256SUMS and commits it to the tap.
func convergeHomebrew(ctx context.Context, run *publishRun) error {
	sums, err := downloadReleaseSums(ctx, run)
	if err != nil {
		return err
	}
	formula, err := renderFormula(run.version.String(), sums)
	if err != nil {
		return err
	}
	return publishPackageFile(ctx, run, tapRepo, formulaPath, formula)
}

// renderFormula renders the whole Homebrew formula for version (vX.Y.Z or a bare X.Y.Z), reading
// the four macOS and Linux archive hashes from the release's SHA256SUMS body.
func renderFormula(version, sums string) (string, error) {
	return renderPackageFile(formulaTemplate, version, sums)
}

// checkHomebrew upgrades a Homebrew-installed capcom and requires it to report the version; a
// host without Homebrew, or without capcom installed through it, skips. So does a version that is
// not known to be the latest published release: the tap pins the latest, so brew upgrade would
// move this machine onto that one instead, and with the latest release unreadable or none
// published there is no telling which version that is.
func (r *smokeRun) checkHomebrew(ctx context.Context) {
	r.step("brew upgrade moves this machine onto %s", r.version)
	if _, err := r.commands.run(ctx, r.options.dir, nil, "brew", "list", "--formula", binaryName); err != nil {
		if isMissingTool(err) {
			r.skip("Homebrew is not installed on this host")
			return
		}
		r.skip("capcom is not installed through Homebrew here")
		return
	}
	latest, isPublished, err := r.repo.LatestRelease(ctx)
	switch {
	case err != nil:
		r.skip("the latest release could not be read, so brew upgrade may not move this machine onto %s: %v", r.version, err)
		return
	case !isPublished:
		r.skip("%s has no published release, so brew upgrade may not move this machine onto %s", capcomRepo, r.version)
		return
	case latest.Tag != r.version.String():
		r.skip("%s is not the latest release (%s); brew upgrade would move this machine onto that one", r.version, latest.Tag)
		return
	}
	for _, args := range [][]string{{"update"}, {"upgrade", binaryName}} {
		if _, err := r.commands.run(ctx, r.options.dir, nil, "brew", args...); err != nil {
			r.fail("brew %s: %v", strings.Join(args, " "), err)
			return
		}
	}
	prefix, err := r.commands.run(ctx, r.options.dir, nil, "brew", "--prefix")
	if err != nil {
		r.fail("brew --prefix: %v", err)
		return
	}
	r.checkReportsVersion(ctx, filepath.Join(strings.TrimSpace(prefix), "bin", binaryName), "brew's capcom")
}
