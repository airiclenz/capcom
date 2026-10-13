package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// The bucket repository and manifest path the scoop step writes to, and so the ones its probe and
// the smoke check read.
const (
	bucketRepo = "airiclenz/scoop-bucket"
	bucketPath = "bucket/capcom.json"
)

// scoopArchitectures pairs each Scoop manifest architecture key with the Windows archive's
// GOARCH whose hash it carries.
var scoopArchitectures = []struct {
	key    string
	goarch string
}{
	{key: "64bit", goarch: "amd64"},
	{key: "arm64", goarch: "arm64"},
}

// scoopManifest is what the scoop probe and the smoke check read of the bucket's manifest: the
// version it pins and the archive hash under each architecture key.
type scoopManifest struct {
	Version      string `json:"version"`
	Architecture map[string]struct {
		Hash string `json:"hash"`
	} `json:"architecture"`
}

// parseScoopManifest parses the bucket's manifest. Its error is the JSON decoder's, unwrapped:
// the probe reads it as the step pending, the smoke check as a failure.
func parseScoopManifest(raw string) (scoopManifest, error) {
	var manifest scoopManifest
	if err := json.Unmarshal([]byte(raw), &manifest); err != nil {
		return scoopManifest{}, err
	}
	return manifest, nil
}

// probeScoop reports the bucket setup-pending while its repository does not exist, and done when
// its manifest describes the version or a newer one. An empty bucket repository is pending, not
// setup-pending: the repository is the one-time setup, the manifest is what publish writes.
func probeScoop(ctx context.Context, env probeEnv, version releaseVersion) stepReport {
	hasBucket, err := env.channels.Exists(ctx, bucketRepo)
	if err != nil {
		return unknownStep(stepScoop, "could not read the %s repository: %v", bucketRepo, err)
	}
	if !hasBucket {
		return stepReport{Name: stepScoop, State: stateSetupPending, Detail: fmt.Sprintf("no %s repository yet", bucketRepo)}
	}

	raw, hasManifest, err := env.channels.ReadFile(ctx, bucketRepo, bucketPath)
	if err != nil {
		return unknownStep(stepScoop, "could not read %s from %s: %v", bucketPath, bucketRepo, err)
	}
	if !hasManifest {
		return stepReport{Name: stepScoop, State: statePending, Detail: fmt.Sprintf("%s has no %s", bucketRepo, bucketPath)}
	}
	manifest, err := parseScoopManifest(raw)
	if err != nil {
		return stepReport{Name: stepScoop, State: statePending, Detail: fmt.Sprintf("%s is not a readable manifest: %v", bucketPath, err)}
	}
	return pinnedVersionStep(stepScoop, "manifest", manifest.Version, version)
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

// renderScoopManifest renders the whole Scoop manifest for version (vX.Y.Z or a bare X.Y.Z),
// reading the two Windows archive hashes from the release's SHA256SUMS body. The result is
// checked to parse as JSON, so a template edit that breaks it fails here rather than in the
// bucket.
func renderScoopManifest(version, sums string) (string, error) {
	manifest, err := renderPackageFile(scoopTemplate, version, sums)
	if err != nil {
		return "", err
	}
	if !json.Valid([]byte(manifest)) {
		return "", fmt.Errorf("render %s: the manifest is not valid JSON", scoopTemplate)
	}
	return manifest, nil
}

// checkScoop requires the bucket's manifest to name the version and both Windows archives'
// published hashes. An unreadable manifest skips; a manifest at another version fails only when
// the version is the latest release, the only one the bucket can be expected to hold.
func (r *smokeRun) checkScoop(ctx context.Context) {
	bare := bareRelease(r.version)
	r.step("the Scoop bucket's manifest names %s and its %s hashes", r.version, checksumsFile)
	sums, err := r.publishedSums(ctx)
	if err != nil {
		r.skip("the release's %s did not download — nothing to check the bucket's hashes against", checksumsFile)
		return
	}
	raw, err := readRepoFile(ctx, r.commands, r.options.dir, bucketRepo, bucketPath)
	if err != nil {
		r.skip("the bucket's manifest %s in %s is unreadable: %v", bucketPath, bucketRepo, err)
		return
	}
	manifest, err := parseScoopManifest(raw)
	if err != nil {
		r.fail("the bucket's manifest %s is not valid JSON: %v", bucketPath, err)
		return
	}

	if manifest.Version != bare {
		r.checkStaleBucket(ctx, manifest.Version)
		return
	}
	matched := 0
	for _, architecture := range scoopArchitectures {
		name := distTarget{goos: "windows", goarch: architecture.goarch}.archiveName(bare)
		published, err := checksumFor(sums, name)
		if err != nil {
			r.fail("the release's %s lists no hash for %s: %v", checksumsFile, name, err)
			continue
		}
		listed := strings.TrimPrefix(strings.ToLower(manifest.Architecture[architecture.key].Hash), "sha256:")
		if listed != published {
			r.fail("the bucket's %s hash is %q, not %s's %s", architecture.key, listed, name, published)
			continue
		}
		matched++
	}
	if matched == len(scoopArchitectures) {
		r.pass("bucket at %s, both Windows hashes match %s", bare, checksumsFile)
	}
}

// checkStaleBucket judges a bucket manifest at another version: a failure when the version is
// the latest release, a skip otherwise, when no release is published or when the latest release
// cannot be read.
func (r *smokeRun) checkStaleBucket(ctx context.Context, bucketVersion string) {
	latest, isPublished, err := r.repo.LatestRelease(ctx)
	switch {
	case err != nil:
		r.skip("the bucket is at %q, not %s, and the latest release could not be read: %v", bucketVersion, bareRelease(r.version), err)
	case !isPublished:
		r.skip("the bucket is at %q, not %s, and %s has no published release", bucketVersion, bareRelease(r.version), capcomRepo)
	case latest.Tag == r.version.String():
		r.fail("the bucket is at %q, not the latest release %s — run publish %s", bucketVersion, bareRelease(r.version), r.version)
	default:
		r.skip("the bucket is at %q; %s is not the latest release (%s)", bucketVersion, r.version, latest.Tag)
	}
}
