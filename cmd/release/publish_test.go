package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// The checkout's top level execRepo's CloneAt test clones, and the poll timing the publish tests'
// tag waits run on. The release they cut is publishVersion (smoke_test.go).
const (
	publishToplevel = "/home/owner/capcom"
	testTagTimeout  = 30 * time.Second
	testPollPause   = 10 * time.Second
)

// mutatingCommands are the commands that change something — origin, a GitHub release, the
// checkout's refs, or the disk a build writes — by their leading words: the runner's commands,
// and the memRepo's changes it records into the same log. A run that may change nothing issues
// none of them.
var mutatingCommands = []string{
	repoCallName + " " + methodPushMain,
	repoCallName + " " + methodFetchTag,
	repoCallName + " " + methodCloneAt,
	repoCallName + " " + methodCreateRelease,
	repoCallName + " " + methodUploadAssets,
	repoCallName + " " + methodUndraftRelease,
	"git push",
	"go build",
	"gh release delete",
	"gh repo clone",
	"git commit",
	"komac update",
	"komac submit",
}

// publishFake is what publish meets: capcom's checkout and GitHub repository held in repo, and
// the commands the runner answers the way git, go and gh would — the build itself, the channel
// repositories' through remote.
type publishFake struct {
	repo memRepo
	// remote is where the channel repositories stand.
	remote fakeRemote
	// tagAppearsAfter is how many polls after the push origin still lacks the tag; -1 for a
	// workflow that never makes it.
	tagAppearsAfter int
	// tagKind is the type of the object the tag the workflow makes points at; tagCommit is the
	// commit it is on.
	tagKind   string
	tagCommit string
	// channels is the tap, the bucket and the komac and gh installs the channel steps meet.
	channels fakeChannels

	mu sync.Mutex
}

// newPublishFake is a first run of publishVersion: HEAD is the VERSION bump, one commit ahead of
// origin/main, whose feature commit CI passed; nothing is tagged or released yet, and the
// workflow tags the bump on the third poll after the push. The tap's formula already pins the
// version, the Scoop bucket's repository is not there yet (setup pending) and an open winget-pkgs
// pull request already submits the version, so the channel steps change nothing.
func newPublishFake() *publishFake {
	fake := &publishFake{
		repo: memRepo{
			commits:      linearHistory(),
			head:         bumpCommit,
			originMain:   featureCommit,
			greenRuns:    []string{featureCommit},
			remoteTags:   map[string]string{},
			releases:     map[string][]string{},
			assetContent: map[string]string{},
			files:        map[string]string{versionFile: strings.TrimPrefix(publishVersion, "v") + "\n"},
			publish:      &memPublish{},
		},
		remote: fakeRemote{
			hasTap:      true,
			formula:     testFormula,
			wingetPulls: wingetPull(301234, "New version", strings.TrimPrefix(publishVersion, "v")),
		},
		tagAppearsAfter: 2,
		tagKind:         gitObjectTag,
		tagCommit:       bumpCommit,
	}
	fake.repo.publish.tagAfterPush = fake.tagAfterPush
	return fake
}

// tagAfterPush is the workflow: it makes the tag once origin has had tagAppearsAfter polls after
// the push.
func (f *publishFake) tagAfterPush(polls int) (memTag, bool) {
	if f.tagAppearsAfter < 0 || polls <= f.tagAppearsAfter {
		return memTag{}, false
	}
	return memTag{name: publishVersion, kind: f.tagKind, commit: f.tagCommit}, true
}

// tagged puts the annotated tag on origin from the start, as on a run resumed after the tag step:
// on the VERSION bump, whose gate commit (the feature commit) CI passed in newPublishFake, and in
// the checkout, as the gate's fetch would bring it.
func (f *publishFake) tagged() {
	f.repo.remoteTags[publishVersion] = gitObjectTag
	if f.repo.tagCommits == nil {
		f.repo.tagCommits = map[string]string{}
	}
	f.repo.tagCommits[publishVersion] = bumpCommit
}

// runner returns a recording runner over the fake, whose call record the repo's changes join.
func (f *publishFake) runner() *fakeRunner {
	commands := &fakeRunner{respond: f.respond}
	f.repo.publish.log = commands
	return commands
}

// opener opens the fake's repo whatever directory it is asked for.
func (f *publishFake) opener() repoOpener {
	return func(string) repo { return f.repo }
}

// respond answers one command.
func (f *publishFake) respond(call recordedCall) (string, error) {
	args := call.args
	switch command := call.name + " " + strings.Join(args, " "); {
	case command == "git log -1 --format=%ct HEAD":
		return testCommitTime, nil
	case call.name == "go" && len(args) > 0 && args[0] == "build":
		output := args[slices.Index(args, "-o")+1]
		return "", os.WriteFile(output, []byte(stubBinary(call.env)), 0o755)
	}
	if output, isAnswered, err := f.respondChannel(call); isAnswered {
		return output, err
	}
	return f.remote.respond(call)
}

// requireFiles fails when an absolute path among args names no file: gh would refuse to upload
// an asset that is not there, so memRepo refuses it too.
func requireFiles(args []string) error {
	for _, arg := range args {
		if !filepath.IsAbs(arg) {
			continue
		}
		if _, err := os.Stat(arg); err != nil {
			return err
		}
	}
	return nil
}

// rebuiltSums is the SHA256SUMS a rebuild of publishVersion writes: dist is byte-identical
// across runs over the same tree, so a release published from an earlier run carries exactly it.
func rebuiltSums(t *testing.T) string {
	t.Helper()
	sourceDir := t.TempDir()
	if err := writeReleaseTree(sourceDir); err != nil {
		t.Fatal(err)
	}
	outDir := runTestDist(t, fakeToolchain(), sourceDir)
	return readTestFile(t, filepath.Join(outDir, checksumsFile))
}

// rebuiltAssetContent is every asset a rebuild of publishVersion writes — the archives and
// SHA256SUMS — by name, as an earlier run's release carries them: a memRepo's assetContent.
func rebuiltAssetContent(t *testing.T) map[string]string {
	t.Helper()
	sourceDir := t.TempDir()
	if err := writeReleaseTree(sourceDir); err != nil {
		t.Fatal(err)
	}
	outDir := runTestDist(t, fakeToolchain(), sourceDir)
	content := map[string]string{}
	for _, name := range releaseAssetsV02411 {
		content[name] = readTestFile(t, filepath.Join(outDir, name))
	}
	return content
}

// publishRunResult is what one publish run did: the error main would turn into an exit code,
// everything it printed, every command it ran, how many times it paused, and its notes file.
type publishRunResult struct {
	err       error
	output    string
	commands  *fakeRunner
	pauses    int
	notesPath string
	// smokes are the smoke runs publish asked for, in order.
	smokes []smokeOptions
}

// runTestPublish runs publish of publishVersion over fake with a written notes file, pausing
// through a counter instead of a clock.
func runTestPublish(t *testing.T, fake *publishFake, isDryRun bool) publishRunResult {
	t.Helper()
	notesPath := filepath.Join(t.TempDir(), "notes.md")
	writeTestFile(t, notesPath, "## What's new\n\n- A release.\n")
	return runTestPublishWith(t, fake, publishOptions{notesFile: notesPath, isDryRun: isDryRun})
}

// runTestPublishWith runs publish of publishVersion over fake with options, filling in the
// version, the test poll timing, a counting sleep and, unless options has one, an environment
// with nothing set and a smoke step that records what it was asked to check and passes.
func runTestPublishWith(t *testing.T, fake *publishFake, options publishOptions) publishRunResult {
	t.Helper()
	result := publishRunResult{commands: fake.runner(), notesPath: options.notesFile}
	if options.openRepo == nil {
		options.openRepo = fake.opener()
	}
	if options.getenv == nil {
		options.getenv = func(string) string { return "" }
	}
	if options.smoke == nil {
		options.smoke = func(_ context.Context, _ io.Writer, smoke smokeOptions) error {
			result.smokes = append(result.smokes, smoke)
			return nil
		}
	}
	options.dir = t.TempDir()
	options.version = publishVersion
	options.tagTimeout = testTagTimeout
	options.pollInterval = testPollPause
	options.sleep = func(context.Context, time.Duration) error {
		result.pauses++
		return nil
	}
	var output bytes.Buffer

	result.err = runPublish(t.Context(), result.commands, &output, options)

	result.output = output.String()
	return result
}

// commandLines returns every command the runner was asked for, one line each, in order.
func commandLines(commands *fakeRunner) []string {
	commands.mu.Lock()
	defer commands.mu.Unlock()
	lines := make([]string, 0, len(commands.calls))
	for _, call := range commands.calls {
		lines = append(lines, call.name+" "+strings.Join(call.args, " "))
	}
	return lines
}

// mutatingLines returns the commands among the runner's calls that change something.
func mutatingLines(commands *fakeRunner) []string {
	var mutating []string
	for _, line := range commandLines(commands) {
		if slices.ContainsFunc(mutatingCommands, func(prefix string) bool { return strings.HasPrefix(line, prefix) }) {
			mutating = append(mutating, line)
		}
	}
	return mutating
}

// hasRow reports whether output has a table row whose cells start with cells, in order: the
// padding between cells, in the row and in cells alike, is collapsed first, so a row reads the
// same however wide its columns are.
func hasRow(output string, cells ...string) bool {
	want := strings.Join(strings.Fields(strings.Join(cells, " ")), " ")
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.Join(strings.Fields(line), " "), want) {
			return true
		}
	}
	return false
}

// lineIndex returns the index of the first line starting with prefix, -1 for none.
func lineIndex(lines []string, prefix string) int {
	return slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, prefix) })
}

// TestPublishFirstRunConvergesTheTagThenTheRelease pins a first run's order end to end: push
// HEAD, wait for the CI-made tag, fetch it — the memRepo resolves no tag before it is fetched, so
// a run that resolved it first would fail — then clone at the tag, build every archive and
// create the release with the seven assets in release order, and print every step done.
func TestPublishFirstRunConvergesTheTagThenTheRelease(t *testing.T) {
	t.Parallel()
	fake := newPublishFake()

	result := runTestPublish(t, fake, false)

	if result.err != nil {
		t.Fatalf("publish failed: %v\n%s", result.err, result.output)
	}
	wantOrder := []string{
		"repo PushMain",
		"repo FetchTag v0.24.11",
		"repo CloneAt v0.24.11 ",
		"go build", "go build", "go build", "go build", "go build", "go build",
		"repo CreateRelease v0.24.11 " + result.notesPath + " ",
	}
	mutating := mutatingLines(result.commands)
	if len(mutating) != len(wantOrder) {
		t.Fatalf("mutating commands = %q, want %d of them in the order %q", mutating, len(wantOrder), wantOrder)
	}
	for index, prefix := range wantOrder {
		if !strings.HasPrefix(mutating[index], prefix) {
			t.Errorf("mutating command %d = %q, want one starting %q", index, mutating[index], prefix)
		}
	}
	if result.pauses != 2 {
		t.Errorf("paused %d times, want 2: the tag appears on the third poll after the push", result.pauses)
	}
	wantAssets := releaseAssetNames(mustVersion(t, publishVersion))
	if created := fake.repo.releases[publishVersion]; !slices.Equal(created, wantAssets) {
		t.Errorf("release created with %q, want the %d assets %q", created, len(wantAssets), wantAssets)
	}
	for _, want := range []string{"tag      done  done by this run", "release  done  done by this run"} {
		if !hasRow(result.output, want) {
			t.Errorf("output lacks %q:\n%s", want, result.output)
		}
	}
}

// TestPublishPushesTheGatedCommit pins that the push names the commit the gate resolved HEAD to,
// never the symbolic HEAD, so what reaches origin/main is what the gate cleared.
func TestPublishPushesTheGatedCommit(t *testing.T) {
	t.Parallel()
	fake := newPublishFake()

	result := runTestPublish(t, fake, false)

	if result.err != nil {
		t.Fatalf("publish failed: %v\n%s", result.err, result.output)
	}
	want := repoCallName + " " + methodPushMain + " " + bumpCommit
	if lines := commandLines(result.commands); !slices.Contains(lines, want) {
		t.Errorf("commands = %q, want %q among them", lines, want)
	}
}

// TestPublishRefusesWhenHeadMovesAfterTheGate pins the refusal of a HEAD that resolves to another
// commit after the gate cleared it: the run fails naming both commits, pushes nothing and still
// prints the step table.
func TestPublishRefusesWhenHeadMovesAfterTheGate(t *testing.T) {
	t.Parallel()
	fake := newPublishFake()
	fake.repo.publish.headAfterGate = rollupCommit

	result := runTestPublish(t, fake, false)

	wantError := "HEAD moved from 300000000000 to 400000000000 after the gate cleared it: run publish again"
	if result.err == nil || !strings.Contains(result.err.Error(), wantError) {
		t.Fatalf("publish error = %v, want one naming %q", result.err, wantError)
	}
	if index := lineIndex(commandLines(result.commands), repoCallName+" "+methodPushMain); index >= 0 {
		t.Errorf("publish pushed after HEAD moved: %q", commandLines(result.commands))
	}
	if !strings.Contains(result.output, "steps") {
		t.Errorf("the step table was not printed:\n%s", result.output)
	}
}

// TestPublishChangesNothingWhenItHasNothingToDo pins the two runs that may change nothing: a
// second run over a finished release, and a dry run over an unstarted one, which still prints
// what each step would do.
func TestPublishChangesNothingWhenItHasNothingToDo(t *testing.T) {
	t.Parallel()

	finished := newPublishFake()
	finished.tagged()
	finished.repo.releases[publishVersion] = releaseAssetsV02411
	cases := map[string]struct {
		fake       *publishFake
		isDryRun   bool
		wantOutput []string
	}{
		"second run over a finished release": {
			fake:       finished,
			wantOutput: []string{"gate     not checked:", "tag      done  annotated tag v0.24.11 on origin", "release  done  release v0.24.11 with all 7 assets"},
		},
		"dry run over an unstarted release": {
			fake:       newPublishFake(),
			isDryRun:   true,
			wantOutput: []string{"gate     cleared at", "plan (dry run, nothing was changed)", "  tag      push HEAD to origin/main", "  release  build the 7 assets in a fresh clone at v0.24.11"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := runTestPublish(t, tc.fake, tc.isDryRun)

			if result.err != nil {
				t.Fatalf("publish failed: %v\n%s", result.err, result.output)
			}
			if mutating := mutatingLines(result.commands); len(mutating) > 0 {
				t.Errorf("issued mutating commands %q, want none", mutating)
			}
			for _, want := range tc.wantOutput {
				if !hasRow(result.output, want) {
					t.Errorf("output lacks %q:\n%s", want, result.output)
				}
			}
		})
	}
}

// TestPublishRefusesBeforeChangingAnything pins every refusal a run makes before its first step:
// each is named, nothing is changed, and the step table is still printed.
func TestPublishRefusesBeforeChangingAnything(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change    func(fake *publishFake)
		notesFile string
		wantError string
	}{
		"blocked gate":   {change: func(fake *publishFake) { fake.repo.isDirty = true }, wantError: "gate: the working tree is not clean"},
		"VERSION behind": {change: func(fake *publishFake) { fake.repo.files[versionFile] = "0.24.10\n" }, wantError: `VERSION at HEAD reads "0.24.10", not 0.24.11: run prepare first`},
		"lightweight tag on origin": {
			change:    func(fake *publishFake) { fake.repo.remoteTags[publishVersion] = "commit" },
			wantError: "could not tell where the tag step stands: v0.24.11 on origin is a lightweight tag",
		},
		"release unreadable": {
			change:    func(fake *publishFake) { fake.repo.failing = methodReleaseAssets },
			wantError: "could not tell where the release step stands",
		},
		"empty notes file":   {notesFile: "  \n", wantError: "is empty"},
		"missing notes file": {notesFile: "-", wantError: "read --notes-file"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fake := newPublishFake()
			if tc.change != nil {
				tc.change(fake)
			}
			notesPath := filepath.Join(t.TempDir(), "notes.md")
			switch tc.notesFile {
			case "":
				writeTestFile(t, notesPath, "- A release.\n")
			case "-":
			default:
				writeTestFile(t, notesPath, tc.notesFile)
			}

			result := runTestPublishWith(t, fake, publishOptions{notesFile: notesPath})

			if result.err == nil || !strings.Contains(result.err.Error(), tc.wantError) {
				t.Fatalf("publish error = %v, want one naming %q", result.err, tc.wantError)
			}
			if !strings.Contains(result.err.Error(), "nothing was changed") {
				t.Errorf("publish error = %v, want it to say nothing was changed", result.err)
			}
			if mutating := mutatingLines(result.commands); len(mutating) > 0 {
				t.Errorf("issued mutating commands %q, want none", mutating)
			}
			if !strings.Contains(result.output, "steps") {
				t.Errorf("the step table was not printed:\n%s", result.output)
			}
		})
	}
}

// TestPublishRefusesAVersionFileWithALeadingV pins the VERSION check's spelling: VERSION holds the
// bare X.Y.Z, and a "v0.24.11" VERSION — even one naming the release — is refused, as dist
// refuses it.
func TestPublishRefusesAVersionFileWithALeadingV(t *testing.T) {
	t.Parallel()
	fake := newPublishFake()
	fake.repo.files[versionFile] = publishVersion + "\n"

	result := runTestPublish(t, fake, true)

	want := `VERSION reads "v0.24.11": VERSION holds a bare X.Y.Z, with no leading v`
	if result.err == nil || !strings.Contains(result.err.Error(), want) {
		t.Fatalf("publish error = %v, want one containing %q\n%s", result.err, want, result.output)
	}
}

// TestPublishDryRunWithABlockedGateFailsButPrintsThePlan pins a dry run's exit status: a blocked
// gate fails it, yet every probe row and every pending step's plan is still printed.
func TestPublishDryRunWithABlockedGateFailsButPrintsThePlan(t *testing.T) {
	t.Parallel()
	fake := newPublishFake()
	fake.repo.isDirty = true

	result := runTestPublish(t, fake, true)

	if result.err == nil || !strings.Contains(result.err.Error(), "the working tree is not clean") {
		t.Fatalf("publish error = %v, want one naming the dirty tree", result.err)
	}
	for _, want := range []string{
		"gate     BLOCKED at",
		"  tag      pending  origin has no tag v0.24.11",
		"  release  pending  no GitHub release v0.24.11",
		"  tag      push HEAD to origin/main",
		"  release  build the 7 assets",
	} {
		if !hasRow(result.output, want) {
			t.Errorf("output lacks %q:\n%s", want, result.output)
		}
	}
	if mutating := mutatingLines(result.commands); len(mutating) > 0 {
		t.Errorf("issued mutating commands %q, want none", mutating)
	}
}

// TestPublishConvergesFromRemoteStateOnceTheTagExists pins the resume: once origin holds the
// annotated tag, only the gate's CI half is checked, over the tag's commit — its gate commit has a
// green run — while the working tree and VERSION at HEAD are not consulted: a later VERSION bump
// and a dirty tree, either of which refuses a run that consults them, do not stop the release
// being built from the tag and created — and nothing is pushed.
func TestPublishConvergesFromRemoteStateOnceTheTagExists(t *testing.T) {
	t.Parallel()
	fake := newPublishFake()
	fake.tagged()
	fake.repo.greenRuns = []string{featureCommit}
	fake.repo.isDirty = true
	fake.repo.files[versionFile] = "0.24.12\n"

	result := runTestPublish(t, fake, false)

	if result.err != nil {
		t.Fatalf("publish failed: %v\n%s", result.err, result.output)
	}
	if want := "gate CI only, the tag is on origin: cleared at " + short(featureCommit); !hasRow(result.output, want) {
		t.Errorf("output lacks %q:\n%s", want, result.output)
	}
	lines := commandLines(result.commands)
	for _, unwanted := range []string{"repo PushMain", "git push"} {
		if index := lineIndex(lines, unwanted); index >= 0 {
			t.Errorf("ran %q, want no push once the tag exists", lines[index])
		}
	}
	if lineIndex(lines, "repo CreateRelease v0.24.11") < 0 {
		t.Errorf("the release was not created; commands: %q", lines)
	}
}

// TestPublishRefusesARedCIOnTheTagCommitWhileTheReleaseIsPending pins the gate's CI half once the
// tag exists: with the tag on origin at the bump and no successful ci.yml run for its gate commit,
// publish refuses before building anything, naming ci.yml, and the gate row says only the CI half
// was judged.
func TestPublishRefusesARedCIOnTheTagCommitWhileTheReleaseIsPending(t *testing.T) {
	t.Parallel()
	fake := newPublishFake()
	fake.tagged()
	fake.repo.greenRuns = nil

	result := runTestPublish(t, fake, false)

	if result.err == nil {
		t.Fatalf("publish succeeded without a green CI run for the tag's commit:\n%s", result.output)
	}
	for _, want := range []string{"gate: " + ciWorkflow + " has no successful run for " + short(featureCommit), "nothing was changed"} {
		if !strings.Contains(result.err.Error(), want) {
			t.Errorf("publish error = %v, want it to name %q", result.err, want)
		}
	}
	if want := "gate CI only, the tag is on origin: BLOCKED at " + short(featureCommit); !hasRow(result.output, want) {
		t.Errorf("output lacks %q:\n%s", want, result.output)
	}
	if index := lineIndex(commandLines(result.commands), repoCallName+" "+methodCloneAt); index >= 0 {
		t.Errorf("cloned at the tag over a red CI run")
	}
	if mutating := mutatingLines(result.commands); len(mutating) > 0 {
		t.Errorf("issued mutating commands %q, want none", mutating)
	}
}

// TestPublishTagStepFailures pins each way the tag step fails after the push: no tag in time, a
// lightweight tag, a tag on the wrong commit. Each stops the run before the release step, with
// the step table printed.
func TestPublishTagStepFailures(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change     func(fake *publishFake)
		wantError  string
		wantPauses int
	}{
		"no tag in time": {
			change:     func(fake *publishFake) { fake.tagAppearsAfter = -1 },
			wantError:  "origin had no tag v0.24.11 after 30s",
			wantPauses: int(testTagTimeout / testPollPause),
		},
		"lightweight tag": {
			change:     func(fake *publishFake) { fake.tagKind = "commit" },
			wantError:  "v0.24.11 on origin is a lightweight tag; a release tag must be annotated",
			wantPauses: 2,
		},
		"tag on another commit": {
			change:     func(fake *publishFake) { fake.tagCommit = featureCommit },
			wantError:  "tag v0.24.11 is on 200000000000, not on the VERSION bump 300000000000",
			wantPauses: 2,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fake := newPublishFake()
			tc.change(fake)

			result := runTestPublish(t, fake, false)

			if result.err == nil || !strings.Contains(result.err.Error(), tc.wantError) {
				t.Fatalf("publish error = %v, want one naming %q", result.err, tc.wantError)
			}
			if result.pauses != tc.wantPauses {
				t.Errorf("paused %d times, want %d", result.pauses, tc.wantPauses)
			}
			if index := lineIndex(commandLines(result.commands), "repo CloneAt"); index >= 0 {
				t.Errorf("the release step ran after the tag step failed")
			}
			if !strings.Contains(result.output, "steps") {
				t.Errorf("the step table was not printed:\n%s", result.output)
			}
		})
	}
}

// TestPublishPartialReleaseUploadsOnlyTheMissingAssets pins the resume of a half-uploaded
// release: the rebuilt sums are checked against its SHA256SUMS when it has one, and only the
// assets it lacks are uploaded — never a create, never an asset it already carries.
func TestPublishPartialReleaseUploadsOnlyTheMissingAssets(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		uploaded             []string
		wantUploads          []string
		wantDownload         bool
		wantArchiveDownloads []string
	}{
		"two archives missing": {
			uploaded:     slices.Delete(slices.Clone(releaseAssetsV02411), 4, 6),
			wantUploads:  []string{"capcom_0.24.11_windows_amd64.zip", "capcom_0.24.11_windows_arm64.zip"},
			wantDownload: true,
		},
		"SHA256SUMS missing": {
			uploaded:             releaseAssetsV02411[:6],
			wantUploads:          []string{checksumsFile},
			wantArchiveDownloads: releaseAssetsV02411[:6],
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fake := newPublishFake()
			fake.tagged()
			fake.repo.releases[publishVersion] = tc.uploaded
			fake.repo.assetContent = rebuiltAssetContent(t)

			result := runTestPublish(t, fake, false)

			if result.err != nil {
				t.Fatalf("publish failed: %v\n%s", result.err, result.output)
			}
			var uploads []recordedCall
			for _, call := range result.commands.recorded(repoCallName) {
				switch call.args[0] {
				case methodCreateRelease:
					t.Errorf("created the release %q, want only an upload to the existing one", call.args)
				case methodUploadAssets:
					uploads = append(uploads, call)
				}
			}
			if len(uploads) != 1 {
				t.Fatalf("got %d UploadAssets calls, want 1", len(uploads))
			}
			args := uploads[0].args
			if args[1] != publishVersion {
				t.Errorf("UploadAssets args = %q, want them to name release %s", args, publishVersion)
			}
			if names := assetNames(args[2:]); !slices.Equal(names, tc.wantUploads) {
				t.Errorf("uploaded %q, want only %q", names, tc.wantUploads)
			}
			isDownloaded := lineIndex(commandLines(result.commands), "repo ReadAsset "+publishVersion+" "+checksumsFile) >= 0
			if isDownloaded != tc.wantDownload {
				t.Errorf("downloaded SHA256SUMS = %t, want %t", isDownloaded, tc.wantDownload)
			}
			downloads := downloadedAssets(result.commands)
			slices.Sort(downloads)
			if !slices.Equal(downloads, tc.wantArchiveDownloads) {
				t.Errorf("downloaded archives %q, want %q", downloads, tc.wantArchiveDownloads)
			}
		})
	}
}

// downloadedAssets returns the name of each asset a run's DownloadAsset calls fetched, in order.
func downloadedAssets(commands *fakeRunner) []string {
	var names []string
	for _, call := range commands.recorded(repoCallName) {
		if call.args[0] == methodDownloadAsset {
			names = append(names, call.args[2])
		}
	}
	return names
}

// TestPublishUndraftsADraftReleaseOnce pins the run over a draft an interrupted `gh release
// create` left: it creates no release, uploads only the assets the draft lacks, then publishes
// the draft exactly once, after the upload, so the release step then probes done.
func TestPublishUndraftsADraftReleaseOnce(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		uploaded    []string
		wantUploads []string
	}{
		"draft with all assets": {
			uploaded: releaseAssetsV02411,
		},
		"draft missing two archives": {
			uploaded:    slices.Delete(slices.Clone(releaseAssetsV02411), 4, 6),
			wantUploads: []string{"capcom_0.24.11_windows_amd64.zip", "capcom_0.24.11_windows_arm64.zip"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fake := newPublishFake()
			fake.tagged()
			fake.repo.releases[publishVersion] = tc.uploaded
			fake.repo.drafts = map[string]bool{publishVersion: true}
			fake.repo.assetContent[checksumsFile] = rebuiltSums(t)

			result := runTestPublish(t, fake, false)

			if result.err != nil {
				t.Fatalf("publish failed: %v\n%s", result.err, result.output)
			}
			lines := commandLines(result.commands)
			var undrafts, uploads []string
			for _, line := range lines {
				switch {
				case strings.HasPrefix(line, repoCallName+" "+methodCreateRelease):
					t.Errorf("created the release %q, want only the draft published", line)
				case strings.HasPrefix(line, repoCallName+" "+methodUploadAssets):
					uploads = append(uploads, line)
				case strings.HasPrefix(line, repoCallName+" "+methodUndraftRelease):
					undrafts = append(undrafts, line)
				}
			}
			if want := []string{"repo UndraftRelease " + publishVersion}; !slices.Equal(undrafts, want) {
				t.Errorf("un-drafts = %q, want exactly %q", undrafts, want)
			}
			if tc.wantUploads == nil {
				if len(uploads) > 0 {
					t.Errorf("uploaded %q to a draft that carries every asset, want no upload", uploads)
				}
			} else {
				if len(uploads) != 1 {
					t.Fatalf("got UploadAssets calls %q, want 1", uploads)
				}
				if names := assetNames(strings.Fields(uploads[0])[3:]); !slices.Equal(names, tc.wantUploads) {
					t.Errorf("uploaded %q, want only %q", names, tc.wantUploads)
				}
				if lineIndex(lines, "repo UploadAssets") > lineIndex(lines, "repo UndraftRelease") {
					t.Errorf("published the draft before uploading the assets it lacks: %q", lines)
				}
			}
			if !hasRow(result.output, "release  done  done by this run") {
				t.Errorf("output lacks the release row done:\n%s", result.output)
			}
			env := probeEnv{repo: fake.repo, commands: result.commands, dir: t.TempDir()}
			if step := probeRelease(t.Context(), env, mustVersion(t, publishVersion)); step.State != stateDone {
				t.Errorf("release probes %s (%s) after the run, want done", step.State, step.Detail)
			}
		})
	}
}

// TestPublishRefusesARebuildThatDoesNotMatchThePublishedSums pins the sums check: an uploaded
// archive whose published hash differs from the rebuilt one stops the run before any upload, and
// the failure names the way out.
func TestPublishRefusesARebuildThatDoesNotMatchThePublishedSums(t *testing.T) {
	t.Parallel()
	fake := newPublishFake()
	fake.tagged()
	fake.repo.releases[publishVersion] = slices.Delete(slices.Clone(releaseAssetsV02411), 4, 6)
	sums := rebuiltSums(t)
	linuxHash, err := checksumFor(sums, "capcom_0.24.11_linux_amd64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	fake.repo.assetContent[checksumsFile] = strings.Replace(sums, linuxHash, strings.Repeat("0", len(linuxHash)), 1)

	result := runTestPublish(t, fake, false)

	if result.err == nil {
		t.Fatalf("publish succeeded over a mismatched SHA256SUMS:\n%s", result.output)
	}
	for _, want := range []string{"capcom_0.24.11_linux_amd64.tar.gz", "gh release delete-asset v0.24.11"} {
		if !strings.Contains(result.err.Error(), want) {
			t.Errorf("publish error = %v, want it to name %q", result.err, want)
		}
	}
	if index := lineIndex(commandLines(result.commands), "repo UploadAssets"); index >= 0 {
		t.Errorf("uploaded assets over a mismatched SHA256SUMS")
	}
}

// TestPublishRefusesAMissingArchiveThePublishedSumsDoNotMatch pins the sums check over the
// archives a release lacks: one whose published hash differs from the rebuilt one stops the run
// before any upload, and the way out names only SHA256SUMS, the one stale asset the release holds.
func TestPublishRefusesAMissingArchiveThePublishedSumsDoNotMatch(t *testing.T) {
	t.Parallel()
	const missing = "capcom_0.24.11_windows_amd64.zip"
	fake := newPublishFake()
	fake.tagged()
	fake.repo.releases[publishVersion] = slices.Delete(slices.Clone(releaseAssetsV02411), 4, 6)
	sums := rebuiltSums(t)
	missingHash, err := checksumFor(sums, missing)
	if err != nil {
		t.Fatal(err)
	}
	fake.repo.assetContent[checksumsFile] = strings.Replace(sums, missingHash, strings.Repeat("0", len(missingHash)), 1)

	result := runTestPublish(t, fake, false)

	if result.err == nil {
		t.Fatalf("publish succeeded over a SHA256SUMS that does not match a missing archive:\n%s", result.output)
	}
	for _, want := range []string{missing, "gh release delete-asset v0.24.11 <name>` (" + checksumsFile + ")"} {
		if !strings.Contains(result.err.Error(), want) {
			t.Errorf("publish error = %v, want it to name %q", result.err, want)
		}
	}
	if index := lineIndex(commandLines(result.commands), "repo UploadAssets"); index >= 0 {
		t.Errorf("uploaded assets beside a SHA256SUMS that does not match them")
	}
}

// TestPublishRefusesUploadedArchivesTheRebuildDoesNotMatch pins the check of a release without
// SHA256SUMS: an uploaded archive that does not hash to the rebuilt one stops the run before the
// rebuilt sums or any archive is uploaded, and the way out names only that archive — never
// SHA256SUMS, which the release lacks.
func TestPublishRefusesUploadedArchivesTheRebuildDoesNotMatch(t *testing.T) {
	t.Parallel()
	const stale, matching = "capcom_0.24.11_linux_amd64.tar.gz", "capcom_0.24.11_darwin_arm64.tar.gz"
	fake := newPublishFake()
	fake.tagged()
	fake.repo.releases[publishVersion] = []string{matching, stale}
	fake.repo.assetContent = rebuiltAssetContent(t)
	fake.repo.assetContent[stale] += "an earlier build"

	result := runTestPublish(t, fake, false)

	if result.err == nil {
		t.Fatalf("publish succeeded beside an uploaded archive the rebuild does not match:\n%s", result.output)
	}
	message := result.err.Error()
	for _, want := range []string{stale, "gh release delete-asset v0.24.11 <name>` (" + stale + ")"} {
		if !strings.Contains(message, want) {
			t.Errorf("publish error = %v, want it to name %q", result.err, want)
		}
	}
	for _, unwanted := range []string{matching, checksumsFile} {
		if strings.Contains(message, unwanted) {
			t.Errorf("publish error = %v, want it not to name %q", result.err, unwanted)
		}
	}
	if index := lineIndex(commandLines(result.commands), "repo UploadAssets"); index >= 0 {
		t.Errorf("uploaded assets beside an archive the rebuild does not match")
	}
}

// TestPublishCommandLine pins the verb's command-line contract: --notes-file is required (a usage
// error, exit 2), and a malformed version is a refused run (exit 1) that probes nothing.
func TestPublishCommandLine(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args      []string
		isUsage   bool
		wantError string
	}{
		"no notes file":     {args: []string{"publish", publishVersion}, isUsage: true, wantError: "notes-file"},
		"malformed version": {args: []string{"publish", "v0.24", "--notes-file", "notes.md"}, wantError: "is not a release version"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			commands := newPublishFake().runner()

			_, err := execute(t, commands, tc.args...)

			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("publish error = %v, want one naming %q", err, tc.wantError)
			}
			var failed runError
			if isRunFailure := errors.As(err, &failed); isRunFailure == tc.isUsage {
				t.Errorf("run failure = %t, want %t", isRunFailure, !tc.isUsage)
			}
			if calls := commandLines(commands); len(calls) > 0 {
				t.Errorf("ran %q, want nothing", calls)
			}
		})
	}
}

// TestSleepContextStopsWhenTheContextEnds pins the production pause: a cancelled context ends it
// with the context's error rather than waiting out the duration.
func TestSleepContextStopsWhenTheContextEnds(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := sleepContext(ctx, time.Hour)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("sleepContext = %v, want %v", err, context.Canceled)
	}
}
