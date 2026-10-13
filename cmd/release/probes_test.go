package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
)

// The errors gh fails with, copied from real runs: a GitHub 404 (a missing path or repository),
// an empty repository's contents, and a release that does not exist.
const (
	ghNotFound        = "exit status 1: gh: Not Found (HTTP 404)"
	ghEmptyRepository = "exit status 1: gh: This repository is empty. (HTTP 404)"
	ghNoRelease       = "exit status 1: release not found"
)

// testFormula is the head of a capcom formula in the tap, in the shape every formula in
// airiclenz/homebrew-tap takes: the version is pinned on its own `version` line.
const testFormula = `class Capcom < Formula
  desc "Git-native issue tracker and knowledge store for coding agents"
  homepage "https://github.com/airiclenz/capcom"
  version "0.24.11"
  license "MIT"
`

// releaseAssetsV02411 are the assets a v0.24.11 release carries, written out by hand: the six
// archives the channels install from, and SHA256SUMS.
var releaseAssetsV02411 = []string{
	"capcom_0.24.11_darwin_amd64.tar.gz",
	"capcom_0.24.11_darwin_arm64.tar.gz",
	"capcom_0.24.11_linux_amd64.tar.gz",
	"capcom_0.24.11_linux_arm64.tar.gz",
	"capcom_0.24.11_windows_amd64.zip",
	"capcom_0.24.11_windows_arm64.zip",
	"SHA256SUMS",
}

// contentsAPIBase64 encodes content the way `gh api .../contents/<path> --jq .content` prints it:
// base64 wrapped at 60 columns.
func contentsAPIBase64(content string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	var lines []string
	for len(encoded) > 60 {
		lines = append(lines, encoded[:60])
		encoded = encoded[60:]
	}
	return strings.Join(append(lines, encoded), "\n") + "\n"
}

// wingetPull is `gh pr list --json number,title,url` output holding one komac pull request
// titled kind ("New version", or "New package" for the first submission).
func wingetPull(number int, kind, version string) string {
	return fmt.Sprintf(`[{"number":%d,"title":"%s: AiricLenz.Capcom version %s","url":"https://github.com/microsoft/winget-pkgs/pull/%d"}]`+"\n",
		number, kind, version, number)
}

// wingetViewerPull is the `gh api graphql` answer listing the signed-in account's open pull
// requests: one komac pull request titled kind against repo, or none when number is 0.
func wingetViewerPull(number int, kind, version, repo string) string {
	nodes := ""
	if number != 0 {
		nodes = fmt.Sprintf(`{"number":%d,"title":"%s: AiricLenz.Capcom version %s","url":"https://github.com/microsoft/winget-pkgs/pull/%d",`+
			`"repository":{"nameWithOwner":%q}}`, number, kind, version, number, repo)
	}
	return `{"data":{"viewer":{"pullRequests":{"nodes":[` + nodes + "]}}}}\n"
}

// mustVersion parses a release version or fails the test.
func mustVersion(t *testing.T, text string) releaseVersion {
	t.Helper()
	version, err := parseReleaseVersion(text)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

// TestReleaseAssetNamesMatchAPublishedRelease pins the seven assets the release probe wants
// against the ones a real release carries.
func TestReleaseAssetNamesMatchAPublishedRelease(t *testing.T) {
	t.Parallel()

	got := releaseAssetNames(mustVersion(t, "v0.24.11"))

	if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(releaseAssetsV02411))) {
		t.Errorf("release asset names = %q, want %q", got, releaseAssetsV02411)
	}
}

// tapHolding is channel repositories where the tap holds formula as its formula, and no formula
// when it is "".
func tapHolding(formula string) memChannelRepos {
	return memChannelRepos{repos: map[string]memChannelRepo{tapRepo: {files: fileHolding(formulaPath, formula)}}}
}

// bucketHolding is channel repositories where the bucket holds manifest as its manifest, and is
// an empty repository when it is "".
func bucketHolding(manifest string) memChannelRepos {
	return memChannelRepos{repos: map[string]memChannelRepo{bucketRepo: {files: fileHolding(bucketPath, manifest)}}}
}

// wingetHolding is channel repositories where winget-pkgs holds one manifest directory per
// version — none of the package's at all when versions is nil — and the open pull requests pulls.
func wingetHolding(versions []string, pulls ...pullRequest) memChannelRepos {
	winget := memChannelRepo{pulls: pulls}
	if versions != nil {
		winget.dirs = map[string][]string{wingetManifestDir: versions}
	}
	return memChannelRepos{repos: map[string]memChannelRepo{wingetRepo: winget}}
}

// channelReposOf is channel repositories holding every repository the parts hold — a later
// part's repository replacing an earlier part's of the same name — and failing as each part
// fails; it records nothing.
func channelReposOf(parts ...memChannelRepos) memChannelRepos {
	channels := memChannelRepos{repos: map[string]memChannelRepo{}}
	for _, part := range parts {
		maps.Copy(channels.repos, part.repos)
		for call, err := range part.failures {
			channels = channels.withFailure(call.method, call.repo, err)
		}
	}
	return channels
}

// fileHolding is a repository's files holding content at path, or no file when content is "".
func fileHolding(path, content string) map[string]string {
	if content == "" {
		return nil
	}
	return map[string]string{path: content}
}

// komacPull is an open komac pull request titled kind ("New version", or "New package" for the
// first submission) for version.
func komacPull(number int, kind, version string) pullRequest {
	return pullRequest{number: number, title: fmt.Sprintf("%s: AiricLenz.Capcom version %s", kind, version),
		url: fmt.Sprintf("https://github.com/microsoft/winget-pkgs/pull/%d", number)}
}

// unparsable is a channel repository answer the adapter could not parse, as a listing that is not
// JSON gives.
var unparsable = unparsableAnswerError{cause: errors.New("invalid character 'o' in literal null (expecting 'u')")}

// TestProbesReportEachState pins every probe's done, pending, setup-pending and unknown branches:
// the tag and release probes over the state a memRepo holds, the channel probes over the state a
// memChannelRepos holds.
func TestProbesReportEachState(t *testing.T) {
	t.Parallel()

	annotated := map[string]string{"v0.24.11": gitObjectTag}
	lightweight := map[string]string{"v0.24.11": "commit"}
	sixAssets := map[string][]string{"v0.24.11": releaseAssetsV02411[:6]}
	allAssets := map[string][]string{"v0.24.11": releaseAssetsV02411}
	draft := map[string]bool{"v0.24.11": true}
	olderFormula := strings.Replace(testFormula, `version "0.24.11"`, `version "0.24.10"`, 1)
	newerFormula := strings.Replace(testFormula, `version "0.24.11"`, `version "0.24.12"`, 1)
	cannotRun := errors.New("cannot run")

	cases := map[string]struct {
		probe      stepProbe
		repo       memRepo
		channels   memChannelRepos
		version    string
		wantState  stepState
		wantDetail string
	}{
		"tag absent":            {probe: probeTag, wantState: statePending, wantDetail: "origin has no tag v0.24.11"},
		"tag annotated":         {probe: probeTag, repo: memRepo{remoteTags: annotated}, wantState: stateDone, wantDetail: "annotated tag v0.24.11"},
		"tag lightweight":       {probe: probeTag, repo: memRepo{remoteTags: lightweight}, wantState: stateUnknown, wantDetail: "lightweight tag"},
		"tag listing fails":     {probe: probeTag, repo: memRepo{remoteTags: annotated, failing: methodRemoteHasTag}, wantState: stateUnknown, wantDetail: "could not list origin's tags"},
		"tag type unreadable":   {probe: probeTag, repo: memRepo{remoteTags: annotated, failing: methodRemoteTagType}, wantState: stateUnknown, wantDetail: "could not read the type of tag v0.24.11"},
		"tag of another object": {probe: probeTag, repo: memRepo{remoteTags: map[string]string{"v0.24.11": "tree"}}, wantState: stateUnknown, wantDetail: `points at a "tree" object`},
		"release absent":        {probe: probeRelease, wantState: statePending, wantDetail: "no GitHub release v0.24.11"},
		"release 6 of 7 assets": {probe: probeRelease, repo: memRepo{releases: sixAssets}, wantState: statePending, wantDetail: "lacks 1 of its 7 assets: SHA256SUMS"},
		"release complete":      {probe: probeRelease, repo: memRepo{releases: allAssets}, wantState: stateDone, wantDetail: "all 7 assets"},
		"release unreadable":    {probe: probeRelease, repo: memRepo{releases: allAssets, failing: methodReleaseAssets}, wantState: stateUnknown, wantDetail: "could not read GitHub release v0.24.11"},
		"tap repo absent":       {probe: probeHomebrew, wantState: stateSetupPending, wantDetail: "no airiclenz/homebrew-tap repository yet"},
		"formula absent":        {probe: probeHomebrew, channels: tapHolding(""), wantState: statePending, wantDetail: "has no Formula/capcom.rb"},
		"formula older":         {probe: probeHomebrew, channels: tapHolding(olderFormula), wantState: statePending, wantDetail: `formula pins "0.24.10", not 0.24.11`},
		"formula current":       {probe: probeHomebrew, channels: tapHolding(testFormula), wantState: stateDone, wantDetail: "formula pins 0.24.11"},
		"formula newer":         {probe: probeHomebrew, channels: tapHolding(newerFormula), wantState: stateDone, wantDetail: "formula pins newer 0.24.12"},
		"tap repo unreadable": {probe: probeHomebrew, channels: tapHolding(testFormula).withFailure(methodExists, tapRepo, cannotRun),
			wantState: stateUnknown, wantDetail: "could not read the airiclenz/homebrew-tap repository"},
		"formula unreadable": {probe: probeHomebrew, channels: tapHolding(testFormula).withFailure(methodReadFile, tapRepo, cannotRun),
			wantState: stateUnknown, wantDetail: "could not read Formula/capcom.rb"},
		"bucket repo absent": {probe: probeScoop, wantState: stateSetupPending, wantDetail: "no airiclenz/scoop-bucket repository yet"},
		"bucket repo empty":  {probe: probeScoop, channels: bucketHolding(""), wantState: statePending, wantDetail: "has no bucket/capcom.json"},
		"bucket older":       {probe: probeScoop, channels: bucketHolding(`{"version": "0.24.10"}`), wantState: statePending, wantDetail: `manifest pins "0.24.10", not 0.24.11`},
		"bucket current":     {probe: probeScoop, channels: bucketHolding(`{"version": "0.24.11"}`), wantState: stateDone, wantDetail: "manifest pins 0.24.11"},
		"bucket newer":       {probe: probeScoop, channels: bucketHolding(`{"version": "0.24.12"}`), wantState: stateDone, wantDetail: "manifest pins newer 0.24.12"},
		"bucket unreadable": {probe: probeScoop, channels: bucketHolding("").withFailure(methodExists, bucketRepo, cannotRun),
			wantState: stateUnknown, wantDetail: "could not read the airiclenz/scoop-bucket repository"},
		"winget not set up": {probe: probeWinget, wantState: statePending,
			wantDetail: "AiricLenz.Capcom is not in microsoft/winget-pkgs yet and no open pull request submits 0.24.11"},
		"winget first submission open": {probe: probeWinget, channels: wingetHolding(nil, komacPull(312345, "New package", "0.24.11")),
			wantState: stateDone, wantDetail: "submitted: pull request #312345 https://github.com/microsoft/winget-pkgs/pull/312345"},
		"winget absent": {probe: probeWinget, channels: wingetHolding([]string{"0.24.10"}),
			wantState: statePending, wantDetail: "no AiricLenz.Capcom 0.24.11 manifest or open pull request"},
		"winget pull request open": {probe: probeWinget, channels: wingetHolding([]string{"0.24.10"}, komacPull(312345, "New version", "0.24.11")),
			wantState: stateDone, wantDetail: "submitted: pull request #312345"},
		"winget pull request for another version": {probe: probeWinget, channels: wingetHolding([]string{"0.24.10"}, komacPull(312345, "New version", "0.24.10")),
			wantState: statePending, wantDetail: "no AiricLenz.Capcom 0.24.11 manifest or open pull request"},
		"winget merged": {probe: probeWinget, channels: wingetHolding([]string{"0.24.10", "0.24.11"}),
			wantState: stateDone, wantDetail: "published: manifests/a/AiricLenz/Capcom/0.24.11"},
		"winget pull request for a longer version": {probe: probeWinget, version: "v0.24.1",
			channels:  wingetHolding([]string{"0.24.0"}, komacPull(312345, "New version", "0.24.11")),
			wantState: statePending, wantDetail: "no AiricLenz.Capcom 0.24.1 manifest"},
		"winget listing unreadable": {probe: probeWinget, channels: wingetHolding(nil).withFailure(methodListDirs, wingetRepo, cannotRun),
			wantState: stateUnknown, wantDetail: "could not read manifests/a/AiricLenz/Capcom from microsoft/winget-pkgs"},
		"winget listing unparsable": {probe: probeWinget, channels: wingetHolding(nil).withFailure(methodListDirs, wingetRepo, unparsable),
			wantState: stateUnknown, wantDetail: "could not parse the manifests/a/AiricLenz/Capcom listing: invalid character"},
		"winget pull requests unparsable": {probe: probeWinget, channels: wingetHolding([]string{"0.24.10"}).withFailure(methodOpenPulls, wingetRepo, unparsable),
			wantState: stateUnknown, wantDetail: "could not parse gh pr list output: invalid character"},
		"winget pull requests unreadable": {probe: probeWinget, channels: wingetHolding([]string{"0.24.10"}).withFailure(methodOpenPulls, wingetRepo, cannotRun),
			wantState: stateUnknown, wantDetail: "could not list the open microsoft/winget-pkgs pull requests"},
		"smoke": {probe: probeSmoke, repo: memRepo{remoteTags: annotated, releases: allAssets}, wantState: statePending, wantDetail: "publish runs it last"},
		// A draft an interrupted `gh release create` left is never done: its download URLs 404.
		"release draft with all assets": {probe: probeRelease, repo: memRepo{releases: allAssets, drafts: draft},
			wantState: statePending, wantDetail: "release v0.24.11 is a draft with all 7 assets"},
		"release draft lacking assets": {probe: probeRelease, repo: memRepo{releases: sixAssets, drafts: draft},
			wantState: statePending, wantDetail: "release v0.24.11 is a draft and lacks 1 of its 7 assets: SHA256SUMS"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			version := tc.version
			if version == "" {
				version = "v0.24.11"
			}

			env := probeEnv{repo: tc.repo, channels: tc.channels}

			step := tc.probe(t.Context(), env, mustVersion(t, version))

			if step.State != tc.wantState || !strings.Contains(step.Detail, tc.wantDetail) {
				t.Errorf("step = %s %q, want %s containing %q", step.State, step.Detail, tc.wantState, tc.wantDetail)
			}
		})
	}
}

// TestProbeStepsRunsEveryStepInPublishOrder pins the step list status reports and publish walks.
func TestProbeStepsRunsEveryStepInPublishOrder(t *testing.T) {
	t.Parallel()

	env := probeEnv{repo: memRepo{}, channels: memChannelRepos{}}

	steps := probeSteps(t.Context(), env, mustVersion(t, "v0.24.14"))

	var names []string
	for _, step := range steps {
		names = append(names, step.Name)
	}
	want := []string{"tag", "release", "homebrew", "scoop", "winget", "smoke"}
	if !slices.Equal(names, want) {
		t.Errorf("steps = %q, want %q", names, want)
	}
}

// failingChannels is channel repositories every one of whose methods fails with err on the tap,
// the bucket and winget-pkgs alike.
func failingChannels(err error) memChannelRepos {
	var channels memChannelRepos
	for _, repo := range []string{tapRepo, bucketRepo, wingetRepo} {
		for _, method := range []string{methodExists, methodReadFile, methodListDirs, methodOpenPulls, methodPublishFile} {
			channels = channels.withFailure(method, repo, err)
		}
	}
	return channels
}

// TestProbesNeverReadAFailedCallAsDone pins that a probe whose every call cannot run — gh
// missing, git unauthenticated — reports unknown, never done (smoke, which runs nothing, aside):
// capcom's own repository is the production execRepo over a runner failing every command, the
// channel repositories a memChannelRepos failing every method with the same error.
func TestProbesNeverReadAFailedCallAsDone(t *testing.T) {
	t.Parallel()
	ghMissing := errors.New(`exec: "gh": executable file not found in $PATH`)
	commands := &fakeRunner{respond: func(recordedCall) (string, error) {
		return "", ghMissing
	}}

	env := probeEnv{repo: execRepo{commands: commands, dir: "."}, channels: failingChannels(ghMissing)}

	for _, step := range probeSteps(t.Context(), env, mustVersion(t, "v0.24.11")) {
		if step.Name == stepSmoke {
			continue
		}
		if step.State != stateUnknown {
			t.Errorf("step %s = %s %q, want unknown", step.Name, step.State, step.Detail)
		}
	}
}
