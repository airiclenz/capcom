package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// The repository, package and manifest directory the winget step submits to, and so the ones its
// probe reads.
const (
	wingetRepo        = "microsoft/winget-pkgs"
	wingetPackageID   = "AiricLenz.Capcom"
	wingetManifestDir = "manifests/a/AiricLenz/Capcom"
)

// probeWinget reports winget done once the version's manifest is published in winget-pkgs or an
// open pull request submits it, and pending otherwise — including while the package is not in
// winget-pkgs at all: winget needs no one-time setup, since publish submits the first manifests
// itself with `komac submit`.
func probeWinget(ctx context.Context, env probeEnv, version releaseVersion) stepReport {
	bare := bareRelease(version)
	versions, isListed, err := env.channels.ListDirs(ctx, wingetRepo, wingetManifestDir)
	switch {
	case errors.Is(err, errUnparsableAnswer):
		return unknownStep(stepWinget, "could not parse the %s listing: %v", wingetManifestDir, err)
	case err != nil:
		return unknownStep(stepWinget, "could not read %s from %s: %v", wingetManifestDir, wingetRepo, err)
	case slices.Contains(versions, bare):
		return stepReport{Name: stepWinget, State: stateDone, Detail: fmt.Sprintf(
			"published: %s/%s in %s", wingetManifestDir, bare, wingetRepo)}
	}

	pulls, err := env.channels.OpenPulls(ctx, wingetRepo, wingetPackageID+" "+bare+" in:title")
	switch {
	case errors.Is(err, errUnparsableAnswer):
		return unknownStep(stepWinget, "could not parse gh pr list output: %v", err)
	case err != nil:
		return unknownStep(stepWinget, "could not list the open %s pull requests: %v", wingetRepo, err)
	}
	for _, pull := range pulls {
		if titleNamesRelease(pull.title, bare) {
			return stepReport{Name: stepWinget, State: stateDone, Detail: fmt.Sprintf(
				"submitted: pull request #%d %s", pull.number, pull.url)}
		}
	}
	if !isListed {
		return stepReport{Name: stepWinget, State: statePending, Detail: fmt.Sprintf(
			"%s is not in %s yet and no open pull request submits %s: publish submits it with `komac submit`",
			wingetPackageID, wingetRepo, bare)}
	}
	return stepReport{Name: stepWinget, State: statePending, Detail: fmt.Sprintf(
		"no %s %s manifest or open pull request in %s", wingetPackageID, bare, wingetRepo)}
}

// titleNamesRelease reports whether a winget pull request's title names the package and exactly
// the version — a search for 0.24.1 also finds 0.24.11, which must not count.
func titleNamesRelease(title, bare string) bool {
	hasPackage, hasVersion := false, false
	for _, word := range strings.Fields(title) {
		word = strings.Trim(word, ".,:;()[]")
		hasPackage = hasPackage || word == wingetPackageID
		hasVersion = hasVersion || word == bare
	}
	return hasPackage && hasVersion
}

// githubTokenVariable is the environment variable komac reads its GitHub token from, and the one
// publish takes the token from first.
const githubTokenVariable = "GITHUB_TOKEN"

// redactedToken stands in for the GitHub token wherever a command line is printed: the token
// reaches komac through its environment alone, never its argv or this program's output.
const redactedToken = "<redacted>"

// komacInstallHint names the ways to install komac, for the step that finds it missing.
const komacInstallHint = "install it with `cargo install --locked komac`, `brew install komac` or " +
	"`winget install komac` (https://github.com/russellbanks/Komac)"

// wingetSubmittedDetail is what the winget step reports when it has run: komac opens a pull
// request — `komac update` and `komac submit` alike — which merges later after winget's
// validation and review, so the release is submitted to winget, never published there, when
// publish returns.
const wingetSubmittedDetail = "submitted by this run: the pull request merges in " + wingetRepo +
	" after its validation and review"

// resolveWingetPackage records in the run whether winget-pkgs already has the package, which picks
// the winget step's komac path. It asks only while the winget step is pending: a done or unknown
// step never reaches the winget step's converge. It reads the listing the winget probe reads; a
// listing that is not there means the package is not there yet.
func (r *publishRun) resolveWingetPackage(ctx context.Context, steps []stepReport) error {
	if stepStateOf(steps, stepWinget) != statePending {
		return nil
	}
	_, isListed, err := r.channels.ListDirs(ctx, wingetRepo, wingetManifestDir)
	if err != nil {
		return fmt.Errorf("could not tell whether %s has %s: %w", wingetRepo, wingetManifestDir, err)
	}
	r.isWingetPackageListed = isListed
	return nil
}

// describeWinget says what the winget step does, with the komac command line it runs — the token
// shown redacted, as it never appears there.
func describeWinget(run *publishRun) string {
	bare := bareRelease(run.version)
	if !run.isWingetPackageListed {
		return fmt.Sprintf("render the first %s %s manifests into a temporary %s/%s and submit them to %s "+
			"(submitted, not published, until the pull request merges): %s",
			wingetPackageID, bare, wingetManifestDir, bare, wingetRepo,
			komacCommandLine(komacSubmitArgs(filepath.Join("<temp>", filepath.FromSlash(wingetManifestDir), bare))))
	}
	return fmt.Sprintf("submit the %s %s manifest update to %s (submitted, not published, until the pull request merges): %s",
		wingetPackageID, bare, wingetRepo, komacCommandLine(komacArgs(run.version)))
}

// komacCommandLine is komac's command line as publish prints it, the token redacted.
func komacCommandLine(args []string) string {
	return githubTokenVariable + "=" + redactedToken + " komac " + strings.Join(args, " ")
}

// convergeWinget submits the version to winget-pkgs with komac, which opens the pull request:
// `komac update` when the package is listed there, otherwise `komac submit` of the first
// manifests. The GitHub token reaches komac through its environment only, and komac's stdin is
// closed, so it can never wait on a prompt.
func convergeWinget(ctx context.Context, run *publishRun) error {
	dir := run.options.dir
	if _, err := run.commands.run(ctx, dir, nil, "komac", "--version"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("komac is not installed — %s", komacInstallHint)
		}
		return fmt.Errorf("run komac: %w", err)
	}
	token, err := githubToken(ctx, run)
	if err != nil {
		return err
	}

	env := append([]string{githubTokenVariable + "=" + token}, nonInteractiveEnv...)
	if !run.isWingetPackageListed {
		return submitWingetManifests(ctx, run, env, token)
	}
	if _, err := run.commands.run(ctx, dir, env, "komac", komacArgs(run.version)...); err != nil {
		// A failing command's error carries its stderr; scrub the token in case komac echoed it.
		return fmt.Errorf("submit the %s %s manifest update: %s",
			wingetPackageID, bareRelease(run.version), strings.ReplaceAll(err.Error(), token, redactedToken))
	}
	return nil
}

// submitWingetManifests renders the version's three winget manifests from the release's SHA256SUMS
// into <temp>/manifests/a/AiricLenz/Capcom/<X.Y.Z>/ and submits them with `komac submit` —
// `komac new` cannot run unattended for a zip installer. The directory is removed once the
// submission is made; when komac fails it is kept and the failure is reported as setup pending,
// naming the directory and the command to rerun it with, the token redacted.
func submitWingetManifests(ctx context.Context, run *publishRun, env []string, token string) error {
	sums, err := downloadReleaseSums(ctx, run)
	if err != nil {
		return err
	}
	manifests, err := renderWingetManifests(run.version.String(), sums)
	if err != nil {
		return err
	}
	workDir, err := os.MkdirTemp("", "capcom-winget-")
	if err != nil {
		return fmt.Errorf("create the winget manifest directory: %w", err)
	}
	manifestDir := filepath.Join(workDir, filepath.FromSlash(wingetManifestDir), bareRelease(run.version))
	if err := writeWingetManifests(manifestDir, manifests); err != nil {
		_ = os.RemoveAll(workDir)
		return err
	}

	args := komacSubmitArgs(manifestDir)
	if _, err := run.commands.run(ctx, run.options.dir, env, "komac", args...); err != nil {
		return setupPendingError{detail: fmt.Sprintf(
			"the first %s submission failed (%s); the manifests are kept in %s: submit them with `%s`",
			wingetPackageID, strings.ReplaceAll(err.Error(), token, redactedToken), manifestDir, komacCommandLine(args))}
	}
	if err := os.RemoveAll(workDir); err != nil {
		_, _ = fmt.Fprintf(run.output, "  could not remove %s: %v\n", workDir, err)
	}
	return nil
}

// writeWingetManifests writes the rendered manifests into manifestDir, creating it.
func writeWingetManifests(manifestDir string, manifests []renderedFile) error {
	if err := os.MkdirAll(manifestDir, directoryMode); err != nil {
		return fmt.Errorf("create %s: %w", manifestDir, err)
	}
	for _, manifest := range manifests {
		path := filepath.Join(manifestDir, manifest.name)
		if err := os.WriteFile(path, []byte(manifest.content), documentMode); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

// komacSubmitArgs is komac's argv for submitting the manifests in manifestDir as they are, with
// --yes answering the confirmation it would otherwise ask for.
func komacSubmitArgs(manifestDir string) []string {
	return []string{"submit", manifestDir, "--yes"}
}

// komacArgs is komac's argv for the version's manifest update: the two Windows archives' release
// URLs, which komac downloads to hash, and --submit to open the pull request.
func komacArgs(version releaseVersion) []string {
	bare := bareRelease(version)
	args := []string{"update", wingetPackageID, "--version", bare, "--urls"}
	for _, target := range distTargets {
		if target.isWindows() {
			args = append(args, releaseDownloadBase+version.String()+"/"+target.archiveName(bare))
		}
	}
	return append(args, "--submit")
}

// githubToken is the token komac opens its pull request with: GITHUB_TOKEN when set, else the
// one gh is signed in with.
func githubToken(ctx context.Context, run *publishRun) (string, error) {
	if token := strings.TrimSpace(run.options.getenv(githubTokenVariable)); token != "" {
		return token, nil
	}
	noToken := fmt.Sprintf("no GitHub token — set %s or sign in with `gh auth login`", githubTokenVariable)
	token, err := run.commands.run(ctx, run.options.dir, nonInteractiveEnv, "gh", "auth", "token")
	if err != nil {
		return "", fmt.Errorf("%s: %w", noToken, err)
	}
	if token = strings.TrimSpace(token); token == "" {
		return "", errors.New(noToken)
	}
	return token, nil
}

// wingetManifest is one manifest of a winget submission: the file name komac reads it under and
// the template it renders from.
type wingetManifest struct {
	name     string
	template string
}

// wingetManifests are the three manifests a winget submission carries, in komac's file naming:
// the version manifest, the installer manifest and the en-US default-locale manifest.
var wingetManifests = []wingetManifest{
	{name: wingetPackageID + ".yaml", template: wingetVersionTemplate},
	{name: wingetPackageID + ".installer.yaml", template: wingetInstallerTemplate},
	{name: wingetPackageID + ".locale.en-US.yaml", template: wingetDefaultLocaleTemplate},
}

// renderWingetManifests renders the three winget manifests for version (vX.Y.Z or a bare X.Y.Z),
// reading the two Windows archive hashes from the release's SHA256SUMS body, and returns them in
// wingetManifests' order under the file names a `komac submit` directory holds them by.
func renderWingetManifests(version, sums string) ([]renderedFile, error) {
	rendered := make([]renderedFile, 0, len(wingetManifests))
	for _, manifest := range wingetManifests {
		content, err := renderPackageFile(manifest.template, version, sums)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, renderedFile{name: manifest.name, content: content})
	}
	return rendered, nil
}
