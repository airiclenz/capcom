package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// statusCheckout is a checkout status reports on: capcom's own repository held in repo, the
// channel repositories the probes read through the runner in remote.
type statusCheckout struct {
	repo   memRepo
	remote fakeRemote
}

// clearedCheckout is a checkout whose gate clears: HEAD is a VERSION bump over a feature commit,
// both pushed, with CI green on the bump. Bare tags newer than the latest release exist, as every
// pushed VERSION bump tags one. The latest release is seven commits back, the whole history
// twelve; nothing is published for any newer version, and no channel's one-time setup has landed.
func clearedCheckout() statusCheckout {
	return statusCheckout{repo: memRepo{
		commits:       linearHistory(),
		head:          bumpCommit,
		originMain:    bumpCommit,
		greenRuns:     []string{bumpCommit},
		tags:          []string{"v0.24.10", "v0.24.11", "v0.24.12", "v0.24.13"},
		latestRelease: githubRelease{Tag: "v0.24.11", Published: "2026-10-08T19:24:24Z"},
		commitCounts:  map[string]int{"v0.24.11..HEAD": 7, "HEAD": 12},
	}}
}

// opener opens the checkout's memRepo whatever directory it is asked for.
func (c statusCheckout) opener() repoOpener {
	return func(string) repo { return c.repo }
}

// runner returns a recording runner over the channel repositories.
func (c statusCheckout) runner() *fakeRunner {
	return c.remote.runner()
}

// run runs the program's command tree over the checkout with args.
func (c statusCheckout) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return executeWith(t, c.runner(), c.opener(), args...)
}

// statusJSON runs status over checkout with args and decodes its JSON report, returning the error
// main would turn into an exit code.
func statusJSON(t *testing.T, checkout statusCheckout, args ...string) (statusReport, error) {
	t.Helper()
	output, err := checkout.run(t, append([]string{"status", "--json"}, args...)...)
	var report statusReport
	if decodeErr := json.Unmarshal([]byte(output), &report); decodeErr != nil {
		t.Fatalf("status --json printed no JSON report: %v\n%s", decodeErr, output)
	}
	return report, err
}

// TestStatusReportsTheGateBaselineAndHighestTag pins the cleared report end to end: the
// baseline is the latest *published* release while newer bare tags exist, the highest tag is the
// newest of them, and a cleared gate exits 0.
func TestStatusReportsTheGateBaselineAndHighestTag(t *testing.T) {
	t.Parallel()

	report, err := statusJSON(t, clearedCheckout(), "v0.24.14")

	if err != nil {
		t.Fatalf("status failed over a cleared gate: %v", err)
	}
	want := statusReport{
		Version:    "v0.24.14",
		HighestTag: "v0.24.13",
		Gate:       gateReport{IsOK: true, Commit: featureCommit, Failures: []string{}},
		Baseline: baselineReport{
			Tag:       "v0.24.11",
			Published: "2026-10-08T19:24:24Z",
			Range:     "v0.24.11..HEAD",
			Commits:   7,
		},
	}
	steps := report.Steps
	report.Steps = nil
	gotJSON, _ := json.Marshal(report)
	wantJSON, _ := json.Marshal(want)
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Errorf("report = %s\nwant     %s", gotJSON, wantJSON)
	}

	// Nothing is published for v0.24.14 and no channel's one-time setup has landed; winget needs
	// none, so it is pending.
	var states []string
	for _, step := range steps {
		states = append(states, step.Name+" "+string(step.State))
	}
	wantStates := []string{"tag pending", "release pending", "homebrew setup-pending", "scoop setup-pending", "winget pending", "smoke pending"}
	if !slices.Equal(states, wantStates) {
		t.Errorf("steps = %q, want %q", states, wantStates)
	}
}

// TestStatusJSONKeysArePinned pins the JSON report's key set: the release skills read these keys,
// so renaming one must fail here first.
func TestStatusJSONKeysArePinned(t *testing.T) {
	t.Parallel()

	output, err := clearedCheckout().run(t, "status", "--json", "v0.24.14")
	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, output)
	}
	var report map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("status --json printed no JSON object: %v\n%s", err, output)
	}
	var gate, baseline map[string]json.RawMessage
	if err := json.Unmarshal(report["gate"], &gate); err != nil {
		t.Fatalf("gate is not an object: %v", err)
	}
	if err := json.Unmarshal(report["baseline"], &baseline); err != nil {
		t.Fatalf("baseline is not an object: %v", err)
	}

	pinned := map[string]struct {
		object map[string]json.RawMessage
		want   []string
	}{
		"report":   {object: report, want: []string{"baseline", "gate", "highestTag", "steps", "version"}},
		"gate":     {object: gate, want: []string{"commit", "failures", "ok"}},
		"baseline": {object: baseline, want: []string{"commits", "published", "range", "tag"}},
	}
	for name, keys := range pinned {
		if got := slices.Sorted(maps.Keys(keys.object)); !slices.Equal(got, keys.want) {
			t.Errorf("%s keys = %v, want %v", name, got, keys.want)
		}
	}
	var steps []map[string]json.RawMessage
	if err := json.Unmarshal(report["steps"], &steps); err != nil {
		t.Fatalf("steps is not an array of objects: %v", err)
	}
	if len(steps) != len(publishSteps) {
		t.Fatalf("steps has %d entries, want %d", len(steps), len(publishSteps))
	}
	for _, step := range steps {
		if got, want := slices.Sorted(maps.Keys(step)), []string{"detail", "name", "state"}; !slices.Equal(got, want) {
			t.Errorf("step keys = %v, want %v", got, want)
		}
	}
}

// TestStatusProbesTheVersionFileOnlyAboveTheBaseline pins when status probes the publish steps
// for the checkout's VERSION: above the baseline a release is in progress and every step is
// probed; at the baseline none is, and no probe command reaches the runner; with no published
// release, and so no baseline tag, the first release is in progress and every step is probed.
func TestStatusProbesTheVersionFileOnlyAboveTheBaseline(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		version      string
		noReleaseYet bool
		wantSteps    int
	}{
		"above the baseline":   {version: "v0.24.14", wantSteps: len(publishSteps)},
		"at the baseline":      {version: "v0.24.11", wantSteps: 0},
		"no published release": {version: "0.0.0", noReleaseYet: true, wantSteps: len(publishSteps)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, versionFile), []byte(tc.version+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			checkout := clearedCheckout()
			if tc.noReleaseYet {
				checkout.repo.latestRelease = githubRelease{}
			}
			commands := checkout.runner()
			var output bytes.Buffer

			err := runStatus(t.Context(), commands, &output, statusOptions{dir: dir, isJSON: true, openRepo: checkout.opener()})

			if err != nil {
				t.Fatalf("status failed: %v\n%s", err, output.String())
			}
			var report statusReport
			if err := json.Unmarshal(output.Bytes(), &report); err != nil {
				t.Fatalf("status --json printed no JSON report: %v", err)
			}
			if len(report.Steps) != tc.wantSteps {
				t.Errorf("steps = %d, want %d", len(report.Steps), tc.wantSteps)
			}
			if tc.wantSteps == 0 && len(commands.calls) != 0 {
				t.Errorf("runner was called %d times, want none: no probe", len(commands.calls))
			}
		})
	}
}

// TestStatusBaselineWithNoPublishedRelease pins the baseline before the first release is
// published: no tag, no publish date, and the whole history as the range — counted as HEAD.
func TestStatusBaselineWithNoPublishedRelease(t *testing.T) {
	t.Parallel()
	checkout := clearedCheckout()
	checkout.repo.latestRelease = githubRelease{}
	checkout.repo.tags = nil
	var output bytes.Buffer

	err := runStatus(t.Context(), checkout.runner(), &output,
		statusOptions{dir: ".", version: "v0.1.0", isJSON: true, openRepo: checkout.opener()})

	if err != nil {
		t.Fatalf("status failed with no published release: %v\n%s", err, output.String())
	}
	var report statusReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("status --json printed no JSON report: %v", err)
	}
	want := baselineReport{Tag: "", Published: "", Range: "HEAD", Commits: 12}
	if report.Baseline != want {
		t.Errorf("baseline = %+v, want %+v", report.Baseline, want)
	}
	if report.HighestTag != "" {
		t.Errorf("highest tag = %q, want none", report.HighestTag)
	}

	text, err := checkout.run(t, "status", "v0.1.0")
	if err != nil {
		t.Fatalf("status failed with no published release: %v\n%s", err, text)
	}
	if want := "baseline     no published release, HEAD: 12 commits\n"; !strings.Contains(text, want) {
		t.Errorf("output lacks %q:\n%s", want, text)
	}
}

// TestStatusFailsOnAStepItCouldNotProbe pins that an unknown step is a run failure (exit 1) even
// when the gate clears, and that the report naming it is still printed.
func TestStatusFailsOnAStepItCouldNotProbe(t *testing.T) {
	t.Parallel()
	checkout := clearedCheckout()
	checkout.remote.failing = "gh api repos/" + tapRepo

	report, err := statusJSON(t, checkout, "v0.24.14")

	if err == nil || exitCodeFor(err) != exitRunFailed {
		t.Fatalf("status = %v, want a run failure", err)
	}
	if !strings.Contains(err.Error(), "could not probe the homebrew step") {
		t.Errorf("error = %v, want it to name the homebrew step", err)
	}
	if !report.Gate.IsOK {
		t.Errorf("gate failures = %q, want a cleared gate", report.Gate.Failures)
	}
	if len(report.Steps) != len(publishSteps) || report.Steps[2].State != stateUnknown {
		t.Errorf("steps = %+v, want homebrew unknown", report.Steps)
	}
}

// TestStatusVersionArgument pins where the reported version comes from: the argument when one is
// given, a bare X.Y.Z read as vX.Y.Z, and a malformed one refused as a usage error before any
// command runs.
func TestStatusVersionArgument(t *testing.T) {
	t.Parallel()

	t.Run("bare X.Y.Z reads as vX.Y.Z", func(t *testing.T) {
		t.Parallel()

		report, err := statusJSON(t, clearedCheckout(), "0.24.14")

		if err != nil {
			t.Fatalf("status failed: %v", err)
		}
		if report.Version != "v0.24.14" {
			t.Errorf("version = %q, want v0.24.14", report.Version)
		}
	})

	t.Run("a malformed version is a usage error", func(t *testing.T) {
		t.Parallel()
		commands := &fakeRunner{respond: func(recordedCall) (string, error) {
			return "", errors.New("the runner must not be reached")
		}}

		_, err := execute(t, commands, "status", "v0.25.0-rc.1")

		if err == nil || exitCodeFor(err) != exitBadUsage {
			t.Errorf("status v0.25.0-rc.1 = %v, want a usage error", err)
		}
		if len(commands.calls) != 0 {
			t.Errorf("runner was called %d times, want none", len(commands.calls))
		}
	})

	t.Run("two versions are a usage error", func(t *testing.T) {
		t.Parallel()

		_, err := clearedCheckout().run(t, "status", "v0.24.14", "v0.24.15")

		if err == nil || exitCodeFor(err) != exitBadUsage {
			t.Errorf("status with two versions = %v, want a usage error", err)
		}
	})
}

// TestStatusDefaultsToTheVersionFile pins that with no argument the checkout's VERSION is the
// version reported.
func TestStatusDefaultsToTheVersionFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, versionFile), []byte("v0.24.14\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkout := clearedCheckout()
	var output bytes.Buffer

	err := runStatus(t.Context(), checkout.runner(), &output, statusOptions{dir: dir, isJSON: true, openRepo: checkout.opener()})

	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, output.String())
	}
	var report statusReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("status --json printed no JSON report: %v", err)
	}
	if report.Version != "v0.24.14" {
		t.Errorf("version = %q, want the VERSION file's v0.24.14", report.Version)
	}
}

// TestStatusFailsAfterPrintingTheReport pins the exit status: a blocked gate, or a check that
// could not run, is a run failure (exit 1) — and the report is still printed, so whoever reads
// it sees what was found.
func TestStatusFailsAfterPrintingTheReport(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change   func(checkout *memRepo)
		wantGate bool
		wantTag  string
	}{
		"a blocked gate":                    {change: func(checkout *memRepo) { checkout.isDirty = true }, wantTag: "v0.24.11"},
		"the latest release cannot be read": {change: func(checkout *memRepo) { checkout.failing = methodLatestRelease }, wantGate: true},
		"the tags cannot be listed":         {change: func(checkout *memRepo) { checkout.failing = methodReleaseTags }, wantGate: true, wantTag: "v0.24.11"},
		"the baseline cannot count":         {change: func(checkout *memRepo) { checkout.failing = methodCountCommits }, wantGate: true, wantTag: "v0.24.11"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkout := clearedCheckout()
			tc.change(&checkout.repo)

			report, err := statusJSON(t, checkout, "v0.24.14")

			if err == nil || exitCodeFor(err) != exitRunFailed {
				t.Fatalf("status = %v, want a run failure", err)
			}
			if report.Gate.IsOK != tc.wantGate {
				t.Errorf("gate ok = %t, want %t (failures %q)", report.Gate.IsOK, tc.wantGate, report.Gate.Failures)
			}
			if report.Baseline.Tag != tc.wantTag {
				t.Errorf("baseline tag = %q, want %q", report.Baseline.Tag, tc.wantTag)
			}
		})
	}
}

// TestStatusTextNamesTheBlockingConditions pins the table form: the gate's verdict, its commit
// and each failure appear in the plain-text report.
func TestStatusTextNamesTheBlockingConditions(t *testing.T) {
	t.Parallel()
	checkout := clearedCheckout()
	checkout.repo.isDirty = true

	output, err := checkout.run(t, "status", "v0.24.14")

	if !errors.Is(err, errGateBlocked) {
		t.Errorf("status = %v, want %v", err, errGateBlocked)
	}
	for _, want := range []string{
		"version      v0.24.14",
		"highest tag  v0.24.13",
		"baseline     v0.24.11 (published 2026-10-08T19:24:24Z), v0.24.11..HEAD: 7 commits",
		"gate         BLOCKED at 200000000000",
		"- the working tree is not clean",
		"\nsteps\n",
		"  tag       pending        origin has no tag v0.24.14\n",
		"  scoop     setup-pending  no airiclenz/scoop-bucket repository yet\n",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output lacks %q:\n%s", want, output)
		}
	}
}
