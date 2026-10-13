package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// gitObjectTag is the object type GitHub and `git cat-file -t` report for an annotated tag; a
// lightweight tag resolves straight to a commit.
const gitObjectTag = "tag"

// nonInteractiveEnv keeps git and gh from prompting on the terminal: the release verbs never
// prompt, so a missing credential fails the command instead of waiting on a human.
var nonInteractiveEnv = []string{"GIT_TERMINAL_PROMPT=0", "GH_PROMPT_DISABLED=1"}

// versionArgument is the subcommand a capcom binary reports its version under; it prints
// "capcom X.Y.Z" with the bare version.
const versionArgument = "version"

// The build settings `-buildvcs` stamps into every release binary: the commit it was built from
// and whether that tree was dirty. They — not the archive bytes — tie a published asset to the tag.
const (
	vcsRevisionSetting = "vcs.revision"
	vcsModifiedSetting = "vcs.modified"
)

// smokeOptions is what the smoke verb was asked to check.
type smokeOptions struct {
	// dir is the checkout whose repo the tag, asset, build-stamp and bucket checks read.
	dir string
	// version is the release to check, as vX.Y.Z; empty reads the checkout's VERSION file.
	version string
	// distDir holds a local build of the release whose SHA256SUMS must equal the published one;
	// empty skips that check.
	distDir string
	// goos and goarch pick the host's archive, the one whose binary is run.
	goos   string
	goarch string
	// readBuildInfo reads the build information embedded in the binary at path.
	readBuildInfo func(path string) (*buildinfo.BuildInfo, error)
	// openRepo opens the checkout's repo; nil opens the execRepo over the run's runner.
	openRepo repoOpener
	// channels is the package channels' repositories; nil is the execChannelRepos over the run's
	// runner in dir.
	channels channelRepos
}

// defaultSmokeOptions checks the checkout at dir from this host, reading build stamps with
// debug/buildinfo — which reads a binary of any platform.
func defaultSmokeOptions(dir string) smokeOptions {
	return smokeOptions{dir: dir, goos: runtime.GOOS, goarch: runtime.GOARCH, readBuildInfo: buildinfo.ReadFile}
}

// smokeRun is one smoke run: what every check reads — the tag, the release's assets and the
// latest release through repo, the bucket's manifest through channels — and the tally the
// summary reports.
type smokeRun struct {
	commands runner
	repo     repo
	channels channelRepos
	output   io.Writer
	options  smokeOptions
	version  releaseVersion
	// workDir holds the downloads, outside the checkout.
	workDir string
	// sums is the release's SHA256SUMS once read; sumsErr is why it could not be.
	sums       string
	sumsErr    error
	isSumsRead bool
	// isDownloaded is set once all six archives downloaded and matched SHA256SUMS.
	isDownloaded bool
	// verified names the archives that downloaded and matched SHA256SUMS: the only ones a check
	// may extract or run. rejected says, by archive name, why one that downloaded was deleted
	// unverified.
	verified map[string]bool
	rejected map[string]string

	passed  int
	failed  int
	skipped int
}

// newSmokeCommand builds the smoke verb.
func newSmokeCommand(commands runner, openRepo repoOpener, channels channelRepos) *cobra.Command {
	options := defaultSmokeOptions(".")
	options.openRepo = openRepo
	options.channels = channels

	cmd := &cobra.Command{
		Use:   "smoke [vX.Y.Z] [--dist-dir D]",
		Short: "Check a published release from the outside",
		Long: "smoke proves a published release is usable: the tag is on origin and annotated;\n" +
			"with --dist-dir, that build's SHA256SUMS equals the published one; the six archives\n" +
			"download and match SHA256SUMS; every binary's build stamp names the tagged commit; the\n" +
			"host's archive holds a binary reporting the version; where capcom is installed with\n" +
			"Homebrew and the version is the latest release, `brew upgrade capcom` moves it onto\n" +
			"the version; and the Scoop bucket's manifest names the version and both Windows\n" +
			"archives' hashes. A check whose tool or input is missing prints SKIP and never counts\n" +
			"as a pass. The version defaults to VERSION; a bare X.Y.Z reads as vX.Y.Z. It exits 1\n" +
			"when any check fails. publish runs it as its last step.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("smoke takes at most one version, got %d arguments", len(args))
			}
			if len(args) == 1 {
				version, err := parseReleaseVersion(args[0])
				if err != nil {
					return err
				}
				options.version = version.String()
			}
			return nil
		},
		RunE: runE(func(cmd *cobra.Command, _ []string) error {
			return runSmoke(cmd.Context(), commands, cmd.OutOrStdout(), options)
		}),
	}
	cmd.Flags().StringVar(&options.distDir, "dist-dir", "", "a local build of the release whose SHA256SUMS must equal the published one")
	return cmd
}

// runSmoke runs every check in order — the package channels' checks in the publish step table's
// order — prints the tally, and fails when any check failed.
func runSmoke(ctx context.Context, commands runner, output io.Writer, options smokeOptions) error {
	version, err := smokeVersion(options)
	if err != nil {
		return err
	}
	workDir, err := os.MkdirTemp("", "capcom-smoke-")
	if err != nil {
		return fmt.Errorf("create the download directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	run := &smokeRun{
		commands: commands,
		repo:     options.openRepo.open(commands, options.dir),
		channels: channelReposOrExec(options.channels, commands, options.dir),
		output:   output,
		options:  options,
		version:  version,
		workDir:  workDir,
	}
	_, _ = fmt.Fprintf(output, "smoke: %s %s\n", capcomRepo, version)
	run.checkTag(ctx)
	run.checkDistSums(ctx)
	run.checkAssets(ctx)
	run.checkBuildStamps(ctx)
	run.checkHostBinary(ctx)
	for _, step := range publishSteps {
		if step.smoke != nil {
			step.smoke(ctx, run)
		}
	}

	_, _ = fmt.Fprintf(output, "\nsmoke: %s: %d passed, %d failed, %d skipped\n", version, run.passed, run.failed, run.skipped)
	if run.failed > 0 {
		return fmt.Errorf("%d smoke check(s) failed for %s", run.failed, version)
	}
	return nil
}

// smokeVersion is the version smoke checks: the one asked for, or the checkout's VERSION.
func smokeVersion(options smokeOptions) (releaseVersion, error) {
	text := options.version
	if text == "" {
		version, err := readCheckoutVersion(options.dir)
		if err != nil {
			return releaseVersion{}, err
		}
		text = version
	}
	return parseReleaseVersion(text)
}

// step prints a check's heading.
func (r *smokeRun) step(format string, args ...any) {
	_, _ = fmt.Fprintf(r.output, "\n==> %s\n", fmt.Sprintf(format, args...))
}

// pass, fail and skip print one check result and count it; warn prints a note that counts as
// none of them.
func (r *smokeRun) pass(format string, args ...any) {
	r.passed++
	r.report("PASS", format, args...)
}

func (r *smokeRun) fail(format string, args ...any) {
	r.failed++
	r.report("FAIL", format, args...)
}

func (r *smokeRun) skip(format string, args ...any) {
	r.skipped++
	r.report("SKIP", format, args...)
}

func (r *smokeRun) warn(format string, args ...any) {
	r.report("warning", format, args...)
}

// report prints one indented result line.
func (r *smokeRun) report(label, format string, args ...any) {
	_, _ = fmt.Fprintf(r.output, "    %s: %s\n", label, fmt.Sprintf(format, args...))
}

// checkTag requires the tag on origin and annotated: a lightweight or missing tag fails, a probe
// that cannot run skips.
func (r *smokeRun) checkTag(ctx context.Context) {
	tag := r.version.String()
	r.step("the tag %s is remote and annotated", tag)
	hasTag, err := r.repo.RemoteHasTag(ctx, tag)
	if err != nil {
		r.skip("could not list %s's tags: %v", remoteName, err)
		return
	}
	if !hasTag {
		r.fail("%s has no tag %s: the release was published without its tag", remoteName, tag)
		return
	}
	kind, err := r.repo.RemoteTagType(ctx, tag)
	if err != nil {
		r.skip("tag %s is on %s, but gh could not read its type: %v", tag, remoteName, err)
		return
	}
	switch kind {
	case gitObjectTag:
		r.pass("annotated tag %s on %s", tag, remoteName)
	case "commit":
		r.fail("%s on %s is a lightweight tag; a release tag must be annotated", tag, remoteName)
	default:
		r.fail("tag %s points at a %q object, not an annotated tag", tag, kind)
	}
}

// checkDistSums requires the local build's SHA256SUMS to list the same hash for every archive as
// the published one.
func (r *smokeRun) checkDistSums(ctx context.Context) {
	r.step("the local build's %s equals the published one", checksumsFile)
	if r.options.distDir == "" {
		r.skip("no --dist-dir: there is no local build to compare")
		return
	}
	published, err := r.publishedSums(ctx)
	if err != nil {
		r.skip("the release's %s did not download: %v", checksumsFile, err)
		return
	}
	local, err := os.ReadFile(filepath.Join(r.options.distDir, checksumsFile))
	if err != nil {
		r.fail("read the local build's %s: %v", checksumsFile, err)
		return
	}

	var differing []string
	for _, name := range r.archiveNames() {
		want, err := checksumFor(published, name)
		got, localErr := checksumFor(string(local), name)
		if err != nil || localErr != nil || got != want {
			differing = append(differing, name)
		}
	}
	if len(differing) > 0 {
		r.fail("%s in %s differs from the published one for %s", checksumsFile, r.options.distDir, strings.Join(differing, ", "))
		return
	}
	r.pass("%s in %s equals the published one", checksumsFile, r.options.distDir)
}

// checkAssets downloads each archive and requires its hash to equal the published SHA256SUMS.
func (r *smokeRun) checkAssets(ctx context.Context) {
	r.step("the published assets download and verify")
	sums, err := r.publishedSums(ctx)
	if err != nil {
		if isMissingTool(err) {
			r.skip("gh is not installed: %v", err)
			return
		}
		r.fail("no %s asset on release %s — is it published? %v", checksumsFile, r.version, err)
		return
	}

	isComplete := true
	for _, name := range r.archiveNames() {
		want, err := checksumFor(sums, name)
		if err != nil {
			r.fail("asset %s is missing from the release's %s: %v", name, checksumsFile, err)
			isComplete = false
			continue
		}
		if err := r.repo.DownloadAsset(ctx, r.version.String(), name, r.workDir); err != nil {
			r.fail("asset %s is listed but does not download: %v", name, err)
			isComplete = false
			continue
		}
		got, err := fileSHA256(filepath.Join(r.workDir, name))
		if err != nil {
			r.fail("asset %s: %v", name, err)
			r.reject(name, "could not be hashed")
			isComplete = false
			continue
		}
		if got != want {
			r.fail("asset %s hashes to %s, not the %s the release's %s lists", name, got, want, checksumsFile)
			r.reject(name, "does not match the release's "+checksumsFile)
			isComplete = false
			continue
		}
		r.markVerified(name)
	}
	if isComplete {
		r.isDownloaded = true
		r.pass("%d assets downloaded, all checksums match", len(distTargets))
	}
}

// markVerified records that the downloaded archive name matched SHA256SUMS.
func (r *smokeRun) markVerified(name string) {
	if r.verified == nil {
		r.verified = make(map[string]bool, len(distTargets))
	}
	r.verified[name] = true
}

// reject deletes the downloaded archive name, which failed its hash, so no later check can
// extract it, and records why.
func (r *smokeRun) reject(name, why string) {
	if r.rejected == nil {
		r.rejected = make(map[string]string)
	}
	r.rejected[name] = why
	if err := os.Remove(filepath.Join(r.workDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		r.warn("asset %s could not be deleted: %v", name, err)
	}
}

// checkBuildStamps requires every downloaded binary's vcs.revision to be the tagged commit; a
// binary built from a modified tree is only warned about, since an untracked file flips the flag.
func (r *smokeRun) checkBuildStamps(ctx context.Context) {
	r.step("every published binary was built from the tagged commit")
	if !r.isDownloaded {
		r.skip("the assets did not all download and verify")
		return
	}
	want, err := r.repo.Resolve(ctx, r.version.String())
	if err != nil {
		r.skip("tag %s is not in the local clone — git fetch --tags: %v", r.version, err)
		return
	}

	stamped := 0
	for _, target := range distTargets {
		name := target.archiveName(bareRelease(r.version))
		binary, err := r.extractBinary(target, filepath.Join(r.workDir, "stamp", name))
		if err != nil {
			r.fail("asset %s: %v", name, err)
			continue
		}
		info, err := r.options.readBuildInfo(binary)
		if err != nil {
			r.fail("asset %s carries no build information: %v", name, err)
			continue
		}
		revision := buildSetting(info, vcsRevisionSetting)
		if revision == "" {
			r.fail("asset %s carries no %s — was it built with -buildvcs=false?", name, vcsRevisionSetting)
			continue
		}
		if revision != want {
			r.fail("asset %s was built from %s, not the tagged commit %s", name, revision, want)
			continue
		}
		if buildSetting(info, vcsModifiedSetting) == "true" {
			r.warn("asset %s was built from a modified tree", name)
		}
		stamped++
	}
	if stamped == len(distTargets) {
		r.pass("%d assets carry %s %s = the tag", stamped, vcsRevisionSetting, short(want))
	}
}

// checkHostBinary runs the binary from the host's own archive, only when that archive matched
// SHA256SUMS, and requires it to report the version.
func (r *smokeRun) checkHostBinary(ctx context.Context) {
	r.step("the host's archive holds a binary reporting %s", r.version)
	host := distTarget{goos: r.options.goos, goarch: r.options.goarch}
	if !slices.Contains(distTargets, host) {
		r.skip("no release archive is built for this host (%s/%s)", host.goos, host.goarch)
		return
	}
	name := host.archiveName(bareRelease(r.version))
	if !r.verified[name] {
		why, isRejected := r.rejected[name]
		if !isRejected {
			why = "did not download"
		}
		r.skip("the host's archive %s %s, so its binary is not run", name, why)
		return
	}
	binary, err := r.extractBinary(host, filepath.Join(r.workDir, "host"))
	if err != nil {
		r.fail("asset %s: %v", name, err)
		return
	}
	r.checkReportsVersion(ctx, binary, "the released binary")
}

// checkReportsVersion runs `binary version` and requires its output to open with
// "capcom X.Y.Z", the bare version.
func (r *smokeRun) checkReportsVersion(ctx context.Context, binary, label string) {
	reported, err := r.commands.run(ctx, r.options.dir, nil, binary, versionArgument)
	if err != nil {
		r.fail("%s %s: %v", label, versionArgument, err)
		return
	}
	reported = strings.TrimSpace(reported)
	want := binaryName + " " + bareRelease(r.version)
	if fields := strings.Fields(reported); len(fields) < 2 || fields[0]+" "+fields[1] != want {
		r.fail("%s reports %q, not %q", label, reported, want)
		return
	}
	r.pass("%s reports %q", label, reported)
}

// publishedSums reads the release's SHA256SUMS once, writing nothing into the checkout, and
// returns it — or why it could not.
func (r *smokeRun) publishedSums(ctx context.Context) (string, error) {
	if !r.isSumsRead {
		r.isSumsRead = true
		r.sums, r.sumsErr = r.repo.ReadAsset(ctx, r.version.String(), checksumsFile)
	}
	return r.sums, r.sumsErr
}

// archiveNames are the six archives a release of the version carries.
func (r *smokeRun) archiveNames() []string {
	names := make([]string, 0, len(distTargets))
	for _, target := range distTargets {
		names = append(names, target.archiveName(bareRelease(r.version)))
	}
	return names
}

// extractBinary copies the target's binary out of its downloaded archive — the one entry
// <stem>/<binary> that the package channels read — into destDir, executable,
// and returns its path.
func (r *smokeRun) extractBinary(target distTarget, destDir string) (string, error) {
	bare := bareRelease(r.version)
	archivePath := filepath.Join(r.workDir, target.archiveName(bare))
	entry := target.stem(bare) + "/" + target.binaryFile()
	if err := os.MkdirAll(destDir, directoryMode); err != nil {
		return "", fmt.Errorf("create %s: %w", destDir, err)
	}
	binary := filepath.Join(destDir, target.binaryFile())

	var err error
	if target.isWindows() {
		err = extractZipEntry(archivePath, entry, binary)
	} else {
		err = extractTarGzEntry(archivePath, entry, binary)
	}
	if err != nil {
		return "", fmt.Errorf("read %s from the archive: %w", entry, err)
	}
	return binary, nil
}

// errNoEntry is an archive lacking the entry asked for.
var errNoEntry = errors.New("the archive holds no such entry")

// extractZipEntry writes the zip archive's entry to destination.
func extractZipEntry(archivePath, entry, destination string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = archive.Close() }()
	for _, file := range archive.File {
		if file.Name != entry {
			continue
		}
		contents, err := file.Open()
		if err != nil {
			return err
		}
		defer func() { _ = contents.Close() }()
		return writeExecutable(destination, contents)
	}
	return errNoEntry
}

// extractTarGzEntry writes the gzip-compressed tar archive's entry to destination.
func extractTarGzEntry(archivePath, entry, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	archive := tar.NewReader(compressed)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return errNoEntry
		}
		if err != nil {
			return err
		}
		if header.Name == entry && header.Typeflag == tar.TypeReg {
			return writeExecutable(destination, archive)
		}
	}
}

// writeExecutable writes contents to an executable file at destination.
func writeExecutable(destination string, contents io.Reader) (err error) {
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, binaryMode)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	_, err = io.Copy(file, contents)
	return err
}

// buildSetting returns the value of the build setting key, "" when the binary carries none.
func buildSetting(info *buildinfo.BuildInfo, key string) string {
	for _, setting := range info.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}

// isMissingTool reports whether a command could not run because its program is not installed.
func isMissingTool(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}

// smokePassedDetail is the step table's line for a smoke test that passed.
const smokePassedDetail = "passed by this run: no check failed"

// probeSmoke reports the smoke test pending always: it is an action publish takes last, not a
// state any remote holds.
func probeSmoke(context.Context, probeEnv, releaseVersion) stepReport {
	return stepReport{Name: stepSmoke, State: statePending, Detail: "an action, not a remote state: publish runs it last"}
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
	options.channels = run.options.channels
	options.version = run.version.String()
	options.distDir = run.distDir
	return run.options.smoke(ctx, run.output, options)
}
