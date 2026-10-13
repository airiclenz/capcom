package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// How long publish waits for the CI-made tag after pushing, and how often it looks. The
// tag-on-version-bump workflow tags a pushed VERSION bump within a minute or two; ten minutes
// leaves room for a queued runner.
const (
	defaultTagTimeout = 10 * time.Minute
	tagPollInterval   = 10 * time.Second
)

// publishOptions is what the publish verb was asked to do.
type publishOptions struct {
	// dir is the checkout publish pushes from and reads origin through.
	dir string
	// version is the release to publish, as typed: publish parses it itself, so a malformed one
	// is a refused run (exit 1) rather than a usage error.
	version string
	// notesFile is the release notes the GitHub release is created with.
	notesFile string
	isDryRun  bool
	// tagTimeout bounds the wait for the CI-made tag; pollInterval is the pause between looks,
	// taken through sleep.
	tagTimeout   time.Duration
	pollInterval time.Duration
	sleep        func(ctx context.Context, duration time.Duration) error
	// getenv reads the environment: the winget step takes its GitHub token from it first.
	getenv func(key string) string
	// smoke runs the smoke verb over options: the last step, run whatever the probes say.
	smoke func(ctx context.Context, output io.Writer, options smokeOptions) error
	// openRepo opens the checkout's repo; nil opens the execRepo over the run's runner.
	openRepo repoOpener
}

// publishRun is one publish run: what every step reads.
type publishRun struct {
	commands runner
	output   io.Writer
	options  publishOptions
	version  releaseVersion
	// repo is the checkout publish pushes from, opened from the options.
	repo repo
	// head is the commit HEAD resolved to when the gate judged it: VERSION is read at it, and the
	// tag step checks, pushes and verifies it, never the symbolic HEAD. It stays empty when the
	// whole gate did not run, as when the tag is already on origin, or HEAD did not resolve.
	head string
	// notesPath is the notes file as an absolute path, so it reads the same from any directory.
	notesPath string
	// buildDir is the temporary directory the release step builds in, removed when the run ends;
	// distDir is the directory in it holding the built assets, which the smoke step compares
	// against. Both stay empty when this run builds nothing.
	buildDir string
	distDir  string
	// isWingetPackageListed is whether winget-pkgs already has the package, resolved once before
	// any step runs while the winget step is pending: the winget step updates a listed package
	// with `komac update` and submits the first manifests of an unlisted one with `komac submit`.
	isWingetPackageListed bool
}

// publishAction is one step publish converges: what a dry run says it would do, the work itself,
// run only while the step's probe reports it pending, and what the step table says once it has
// run ("done by this run" when empty). A converge that returns a setupPendingError has not
// failed: the step is reported setup-pending and the run goes on.
type publishAction struct {
	name       string
	describe   func(run *publishRun) string
	converge   func(ctx context.Context, run *publishRun) error
	doneDetail string
}

// publishActions are the steps publish converges, in the order it runs them; each one's probe
// is the publishSteps entry of the same name, so what status reports done is what publish skips.
// The channel steps (channels.go) run after the release, whose SHA256SUMS they render from, and
// the smoke step (smoke.go) runs last, over everything the others published.
var publishActions = []publishAction{
	{name: stepTag, describe: describeTag, converge: convergeTag},
	{name: stepRelease, describe: describeRelease, converge: convergeRelease},
	{name: stepHomebrew, describe: describeHomebrew, converge: convergeHomebrew},
	{name: stepScoop, describe: describeScoop, converge: convergeScoop},
	{name: stepWinget, describe: describeWinget, converge: convergeWinget, doneDetail: wingetSubmittedDetail},
	{name: stepSmoke, describe: describeSmoke, converge: convergeSmoke, doneDetail: smokePassedDetail},
}

// smokePassedDetail is the step table's line for a smoke test that passed.
const smokePassedDetail = "passed by this run: no check failed"

// newPublishCommand builds the publish verb.
func newPublishCommand(commands runner, openRepo repoOpener) *cobra.Command {
	options := publishOptions{
		dir:          ".",
		tagTimeout:   defaultTagTimeout,
		pollInterval: tagPollInterval,
		sleep:        sleepContext,
		getenv:       os.Getenv,
		openRepo:     openRepo,
	}
	options.smoke = func(ctx context.Context, output io.Writer, smoke smokeOptions) error {
		return runSmoke(ctx, commands, output, smoke)
	}

	cmd := &cobra.Command{
		Use:   "publish vX.Y.Z --notes-file F",
		Short: "Publish a prepared release: push, wait for the tag, create the GitHub release, update the channels",
		Long: "publish converges each publish step, skipping every step its probe reports done, so\n" +
			"a half-finished release is resumed by running it again. While the tag is not on origin\n" +
			"it refuses unless the release gate clears and VERSION at HEAD reads the version; it\n" +
			"then pushes the commit the gate judged to main, refusing and pushing nothing if HEAD\n" +
			"has moved since, and waits (--tag-timeout) for the CI-made annotated tag — it never\n" +
			"creates one. Once the tag is on origin and the release is not done, it refuses unless\n" +
			"ci.yml has a successful run for the tag's gate commit (the tag's commit with its\n" +
			"trailing CHANGELOG/VERSION-only commits peeled off) or a peeled commit origin/main\n" +
			"contains; the working tree, HEAD and VERSION are not consulted. Once the release is\n" +
			"done too, the gate is not consulted. The release step builds the archives in a fresh\n" +
			"clone at the tag and creates the GitHub release, or uploads only the assets it lacks\n" +
			"after checking the rebuilt sums against its SHA256SUMS, then publishes a draft an\n" +
			"interrupted run left; a draft release is never done. The homebrew and scoop steps render the formula\n" +
			"and the manifest from the release's SHA256SUMS and commit `capcom X.Y.Z` straight to\n" +
			"the tap and the bucket, pushing only when the file changes. The winget step submits\n" +
			"with komac (GITHUB_TOKEN, else `gh auth token`): `komac update` once winget-pkgs has\n" +
			"the package, and before that `komac submit` of the three manifests it renders from the\n" +
			"SHA256SUMS; either pull request merges later, so the release is submitted to winget,\n" +
			"not published there. A channel whose one-time setup is pending — a missing tap or\n" +
			"bucket repository — is skipped and named; a first `komac submit` that fails leaves\n" +
			"winget setup-pending, naming the kept manifest directory and the command to rerun.\n" +
			"Either way the run still succeeds. Last, the smoke step checks the published release from the outside, as\n" +
			"the smoke verb does, comparing its SHA256SUMS with this run's build when it made one; a\n" +
			"failed check fails the run. It never prompts or reads stdin, and exits 1 after\n" +
			"printing the step table when anything fails. --dry-run runs the gate and the probes\n" +
			"and prints what each pending step would do, changing nothing.",
		Args: cobra.ExactArgs(1),
		RunE: runE(func(cmd *cobra.Command, args []string) error {
			options.version = args[0]
			return runPublish(cmd.Context(), commands, cmd.OutOrStdout(), options)
		}),
	}
	cmd.Flags().StringVar(&options.notesFile, "notes-file", "", "the release notes file (required)")
	cmd.Flags().BoolVar(&options.isDryRun, "dry-run", false, "print what each pending step would do, changing nothing")
	cmd.Flags().DurationVar(&options.tagTimeout, "tag-timeout", defaultTagTimeout, "how long to wait for the CI-made tag after pushing")
	if err := cmd.MarkFlagRequired("notes-file"); err != nil {
		panic(err) // the flag is declared just above; failing here is a defect of this program
	}
	return cmd
}

// runPublish probes every step, refuses before changing anything when a precondition fails, and
// then converges each pending step in order. The step table is printed whatever happens.
func runPublish(ctx context.Context, commands runner, output io.Writer, options publishOptions) error {
	version, err := parseReleaseVersion(options.version)
	if err != nil {
		return err
	}
	run := &publishRun{
		commands: commands,
		repo:     options.openRepo.open(commands, options.dir),
		output:   output,
		options:  options,
		version:  version,
	}

	var refusals []error
	run.notesPath, err = checkNotesFile(options.notesFile)
	if err != nil {
		refusals = append(refusals, err)
	}

	steps := run.probe(ctx)
	var gate string
	gate, refusals = run.checkGateFor(ctx, steps, refusals)
	if err := run.resolveWingetPackage(ctx, steps); err != nil {
		refusals = append(refusals, err)
	}
	for _, step := range steps {
		if step.State == stateUnknown {
			refusals = append(refusals, fmt.Errorf("could not tell where the %s step stands: %s", step.Name, step.Detail))
		}
	}

	if options.isDryRun {
		if err := printPublish(output, run, gate, steps, true); err != nil {
			return err
		}
		return errors.Join(refusals...)
	}
	if len(refusals) > 0 {
		if err := printPublish(output, run, gate, steps, false); err != nil {
			return err
		}
		return fmt.Errorf("refusing to publish %s, nothing was changed:\n%w", version, errors.Join(refusals...))
	}
	return run.converge(ctx, gate, steps)
}

// converge runs every pending step in order; a done or setup-pending step is skipped, the latter
// named, since the run succeeds without it. A step whose converge reports its setup pending is
// recorded so and the run goes on. A failed step stops the run, which prints the steps as the
// remote now holds them.
func (r *publishRun) converge(ctx context.Context, gate string, steps []stepReport) error {
	defer r.removeBuildDir()
	for index, action := range publishActions {
		if steps[index].State == stateSetupPending {
			_, _ = fmt.Fprintf(r.output, "%s: skipped, its one-time setup is pending: %s\n", action.name, steps[index].Detail)
		}
		if steps[index].State != statePending {
			continue
		}
		_, _ = fmt.Fprintf(r.output, "%s: %s\n", action.name, action.describe(r))
		err := action.converge(ctx, r)
		var setupPending setupPendingError
		if errors.As(err, &setupPending) {
			_, _ = fmt.Fprintf(r.output, "%s: its one-time setup is pending: %s\n", action.name, setupPending.detail)
			steps[index] = stepReport{Name: action.name, State: stateSetupPending, Detail: setupPending.detail}
			continue
		}
		if err != nil {
			if printErr := printPublish(r.output, r, gate, r.probe(ctx), false); printErr != nil {
				return errors.Join(err, printErr)
			}
			return fmt.Errorf("the %s step failed: %w", action.name, err)
		}
		detail := action.doneDetail
		if detail == "" {
			detail = "done by this run"
		}
		steps[index] = stepReport{Name: action.name, State: stateDone, Detail: detail}
	}
	return printPublish(r.output, r, gate, steps, false)
}

// probe asks the remote where each of publish's steps stands, in publish order.
func (r *publishRun) probe(ctx context.Context) []stepReport {
	env := probeEnv{repo: r.repo, commands: r.commands, dir: r.options.dir}
	steps := make([]stepReport, 0, len(publishActions))
	for _, action := range publishActions {
		index := slices.IndexFunc(publishSteps, func(step publishStep) bool { return step.name == action.name })
		steps = append(steps, publishSteps[index].probe(ctx, env, r.version))
	}
	return steps
}

// checkGateFor judges as much of the release gate as the steps still need, and returns the
// gate's line for the table and refusals with every blocking condition added. While the tag is
// not on origin, the whole gate and VERSION at HEAD are judged (checkPushable). Once it is and the
// release is not done, only the CI half is, over the tag's commit (checkTagCI): the release is
// built from the tag, so the checkout's tree, HEAD and VERSION no longer matter, but a tag whose
// commit never passed CI must not be released. With the release done too, the gate is not
// consulted: the channels and the smoke test converge from the published release.
func (r *publishRun) checkGateFor(ctx context.Context, steps []stepReport, refusals []error) (string, []error) {
	if stepStateOf(steps, stepTag) != stateDone {
		return r.checkPushable(ctx, refusals)
	}
	if stepStateOf(steps, stepRelease) != stateDone {
		report := checkTagCI(ctx, r.repo, r.version.String())
		return "CI only, the tag is on " + remoteName + ": " + gateRow(report), gateRefusals(report, refusals)
	}
	return fmt.Sprintf("not checked: %s is on %s and released, so the rest converges from remote state",
		r.version, remoteName), refusals
}

// stepStateOf returns the state of the step named name among steps, stateUnknown when it is not
// there.
func stepStateOf(steps []stepReport, name string) stepState {
	index := slices.IndexFunc(steps, func(step stepReport) bool { return step.Name == name })
	if index < 0 {
		return stateUnknown
	}
	return steps[index].State
}

// gateRow is the step table's verdict on a gate report: cleared or BLOCKED, at the gate commit
// when it was found.
func gateRow(report gateReport) string {
	row := "cleared"
	if !report.IsOK {
		row = "BLOCKED"
	}
	if report.Commit != "" {
		row += " at " + short(report.Commit)
	}
	return row
}

// gateRefusals returns refusals with each of the report's failures added as a gate refusal.
func gateRefusals(report gateReport, refusals []error) []error {
	for _, failure := range report.Failures {
		refusals = append(refusals, fmt.Errorf("gate: %s", failure))
	}
	return refusals
}

// checkPushable judges what pushing the gated commit needs: the release gate, and VERSION at the
// commit the gate resolved HEAD to reading the version — the bump the CI workflow tags. It keeps
// that commit as the run's head, and returns the gate's line for the table and refusals with
// every blocking condition added.
func (r *publishRun) checkPushable(ctx context.Context, refusals []error) (string, []error) {
	report := checkGate(ctx, r.repo)
	r.head = report.Head
	gate := gateRow(report)
	refusals = gateRefusals(report, refusals)

	// A HEAD that did not resolve is already a gate failure, and leaves no commit to read at.
	if r.head == "" {
		return gate, refusals
	}
	raw, err := r.repo.ShowFile(ctx, r.head, versionFile)
	if err != nil {
		return gate, append(refusals, fmt.Errorf("read %s at HEAD: %w", versionFile, err))
	}
	// VERSION holds the bare X.Y.Z; a leading "v" is tolerated, as dist tolerates it.
	bare := bareRelease(r.version)
	if current := strings.TrimSpace(raw); strings.TrimPrefix(current, "v") != bare {
		return gate, append(refusals, fmt.Errorf("%s at HEAD reads %q, not %s: run prepare first", versionFile, current, bare))
	}
	return gate, refusals
}

// checkNotesFile requires the notes file to be readable and to hold some text, and returns its
// absolute path.
func checkNotesFile(notesFile string) (string, error) {
	notesPath, err := filepath.Abs(notesFile)
	if err != nil {
		return "", fmt.Errorf("resolve --notes-file: %w", err)
	}
	notes, err := os.ReadFile(notesPath)
	if err != nil {
		return "", fmt.Errorf("read --notes-file: %w", err)
	}
	if strings.TrimSpace(string(notes)) == "" {
		return "", fmt.Errorf("--notes-file %s is empty", notesFile)
	}
	return notesPath, nil
}

// printPublish writes the step table: the version, the gate, and each step's state. A dry run
// adds what each step would do.
func printPublish(output io.Writer, run *publishRun, gate string, steps []stepReport, isDryRun bool) error {
	table := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(table, "version\t%s\n", run.version)
	_, _ = fmt.Fprintf(table, "gate\t%s\n", gate)
	_, _ = fmt.Fprintln(table, "steps")
	for _, step := range steps {
		_, _ = fmt.Fprintf(table, "  %s\t%s\t%s\n", step.Name, step.State, step.Detail)
	}
	if isDryRun {
		_, _ = fmt.Fprintln(table, "plan (dry run, nothing was changed)")
		for index, action := range publishActions {
			plan := "skip: " + string(steps[index].State)
			if steps[index].State == statePending {
				plan = action.describe(run)
			}
			_, _ = fmt.Fprintf(table, "  %s\t%s\n", action.name, plan)
		}
	}
	if err := table.Flush(); err != nil {
		return fmt.Errorf("write the step table: %w", err)
	}
	return nil
}

// describeTag says what the tag step does.
func describeTag(run *publishRun) string {
	return fmt.Sprintf("push HEAD to %s when it is behind, then wait up to %s for the CI-made annotated tag %s at the %s bump",
		remoteMain, run.options.tagTimeout, run.version, versionFile)
}

// convergeTag pushes the commit the gate judged to main when origin/main is behind it, waits for
// the tag-on-version-bump workflow to tag the bump, and then requires that tag annotated and on
// the commit that set VERSION. HEAD resolving to another commit than the gated one refuses the
// step before anything is pushed: a commit made after the gate cleared was never judged. It never
// creates a tag: the workflow is the one tagger.
func convergeTag(ctx context.Context, run *publishRun) error {
	current, err := run.repo.Resolve(ctx, "HEAD")
	if err != nil {
		return fmt.Errorf("resolve HEAD: %w", err)
	}
	if current != run.head {
		return fmt.Errorf("HEAD moved from %s to %s after the gate cleared it: run publish again",
			short(run.head), short(current))
	}

	isPushed, err := run.repo.IsAncestor(ctx, run.head, remoteMain)
	if err != nil {
		return fmt.Errorf("check whether %s holds %s: %w", remoteMain, short(run.head), err)
	}
	if !isPushed {
		if err := run.repo.PushMain(ctx, run.head); err != nil {
			return fmt.Errorf("push %s to %s: %w", short(run.head), remoteMain, err)
		}
	}
	if err := waitForTag(ctx, run); err != nil {
		return err
	}
	return verifyTag(ctx, run)
}

// waitForTag polls origin for the version's tag until it appears or the timeout passes.
func waitForTag(ctx context.Context, run *publishRun) error {
	tag := run.version.String()
	for waited := time.Duration(0); ; waited += run.options.pollInterval {
		hasTag, err := run.repo.RemoteHasTag(ctx, tag)
		if err != nil {
			return fmt.Errorf("list %s's tags: %w", remoteName, err)
		}
		if hasTag {
			return nil
		}
		if waited >= run.options.tagTimeout {
			return fmt.Errorf("%s had no tag %s after %s: check the tag-on-version-bump workflow's run for the push, then run publish again",
				remoteName, run.version, run.options.tagTimeout)
		}
		if err := run.options.sleep(ctx, run.options.pollInterval); err != nil {
			return fmt.Errorf("wait for tag %s: %w", run.version, err)
		}
	}
}

// verifyTag fetches the tag — on a first run the workflow made it after the gate's fetch, so it
// is not local yet — and requires it annotated and on the last commit at or before the gated
// commit that changed VERSION.
func verifyTag(ctx context.Context, run *publishRun) error {
	tag := run.version.String()
	if err := run.repo.FetchTag(ctx, tag); err != nil {
		return fmt.Errorf("fetch tag %s: %w", tag, err)
	}

	isAnnotated, err := run.repo.IsAnnotated(ctx, tag)
	if err != nil {
		return fmt.Errorf("read the type of tag %s: %w", tag, err)
	}
	if !isAnnotated {
		return fmt.Errorf("%s on %s is a lightweight tag; a release tag must be annotated", tag, remoteName)
	}

	tagged, err := run.repo.Resolve(ctx, tag)
	if err != nil {
		return fmt.Errorf("resolve tag %s: %w", tag, err)
	}
	bump, err := run.repo.LastCommitTouching(ctx, run.head, versionFile)
	if err != nil {
		return fmt.Errorf("find the %s bump commit: %w", versionFile, err)
	}
	if tagged != bump {
		return fmt.Errorf("tag %s is on %s, not on the %s bump %s", tag, short(tagged), versionFile, short(bump))
	}
	return nil
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

// removeBuildDir removes the release step's build directory, if this run made one.
func (r *publishRun) removeBuildDir() {
	if r.buildDir != "" {
		_ = os.RemoveAll(r.buildDir)
	}
}

// describeSmoke says what the smoke step does.
func describeSmoke(run *publishRun) string {
	return fmt.Sprintf("smoke-test %s as published, comparing its %s with this run's build when it made one",
		run.version, checksumsFile)
}

// convergeSmoke runs the smoke verb over the published release, handing it this run's build
// directory so the published SHA256SUMS is compared against what the run itself built.
func convergeSmoke(ctx context.Context, run *publishRun) error {
	options := defaultSmokeOptions(run.options.dir)
	options.openRepo = run.options.openRepo
	options.version = run.version.String()
	options.distDir = run.distDir
	return run.options.smoke(ctx, run.output, options)
}

// assetPaths joins each asset name onto outDir.
func assetPaths(outDir string, names []string) []string {
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, filepath.Join(outDir, name))
	}
	return paths
}

// sleepContext waits for duration or until ctx is done, whichever is first.
func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
