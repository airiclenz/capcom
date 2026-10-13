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
	"testing"
)

// The token the winget tests hand publish, the release URLs komac is given for v0.24.11, and
// the manifest directory a first submission renders under its temporary directory.
const (
	sentinelToken     = "sentinel-token-must-not-print"
	testReleaseBase   = "https://github.com/airiclenz/capcom/releases/download/v0.24.11/"
	testWingetCommand = "GITHUB_TOKEN=<redacted> komac update AiricLenz.Capcom --version 0.24.11 --urls " +
		testReleaseBase + "capcom_0.24.11_windows_amd64.zip " + testReleaseBase + "capcom_0.24.11_windows_arm64.zip --submit"
	testManifestSubdir = "manifests/a/AiricLenz/Capcom/0.24.11"
)

// testWingetManifests are the file names a first winget submission's directory holds.
var testWingetManifests = []string{
	"AiricLenz.Capcom.installer.yaml",
	"AiricLenz.Capcom.locale.en-US.yaml",
	"AiricLenz.Capcom.yaml",
}

// fakeChannels is what the winget step meets beyond winget-pkgs: whether komac is installed,
// whether `komac submit` or `komac update` fails, and the token gh is signed in with ("" for
// none).
type fakeChannels struct {
	isKomacMissing       bool
	isKomacSubmitFailing bool
	isKomacUpdateFailing bool
	ghToken              string
	// submitted lists the files the manifest directory held when `komac submit` ran.
	submitted []string
}

// channelFile is the generated file publish writes in each channel repository.
var channelFile = map[string]string{tapRepo: formulaPath, bucketRepo: bucketPath}

// respondTools answers the commands the channel steps run beside the channel repositories: komac
// and `gh auth token`. Any other command is unexpected.
func (f *publishFake) respondTools(call recordedCall) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	args := call.args
	command := call.name + " " + strings.Join(args, " ")
	switch {
	case command == "komac --version":
		if f.channels.isKomacMissing {
			return "", fmt.Errorf("komac --version: %w", &exec.Error{Name: "komac", Err: exec.ErrNotFound})
		}
		return "komac 2.11.2\n", nil
	case call.name == "komac" && len(args) == 3 && args[0] == "submit":
		return f.submitManifests(call)
	case call.name == "komac" && len(args) > 0 && args[0] == "update" && f.channels.isKomacUpdateFailing:
		return "", komacCredentialError(call)
	case call.name == "komac":
		return "", nil
	case command == "gh auth token":
		if f.channels.ghToken == "" {
			return "", fmt.Errorf("gh auth token: exit status 1: no oauth token found for github.com")
		}
		return f.channels.ghToken + "\n", nil
	}
	return "", fmt.Errorf("unexpected command %s", command)
}

// submitManifests answers `komac submit <dir> --yes`: it records the files the directory holds,
// and when the submission is set to fail, fails echoing the token from komac's environment, as
// a credential error might.
func (f *publishFake) submitManifests(call recordedCall) (string, error) {
	entries, err := os.ReadDir(call.args[1])
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		f.channels.submitted = append(f.channels.submitted, entry.Name())
	}
	if !f.channels.isKomacSubmitFailing {
		return "", nil
	}
	return "", komacCredentialError(call)
}

// komacCredentialError is a failing komac call's error echoing the token from its environment,
// as a credential error might.
func komacCredentialError(call recordedCall) error {
	var token string
	for _, variable := range call.env {
		if value, isToken := strings.CutPrefix(variable, githubTokenVariable+"="); isToken {
			token = value
		}
	}
	return fmt.Errorf("komac %s: exit status 1: bad credentials %s", strings.Join(call.args, " "), token)
}

// testReleaseSums is a SHA256SUMS body listing every archive of v0.24.11, one fixture hash each.
func testReleaseSums() string {
	var sums strings.Builder
	for index, name := range releaseAssetsV02411[:len(distTargets)] {
		sums.WriteString(strings.Repeat(fmt.Sprintf("%x", index+10), 64) + "  " + name + "\n")
	}
	return sums.String()
}

// releasedFake is a resumed run of publishVersion: the tag and the whole release are on GitHub,
// the tap's formula and the bucket's manifest pin the previous release, and winget-pkgs has the
// package but no manifest or pull request for the version — so every channel step is pending.
func releasedFake() *publishFake {
	fake := newPublishFake()
	fake.tagged()
	fake.repo.releases[publishVersion] = releaseAssetsV02411
	fake.remote = channelReposOf(
		tapHolding(strings.Replace(testFormula, "0.24.11", "0.24.10", 1)),
		bucketHolding(`{"version": "0.24.10"}`),
		wingetHolding([]string{"0.24.10"}),
	)
	fake.repo.assetContent[checksumsFile] = testReleaseSums()
	return fake
}

// runChannelPublish runs publish of publishVersion over fake with a written notes file and an
// environment whose GITHUB_TOKEN is token ("" for unset).
func runChannelPublish(t *testing.T, fake *publishFake, isDryRun bool, token string) publishRunResult {
	t.Helper()
	notesPath := filepath.Join(t.TempDir(), "notes.md")
	writeTestFile(t, notesPath, "## What's new\n\n- A release.\n")
	getenv := func(key string) string {
		if key == githubTokenVariable {
			return token
		}
		return ""
	}
	return runTestPublishWith(t, fake, publishOptions{notesFile: notesPath, isDryRun: isDryRun, getenv: getenv})
}

// channelRun is a publish run of publishVersion over fake, its channel repositories recording
// what they are pushed, for a test to drive the tap and bucket steps' converge through directly;
// output holds what the run prints.
func channelRun(t *testing.T, fake *publishFake) (run *publishRun, output *strings.Builder) {
	t.Helper()
	commands := fake.runner()
	output = &strings.Builder{}
	run = &publishRun{commands: commands, repo: fake.repo, channels: fake.remote, output: output,
		version: mustVersion(t, publishVersion), options: publishOptions{dir: t.TempDir()}}
	return run, output
}

// publishedRepos returns the repository of each file the channel repositories were pushed, in
// order.
func publishedRepos(channels memChannelRepos) []string {
	var repos []string
	for _, write := range channels.publishedFiles() {
		repos = append(repos, write.repo)
	}
	return repos
}

// TestPublishWingetDryRunPrintsTheKomacCommandWithTheTokenRedacted ports the bash script's dry-run
// test: the plan shows komac's exact command line with the token redacted, the token itself is
// printed nowhere, and nothing — not even komac's version check — is run.
func TestPublishWingetDryRunPrintsTheKomacCommandWithTheTokenRedacted(t *testing.T) {
	t.Parallel()

	result := runChannelPublish(t, releasedFake(), true, sentinelToken)

	if result.err != nil {
		t.Fatalf("publish failed: %v\n%s", result.err, result.output)
	}
	if !strings.Contains(result.output, testWingetCommand) {
		t.Errorf("the dry run's plan lacks the komac command\n%s\nin:\n%s", testWingetCommand, result.output)
	}
	if strings.Contains(result.output, sentinelToken) {
		t.Errorf("the dry run printed the GITHUB_TOKEN value:\n%s", result.output)
	}
	if calls := result.commands.recorded("komac"); len(calls) > 0 {
		t.Errorf("the dry run ran komac %d times, want never", len(calls))
	}
	if mutating := mutatingLines(result.commands); len(mutating) > 0 {
		t.Errorf("issued mutating commands %q, want none", mutating)
	}
}

// TestPublishWingetSubmitsWithTheTokenInKomacsEnvironment pins the update of a package winget-pkgs
// already lists: it runs `komac update` with the dry run's argv, the token reaches komac through
// GITHUB_TOKEN in its environment — taken from the environment, else from `gh auth token` — and
// never through its argv or the output, and the step reports the release submitted, never
// published.
func TestPublishWingetSubmitsWithTheTokenInKomacsEnvironment(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		envToken string
		ghToken  string
	}{
		"GITHUB_TOKEN set":         {envToken: sentinelToken, ghToken: "gh-token-unused"},
		"token from gh auth token": {ghToken: sentinelToken},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fake := releasedFake()
			fake.channels.ghToken = tc.ghToken

			result := runChannelPublish(t, fake, false, tc.envToken)

			if result.err != nil {
				t.Fatalf("publish failed: %v\n%s", result.err, result.output)
			}
			updates := slices.DeleteFunc(result.commands.recorded("komac"), func(call recordedCall) bool {
				return len(call.args) == 0 || call.args[0] != "update"
			})
			if len(updates) != 1 {
				t.Fatalf("ran komac update %d times, want once", len(updates))
			}
			argv := "komac " + strings.Join(updates[0].args, " ")
			if want := strings.TrimPrefix(testWingetCommand, "GITHUB_TOKEN=<redacted> "); argv != want {
				t.Errorf("komac argv:\n got: %s\nwant: %s", argv, want)
			}
			if !slices.Contains(updates[0].env, githubTokenVariable+"="+sentinelToken) {
				t.Errorf("komac's environment %q lacks GITHUB_TOKEN=<the token>", updates[0].env)
			}
			if strings.Contains(result.output, sentinelToken) {
				t.Errorf("publish printed the token:\n%s", result.output)
			}
			if !hasRow(result.output, "winget", "done", wingetSubmittedDetail) {
				t.Errorf("the winget row does not report the release submitted:\n%s", result.output)
			}
		})
	}
}

// TestPublishWingetUpdateFailureRedactsTheToken pins a failed `komac update` whose error echoes
// the token: the winget step fails naming the manifest update, and the token reaches neither the
// error nor the output.
func TestPublishWingetUpdateFailureRedactsTheToken(t *testing.T) {
	t.Parallel()
	fake := releasedFake()
	fake.channels.isKomacUpdateFailing = true

	result := runChannelPublish(t, fake, false, sentinelToken)

	if result.err == nil {
		t.Fatalf("publish succeeded with komac update failing:\n%s", result.output)
	}
	const want = "the winget step failed: submit the AiricLenz.Capcom 0.24.11 manifest update"
	if !strings.Contains(result.err.Error(), want) {
		t.Errorf("publish error = %v, want it to contain %q", result.err, want)
	}
	if strings.Contains(result.err.Error(), sentinelToken) {
		t.Errorf("publish error carries the token: %v", result.err)
	}
	if strings.Contains(result.output, sentinelToken) {
		t.Errorf("publish printed the token:\n%s", result.output)
	}
}

// TestPublishWingetRequiresKomac ports the bash script's missing-komac test: the winget step fails
// with an install hint and submits nothing.
func TestPublishWingetRequiresKomac(t *testing.T) {
	t.Parallel()
	fake := releasedFake()
	fake.channels.isKomacMissing = true

	result := runChannelPublish(t, fake, false, sentinelToken)

	if result.err == nil {
		t.Fatalf("publish succeeded without komac:\n%s", result.output)
	}
	for _, want := range []string{"the winget step failed", "komac is not installed", "install komac"} {
		if !strings.Contains(result.err.Error(), want) {
			t.Errorf("publish error = %v, want it to name %q", result.err, want)
		}
	}
	if index := lineIndex(commandLines(result.commands), "komac update"); index >= 0 {
		t.Errorf("ran komac update without komac installed")
	}
}

// TestPublishCommitsAChannelFileOnlyWhenItChanges pins the tap and the bucket: each takes the
// rendered file, committed as `capcom X.Y.Z` and pushed to its branch, only when that changes the
// file; a repository already holding it is pushed nothing, and the step says so.
func TestPublishCommitsAChannelFileOnlyWhenItChanges(t *testing.T) {
	t.Parallel()
	sums := testReleaseSums()
	formula, err := renderFormula(publishVersion, sums)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := renderScoopManifest(publishVersion, sums)
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]struct {
		tap, bucket string
		wantPushed  []string
	}{
		"both behind": {
			wantPushed: []string{tapRepo, bucketRepo},
		},
		"tap already current": {
			tap:        formula,
			wantPushed: []string{bucketRepo},
		},
		"bucket already current": {
			bucket:     manifest,
			wantPushed: []string{tapRepo},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fake := releasedFake()
			if tc.tap != "" {
				fake.remote = channelReposOf(fake.remote, tapHolding(tc.tap))
			}
			if tc.bucket != "" {
				fake.remote = channelReposOf(fake.remote, bucketHolding(tc.bucket))
			}
			run, output := channelRun(t, fake)

			for _, converge := range []func(context.Context, *publishRun) error{convergeHomebrew, convergeScoop} {
				if err := converge(t.Context(), run); err != nil {
					t.Fatal(err)
				}
			}

			if pushed := publishedRepos(fake.remote); !slices.Equal(pushed, tc.wantPushed) {
				t.Errorf("pushed to %q, want %q", pushed, tc.wantPushed)
			}
			for _, write := range fake.remote.publishedFiles() {
				if write.subject != "capcom 0.24.11" {
					t.Errorf("%s: committed as %q, want %q", write.repo, write.subject, "capcom 0.24.11")
				}
			}
			for repo, current := range map[string]string{tapRepo: tc.tap, bucketRepo: tc.bucket} {
				said := repo + " already holds this " + channelFile[repo] + ": nothing to push"
				if isSaid := strings.Contains(output.String(), said); isSaid != (current != "") {
					t.Errorf("%s: output says %q: %t, want %t:\n%s", repo, said, isSaid, current != "", output)
				}
			}
		})
	}
}

// TestPublishWritesTheRenderedFileToTheChannel pins what the tap and the bucket receive: exactly
// the file rendered from the release's SHA256SUMS, at its path.
func TestPublishWritesTheRenderedFileToTheChannel(t *testing.T) {
	t.Parallel()
	fake := releasedFake()
	run, _ := channelRun(t, fake)

	for _, converge := range []func(context.Context, *publishRun) error{convergeHomebrew, convergeScoop} {
		if err := converge(t.Context(), run); err != nil {
			t.Fatal(err)
		}
	}

	formula, err := renderFormula(publishVersion, testReleaseSums())
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := renderScoopManifest(publishVersion, testReleaseSums())
	if err != nil {
		t.Fatal(err)
	}
	want := []memChannelPublish{
		{repo: tapRepo, path: formulaPath, content: formula, subject: "capcom 0.24.11"},
		{repo: bucketRepo, path: bucketPath, content: manifest, subject: "capcom 0.24.11"},
	}
	if got := fake.remote.publishedFiles(); !slices.Equal(got, want) {
		t.Errorf("pushed:\n%+v\nwant the rendered formula and manifest:\n%+v", got, want)
	}
}

// TestPublishSkipsChannelsWhoseSetupIsPending pins the channels' one-time setup: a missing tap or
// bucket repository is skipped and named, touched never, and the run succeeds; a package not yet
// in winget-pkgs is no setup at all — publish submits its first manifests itself.
func TestPublishSkipsChannelsWhoseSetupIsPending(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		missingRepo string
		skipped     string
		updatedRepo string
	}{
		"no tap repository": {
			missingRepo: tapRepo,
			skipped:     "homebrew: skipped, its one-time setup is pending",
			updatedRepo: bucketRepo,
		},
		"no bucket repository": {
			missingRepo: bucketRepo,
			skipped:     "scoop: skipped, its one-time setup is pending",
			updatedRepo: tapRepo,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fake := releasedFake()
			delete(fake.remote.repos, tc.missingRepo)
			fake.remote = channelReposOf(fake.remote, wingetHolding(nil))

			result := runChannelPublish(t, fake, false, sentinelToken)

			if result.err != nil {
				t.Fatalf("publish failed: %v\n%s", result.err, result.output)
			}
			if !strings.Contains(result.output, tc.skipped) {
				t.Errorf("output lacks %q:\n%s", tc.skipped, result.output)
			}
			if strings.Contains(result.output, "winget: skipped") {
				t.Errorf("winget was skipped, want its first manifests submitted:\n%s", result.output)
			}
			pushed := publishedRepos(fake.remote)
			if slices.Contains(pushed, tc.missingRepo) {
				t.Errorf("pushed to %s, a channel whose setup is pending", tc.missingRepo)
			}
			if !slices.Contains(pushed, tc.updatedRepo) {
				t.Errorf("%s was not updated; pushed to %q", tc.updatedRepo, pushed)
			}
			if index := lineIndex(commandLines(result.commands), "komac submit"); index < 0 {
				t.Errorf("the first winget manifests were not submitted; commands: %q", commandLines(result.commands))
			}
		})
	}
}

// TestPublishWingetSubmitsTheFirstManifestsWhenThePackageIsAbsent pins the first submission: with
// no AiricLenz.Capcom in winget-pkgs, publish renders the three manifests into
// manifests/a/AiricLenz/Capcom/<X.Y.Z>/ and runs `komac submit <dir> --yes` with the token in
// komac's environment only, never `komac update`; the directory is removed once it is submitted.
func TestPublishWingetSubmitsTheFirstManifestsWhenThePackageIsAbsent(t *testing.T) {
	t.Parallel()
	fake := releasedFake()
	fake.remote = channelReposOf(fake.remote, wingetHolding(nil))

	result := runChannelPublish(t, fake, false, sentinelToken)

	if result.err != nil {
		t.Fatalf("publish failed: %v\n%s", result.err, result.output)
	}
	calls := result.commands.recorded("komac")
	submits := slices.DeleteFunc(slices.Clone(calls), func(call recordedCall) bool { return call.args[0] != "submit" })
	if len(submits) != 1 {
		t.Fatalf("ran komac submit %d times, want once; komac calls: %v", len(submits), calls)
	}
	args := submits[0].args
	if len(args) != 3 || args[2] != "--yes" || !strings.HasSuffix(filepath.ToSlash(args[1]), "/"+testManifestSubdir) {
		t.Errorf("komac argv = %q, want submit <temp>/%s --yes", args, testManifestSubdir)
	}
	if !slices.Contains(submits[0].env, githubTokenVariable+"="+sentinelToken) {
		t.Errorf("komac's environment %q lacks GITHUB_TOKEN=<the token>", submits[0].env)
	}
	if index := lineIndex(commandLines(result.commands), "komac update"); index >= 0 {
		t.Errorf("ran komac update for a package winget-pkgs does not have")
	}
	slices.Sort(fake.channels.submitted)
	if !slices.Equal(fake.channels.submitted, testWingetManifests) {
		t.Errorf("the submitted directory held %q, want %q", fake.channels.submitted, testWingetManifests)
	}
	if _, err := os.Stat(args[1]); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the manifest directory %s outlived its submission: %v", args[1], err)
	}
	if strings.Contains(result.output, sentinelToken) {
		t.Errorf("publish printed the token:\n%s", result.output)
	}
	if !hasRow(result.output, "winget", "done", wingetSubmittedDetail) {
		t.Errorf("the winget row does not report the release submitted:\n%s", result.output)
	}
}

// TestPublishWingetSubmitFailureIsSetupPending pins a failed first submission: it does not fail
// the run — the winget row reads setup-pending, naming the kept manifest directory and the
// command to submit it with, the token redacted — and the smoke step still runs.
func TestPublishWingetSubmitFailureIsSetupPending(t *testing.T) {
	t.Parallel()
	fake := releasedFake()
	fake.remote = channelReposOf(fake.remote, wingetHolding(nil))
	fake.channels.isKomacSubmitFailing = true

	result := runChannelPublish(t, fake, false, sentinelToken)

	var manifestDir string
	for _, call := range result.commands.recorded("komac") {
		if call.args[0] == "submit" {
			manifestDir = call.args[1]
		}
	}
	if manifestDir == "" {
		t.Fatalf("komac submit never ran:\n%s", result.output)
	}
	t.Cleanup(func() {
		suffix := string(filepath.Separator) + filepath.FromSlash(testManifestSubdir)
		_ = os.RemoveAll(strings.TrimSuffix(manifestDir, suffix))
	})
	if result.err != nil {
		t.Fatalf("publish failed: %v\n%s", result.err, result.output)
	}
	wantCommand := "`GITHUB_TOKEN=<redacted> komac submit " + manifestDir + " --yes`"
	if !hasRow(result.output, "winget", "setup-pending", "the first AiricLenz.Capcom submission failed") ||
		!strings.Contains(result.output, wantCommand) {
		t.Errorf("the winget row does not report setup pending with %s:\n%s", wantCommand, result.output)
	}
	if strings.Contains(result.output, sentinelToken) {
		t.Errorf("publish printed the token:\n%s", result.output)
	}
	if len(result.smokes) != 1 {
		t.Errorf("smoke ran %d times after the failed submission, want once", len(result.smokes))
	}
	if kept := listDir(t, manifestDir); !slices.Equal(kept, testWingetManifests) {
		t.Errorf("the manifest directory holds %q after the run, want %q", kept, testWingetManifests)
	}
}

// TestPublishWingetDryRunForAnAbsentPackage pins the dry run of a first submission: the plan names
// `komac submit` with the token redacted, and nothing is rendered or submitted.
func TestPublishWingetDryRunForAnAbsentPackage(t *testing.T) {
	t.Parallel()
	fake := releasedFake()
	fake.remote = channelReposOf(fake.remote, wingetHolding(nil))

	result := runChannelPublish(t, fake, true, sentinelToken)

	if result.err != nil {
		t.Fatalf("publish failed: %v\n%s", result.err, result.output)
	}
	if !strings.Contains(result.output, "GITHUB_TOKEN=<redacted> komac submit") {
		t.Errorf("the dry run's plan lacks the komac submit command:\n%s", result.output)
	}
	if strings.Contains(result.output, sentinelToken) {
		t.Errorf("the dry run printed the GITHUB_TOKEN value:\n%s", result.output)
	}
	if calls := result.commands.recorded("komac"); len(calls) > 0 {
		t.Errorf("the dry run ran komac %d times, want never", len(calls))
	}
	if mutating := mutatingLines(result.commands); len(mutating) > 0 {
		t.Errorf("issued mutating commands %q, want none", mutating)
	}
}

// TestPublishLeavesASubmittedWingetReleaseAlone pins the winget probe's hold over komac: an open
// pull request for the version is a submission already made, so komac is never run again.
func TestPublishLeavesASubmittedWingetReleaseAlone(t *testing.T) {
	t.Parallel()
	fake := releasedFake()
	fake.remote = channelReposOf(fake.remote, wingetHolding([]string{"0.24.10"}, komacPull(301234, "New version", "0.24.11")))

	result := runChannelPublish(t, fake, false, sentinelToken)

	if result.err != nil {
		t.Fatalf("publish failed: %v\n%s", result.err, result.output)
	}
	if calls := result.commands.recorded("komac"); len(calls) > 0 {
		t.Errorf("ran komac %d times over a submitted release, want never", len(calls))
	}
	if !hasRow(result.output, "winget", "done", "submitted: pull request #301234") {
		t.Errorf("the winget row does not report the open pull request:\n%s", result.output)
	}
}

// TestPublishLeavesAChannelPinnedToANewerVersionAlone pins the no-downgrade rule: a rerun of
// publish for an older release finds the tap and the bucket already on a newer one, reports both
// done, and never publishes to either.
func TestPublishLeavesAChannelPinnedToANewerVersionAlone(t *testing.T) {
	t.Parallel()
	fake := releasedFake()
	newerFormula := strings.Replace(testFormula, "0.24.11", "0.24.12", 1)
	newerManifest := `{"version": "0.24.12"}`
	fake.remote = channelReposOf(fake.remote, tapHolding(newerFormula), bucketHolding(newerManifest))

	result := runChannelPublish(t, fake, false, sentinelToken)

	if result.err != nil {
		t.Fatalf("publish failed: %v\n%s", result.err, result.output)
	}
	lines := commandLines(result.commands)
	for _, unwanted := range []string{channelsCallName + " " + methodPublishFile} {
		if index := lineIndex(lines, unwanted); index >= 0 {
			t.Errorf("ran %q over a channel pinned to a newer version", lines[index])
		}
	}
	for step, detail := range map[string]string{stepHomebrew: "formula pins newer 0.24.12", stepScoop: "manifest pins newer 0.24.12"} {
		if !hasRow(result.output, step, "done", detail) {
			t.Errorf("the %s row does not report %q as done:\n%s", step, detail, result.output)
		}
	}
}

// TestPublishWingetConvergesFromRemoteStateOnceTheTagExists pins item 6's resume for the channel
// steps: days later, with VERSION moved on and a dirty tree — either of which refuses a run that
// consults them — publish still submits the pending winget manifest without consulting the gate
// or VERSION at HEAD, and pushes nothing.
func TestPublishWingetConvergesFromRemoteStateOnceTheTagExists(t *testing.T) {
	t.Parallel()
	fake := releasedFake()
	fake.remote = channelReposOf(fake.remote, tapHolding(testFormula), bucketHolding(`{"version": "0.24.11"}`))
	fake.repo.isDirty = true
	fake.repo.files[versionFile] = "v0.24.12\n"

	result := runChannelPublish(t, fake, false, sentinelToken)

	if result.err != nil {
		t.Fatalf("publish failed: %v\n%s", result.err, result.output)
	}
	lines := commandLines(result.commands)
	for _, unwanted := range []string{"repo PushMain", channelsCallName + " " + methodPublishFile} {
		if index := lineIndex(lines, unwanted); index >= 0 {
			t.Errorf("ran %q, want only the winget submission once the tag exists", lines[index])
		}
	}
	if index := lineIndex(lines, "komac update "+wingetPackageID); index < 0 {
		t.Errorf("winget was not submitted; commands: %q", lines)
	}
}

// TestPublishChannelStepFailureStopsTheRun characterises a tap or bucket the publish fails for:
// publish stops at that channel's step with an error naming the step and the failure, prints the
// step table as the remote now holds it, and never reaches the smoke step. Which step of the
// publish itself failed — the clone, the history read, the commit or the push — is
// execChannelRepos' to name (channelrepos_test.go).
func TestPublishChannelStepFailureStopsTheRun(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo      string
		wantError string
	}{
		"tap": {
			repo:      tapRepo,
			wantError: "the homebrew step failed: push " + formulaPath + " to " + tapRepo + ": remote rejected the request",
		},
		"bucket": {
			repo:      bucketRepo,
			wantError: "the scoop step failed: push " + bucketPath + " to " + bucketRepo + ": remote rejected the request",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fake := releasedFake()
			if tc.repo == bucketRepo {
				fake.remote = channelReposOf(fake.remote, tapHolding(testFormula))
			}
			failure := fmt.Errorf("push %s to %s: remote rejected the request", channelFile[tc.repo], tc.repo)
			fake.remote = fake.remote.withFailure(methodPublishFile, tc.repo, failure)

			result := runChannelPublish(t, fake, false, sentinelToken)

			if result.err == nil || !strings.Contains(result.err.Error(), tc.wantError) {
				t.Fatalf("publish error = %v, want one naming %q", result.err, tc.wantError)
			}
			if !strings.Contains(result.output, "steps") {
				t.Errorf("the step table was not printed:\n%s", result.output)
			}
			if len(result.smokes) != 0 {
				t.Errorf("the smoke step ran %d times after the %s step failed", len(result.smokes), tc.repo)
			}
		})
	}
}
