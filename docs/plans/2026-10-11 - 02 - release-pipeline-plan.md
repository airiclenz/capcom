# Release pipeline and the last two v1 bugs — plan

**Goal:** Fix the two open P2 bugs, then give capcom a repo-owned release pipeline ported from
apogee's: a `cmd/release` dev tool (dist, status, prepare, publish, smoke) that publishes the
GitHub release, the Homebrew formula, the Scoop manifest and the winget submission, a tag-on-bump
workflow, and two project-local release skills.
**Date:** 2026-10-11
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 0c32d15
**Closes:** release-pipeline

**Sources:**
- `/home/airic/repos/apogee/cmd/release/` (port source: code, tests, templates)
- `/home/airic/repos/apogee/docs/adr/0098-the-repo-owns-its-release-pipeline.md`
- `/home/airic/repos/apogee/.apogee/skills/release-apogee/`, `/home/airic/repos/apogee/.apogee/skills/release-notes-apogee/`
- `/home/airic/repos/apogee/.github/workflows/tag-on-version-bump.yml`, `/home/airic/repos/apogee/.github/scripts/version-bump.sh`
- `docs/adr/0007-tests-use-real-temp-files-sqlite-and-git.md`, `CHANGELOG.md`, `version.go`

**Ratified design calls:**
- **Where publishing runs:** locally, `go run ./cmd/release publish` driven by the release skill; no CI publishing, no stored secrets (user, 2026-10-11).
- **Release notes:** port apogee's two skills with their scout pipeline as `release-notes-capcom` and `release-capcom` (user, 2026-10-11).
- **VERSION spelling:** stays bare (`X.Y.Z`); the tag is `v` + VERSION; `capcom version` output is unchanged (user, 2026-10-11).
- **winget first submission:** automatic — `publish` renders the first manifests and runs `komac submit --yes` (komac new prompts for zip installers, found by the regression check); `komac update` afterwards (user, 2026-10-11; mechanism amended by the plan author, 2026-10-11).
- **Skill location:** canonical files in `.agents/skills/<id>/`; `.claude/skills/<id>/SKILL.md` is a one-file pointer (plan author, 2026-10-11).
- **Build provenance:** no ldflags vars and no Makefile; smoke reads `vcs.revision` from `debug/buildinfo` (plan author, 2026-10-11).

**Standing requirements:**
- skills: coding-standards
- Port by copying apogee files and adapting them; keep apogee's structure, names and test seams unless an item says otherwise.
- Every error message names the file, and the line when known (AGENTS.md).
- `cmd/release` imports nothing from `internal/`; it is a dev tool and never a release asset.
- Tests run as `GOMEMLIMIT=2GiB go test -count=1 <pkg>`; never `./...` with `-race`, never two at once.

**Out of scope:**
- Cutting a release, bumping VERSION or editing CHANGELOG release headings.
- An in-binary update check (apogee ADR 0097).
- The `cli-package-file-map` issue.
- CI-driven publishing; signing Windows binaries.

**Regression check (2026-10-11, 0c32d15):**
- 1: guard folded (decision: no `\U%08X` branch)
- 2: guard folded
- 3: guard folded (decision); ADR 0007 keeps covering capcom's packages, cmd/release is a scoped exception recorded by item 11
- 4: guard folded
- 5: guard folded; winget-absent sentence amended (item 7's decision)
- 6: recast — winget manifest templates added (decision); guard folded
- 7: recast — `komac submit` of rendered manifests replaces `komac new` (decision); guard folded
- 8: guard folded
- 9: guard folded (decision: no-release baseline, `.agents/skills` mapping)
- 10: guard folded (decision: `.agents/skills` mapping)
- 11: guard folded (decision: AGENTS.md opening paragraph, ADR 0007 exception)
- header: ratified winget first-submission call amended (item 7's decision)
- 6: guard folded (re-check: winget templates embedded, exported release-page URL, per-installer NestedInstallerFiles, ManifestVersion 1.12.0)
- 7: guard folded (re-check: done winget state and hasTap in the publish fakes, manifest dir kept on failed submit, winget absence resolved once into publishRun, `komac submit` in mutatingCommands)
- 9: guard folded (decision: no-release baseline read as whole history, root commit from `git rev-list --max-parents=0 HEAD`)

## 1. Escape every YAML non-printable rune in issue frontmatter — ✅ DONE (2026-10-11)

**What:**
**Goal:** `issue.Serialize` output re-parses for any frontmatter text value, including ones holding U+FFFE and U+FFFF, and the round-trip stays byte-stable.
**Approach (assumed at the header base):** Fix for the `serialize-escape-nonprintable` bug.
`needsEscape` in `internal/issue/file.go` covers only control characters, U+2028/U+2029 and the
BOM. Make it report every rune outside YAML 1.2's printable set (`#x9 | #xA | #xD | [#x20-#x7E] |
#x85 | [#xA0-#xD7FF] | [#xE000-#xFFFD] | [#x10000-#x10FFFF]`) plus the runes it escapes today.
`doubleQuoted` writes `\u%04X`. Both callers
(`isPlainSafe`, `doubleQuoted`) keep using the one predicate.
**Regression guard.** Drop the `\U%08X` branch — no rune above U+FFFF is YAML-non-printable — so `doubleQuoted` keeps only `\u%04X`.
**Files:** internal/issue/file.go, internal/issue/file_test.go
**Read first:** internal/issue/file.go — needsEscape, doubleQuoted, isPlainSafe, scalar; internal/issue/file_test.go — TestSerialize_TrickyText_RoundTrips, mustParse, TestParseSerialize_CanonicalGoldens_AreIdentity;
internal/knowledge/note.go — Note.Serialize (yaml.v3 encoder, already escapes U+FFFE: probed)
**Tests:** extend `TestSerialize_TrickyText_RoundTrips` (or add a sibling table test) with titles and labels holding U+FFFE and U+FFFF: serialize, re-parse, compare equal, re-serialize byte-identical.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/issue/`
**Closes:** serialize-escape-nonprintable
**Commit:** `fix(issue): escape YAML non-printable runes when serializing frontmatter`

## 2. A Ref below a file is a broken Ref, not a fatal error — ✅ DONE (2026-10-11)

**What:**
**Goal:** `capcom know verify` lists a Ref like `README.md/sub` (parent is a file) as a broken Ref with the message `no such file or directory`, and every other finding still prints.
**Approach (assumed at the header base):** Fix for the `know-verify-enotdir` bug. `checkRef` in
`internal/knowledge/verify.go` treats only `fs.ErrNotExist` from `filepath.EvalSymlinks` as
missing. Also treat `syscall.ENOTDIR` (portable: the constant exists on every GOOS) as missing,
with the same message. Any other error stays a hard failure.
**Regression guard.** Name the CLI test with the `TestKnowVerify_` prefix (e.g. `TestKnowVerify_RefBelowAFile_IsListedAsBroken`) so the Acceptance `-run KnowVerify` filter runs it.
**Files:** internal/knowledge/verify.go, internal/knowledge/knowledge_test.go, internal/cli/know_test.go
**Read first:** internal/knowledge/verify.go — checkRef, Verify; internal/knowledge/knowledge_test.go — TestVerify_Refs_FlagOnlyTheBrokenOnes, newWorktree; internal/cli/know_test.go — TestKnowVerify_BrokenRefs_ListsEachWithItsLineAndFails, newKnowStore;
internal/check/check.go — checker.checkNote (second Verify caller: `capcom check` now reports CodeBrokenRef instead of aborting); internal/cli/know.go — the know verify command's knowledge.Verify call
**Tests:** add `internal/cli/harness.go/sub` to the refs in `TestVerify_Refs_FlagOnlyTheBrokenOnes` with its expected finding; a CLI test `TestKnowVerify_RefBelowAFile_IsListedAsBroken` via `runCLI` where a note holds a `<file>/sub` Ref and a missing-path Ref: both are listed with their lines and exit is the existing broken-refs code.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/knowledge/` and `GOMEMLIMIT=2GiB go test -count=1 -run KnowVerify ./internal/cli/`
**Closes:** know-verify-enotdir
**Commit:** `fix(knowledge): report a Ref below a file as broken instead of aborting`

## 3. cmd/release foundation and the dist verb — ✅ DONE (2026-10-11)

NOTES (2026-10-11): dropped the `modulePath` constant instead of setting it to `github.com/airiclenz/capcom` — its only use was the removed -X ldflags, so it would have been dead code.
NOTES (2026-10-11): the ported ldflags test is recast as `TestDistBuildsEveryTargetWithTrimpathAndNoLdflags` (one case: `build -trimpath -o <path> ./cmd/capcom`, a single git call for the HEAD commit time); the "no release tag" case was deleted with the retraction cases as the plan says.
NOTES (2026-10-11): `readBareVersion` errors name the full VERSION path (and `:1` for an empty file) per AGENTS.md, where apogee named only "VERSION".

**What:**
**Goal:** `go run ./cmd/release dist [--src DIR] [--out DIR]` writes six reproducible archives `capcom_<VERSION>_<os>_<arch>` (`.tar.gz`, `.zip` on Windows) for linux, darwin, windows × amd64, arm64, each holding `capcom_<ver>_<os>_<arch>/` with the binary, `LICENSE` and `README.md`, plus a sorted `SHA256SUMS`.
**Approach (assumed at the header base):** Copy apogee's `cmd/release/main.go`, `runner.go`,
`version.go`, `dist.go` and their tests (`main_test.go` holds `fakeRunner` and `execute`;
`dist_test.go` holds `fakeToolchain`). Adapt: `binaryName="capcom"`, `mainPackage="./cmd/capcom"`,
`modulePath="github.com/airiclenz/capcom"`, temp prefixes `capcom-*`; drop the retraction bounds
(`retractedLow/High`) from `version.go`; drop the `buildCount`/`distBuild` ldflags (build is
`CGO_ENABLED=0 go build -trimpath`, default `-buildvcs`). VERSION is read bare; a leading `v` is
tolerated. Root command registers only `dist` in this item; Short/Long name capcom. Match capcom's
Go version in `go.mod`; replace any newer stdlib API.
**Regression guard.** cmd/release tests fake git, gh, go and komac through the ported `runner` seam (`fakeRunner`, `fakeToolchain`) as an explicit, scoped exception to ADR 0007, because gh and the network cannot be hermetic; ADR 0007 keeps covering capcom's own packages; item 11 records this exception in ADR 0008 and in AGENTS.md's test rule. Delete the retraction cases ("retracted v1.8.0", "whole retracted range", and the retraction-dependent "no release tag" expectation) from the ported highest-tag test and rename it to drop "Retracted". `fakeToolchain` takes no build-count parameter; items 6 and 7 adapt their call sites.
**Files:** cmd/release/main.go, cmd/release/runner.go, cmd/release/version.go, cmd/release/dist.go, cmd/release/main_test.go, cmd/release/version_test.go, cmd/release/dist_test.go
**Read first:** apogee cmd/release/dist.go — runDist, releaseLdflags, buildArchive; apogee cmd/release/version.go — highestReleaseTag, isRetracted; apogee cmd/release/main.go — newRootCommand;
apogee cmd/release/dist_test.go — fakeToolchain; apogee cmd/release/main_test.go — fakeRunner
**Tests:** the ported tests, adapted to capcom names; `TestHighestReleaseTagOrdersNumericallyAndSetsAsideTheRetractedSeries` loses its three retraction cases and is renamed without "Retracted"; `fakeToolchain()` takes no build count; a test that VERSION `0.0.0` yields `capcom_0.0.0_linux_amd64.tar.gz`.
**Acceptance:** `go vet ./cmd/release/` ; `GOMEMLIMIT=2GiB go test -count=1 ./cmd/release/` ; `GOMEMLIMIT=2GiB go run ./cmd/release dist --out "$(mktemp -d)"` lists six archives and SHA256SUMS
**Commit:** `feat(release): add the cmd/release dev tool with the dist verb`

## 4. Gate and prepare verbs — ✅ DONE (2026-10-11)

NOTES (2026-10-11): the two CHANGELOG.md refusals whose line is known name it as `CHANGELOG.md:<line>:` ("already has a ## [X.Y.Z] heading", "no bullet under ## [Unreleased] to release") per AGENTS.md, where apogee named only the file; the VERSION-already-reads refusal names the bare version and says "an unpushed version bump" instead of apogee's "micro-bump".
NOTES (2026-10-11): main.go's package doc and the root Long text now name prepare alongside dist; peelablePaths uses the changelogFile constant, and the apogee-specific commit references in the ported comments and fixtures are dropped.
NOTES (2026-10-11): fakeRepo keeps apogee's `release`/`tags` fields and the `gh release view` / `git rev-list --count` answers for item 5's baseline; only `remote` is dropped, its fallthrough returning "unexpected command".
NOTES (2026-10-11): added TestPrepareCutsTheFirstReleaseOfAKeepAChangelogFile (no `v*` tag, VERSION `0.0.0`, `### Added` under Unreleased, no prior release heading) and a refusal case for a VERSION written with its `v`.

**What:**
**Goal:** `go run ./cmd/release prepare vX.Y.Z` makes two local commits — `chore(release): cut vX.Y.Z` turning `## [Unreleased]` content into `## [X.Y.Z] - YYYY-MM-DD` under an empty `## [Unreleased]`, and `chore(version): bump version to vX.Y.Z` writing `X.Y.Z\n` (bare) to VERSION — and refuses, writing nothing, on every condition apogee's prepare refuses.
**Approach (assumed at the header base):** Copy apogee's `gate.go`, `prepare.go` and tests.
Adapt: VERSION written bare; heading uses ` - ` (Keep a Changelog), not an em dash; "not above the
highest tag" passes when no `v*` tag exists; gate keeps `origin`, `main`, `ci.yml`. Register
`prepare` on the root command.
**Regression guard.** Port `fakeRepo` without the `remote` field (`fakeRemote` lives in probes_test.go, item 5); its fallthrough returns an "unexpected command" error, and item 5 adds the field back. Drop the v1.x tags from `preparedTags` and its "retracted" comment, since item 3 drops the retraction.
**Files:** cmd/release/gate.go, cmd/release/prepare.go, cmd/release/gate_test.go, cmd/release/prepare_test.go, cmd/release/main.go
**Read first:** apogee cmd/release/prepare.go — runPrepare, rollUpChangelog, checkVersionFile; apogee cmd/release/gate.go — checkGate; apogee cmd/release/gate_test.go — fakeRepo.respond;
apogee cmd/release/prepare_test.go — prepareRepo, preparedTags; CHANGELOG.md — ## [Unreleased] / ### Added
**Tests:** the ported tests adapted (`fakeRepo` without `remote`, `preparedTags` without v1.x); a prepare test against capcom's real CHANGELOG shape (Keep a Changelog header, `### Added` under Unreleased, no prior release heading).
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./cmd/release/`
**Depends on:** item 3
**Commit:** `feat(release): add the release gate and the prepare verb`

## 5. Status verb and channel probes — ✅ DONE (2026-10-11)

NOTES (2026-10-11): no published release is detected by `gh release view` failing with "release not found" (new isReleaseNotFound helper, also used by readReleaseAssets); the baseline is then tag "", range HEAD, counted with `git rev-list --count HEAD`; stepsVersion probes VERSION's steps when the baseline has no tag but a range, and probes nothing when the baseline could not be read (range ""). The text report prints "no published release, HEAD: N commits"; verified live: `go run ./cmd/release status --json` exits 1 on the blocked gate with valid JSON, baseline range HEAD, all six steps pending.
NOTES (2026-10-11): probeWinget treats a 404 on the package directory as not-yet-listed and still checks open pull requests, so an open first-submission ("New package: ...") pull request reads done; with none it is pending, naming `komac submit`. wingetPull test helper gained a title-kind parameter; added cases "tap repo absent", "tap repo unreadable", "winget first submission open"; "formula unreadable" now fails only the contents call.
NOTES (2026-10-11): fakeRepo's latest-release answer fails with "release not found" when its `release` field is "" (the no-published-release case); apogee-only fixtures (apogee formula head, assets copied from a real apogee release, the retracted v1.8.0 tag, "micro-bump" wording) are replaced with capcom ones; main.go's package doc and root Long text now name status.

**What:**
**Goal:** `go run ./cmd/release status [vX.Y.Z] [--json]` reports the gate, the baseline and the state (`done|pending|setup-pending|unknown`) of the steps tag, release, homebrew, scoop, winget, smoke for capcom's channels, and works when no release was ever published.
**Approach (assumed at the header base):** Copy apogee's `probes.go`, `status.go` and tests.
Constants: repo `airiclenz/capcom`, tap `airiclenz/homebrew-tap` `Formula/capcom.rb`, bucket
`airiclenz/scoop-bucket` `bucket/capcom.json`, winget `AiricLenz.Capcom` at
`manifests/a/AiricLenz/Capcom`. Changes: with no published release the baseline is the root
commit and the range is the whole history; an empty bucket repo (contents 404 on the manifest)
is `pending`, not setup-pending; winget absent with no open PR is `pending` (item 7 submits it
with `komac submit`); setup-pending stays only for a missing tap/bucket repo. Release asset count is
6 archives + SHA256SUMS. Register `status`.
**Regression guard.** With no published release the baseline is tag `""`, range `HEAD`, commits from `git rev-list --count HEAD`, and `stepsVersion` probes VERSION's steps when the baseline has no tag. `probeHomebrew` first probes `gh api repos/airiclenz/homebrew-tap` (a 404 is setup-pending), as `probeScoop` does for the bucket; `fakeRemote` gains a `hasTap` field. Drop v1.8.0 from `clearedRepo`'s tags; pin winget `pending` in TestStatusReportsTheGateBaselineAndHighestTag and in TestProbesReportEachState's "winget not set up".
**Files:** cmd/release/probes.go, cmd/release/status.go, cmd/release/probes_test.go, cmd/release/status_test.go, cmd/release/gate_test.go, cmd/release/main.go
**Read first:** apogee cmd/release/status.go — runStatus, readBaseline, stepsVersion; apogee cmd/release/probes.go — probeWinget, probeHomebrew, probeScoop;
apogee cmd/release/probes_test.go — fakeRemote; apogee cmd/release/status_test.go — clearedRepo
**Tests:** ported tests adapted (`fakeRepo` regains `remote`); no-published-release baseline (tag `""`, range `HEAD`) and a no-tag case in TestStatusProbesTheVersionFileOnlyAboveTheBaseline; empty bucket → pending; missing tap repo → setup-pending; winget absent → pending.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./cmd/release/` ; `go run ./cmd/release status --json | jq . >/dev/null` (exit 1 from a blocked gate is expected; valid JSON on stdout is the check)
**Depends on:** item 4
**Commit:** `feat(release): add the status verb and the channel probes`

## 6. Formula and Scoop templates, and the smoke verb — ✅ DONE (2026-10-11)

NOTES (2026-10-11): winget rendering is `renderWingetManifests(version, sums) ([]renderedFile, error)` returning {name, content} in komac file naming (AiricLenz.Capcom.yaml, .installer.yaml, .locale.en-US.yaml) from the `wingetManifests` table; templates are named winget.version/installer/defaultLocale.yaml.tmpl. packageData gained `ReleaseURL()` (release page, for ReleaseNotesUrl) and `WingetPackageID()` (the probes.go constant); packageArchive gained `UpperSHA256()`, since InstallerSha256 is written upper-case as komac writes it.
NOTES (2026-10-11): golden formula and winget manifests are rendered at 0.0.0 from fixture hashes (capcom has no published release to pin live hashes from); the winget golden test also parses each manifest with yaml.v3. The formula's install comment drops apogee's build-tag sentence, which does not apply to capcom.
NOTES (2026-10-11): smoke's version check runs `<binary> version` and requires the first two fields of its output to be exactly `capcom X.Y.Z` (apogee used a substring match on `--version`); gitObjectTag, nonInteractiveEnv and the new versionArgument constant live in smoke.go; publishVersion ("v0.24.11") and writeReleaseTree (bare VERSION) live in smoke_test.go. The smoke Long text drops apogee's "publish runs it as its last step" sentence until item 7 adds publish; item 7 should restore it.

**What:** Recast at the regression check (2026-10-11).
**Goal:** `cmd/release` renders `Formula/capcom.rb` and `bucket/capcom.json` from embedded templates and a release's `SHA256SUMS`, and `go run ./cmd/release smoke [vX.Y.Z] [--dist-dir D]` runs apogee's smoke checks against capcom's release.
**Approach (assumed at the header base):** Copy apogee's `render.go`, `templates/`, `smoke.go`,
their tests and the golden testdata. Templates: `capcom.rb.tmpl` (`class Capcom < Formula`, desc
"Git-native issue tracker and knowledge store for coding agents", homepage the repo, MIT, the
four `on_macos`/`on_linux` × `on_arm`/`on_intel` blocks, `bin.install "capcom"`, no caveats,
`test do` asserting `capcom version` contains `version.to_s`) and `scoop.json.tmpl` (`bin`
`capcom.exe`, capcom URLs in `autoupdate`). Smoke's version check runs `capcom version` and
expects `capcom X.Y.Z` (bare); brew checks use `capcom`. Register `smoke`.
**Regression guard.** Also add winget manifest templates under cmd/release/templates/winget/ (version, installer, defaultLocale en-US manifests, ManifestVersion 1.12.0, the winget-manifest.*.1.12.0 schemas komac 2.16.0 carries) rendered from SHA256SUMS: InstallerType zip, NestedInstallerType portable, NestedInstallerFiles RelativeFilePath `capcom_<X.Y.Z>_windows_<arch>/capcom.exe` with PortableCommandAlias capcom, one installer per architecture (x64 → windows_amd64 zip, arm64 → windows_arm64 zip) with InstallerSha256 from SHA256SUMS; locale fields Publisher "Airic Lenz", PublisherUrl https://github.com/airiclenz, PackageName capcom, PackageUrl https://github.com/airiclenz/capcom, Moniker capcom, License MIT, LicenseUrl https://github.com/airiclenz/capcom/blob/main/LICENSE, ShortDescription "Git-native issue tracker and knowledge store for coding agents", ReleaseNotesUrl the release page; golden tests for all three. Declare each test identifier items 6 and 7 share (publishVersion, writeReleaseTree, gitObjectTag, nonInteractiveEnv) once, in item 6.
`gitObjectTag` and `nonInteractiveEnv` go in smoke.go (or runner.go); `publishVersion` and `writeReleaseTree` (writing a bare VERSION) go in smoke_test.go; item 7's files must not redeclare them. TestPublishRunsTheSmokeTestLast and TestPublishSmokeStepWithoutABuild move to item 7. smoke_test.go calls `fakeToolchain()` (item 3).
render.go embeds and parses `templates/winget/*.tmpl` as well as `templates/*.tmpl` (apogee's `templateFiles`/`packageTemplates` match only the latter), with template base names distinct from capcom.rb.tmpl and scoop.json.tmpl. packageData gains an exported release-page URL (method or field) for ReleaseNotesUrl, since `tag` is unexported; NestedInstallerFiles sits inside each Installers entry, not at the root, because RelativeFilePath differs per arch.
**Files:** cmd/release/render.go, cmd/release/templates/capcom.rb.tmpl, cmd/release/templates/scoop.json.tmpl, cmd/release/templates/winget/*, cmd/release/render_test.go, cmd/release/testdata/capcom-0.0.0.rb, cmd/release/testdata/winget/*, cmd/release/smoke.go, cmd/release/smoke_test.go, cmd/release/main.go
**Read first:** apogee cmd/release/render.go — templateFiles, packageTemplates, packageData.Archive, renderPackageFile; apogee cmd/release/templates/apogee.rb.tmpl, scoop.json.tmpl;
apogee cmd/release/smoke.go — runSmoke, checkReportsVersion; apogee cmd/release/smoke_test.go — newSmokeFake; apogee cmd/release/publish_test.go — writeReleaseTree; internal/cli/root.go — newVersionCommand
**Tests:** golden render of the formula; Scoop render is valid JSON with both Windows hashes; golden renders of the three winget manifests; ported smoke tests adapted, without TestPublishRunsTheSmokeTestLast and TestPublishSmokeStepWithoutABuild (item 7).
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./cmd/release/`
**Depends on:** item 5
**Commit:** `feat(release): render the Homebrew formula and Scoop manifest, add the smoke verb`

## 7. Publish verb with the channel steps and the first winget submission — ✅ DONE (2026-10-11)

NOTES (2026-10-11): consequential edit — cmd/release/smoke.go: made necessary by registering publish (the smoke verb's help again says publish runs it as its last step, as apogee's does)
NOTES (2026-10-11): consequential edit — cmd/release/probes.go: made necessary by the failed `komac submit` outcome (the stateSetupPending comment named only a tap or bucket repository)
NOTES (2026-10-11): the VERSION-at-HEAD check compares to the bare X.Y.Z and tolerates a leading "v", as readBareVersion and prepare's check do; TestPublishAcceptsAVersionFileWithALeadingV pins it
NOTES (2026-10-11): winget presence is resolved by publishRun.resolveWingetPackage (channels.go) with its own `gh api .../contents/manifests/a/AiricLenz/Capcom` read, called in runPublish only while the winget step is pending; a non-404 failure is a refusal. A failed `komac submit` travels as a setupPendingError sentinel that converge records as a setup-pending row

**What:** Recast at the regression check (2026-10-11).
**Goal:** `go run ./cmd/release publish vX.Y.Z --notes-file F [--dry-run]` converges tag → GitHub release → homebrew → scoop → winget → smoke, skipping done steps, and submits winget with `komac submit` of the rendered manifests when the package is absent and `komac update` otherwise.
**Approach (assumed at the header base):** Copy apogee's `publish.go`, `channels.go` and tests.
Adapt: the tag check compares `HEAD:VERSION` to the bare `X.Y.Z`; tap/bucket commit message
`capcom X.Y.Z`. winget: the first submission is `komac submit` of the rendered manifests, later
ones `komac update` (Regression guard). Both komac paths report "submitted", never "published".
Register `publish`.
**Regression guard.** Regression-check finding: komac 2.16.0 `komac new` cannot run non-interactively for a zip installer (the nested-file prompt has no flag; with stdin closed it exits "The input device is not a TTY"). Replace the `komac new` path: when the probe finds no `manifests/a/AiricLenz/Capcom` and no open PR, render the three winget manifests (item 6) into a temp dir `manifests/a/AiricLenz/Capcom/<X.Y.Z>/` and run `komac submit <dir> --yes` with GITHUB_TOKEN only in env and stdin closed; `komac update` when the package exists. On a failed `komac submit`, report winget setup-pending naming the manifest dir and the manual `komac submit` command (token redacted). Rewrite TestPublishSkipsChannelsWhoseSetupIsPending: only a missing tap or bucket repo stays setup-pending; a winget 404 takes the submit path. Tests: `komac submit` argv when absent, `komac update` when present, submit failure → setup-pending with redacted command.
Absent versus present is resolved once (in runPublish, or in converge before describe) into a publishRun field that `describeWinget` and `convergeWinget` read, since `describe` is `func(*publishRun) string` (`stepReport` keeps its pinned JSON keys). A failed `komac submit` is not fatal: its setup-pending outcome travels in a publishRun field or a sentinel error so converge stays `func(ctx, *publishRun) error`, the row is setup-pending with the redacted command, smoke still runs, exit 0; a failed `komac update` and a missing komac stay fatal (TestPublishWingetRequiresKomac). publish_test.go calls `fakeToolchain()` (item 3) and redeclares none of item 6's shared identifiers.
newPublishFake gets a done winget state (`wingetPulls: wingetPull(n, version)`, doc comment fixed) and `hasTap: true`, as does releasedFake, so full runs neither reach convergeWinget nor skip the tap. The rendered manifest dir is kept when `komac submit` fails and removed only on success. Add "komac submit" to mutatingCommands.
**Files:** cmd/release/publish.go, cmd/release/channels.go, cmd/release/publish_test.go, cmd/release/channels_test.go, cmd/release/smoke_test.go, cmd/release/main.go
**Read first:** apogee cmd/release/publish.go — publishAction, runPublish; apogee cmd/release/channels.go — convergeWinget, describeWinget;
apogee cmd/release/publish_test.go — newPublishFake, mutatingCommands; apogee cmd/release/channels_test.go — releasedFake, TestPublishWritesTheRenderedFileIntoTheClone
**Tests:** ported tests adapted; `komac submit` argv when absent; `komac update` when present; a `komac submit` failure reports setup-pending with the redacted command, smoke still runs, exit is 0, and the named manifest dir still holds the three manifests after runPublish returns; TestPublishSkipsChannelsWhoseSetupIsPending rewritten as above; TestPublishRunsTheSmokeTestLast and TestPublishSmokeStepWithoutABuild (moved from item 6) in smoke_test.go; `--dry-run` runs no mutating command (`komac submit` in mutatingCommands); the existing full-run tests stay green with newPublishFake's done winget state and `hasTap: true`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./cmd/release/` ; `go vet ./cmd/release/`
**Depends on:** item 6
**Commit:** `feat(release): add the publish verb with Homebrew, Scoop and winget steps`

## 8. Tag on VERSION bump and cross-build in CI — ✅ DONE (2026-10-11)

NOTES (2026-10-11): version-bump.sh's refusal message is prefixed `VERSION:1:` so it names the file and line (AGENTS.md error rule); otherwise the apogee script is kept, with the bare `[0-9]+.[0-9]+.[0-9]+` check and the `v` prefix added to the emitted tag.
NOTES (2026-10-11): the new `cross` job uses `actions/checkout@v4` / `actions/setup-go@v5` to match capcom's existing ci.yml rather than apogee's SHA pins, and builds only `./cmd/capcom` (apogee's extra `go build ./...` step dropped, per the Goal); tag-on-version-bump.yml keeps apogee's SHA pins as copied.

**What:**
**Goal:** A push to `main` that changes VERSION to a bare `X.Y.Z` creates the annotated tag `vX.Y.Z` on that commit (existing tags are left alone), and CI cross-builds `./cmd/capcom` for the six release targets.
**Approach (assumed at the header base):** Copy apogee's `tag-on-version-bump.yml` and
`.github/scripts/version-bump.sh`; adapt the script to accept a bare `[0-9]+.[0-9]+.[0-9]+` VERSION
(refuse a `v`-prefixed one) and emit `<sha>=v<version>`. In `ci.yml` (name unchanged — the gate
looks for it) add a `cross` job running `CGO_ENABLED=0 GOOS=… GOARCH=… go build ./cmd/capcom` over
the six targets.
**Regression guard.** Copy version-bump.sh with its 100755 mode kept (apogee holds it 100755; the workflow runs it directly, so a 100644 file fails every push with "Permission denied").
**Files:** .github/workflows/tag-on-version-bump.yml, .github/scripts/version-bump.sh, .github/workflows/ci.yml
**Read first:** /home/airic/repos/apogee/.github/scripts/version-bump.sh — read_version, bumps loop; /home/airic/repos/apogee/.github/workflows/tag-on-version-bump.yml — detect step, github-script createTag/createRef;
.github/workflows/ci.yml — jobs.test; /home/airic/repos/apogee/.github/workflows/ci.yml — cross job; /home/airic/repos/apogee/cmd/release/gate.go — ciWorkflow; /home/airic/repos/apogee/cmd/release/publish.go — waitForTag
**Tests:** run the script locally on a throwaway repo: a commit bumping VERSION `0.0.0`→`0.0.1` yields `bumps=<sha>=v0.0.1`; a `v0.0.1` value is refused.
**Acceptance:** `sh -n .github/scripts/version-bump.sh` ; `git ls-files -s .github/scripts/version-bump.sh | grep -q '^100755'` ; the throwaway-repo run above ; `CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o /dev/null ./cmd/capcom` ; `grep -q '^  cross:' .github/workflows/ci.yml && grep -q 'goos: \[linux, darwin, windows\]' .github/workflows/ci.yml && grep -q 'goarch: \[amd64, arm64\]' .github/workflows/ci.yml && grep -q "CGO_ENABLED: '0'" .github/workflows/ci.yml` ; `grep -q 'contents: write' .github/workflows/tag-on-version-bump.yml && grep -q 'github-script' .github/workflows/tag-on-version-bump.yml && grep -q 'createTag' .github/workflows/tag-on-version-bump.yml`
**Commit:** `ci: tag VERSION bumps and cross-build the release targets`

## 9. release-notes-capcom skill — ✅ DONE (2026-10-11)

NOTES (2026-10-11): the canonical SKILL.md frontmatter is `name` + `description` (the host-agnostic `.agents/skills` shape) in place of apogee's loader-specific `id`/`displayName`/`summary`/`triggers` keys.
NOTES (2026-10-11): the no-release baseline is carried to every sub-agent as a `ROOT:` dispatch key (Phase 1 derives it with `git rev-list --max-parents=0 HEAD`); `_capcom.md` defines "the diff base" (tag, else ROOT) and the log range (`baseline.range`, `HEAD` with no release), and the scouts and verify diff from the diff base; the commits scout's `git diff <range> --stat` became `git diff <diff base> HEAD --stat`, since `git diff HEAD --stat` with range `HEAD` would show only uncommitted work.
NOTES (2026-10-11): capcom issue ids are bare slugs, so trace-check's `apogee-…` bead-id pattern became a second grep of `notes.md` against the `.capcom/issues/` file slugs, written to `RUN_DIR/scratch/issue-ids.txt`.
NOTES (2026-10-11): the changelog scout also reads `### Deprecated/Removed/Security` subsections when present, beyond the plan's Added/Changed/Fixed, and takes the subsection as a `Kind:` hint.

**What:**
**Goal:** `.agents/skills/release-notes-capcom/` holds a read-only, host-agnostic skill that drafts plain-language release notes for capcom from the baseline `go run ./cmd/release status --json` reports, and `.claude/skills/release-notes-capcom/SKILL.md` points to it.
**Approach (assumed at the header base):** Copy apogee's `.apogee/skills/release-notes-apogee/`
(SKILL.md and `prompts/`) to `.agents/skills/release-notes-capcom/`; rename every `apogee` to
`capcom` and `.apogee/skills` to `.agents/skills`; rewrite `prompts/_apogee.md` as
`prompts/_capcom.md` with capcom's facts: Keep a Changelog shape of `CHANGELOG.md`, conventional
commits, and a user-surfaces table (`internal/cli/` commands and flags, `docs/format.md`,
`.capcom/config.yaml` defaults, `README.md`, the AGENTS.md managed block and Claude hooks from
`capcom prime`). Run dir `docs/skill-runs/release-notes-capcom/`. The pointer file is one line of
instruction plus frontmatter, like apogee's.
**Regression guard.** Item 5's no-release baseline is tag "" with range HEAD; the skill reads that as "whole history": scouts read `git log HEAD` and diff from the root commit the skill derives with `git rev-list --max-parents=0 HEAD`; it never stops for a missing tag. Rename mapping: `.apogee/skills` → `.agents/skills` first (never `.capcom/skills`), then apogee → capcom.
`prompts/changelog-scout.md` is rewritten too: one entry per `- ` bullet under `### Added/Changed/Fixed`, and the baseline heading is `## [X.Y.Z] - date` (hyphen). Add `/docs/skill-runs/` to the root `.gitignore` so a notes run leaves the tree clean for the release gate.
**Files:** .agents/skills/release-notes-capcom/SKILL.md, .agents/skills/release-notes-capcom/prompts/*, .claude/skills/release-notes-capcom/SKILL.md, .gitignore
**Read first:** /home/airic/repos/apogee/.apogee/skills/release-notes-apogee/SKILL.md — Phase 1 Baseline, phase table; /home/airic/repos/apogee/.apogee/skills/release-notes-apogee/prompts/changelog-scout.md — Group into entries;
/home/airic/repos/apogee/.apogee/skills/release-notes-apogee/prompts/_apogee.md — user surfaces; /home/airic/repos/apogee/.apogee/skills/release-notes-apogee/prompts/surface-scout.md — git diff baseline.tag;
/home/airic/repos/apogee/.claude/skills/release-notes-apogee/SKILL.md — pointer; .gitignore; docs/skill-runs/.gitignore
**Tests:** none (prose); acceptance greps.
**Acceptance:** `! grep -rni apogee .agents/skills/release-notes-capcom .claude/skills/release-notes-capcom` ; `! grep -rn '\.capcom/skills' .agents/skills/release-notes-capcom .claude/skills/release-notes-capcom` ; every prompt file SKILL.md names exists under `prompts/` ; every `.agents/skills/...` path the files name exists ; `git check-ignore -q docs/skill-runs/x`
**Depends on:** item 5
**Commit:** `feat(skills): add the release-notes-capcom skill`

## 10. release-capcom skill — ✅ DONE (2026-10-11)

NOTES (2026-10-11): frontmatter follows item 9's `release-notes-capcom` shape (`name` + `description`) rather than apogee's `id`/`displayName`/`summary`/`triggers` keys, so both canonical capcom skills read alike.
NOTES (2026-10-11): with no tag yet the version proposal reads `highestTag` as `v0.0.0` and always proposes minor (`v0.1.0`) for the first release; the report names `gh repo create airiclenz/homebrew-tap --public` / `gh repo create airiclenz/scoop-bucket --public` as the owner's manual setup commands, and relays the kept-manifest command `publish` prints for winget.
NOTES (2026-10-11): the skill cites `docs/adr/0008-the-repo-owns-its-release-pipeline.md`, which item 11 creates; it does not exist yet.

**What:**
**Goal:** `.agents/skills/release-capcom/SKILL.md` cuts a capcom release end to end — status, notes via release-notes-capcom, notes approval, version confirmation, `prepare`, `publish --dry-run`, one confirmation, `publish`, report — and `.claude/skills/release-capcom/SKILL.md` points to it.
**Approach (assumed at the header base):** Copy apogee's `.apogee/skills/release-apogee/SKILL.md`;
rename to capcom; commands `go run ./cmd/release …`; the version proposal reads `highestTag` and
handles "no tag yet"; VERSION is bare and the tag is `v` + VERSION; the report names winget as
"submitted" and any setup-pending channel with its manual command. State that the personal
`cut-release` and `release-notes` skills stand aside in this repo.
**Regression guard.** Same rename mapping as item 9 (`.apogee/skills` → `.agents/skills`, never `.capcom/skills`).
ADR 0098 maps to `docs/adr/0008-the-repo-owns-its-release-pipeline.md` (item 11); drop the "retracted `v1.x` tags" clause from the version proposal.
**Files:** .agents/skills/release-capcom/SKILL.md, .claude/skills/release-capcom/SKILL.md
**Read first:** /home/airic/repos/apogee/.apogee/skills/release-apogee/SKILL.md — steps 1 (resume subjects), 2, 4, 6, 7, Abort, Report; /home/airic/repos/apogee/.claude/skills/release-apogee/SKILL.md — pointer;
/home/airic/repos/apogee/cmd/release/publish.go — newPublishCommand flags (--notes-file, --dry-run, --tag-timeout); /home/airic/repos/apogee/cmd/release/status.go — statusReport HighestTag, readBaseline
**Tests:** none (prose); acceptance greps.
**Acceptance:** `! grep -rni apogee .agents/skills/release-capcom .claude/skills/release-capcom` ; `! grep -rn '\.capcom/skills' .agents/skills/release-capcom .claude/skills/release-capcom` ; every repo path the skill names exists or is item 11's ADR ; every `cmd/release` verb and flag the skill names exists in `go run ./cmd/release --help` output of its verb
**Depends on:** items 7, 9
**Commit:** `feat(skills): add the release-capcom skill`

## 11. Release docs, ADR and install instructions

**What:**
**Goal:** The repo documents its release pipeline: an ADR records it, `docs/releasing.md` covers one-time setup and the cut, `README.md` shows brew, scoop and winget installs, and AGENTS.md points agents at the release skill.
**Approach (assumed at the header base):** `docs/adr/0008-the-repo-owns-its-release-pipeline.md`
in capcom's ADR style, adapting apogee ADR 0098 to the ratified calls above. `docs/releasing.md`:
prerequisites (Go, git, `gh auth login`, `komac`, a classic PAT with `public_repo` in
`GITHUB_TOKEN` or `gh auth token`), the tap and bucket repos, the verbs, resuming a failed
publish. README Install section adds `brew install airiclenz/tap/capcom`, `scoop bucket add
airiclenz https://github.com/airiclenz/scoop-bucket` + `scoop install capcom`, `winget install
AiricLenz.Capcom`, keeping `go install`. AGENTS.md gets a short "Releases" section outside the
capcom managed block.
**Regression guard.** Also update AGENTS.md's opening paragraph ("The binary is built from `cmd/capcom`; everything else is in `internal/`") to name `cmd/release` as the release dev tool, never shipped; ADR 0008 and AGENTS.md's test rule record the cmd/release exception to ADR 0007.
README's status banner ("there are no tagged releases yet, so build it from source") and its Go-requirement line are rewritten too, and the brew, scoop and winget commands are marked as available from the first published release until one exists.
**Files:** docs/adr/0008-the-repo-owns-its-release-pipeline.md, docs/releasing.md, README.md, AGENTS.md
**Read first:** README.md — status banner, ## Install; AGENTS.md — BEGIN CAPCOM/END CAPCOM markers; internal/agents/block.go — BeginMarker, EndMarker; internal/check/check.go — agentsBlock;
docs/adr/0007-tests-use-real-temp-files-sqlite-and-git.md — ADR style; /home/airic/repos/apogee/docs/adr/0098-the-repo-owns-its-release-pipeline.md
**Tests:** none (docs).
**Acceptance:** every command and path the docs name exists (`go run ./cmd/release <verb> --help` for each verb named); `capcom check` reports no problems
**Depends on:** item 10
**Commit:** `docs: record the release pipeline and the install channels`
