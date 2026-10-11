# Releasing capcom

A release is cut locally, from a clone of this repository, on Linux, macOS or Windows. The
`release-capcom` skill does the judgment work (release notes, the version) and hands the rest to
`go run ./cmd/release`, which builds, publishes and checks. Why it is split this way is recorded in
[ADR 0008](adr/0008-the-repo-owns-its-release-pipeline.md).

One release publishes to four channels:

| Channel | Where | How |
| --- | --- | --- |
| GitHub release | `airiclenz/capcom` | six archives and `SHA256SUMS` |
| Homebrew | `airiclenz/homebrew-tap`, `Formula/capcom.rb` | a direct `capcom X.Y.Z` commit |
| Scoop | `airiclenz/scoop-bucket`, `bucket/capcom.json` | a direct `capcom X.Y.Z` commit |
| winget | `microsoft/winget-pkgs`, `AiricLenz.Capcom` | a pull request opened by `komac` |

## Prerequisites

- **Go**, at the version `go.mod` names, and **git**.
- **gh**, signed in with `gh auth login` and allowed to push to `airiclenz/capcom`,
  `airiclenz/homebrew-tap` and `airiclenz/scoop-bucket`. The tap and the bucket are cloned with
  `gh repo clone` and pushed with `git push`, so git must be able to push over the protocol gh
  uses (`gh auth setup-git` sets that up for HTTPS).
- **komac**, for winget: `cargo install --locked komac`, `brew install komac` or
  `winget install komac` (<https://github.com/russellbanks/Komac>).
- **A GitHub token for komac**: a classic personal access token with the `public_repo` scope in
  `GITHUB_TOKEN`. Without it, `publish` hands komac the token from `gh auth token`. The token
  reaches komac through its environment only and is never printed.
- Optional, for the smoke checks: Homebrew with capcom installed through it, so `smoke` can check
  that `brew upgrade capcom` reaches the new version. A check whose tool is missing prints `SKIP`.

## One-time setup

- **The tap and the bucket.** `publish` writes into `airiclenz/homebrew-tap` and
  `airiclenz/scoop-bucket` and never creates them. While one of them does not exist, `status`
  reports its step `setup-pending` and `publish` skips it and still succeeds. Create the missing
  one, then run `publish` again for the same version:

  ```sh
  gh repo create airiclenz/homebrew-tap --public
  gh repo create airiclenz/scoop-bucket --public
  ```

  An existing but empty repository is fine: the formula and the manifest are what `publish`
  writes.
- **winget** needs no setup. While `AiricLenz.Capcom` is not in winget-pkgs, `publish` renders the
  first three manifests from `cmd/release/templates/winget/` and submits them with
  `komac submit`; once the package is listed it runs `komac update`.

## Cutting a release

Ask an agent to cut the release: the `release-capcom` skill (`/release-capcom` in Claude Code;
canonical file `.agents/skills/release-capcom/SKILL.md`) checks the gate, drafts the notes with
`release-notes-capcom`, has you approve them and confirm the version, runs `prepare`, shows you a
dry run, and runs `publish` after one explicit confirmation. To only see what shipped, run
`release-notes-capcom`; it never publishes.

By hand, the same steps are:

```sh
go run ./cmd/release status                       # the gate, the baseline, each publish step
go run ./cmd/release prepare vX.Y.Z               # two local commits: CHANGELOG rollup, VERSION bump
go run ./cmd/release publish vX.Y.Z --notes-file /tmp/notes.md --dry-run
go run ./cmd/release publish vX.Y.Z --notes-file /tmp/notes.md
```

Keep the notes file outside the repository, so it never dirties the tree the gate checks.

### The verbs

- `dist [--src DIR] [--out DIR]` cross-builds linux, darwin and windows on amd64 and arm64 into
  `capcom_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows) plus `SHA256SUMS`, by default under
  `dist/` (ignored by git). The archives are byte-identical across runs over the same commit.
  `publish` runs the same build itself, in a fresh clone at the tag.
- `status [vX.Y.Z] [--json]` is read-only apart from fetching origin. It exits 0 when the gate
  clears and 1 when it is blocked or a check could not run, printing its report either way.
- `prepare vX.Y.Z` commits `chore(release): cut vX.Y.Z`, which inserts `## [X.Y.Z] - <today>` in
  `CHANGELOG.md` directly under `## [Unreleased]`, so the unreleased bullets move under the release
  and an empty `## [Unreleased]` stays at the top, then `chore(version): bump version to vX.Y.Z`,
  which writes the bare `X.Y.Z` to `VERSION`. It pushes nothing and refuses on a dirty tree, an
  empty `## [Unreleased]`, or a version not above the highest `vX.Y.Z` tag.
- `publish vX.Y.Z --notes-file F [--dry-run] [--tag-timeout D]` pushes `HEAD` to `main`, waits
  (10 minutes by default) for CI to tag the `VERSION` bump, creates the GitHub release, commits
  the rendered formula and Scoop manifest, submits to winget, and finally runs `smoke`. It never
  prompts and never reads stdin.
- `smoke [vX.Y.Z] [--dist-dir D]` checks a published release from the outside: the annotated tag,
  the downloaded archives against `SHA256SUMS`, each binary's build stamp against the tagged
  commit, the host binary's version, Homebrew and the Scoop manifest.

Every verb prints its full description with `go run ./cmd/release <verb> --help`.

### The gate

`status` and `publish` clear a cut only when the working tree is clean, the gate commit (`HEAD`
with its trailing `CHANGELOG.md`/`VERSION`-only commits peeled off) is on `origin/main`,
`origin/main` is contained in `HEAD`, and `ci.yml` has a successful run for the gate commit or one
of the peeled commits. There is no bypass: red CI blocks the release until it is fixed.

### The tag

You never tag. `VERSION` holds a bare `X.Y.Z` and the tag is `v` + VERSION: when a push to `main`
changes `VERSION`, the `tag-on-version-bump` workflow creates the annotated tag on that commit.
`publish` waits for it.

## Resuming a failed publish

`publish` converges: before each step it asks the remote whether that step is already done and
skips it if so. After a failure, fix the cause and run the **same** command again. There is no
local state to clean up, and a second run cannot duplicate a commit or a pull request.
`go run ./cmd/release status vX.Y.Z` shows which steps are done.

- A step reported `setup-pending` did not fail the run; do its one-time setup (above) and run
  `publish` again.
- A first `komac submit` that fails leaves winget `setup-pending`. `publish` prints the kept
  manifest directory and the exact `komac submit` command; run that command, or run `publish`
  again.
- winget reports done once a pull request for the version is open: the package is *submitted*,
  and it appears in `winget` only after winget's maintainers merge that pull request.

To abandon a cut before `publish` has run, undo the two `prepare` commits with
`git reset --keep HEAD~2`. Once `publish` has pushed, nothing is undone; a broken release is fixed
with a new version.
