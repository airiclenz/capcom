---
title: "ADR 0008 decision: two committed skills write the release notes, and the cmd/release dev tool builds, gates and publishes"
---

A capcom release is cut from a clone of this repository with nothing else: the Go toolchain,
`git`, `gh` and, for winget, `komac`, on Linux, macOS or Windows. The work splits in two. The
judgment half (what shipped, the release notes, the version to cut) is done by an agent through
two committed skills. The deterministic half (archives, checksums, commits, publishing, checks)
is code, because publishing to four channels is exact, repeatable work that prose instructions
reproduce badly. The pipeline is ported from apogee's (its ADR 0098) and adapted to the calls the
owner made on 2026-10-11.

**The skills.** `release-notes-capcom` is read-only: scouts build an evidence inventory of
everything since the last published GitHub release (the whole history before the first one) and
the skill renders plain-language notes from it. `release-capcom` is the cut: it reads the notes
skill's files in place, proposes the version, has the owner confirm it, and hands publishing to
code. Each is canonical in `.agents/skills/<id>/`. Claude Code reaches it through a one-file
pointer, `.claude/skills/<id>/SKILL.md`, which is a file and not a symlink because a Windows
checkout does not reliably keep symlinks. The skills name only generic host capabilities, so any
agent host can run them. Their ids carry `-capcom` so they never collide with same-named skills in
a personal library; the owner's personal `cut-release` and `release-notes` stand aside here.

**The dev tool.** `cmd/release` runs as `go run ./cmd/release <verb>`. It is never a release
asset (`dist` does not ship it) and imports nothing from `internal/`. Its verbs:

- `dist` cross-builds linux, darwin and windows on amd64 and arm64 with `CGO_ENABLED=0 go build
  -trimpath`, packs each binary with `LICENSE` and `README.md` into a `.tar.gz` (`.zip` on
  Windows), and writes `SHA256SUMS`. The Go standard library does the archiving and hashing, so no
  `zip`, `tar` or `sha256sum` is needed.
- `status [vX.Y.Z]` is read-only. It reports the release gate, the baseline release and, for the
  version, whether each publish step is done, pending, setup-pending or unknown.
- `prepare vX.Y.Z` makes two **local** commits, the CHANGELOG rollup and then the `VERSION` bump,
  and pushes nothing, so an aborted cut is undone with a reset.
- `publish vX.Y.Z --notes-file F` is the only verb that changes anything outside the machine. It
  pushes, waits for the CI-made tag, builds the archives in a fresh clone at the tag, creates the
  GitHub release, updates the Homebrew tap and the Scoop bucket, submits to winget, then runs
  `smoke`. `--dry-run` prints the plan instead. GitHub is reached through `gh`, which already
  holds the owner's auth on every OS; the notes arrive as a file outside the repo.
- `smoke [vX.Y.Z]` checks a published release from the outside.

There is no Makefile and there are no `-ldflags` variables. `VERSION` holds a bare `X.Y.Z`, the
tag is `v` + VERSION, and `capcom version` prints what it always has. The `tag-on-version-bump`
workflow tags the commit that changes `VERSION` on `main`; nothing tags by hand. Build provenance
comes from the default `-buildvcs` stamp: `smoke` reads `vcs.revision` from each binary's build
info and requires the tagged commit.

**`publish` converges.** Before each step it asks the remote whether that step is already done
and skips it if so: the tag exists; the release is published, not a draft, and has all seven
assets; the formula pins the version or a newer one; the bucket manifest is at the version or a newer one; a winget pull
request for the version is open or merged. A tap or bucket already on a newer release reads done,
so a rerun for an older version never moves it back. A half-finished release is resumed by
running `publish` again. There is no local state file, because the remote is the truth, and a
second run cannot duplicate a commit or a pull request. `status` reports the same probes.

**The gate is green CI on the gated commit.** The gate commit is `HEAD` with its trailing
first-parent commits that touch only `CHANGELOG.md` and/or `VERSION` peeled off; the peel stops at
a merge. A cut clears only when the tree is clean, the gate commit is on `origin/main`,
`origin/main` is contained in `HEAD`, and `ci.yml` has a successful run for the gate commit or a
peeled commit `origin/main` contains. CI runs on Linux, macOS and Windows, so the answer does not
depend on which OS cuts the release. The gate has no bypass.

**Package manifests are generated.** The Homebrew formula (`Formula/capcom.rb` in
`airiclenz/homebrew-tap`), the Scoop manifest (`bucket/capcom.json` in `airiclenz/scoop-bucket`)
and the three first winget manifests are rendered whole from templates in
`cmd/release/templates/`, filled with the version and the hashes from `SHA256SUMS`. The tap and the
bucket take a direct `capcom X.Y.Z` commit and a push, with no pull request; a hand edit there is
overwritten at the next release. winget (`AiricLenz.Capcom` in `microsoft/winget-pkgs`) is
submitted, not published: the first release runs `komac submit` on the rendered manifests,
because `komac new` prompts for zip installers, and later releases run `komac update`. Either way
a pull request opens and merges after winget's own review.

**A channel whose one-time setup is pending is skipped, not failed.** A missing tap or bucket
repository leaves its step setup-pending; `publish` names the skip and still succeeds, and a rerun
once the repository exists converges it. A first `komac submit` that fails leaves winget
setup-pending too, keeping the rendered manifest directory and naming the command to rerun.

**Releases are cut on the owner's machine, never in CI.** No secrets are stored. `publish` never
prompts and never reads stdin; every input is a flag or an environment variable, and a failure
exits non-zero after printing the step table, so a later workflow could run it unchanged. What
stands in the way is outside the program: the CI tag is made with `GITHUB_TOKEN` and cannot
trigger another workflow, the tap, bucket and winget pushes need a personal token, and the notes
come from an agent.

**`cmd/release` is a scoped exception to ADR 0007.**
Its tests never run `git`, `gh`, `go` or `komac`, because `gh`, GitHub and winget-pkgs cannot be
made hermetic, and a test must never publish. Two seams carry the fakes. What the release verbs ask
of the capcom checkout and its GitHub repository goes through one semantic `repo` interface
(`repo.go`): `execRepo` is the production adapter over git and gh, every `gh release` call naming
`--repo airiclenz/capcom`, and the verbs' tests hold that repository as plain state in `memRepo`,
never as canned argv; only `execRepo`'s own tests pin the argv it runs. Everything else (the
toolchain, dist's read of the commit time of the source tree it builds, and the tap, bucket and
winget-pkgs commands) goes through one runner seam (`runner.go`; `fakeRunner`, `fakeToolchain`).
Files are still real files in `t.TempDir()`. ADR 0007 keeps covering every package of capcom itself.
