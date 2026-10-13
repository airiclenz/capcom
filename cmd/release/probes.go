package main

import (
	"context"
	"encoding/json"
	"fmt"
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
