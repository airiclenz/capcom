// Command release is capcom's release maintainer program: it does the deterministic half of
// cutting a release — building the archives, reporting where the release stands, making the
// rollup and bump commits, publishing the release and its package channels, and smoke-testing a
// published release — the same way on Linux, macOS and Windows, leaving the judgment half
// (release notes, the version to cut) to the committed release skills.
//
//	go run ./cmd/release dist [--src DIR] [--out DIR]
//	go run ./cmd/release status [vX.Y.Z] [--json]
//	go run ./cmd/release prepare vX.Y.Z
//	go run ./cmd/release publish vX.Y.Z --notes-file F [--dry-run] [--tag-timeout D]
//	go run ./cmd/release smoke [vX.Y.Z] [--dist-dir D]
//
// Every external command it runs goes through one runner seam (runner.go), so the tests of its
// verbs never reach the network, a real `git` or a real toolchain. The one exception is
// versionbump_test.go, which runs the tag-on-version-bump workflow's script against a throwaway
// local git repository, with no network and no gh (ADR 0008's bumptest amendment). What it asks of the capcom checkout and its
// GitHub repository goes through the repo seam (repo.go) above it: the verbs' tests hold that
// state in memRepo and never fake an own-repo command by argv, which only execRepo's own tests
// pin. The runner's fakes answer the toolchain, dist's read of its source tree's commit time, and
// the tap, bucket and winget-pkgs commands.
//
// It is a dev tool, never a release asset: dist does not ship it, and it imports nothing from
// capcom's internal packages.
//
// # Files
//
// Every non-test file of the package, with its responsibility. TestDocFileMap_MatchesDirectory
// fails when a file is missing from this map or the map names a file that does not exist.
//
//   - `channels.go` — the helpers the package channels share — publishing a generated file to a channel repository, reading one, judging the version it pins — and the winget publish step: describe and converge
//   - `dist.go` — release dist: builds the archives for every release platform and SHA256SUMS
//   - `doc.go` — the package comment and this file map
//   - `gate.go` — the release gate: a clean, pushed checkout whose commit passed CI
//   - `githubrelease.go` — the GitHub release step: probe, describe and converge, the asset build and the checks against what the release already carries
//   - `homebrew.go` — the homebrew channel: its probe, describe, converge, formula rendering and smoke check
//   - `main.go` — main, the root command and the exit code a run failure maps to
//   - `prepare.go` — release prepare: the CHANGELOG rollup and VERSION bump commits
//   - `probes.go` — the read-only probes that tell where the winget channel and the smoke step stand on the remotes
//   - `publish.go` — release publish: the gate, the step loop and the smoke step
//   - `render.go` — the package-channel templates, the SHA256SUMS reading they render from, and the winget manifests' rendering
//   - `repo.go` — the repo seam over the capcom checkout and its GitHub repository, and execRepo
//   - `runner.go` — the runner seam every external command goes through
//   - `scoop.go` — the scoop channel: its probe, describe, converge, manifest parsing and rendering, and smoke checks
//   - `smoke.go` — release smoke: checks a published release from the outside
//   - `status.go` — release status: the gate, the baseline and each publish step's probed state
//   - `steps.go` — the publish step table status and publish both read, and the step states
//   - `tag.go` — the tag step: probe, describe, and converge by pushing main and waiting for the CI-made annotated tag
//   - `version.go` — the release version: parsing, ordering, its bare form and the highest release tag
package main
