package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// The repositories and paths the publish steps write to, and so the ones their probes read.
const (
	tapRepo           = "airiclenz/homebrew-tap"
	formulaPath       = "Formula/capcom.rb"
	bucketRepo        = "airiclenz/scoop-bucket"
	bucketPath        = "bucket/capcom.json"
	wingetRepo        = "microsoft/winget-pkgs"
	wingetPackageID   = "AiricLenz.Capcom"
	wingetManifestDir = "manifests/a/AiricLenz/Capcom"
)

// formulaVersionPattern finds the version a Homebrew formula pins: its `version "X.Y.Z"` line.
var formulaVersionPattern = regexp.MustCompile(`(?m)^\s*version\s+"([^"]*)"`)

// probeHomebrew reports the tap setup-pending while its repository does not exist, and done when
// its formula pins the version or a newer one. A tap without the formula is pending: the
// repository is the one-time setup, the formula is what publish writes.
func probeHomebrew(ctx context.Context, env probeEnv, version releaseVersion) stepReport {
	if _, err := env.commands.run(ctx, env.dir, nil, "gh", "api", "repos/"+tapRepo, "--jq", ".full_name"); err != nil {
		if isNotFound(err) {
			return stepReport{Name: stepHomebrew, State: stateSetupPending, Detail: fmt.Sprintf("no %s repository yet", tapRepo)}
		}
		return unknownStep(stepHomebrew, "could not read the %s repository: %v", tapRepo, err)
	}

	formula, err := readRepoFile(ctx, env.commands, env.dir, tapRepo, formulaPath)
	if err != nil {
		if isNotFound(err) {
			return stepReport{Name: stepHomebrew, State: statePending, Detail: fmt.Sprintf("%s has no %s", tapRepo, formulaPath)}
		}
		return unknownStep(stepHomebrew, "could not read %s from %s: %v", formulaPath, tapRepo, err)
	}

	fields := formulaVersionPattern.FindStringSubmatch(formula)
	if fields == nil {
		return stepReport{Name: stepHomebrew, State: statePending, Detail: fmt.Sprintf("%s pins no version", formulaPath)}
	}
	return pinnedVersionStep(stepHomebrew, "formula", fields[1], version)
}

// probeScoop reports the bucket setup-pending while its repository does not exist, and done when
// its manifest describes the version or a newer one. An empty bucket repository is pending, not
// setup-pending: the repository is the one-time setup, the manifest is what publish writes.
func probeScoop(ctx context.Context, env probeEnv, version releaseVersion) stepReport {
	if _, err := env.commands.run(ctx, env.dir, nil, "gh", "api", "repos/"+bucketRepo, "--jq", ".full_name"); err != nil {
		if isNotFound(err) {
			return stepReport{Name: stepScoop, State: stateSetupPending, Detail: fmt.Sprintf("no %s repository yet", bucketRepo)}
		}
		return unknownStep(stepScoop, "could not read the %s repository: %v", bucketRepo, err)
	}

	raw, err := readRepoFile(ctx, env.commands, env.dir, bucketRepo, bucketPath)
	if err != nil {
		if isNotFound(err) {
			return stepReport{Name: stepScoop, State: statePending, Detail: fmt.Sprintf("%s has no %s", bucketRepo, bucketPath)}
		}
		return unknownStep(stepScoop, "could not read %s from %s: %v", bucketPath, bucketRepo, err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(raw), &manifest); err != nil {
		return stepReport{Name: stepScoop, State: statePending, Detail: fmt.Sprintf("%s is not a readable manifest: %v", bucketPath, err)}
	}
	return pinnedVersionStep(stepScoop, "manifest", manifest.Version, version)
}

// probeWinget reports winget done once the version's manifest is published in winget-pkgs or an
// open pull request submits it, and pending otherwise — including while the package is not in
// winget-pkgs at all: winget needs no one-time setup, since publish submits the first manifests
// itself with `komac submit`.
func probeWinget(ctx context.Context, env probeEnv, version releaseVersion) stepReport {
	bare := bareRelease(version)
	isListed := true
	listing, err := env.commands.run(ctx, env.dir, nil, "gh", "api", "repos/"+wingetRepo+"/contents/"+wingetManifestDir)
	if err != nil {
		if !isNotFound(err) {
			return unknownStep(stepWinget, "could not read %s from %s: %v", wingetManifestDir, wingetRepo, err)
		}
		isListed = false
	}
	if isListed {
		var entries []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(listing), &entries); err != nil {
			return unknownStep(stepWinget, "could not parse the %s listing: %v", wingetManifestDir, err)
		}
		for _, entry := range entries {
			if entry.Type == "dir" && entry.Name == bare {
				return stepReport{Name: stepWinget, State: stateDone, Detail: fmt.Sprintf(
					"published: %s/%s in %s", wingetManifestDir, bare, wingetRepo)}
			}
		}
	}

	output, err := env.commands.run(ctx, env.dir, nil, "gh", "pr", "list", "--repo", wingetRepo, "--state", "open",
		"--search", wingetPackageID+" "+bare+" in:title", "--json", "number,title,url")
	if err != nil {
		return unknownStep(stepWinget, "could not list the open %s pull requests: %v", wingetRepo, err)
	}
	var pulls []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal([]byte(output), &pulls); err != nil {
		return unknownStep(stepWinget, "could not parse gh pr list output: %v", err)
	}
	for _, pull := range pulls {
		if titleNamesRelease(pull.Title, bare) {
			return stepReport{Name: stepWinget, State: stateDone, Detail: fmt.Sprintf(
				"submitted: pull request #%d %s", pull.Number, pull.URL)}
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

// probeSmoke reports the smoke test pending always: it is an action publish takes last, not a
// state any remote holds.
func probeSmoke(context.Context, probeEnv, releaseVersion) stepReport {
	return stepReport{Name: stepSmoke, State: statePending, Detail: "an action, not a remote state: publish runs it last"}
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

// readRepoFile reads one file of a GitHub repository through the contents API.
func readRepoFile(ctx context.Context, commands runner, dir, repo, path string) (string, error) {
	encoded, err := commands.run(ctx, dir, nil, "gh", "api", "repos/"+repo+"/contents/"+path, "--jq", ".content")
	if err != nil {
		return "", err
	}
	// The contents API wraps its base64 at 60 columns.
	decoded, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(encoded), ""))
	if err != nil {
		return "", fmt.Errorf("decode %s: %w", path, err)
	}
	return string(decoded), nil
}

// isNotFound reports whether a gh api failure is GitHub answering 404 — the path or repository
// does not exist, or the repository is empty — as opposed to a call that could not be made.
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "(HTTP 404)")
}
