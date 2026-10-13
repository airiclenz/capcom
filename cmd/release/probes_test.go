package main

import (
	"encoding/base64"
	"errors"
	"fmt"
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

// fakeRemote answers the channel probes' gh commands the way gh would over the tap, bucket and
// winget-pkgs repositories in a given state; capcom's own repository is a memRepo, never argv
// here. Its zero value is channels whose one-time setup is still pending.
type fakeRemote struct {
	// hasTap is whether the tap repository exists; formula is its formula, "" for none.
	hasTap  bool
	formula string
	// hasBucket is whether the bucket repository exists; bucketManifest is its manifest, ""
	// for an empty repository.
	hasBucket      bool
	bucketManifest string
	// wingetListing is the package's manifest directory listing in winget-pkgs, "" while the
	// package is not there; wingetPulls is what `gh pr list` prints, "" for no pull request.
	wingetListing string
	wingetPulls   string
	// failing names a command — "gh api repos/airiclenz/homebrew-tap" — that cannot run.
	failing string
}

// respond answers one channel probe command.
func (r fakeRemote) respond(call recordedCall) (string, error) {
	command := call.name + " " + strings.Join(call.args, " ")
	fail := func(detail string) (string, error) {
		return "", fmt.Errorf("%s: %s", command, detail)
	}
	if r.failing != "" && strings.HasPrefix(command, r.failing) {
		return fail("cannot run")
	}
	args := call.args
	switch {
	case command == "gh api repos/"+tapRepo+" --jq .full_name":
		if !r.hasTap {
			return fail(ghNotFound)
		}
		return tapRepo + "\n", nil
	case command == "gh api repos/"+tapRepo+"/contents/"+formulaPath+" --jq .content":
		if !r.hasTap || r.formula == "" {
			return fail(ghNotFound)
		}
		return contentsAPIBase64(r.formula), nil
	case command == "gh api repos/"+bucketRepo+" --jq .full_name":
		if !r.hasBucket {
			return fail(ghNotFound)
		}
		return bucketRepo + "\n", nil
	case command == "gh api repos/"+bucketRepo+"/contents/"+bucketPath+" --jq .content":
		switch {
		case !r.hasBucket:
			return fail(ghNotFound)
		case r.bucketManifest == "":
			return fail(ghEmptyRepository)
		}
		return contentsAPIBase64(r.bucketManifest), nil
	case command == "gh api repos/"+wingetRepo+"/contents/"+wingetManifestDir:
		if r.wingetListing == "" {
			return fail(ghNotFound)
		}
		return r.wingetListing, nil
	case call.name == "gh" && len(args) > 1 && args[0] == "pr" && args[1] == "list":
		if r.wingetPulls == "" {
			return "[]\n", nil
		}
		return r.wingetPulls, nil
	}
	return "", fmt.Errorf("unexpected command %s", command)
}

// runner is a fake runner over the remote alone.
func (r fakeRemote) runner() *fakeRunner {
	return &fakeRunner{respond: r.respond}
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

// wingetListing is a winget-pkgs contents listing of the package's manifest directory holding one
// directory per version.
func wingetListing(versions ...string) string {
	entries := make([]string, 0, len(versions))
	for _, version := range versions {
		entries = append(entries, fmt.Sprintf(`{"name":%q,"path":"%s/%s","type":"dir"}`, version, wingetManifestDir, version))
	}
	return "[" + strings.Join(entries, ",") + "]\n"
}

// wingetPull is `gh pr list --json number,title,url` output holding one komac pull request
// titled kind ("New version", or "New package" for the first submission).
func wingetPull(number int, kind, version string) string {
	return fmt.Sprintf(`[{"number":%d,"title":"%s: AiricLenz.Capcom version %s","url":"https://github.com/microsoft/winget-pkgs/pull/%d"}]`+"\n",
		number, kind, version, number)
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

// TestProbesReportEachState pins every probe's done, pending, setup-pending and unknown branches:
// the tag and release probes over the state a memRepo holds, the channel probes over canned
// remote answers.
func TestProbesReportEachState(t *testing.T) {
	t.Parallel()

	annotated := map[string]string{"v0.24.11": gitObjectTag}
	lightweight := map[string]string{"v0.24.11": "commit"}
	sixAssets := map[string][]string{"v0.24.11": releaseAssetsV02411[:6]}
	allAssets := map[string][]string{"v0.24.11": releaseAssetsV02411}
	draft := map[string]bool{"v0.24.11": true}
	olderFormula := strings.Replace(testFormula, `version "0.24.11"`, `version "0.24.10"`, 1)
	newerFormula := strings.Replace(testFormula, `version "0.24.11"`, `version "0.24.12"`, 1)

	cases := map[string]struct {
		probe      stepProbe
		repo       memRepo
		remote     fakeRemote
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
		"formula absent":        {probe: probeHomebrew, remote: fakeRemote{hasTap: true}, wantState: statePending, wantDetail: "has no Formula/capcom.rb"},
		"formula older":         {probe: probeHomebrew, remote: fakeRemote{hasTap: true, formula: olderFormula}, wantState: statePending, wantDetail: `formula pins "0.24.10", not 0.24.11`},
		"formula current":       {probe: probeHomebrew, remote: fakeRemote{hasTap: true, formula: testFormula}, wantState: stateDone, wantDetail: "formula pins 0.24.11"},
		"formula newer":         {probe: probeHomebrew, remote: fakeRemote{hasTap: true, formula: newerFormula}, wantState: stateDone, wantDetail: "formula pins newer 0.24.12"},
		"tap repo unreadable":   {probe: probeHomebrew, remote: fakeRemote{hasTap: true, formula: testFormula, failing: "gh api"}, wantState: stateUnknown, wantDetail: "could not read the airiclenz/homebrew-tap repository"},
		"formula unreadable": {probe: probeHomebrew, remote: fakeRemote{hasTap: true, formula: testFormula, failing: "gh api repos/airiclenz/homebrew-tap/contents"},
			wantState: stateUnknown, wantDetail: "could not read Formula/capcom.rb"},
		"bucket repo absent": {probe: probeScoop, wantState: stateSetupPending, wantDetail: "no airiclenz/scoop-bucket repository yet"},
		"bucket repo empty":  {probe: probeScoop, remote: fakeRemote{hasBucket: true}, wantState: statePending, wantDetail: "has no bucket/capcom.json"},
		"bucket older":       {probe: probeScoop, remote: fakeRemote{hasBucket: true, bucketManifest: `{"version": "0.24.10"}`}, wantState: statePending, wantDetail: `manifest pins "0.24.10", not 0.24.11`},
		"bucket current":     {probe: probeScoop, remote: fakeRemote{hasBucket: true, bucketManifest: `{"version": "0.24.11"}`}, wantState: stateDone, wantDetail: "manifest pins 0.24.11"},
		"bucket newer":       {probe: probeScoop, remote: fakeRemote{hasBucket: true, bucketManifest: `{"version": "0.24.12"}`}, wantState: stateDone, wantDetail: "manifest pins newer 0.24.12"},
		"bucket unreadable":  {probe: probeScoop, remote: fakeRemote{hasBucket: true, failing: "gh api"}, wantState: stateUnknown, wantDetail: "could not read the airiclenz/scoop-bucket repository"},
		"winget not set up": {probe: probeWinget, wantState: statePending,
			wantDetail: "AiricLenz.Capcom is not in microsoft/winget-pkgs yet and no open pull request submits 0.24.11"},
		"winget first submission open": {probe: probeWinget, remote: fakeRemote{wingetPulls: wingetPull(312345, "New package", "0.24.11")},
			wantState: stateDone, wantDetail: "submitted: pull request #312345"},
		"winget absent": {probe: probeWinget, remote: fakeRemote{wingetListing: wingetListing("0.24.10")},
			wantState: statePending, wantDetail: "no AiricLenz.Capcom 0.24.11 manifest or open pull request"},
		"winget pull request open": {probe: probeWinget, remote: fakeRemote{wingetListing: wingetListing("0.24.10"), wingetPulls: wingetPull(312345, "New version", "0.24.11")},
			wantState: stateDone, wantDetail: "submitted: pull request #312345"},
		"winget merged": {probe: probeWinget, remote: fakeRemote{wingetListing: wingetListing("0.24.10", "0.24.11")},
			wantState: stateDone, wantDetail: "published: manifests/a/AiricLenz/Capcom/0.24.11"},
		"winget pull request for a longer version": {probe: probeWinget, version: "v0.24.1",
			remote:    fakeRemote{wingetListing: wingetListing("0.24.0"), wingetPulls: wingetPull(312345, "New version", "0.24.11")},
			wantState: statePending, wantDetail: "no AiricLenz.Capcom 0.24.1 manifest"},
		"winget listing unparsable": {probe: probeWinget, remote: fakeRemote{wingetListing: "not json"},
			wantState: stateUnknown, wantDetail: "could not parse the manifests/a/AiricLenz/Capcom listing: invalid character"},
		"winget pull requests unparsable": {probe: probeWinget, remote: fakeRemote{wingetListing: wingetListing("0.24.10"), wingetPulls: "not json"},
			wantState: stateUnknown, wantDetail: "could not parse gh pr list output: invalid character"},
		"winget pull requests unreadable": {probe: probeWinget, remote: fakeRemote{wingetListing: wingetListing("0.24.10"), failing: "gh pr list"},
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

			env := probeEnv{repo: tc.repo, channels: execChannelRepos{commands: tc.remote.runner(), dir: "."}}

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

	env := probeEnv{repo: memRepo{}, channels: execChannelRepos{commands: fakeRemote{}.runner(), dir: "."}}

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

// TestProbesNeverReadAFailedCallAsDone pins that a probe whose every command cannot run — gh
// missing, git unauthenticated — reports unknown, never done (smoke, which runs nothing, aside):
// capcom's own repository is the production execRepo over the same failing runner.
func TestProbesNeverReadAFailedCallAsDone(t *testing.T) {
	t.Parallel()
	commands := &fakeRunner{respond: func(recordedCall) (string, error) {
		return "", errors.New(`exec: "gh": executable file not found in $PATH`)
	}}

	env := probeEnv{repo: execRepo{commands: commands, dir: "."}, channels: execChannelRepos{commands: commands, dir: "."}}

	for _, step := range probeSteps(t.Context(), env, mustVersion(t, "v0.24.11")) {
		if step.Name == stepSmoke {
			continue
		}
		if step.State != stateUnknown {
			t.Errorf("step %s = %s %q, want unknown", step.Name, step.State, step.Detail)
		}
	}
}
