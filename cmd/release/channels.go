package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

// describeScoop says what the scoop step does.
func describeScoop(run *publishRun) string {
	return describePackageFile(run, bucketRepo, bucketPath)
}

// convergeScoop renders the manifest from the release's SHA256SUMS and commits it to the bucket.
func convergeScoop(ctx context.Context, run *publishRun) error {
	sums, err := downloadReleaseSums(ctx, run)
	if err != nil {
		return err
	}
	manifest, err := renderScoopManifest(run.version.String(), sums)
	if err != nil {
		return err
	}
	return publishPackageFile(ctx, run, bucketRepo, bucketPath, manifest)
}

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

// setupPendingError is a step's converge reporting, without failing, that the step cannot finish
// until a human does a one-time setup: publish records the step setup-pending with detail and
// goes on to the next step.
type setupPendingError struct {
	detail string
}

func (e setupPendingError) Error() string { return e.detail }

// resolveWingetPackage records in the run whether winget-pkgs already has the package, which picks
// the winget step's komac path. It asks only while the winget step is pending: a done or unknown
// step never reaches the winget step's converge. It reads the listing the winget probe reads; a
// 404 means the package is not there yet.
func (r *publishRun) resolveWingetPackage(ctx context.Context, steps []stepReport) error {
	if stepStateOf(steps, stepWinget) != statePending {
		return nil
	}
	_, err := r.commands.run(ctx, r.options.dir, nil, "gh", "api", "repos/"+wingetRepo+"/contents/"+wingetManifestDir)
	switch {
	case err == nil:
		r.isWingetPackageListed = true
	case !isNotFound(err):
		return fmt.Errorf("could not tell whether %s has %s: %w", wingetRepo, wingetManifestDir, err)
	}
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
