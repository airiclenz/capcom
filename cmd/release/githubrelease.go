package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// probeRelease reports the GitHub release done only when it is published, not a draft, and
// carries every asset a release ships: the six archives and SHA256SUMS, each fully uploaded. A
// draft's download URLs answer 404, so the channel steps must not run over one.
func probeRelease(ctx context.Context, env probeEnv, version releaseVersion) stepReport {
	tag := version.String()
	release, isFound, err := env.repo.ReleaseAssets(ctx, tag)
	if err != nil {
		return unknownStep(stepRelease, "could not read GitHub release %s: %v", tag, err)
	}
	if !isFound {
		return stepReport{Name: stepRelease, State: statePending, Detail: fmt.Sprintf("no GitHub release %s", tag)}
	}

	want := releaseAssetNames(version)
	missing := missingAssets(want, release.names)
	switch {
	case release.isDraft && len(missing) > 0:
		return stepReport{Name: stepRelease, State: statePending, Detail: fmt.Sprintf(
			"release %s is a draft and lacks %d of its %d assets: %s", tag, len(missing), len(want), strings.Join(missing, ", "))}
	case release.isDraft:
		return stepReport{Name: stepRelease, State: statePending, Detail: fmt.Sprintf(
			"release %s is a draft with all %d assets", tag, len(want))}
	case len(missing) > 0:
		return stepReport{Name: stepRelease, State: statePending, Detail: fmt.Sprintf(
			"release %s lacks %d of its %d assets: %s", tag, len(missing), len(want), strings.Join(missing, ", "))}
	}
	return stepReport{Name: stepRelease, State: stateDone, Detail: fmt.Sprintf("release %s with all %d assets", tag, len(want))}
}

// describeRelease says what the release step does.
func describeRelease(run *publishRun) string {
	return fmt.Sprintf("build the %d assets in a fresh clone at %s, then create GitHub release %s or upload the assets it lacks and publish it if it is a draft",
		len(releaseAssetNames(run.version)), run.version, run.version)
}

// convergeRelease builds the release's assets from the tag and creates the GitHub release, or,
// when the release exists, uploads only the assets it lacks — after checking the rebuild against
// what the release already carries, so a rebuild never mixes with assets it does not match. A
// release with SHA256SUMS must list the rebuilt hash of every archive, uploaded or about to be;
// a release without it must carry only archives that hash to the rebuilt ones before the
// rebuild's own sums are uploaded beside them. A draft release is published once it carries
// every asset.
func convergeRelease(ctx context.Context, run *publishRun) error {
	tag := run.version.String()
	release, isFound, err := run.repo.ReleaseAssets(ctx, tag)
	if err != nil {
		return fmt.Errorf("read GitHub release %s: %w", tag, err)
	}
	uploaded := release.names

	workDir, err := os.MkdirTemp("", "capcom-publish-")
	if err != nil {
		return fmt.Errorf("create the build directory: %w", err)
	}
	// The build outlives this step: the smoke step compares the published sums against it, and
	// the run removes it when it ends.
	run.buildDir = workDir
	outDir, err := buildReleaseAssets(ctx, run, workDir)
	if err != nil {
		return err
	}
	run.distDir = outDir

	if !isFound {
		assets := assetPaths(outDir, releaseAssetNames(run.version))
		if err := run.repo.CreateRelease(ctx, tag, run.notesPath, assets); err != nil {
			return fmt.Errorf("create GitHub release %s: %w", tag, err)
		}
		return nil
	}

	if slices.Contains(uploaded, checksumsFile) {
		if err := checkPublishedSums(ctx, run, outDir, uploaded); err != nil {
			return err
		}
	} else if err := checkUploadedArchives(ctx, run, outDir, uploaded); err != nil {
		return err
	}
	missing := missingAssets(releaseAssetNames(run.version), uploaded)
	// UploadAssets replaces an asset of the same name, which here is only one left in a failed
	// upload state: an uploaded one is never in missing.
	if len(missing) > 0 {
		if err := run.repo.UploadAssets(ctx, tag, assetPaths(outDir, missing)); err != nil {
			return fmt.Errorf("upload %s to GitHub release %s: %w", strings.Join(missing, ", "), tag, err)
		}
	}
	// A draft — one an interrupted `gh release create` left — is published only once it carries
	// every asset, so no channel ever points at a download that is not there.
	if release.isDraft {
		if err := run.repo.UndraftRelease(ctx, tag); err != nil {
			return fmt.Errorf("publish draft GitHub release %s: %w", tag, err)
		}
	}
	return nil
}

// buildReleaseAssets clones the checkout fresh into workDir at the tag and runs dist over the
// clone — never over the current checkout, whatever it holds. It returns the directory holding
// the assets.
func buildReleaseAssets(ctx context.Context, run *publishRun, workDir string) (string, error) {
	tag := run.version.String()
	sourceDir := filepath.Join(workDir, "src")
	if err := run.repo.CloneAt(ctx, tag, sourceDir); err != nil {
		return "", err
	}
	bare, err := readBareVersion(sourceDir)
	if err != nil {
		return "", err
	}
	if bare != bareRelease(run.version) {
		return "", fmt.Errorf("%s at tag %s reads %s, not %s", versionFile, tag, bare, bareRelease(run.version))
	}

	outDir := filepath.Join(workDir, defaultOutDir)
	if err := runDist(ctx, run.commands, run.output, distOptions{sourceDir: sourceDir, outDir: outDir}); err != nil {
		return "", fmt.Errorf("build the %s assets: %w", tag, err)
	}
	return outDir, nil
}

// checkPublishedSums downloads the release's SHA256SUMS and requires the published hash of every
// archive, uploaded or about to be, to equal the rebuilt one, naming the way out when one does
// not: removing SHA256SUMS and the uploaded archives it vouches for wrongly.
func checkPublishedSums(ctx context.Context, run *publishRun, outDir string, uploaded []string) error {
	tag := run.version.String()
	published, err := downloadReleaseSums(ctx, run)
	if err != nil {
		return err
	}
	rebuilt, err := readRebuiltSums(outDir)
	if err != nil {
		return err
	}

	var mismatched []string
	stale := []string{checksumsFile}
	for _, name := range releaseAssetNames(run.version) {
		if name == checksumsFile {
			continue
		}
		want, err := checksumFor(rebuilt, name)
		if err != nil {
			return fmt.Errorf("read the rebuilt %s: %w", checksumsFile, err)
		}
		if got, err := checksumFor(published, name); err != nil || got != want {
			mismatched = append(mismatched, name)
			if slices.Contains(uploaded, name) {
				stale = append(stale, name)
			}
		}
	}
	if len(mismatched) > 0 {
		return fmt.Errorf("the rebuilt %s does not match release %s's %s; remove the stale assets with "+
			"`gh release delete-asset %s <name>` (%s), then run publish again",
			strings.Join(mismatched, ", "), tag, checksumsFile, tag, strings.Join(stale, ", "))
	}
	return nil
}

// checkUploadedArchives downloads every archive the release already carries into the build
// directory and requires each to hash to the rebuilt one, naming the archives to remove when one
// does not. It is the check for a release without SHA256SUMS, whose rebuilt sums are about to be
// uploaded beside those archives.
func checkUploadedArchives(ctx context.Context, run *publishRun, outDir string, uploaded []string) error {
	tag := run.version.String()
	var archives []string
	for _, name := range releaseAssetNames(run.version) {
		if name != checksumsFile && slices.Contains(uploaded, name) {
			archives = append(archives, name)
		}
	}
	if len(archives) == 0 {
		return nil
	}
	rebuilt, err := readRebuiltSums(outDir)
	if err != nil {
		return err
	}
	downloadDir := filepath.Join(run.buildDir, "uploaded")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return fmt.Errorf("create the download directory: %w", err)
	}

	var mismatched []string
	for _, name := range archives {
		want, err := checksumFor(rebuilt, name)
		if err != nil {
			return fmt.Errorf("read the rebuilt %s: %w", checksumsFile, err)
		}
		if err := run.repo.DownloadAsset(ctx, tag, name, downloadDir); err != nil {
			return fmt.Errorf("download %s from GitHub release %s: %w", name, tag, err)
		}
		got, err := fileSHA256(filepath.Join(downloadDir, name))
		if err != nil {
			return err
		}
		if got != want {
			mismatched = append(mismatched, name)
		}
	}
	if len(mismatched) > 0 {
		return fmt.Errorf("release %s's uploaded %s does not match the rebuilt one; remove it with "+
			"`gh release delete-asset %s <name>` (%s), then run publish again",
			tag, strings.Join(mismatched, ", "), tag, strings.Join(mismatched, ", "))
	}
	return nil
}

// readRebuiltSums reads the SHA256SUMS the rebuild wrote into outDir.
func readRebuiltSums(outDir string) (string, error) {
	rebuilt, err := os.ReadFile(filepath.Join(outDir, checksumsFile))
	if err != nil {
		return "", fmt.Errorf("read the rebuilt %s: %w", checksumsFile, err)
	}
	return string(rebuilt), nil
}

// missingAssets returns the names in want that uploaded lacks, in want's order.
func missingAssets(want, uploaded []string) []string {
	var missing []string
	for _, name := range want {
		if !slices.Contains(uploaded, name) {
			missing = append(missing, name)
		}
	}
	return missing
}

// releaseAssetNames are the files a release of version carries: one archive per release
// platform, then SHA256SUMS.
func releaseAssetNames(version releaseVersion) []string {
	bare := bareRelease(version)
	names := make([]string, 0, len(distTargets)+1)
	for _, target := range distTargets {
		names = append(names, target.archiveName(bare))
	}
	return append(names, checksumsFile)
}

// assetPaths joins each asset name onto outDir.
func assetPaths(outDir string, names []string) []string {
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, filepath.Join(outDir, name))
	}
	return paths
}

// downloadReleaseSums reads the release's SHA256SUMS from GitHub, writing nothing into the
// checkout, whose clean tree the next status or publish gate checks.
func downloadReleaseSums(ctx context.Context, run *publishRun) (string, error) {
	tag := run.version.String()
	sums, err := run.repo.ReadAsset(ctx, tag, checksumsFile)
	if err != nil {
		return "", fmt.Errorf("download %s from GitHub release %s: %w", checksumsFile, tag, err)
	}
	return sums, nil
}
