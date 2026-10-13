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
	for index, step := range publishSteps {
		if steps[index].State == stateSetupPending {
			_, _ = fmt.Fprintf(r.output, "%s: skipped, its one-time setup is pending: %s\n", step.name, steps[index].Detail)
		}
		if steps[index].State != statePending {
			continue
		}
		_, _ = fmt.Fprintf(r.output, "%s: %s\n", step.name, step.describe(r))
		err := step.converge(ctx, r)
		var setupPending setupPendingError
		if errors.As(err, &setupPending) {
			_, _ = fmt.Fprintf(r.output, "%s: its one-time setup is pending: %s\n", step.name, setupPending.detail)
			steps[index] = stepReport{Name: step.name, State: stateSetupPending, Detail: setupPending.detail}
			continue
		}
		if err != nil {
			if printErr := printPublish(r.output, r, gate, r.probe(ctx), false); printErr != nil {
				return errors.Join(err, printErr)
			}
			return fmt.Errorf("the %s step failed: %w", step.name, err)
		}
		detail := step.doneDetail
		if detail == "" {
			detail = "done by this run"
		}
		steps[index] = stepReport{Name: step.name, State: stateDone, Detail: detail}
	}
	return printPublish(r.output, r, gate, steps, false)
}

// probe asks the remote where each of publish's steps stands, in publish order.
func (r *publishRun) probe(ctx context.Context) []stepReport {
	env := probeEnv{repo: r.repo, commands: r.commands, dir: r.options.dir}
	return probeSteps(ctx, env, r.version)
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
	bare := bareRelease(r.version)
	current := strings.TrimSpace(raw)
	if strings.HasPrefix(current, "v") {
		return gate, append(refusals, fmt.Errorf("%s reads %q: %s holds a bare X.Y.Z, with no leading v", versionFile, current, versionFile))
	}
	if current != bare {
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
		for index, step := range publishSteps {
			plan := "skip: " + string(steps[index].State)
			if steps[index].State == statePending {
				plan = step.describe(run)
			}
			_, _ = fmt.Fprintf(table, "  %s\t%s\n", step.name, plan)
		}
	}
	if err := table.Flush(); err != nil {
		return fmt.Errorf("write the step table: %w", err)
	}
	return nil
}

// removeBuildDir removes the release step's build directory, if this run made one.
func (r *publishRun) removeBuildDir() {
	if r.buildDir != "" {
		_ = os.RemoveAll(r.buildDir)
	}
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
