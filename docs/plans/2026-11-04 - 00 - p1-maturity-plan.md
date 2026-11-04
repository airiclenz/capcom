# P1 code maturity: static analysis, fuzzing, merge-driver properties

**Goal:** Close the three P1 issues under `code-maturity`: CI gains staticcheck and govulncheck on a patched, pinned Go toolchain; the hostile-input parsers gain native fuzz targets run in CI; the merge driver gains seeded property tests at the unit and git level.
**Date:** 2026-11-04
**Status:** draft — regression check pending
**sized for:** ~200k-context host
**base:** ae5ea40

**Sources:**
- `.capcom/issues/code-maturity/ci-static-analysis.md`, `fuzz-parsers.md`, `merge-driver-property-test.md` (each item's Acceptance)
- `.github/workflows/ci.yml`, `Makefile`, `CONTRIBUTING.md`
- Notes `cli-test-harness`, `coding-conventions`, `store-layout`

**Ratified design calls:**
- **Go toolchain:** pin `toolchain go1.25.<latest patch>` in `go.mod`, the patch read when the item runs, so CI, release and local builds use it and govulncheck checks what ships. (Airic, 2026-11-04)
- **Linters:** staticcheck and govulncheck at pinned versions via `go run <module>@<version>`, never `@latest`; no golangci-lint. (issue `ci-static-analysis`)
- **Fuzz budget:** 30s per target in CI, on every push and pull request, one matrix job per target. (issue `fuzz-parsers`)
- **Crashers:** a crasher a fuzz target finds is fixed in the item that found it, its input committed under `testdata/fuzz/<Target>/`, with a `fix(…)` CHANGELOG entry in the sidecar.
- **Property iterations:** 200 by default, `CAPCOM_PROPERTY_ITERATIONS` overrides. (issue `merge-driver-property-test`)

**Standing requirements:**
- skills: coding-standards
- Tests on the Pi: single packages with `GOMEMLIMIT=2GiB`, never `-race` on `./...`, never two test commands at once; fuzz with `-parallel=2`.
- CLI tests follow Note `cli-test-harness`: `runCLI`, real temp git repo, never `t.Parallel()`.
- Fuzz and property tests use only the standard library (`testing.F`, `math/rand/v2`); no new module.
- Every authorised deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- golangci-lint, `staticcheck.conf` beyond a check a finding shows is deliberate
- The other `code-maturity` issues (`yaml-module-swap`, golden tests, fixtures, benchmarks, stress)
- Fuzzing `internal/userconfig`, `internal/hooks`, `cmd/release`

## 1. Pin a patched Go toolchain in go.mod

**What:** `go.mod` declares `go 1.25.0` and no `toolchain`, so setup-go and the release host build with Go 1.25.0 and its unpatched standard library.
**Goal:** `go.mod` carries a `toolchain go1.25.N` line naming the newest Go 1.25 patch release published at run time (read from `https://go.dev/dl/?mode=json&include=all` or `go list -m -versions golang.org/toolchain`), and the `go` line is unchanged.
**Approach (assumed at the header base):** `go mod edit -toolchain=go1.25.N`; `actions/setup-go@v5` with `go-version-file: go.mod` reads the `toolchain` directive first, so `ci.yml` needs no change. Note the patch in `CONTRIBUTING.md` where it says how to build (one sentence: the toolchain line pins the patched Go; `GOTOOLCHAIN=auto` fetches it).
**Files:** go.mod, CONTRIBUTING.md
**Tests:** none new; the build and vet prove the toolchain resolves.
**Acceptance:**
- `grep -E '^toolchain go1\.25\.[0-9]+$' go.mod`
- `go version` (run in the repo) reports the pinned patch
- `go build ./cmd/capcom && go vet ./internal/config/`
**Commit:** `chore(build): pin the patched Go 1.25 toolchain`

## 2. Fix the staticcheck findings in test files

**What:** `staticcheck@2025.1.1 ./...` at the base reports five findings, all in tests: `internal/index/agreement_test.go` unused const `formValid` (U1000); `internal/ui/closed_test.go`, `internal/ui/mouse_test.go`, `internal/ui/uistate_test.go`, `internal/ui/write_close_test.go` a value of `m` never used (SA4006).
**Goal:** `staticcheck ./...` at the version item 3 pins exits 0, with no `//lint:ignore` and no `staticcheck.conf`.
**Approach (assumed at the header base):** delete the unused const; for each SA4006, either assert on the reassigned model (when the test meant to check it) or drop the dead assignment (`_ = ` is not a fix). Re-run staticcheck to catch any finding the pinned version adds.
**Files:** internal/index/agreement_test.go, internal/ui/closed_test.go, internal/ui/mouse_test.go, internal/ui/uistate_test.go, internal/ui/write_close_test.go
**Tests:** the touched tests still pass.
**Acceptance:**
- `go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 ./...`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
**Depends on item 1.**
**Commit:** `test: drop dead assignments staticcheck reports`

## 3. Run staticcheck and govulncheck in CI and make lint

**What:** add a `lint` job and a `make lint` target (issue `ci-static-analysis`). The job is ubuntu-latest only, `actions/checkout@v4`, `actions/setup-go@v5` with `go-version-file: go.mod`, then `go run honnef.co/go/tools/cmd/staticcheck@<pin> ./...` and `go run golang.org/x/vuln/cmd/govulncheck@<pin> ./...`; pins are the newest releases at run time that build under the item-1 toolchain (staticcheck `2025.1.1` and govulncheck `v1.1.4` at the base), written once as job `env` values and once as Makefile variables.
**Goal:** CI fails on any staticcheck finding or any govulncheck-reachable vulnerability; `make lint` runs the same two commands at the same pins; `CONTRIBUTING.md` lists both among the CI checks.
**Approach (assumed at the header base):** a third job beside `test` and `cross` in `.github/workflows/ci.yml`; `Makefile` gains `STATICCHECK_VERSION`, `GOVULNCHECK_VERSION`, a `lint` target and `lint` in `.PHONY`. If govulncheck reports a reachable module vulnerability, `go get <module>@<fixed>` and `go mod tidy` in this item, then run the affected package's tests. `vuln.go.dev` may be unreachable from a sandbox: then the local govulncheck run is recorded as a NOTES line and the CI run is the check.
**Files:** .github/workflows/ci.yml, Makefile, CONTRIBUTING.md, go.mod, go.sum
**Tests:** none new.
**Acceptance:**
- `make lint` exits 0 (govulncheck may be NOTES-excused only for an unreachable `vuln.go.dev`)
- `grep -n 'staticcheck@\|govulncheck@' .github/workflows/ci.yml Makefile` shows no `@latest`
- `grep -n 'staticcheck' CONTRIBUTING.md && grep -n 'govulncheck' CONTRIBUTING.md`
**Depends on item 2.**
**Closes:** ci-static-analysis
**Commit:** `ci: run staticcheck and govulncheck`

## 4. Fuzz frontmatter.Decode and the rawfm scanners

**What:** add `internal/frontmatter/fuzz_test.go` with `FuzzDecode` and `internal/rawfm/fuzz_test.go` with `FuzzConflictMarkers` (issue `fuzz-parsers`).
**Goal:** `FuzzDecode` calls `frontmatter.Decode("x.md", data, kind, apply)` for an issue-like and a Note-like `Kind` (built in the test from a fixed key list) with an `apply` that reads each `Field` through its helpers; it never panics and every error's text contains `x.md`. `FuzzConflictMarkers` calls `rawfm.ConflictMarkers(data)` and `rawfm.Aliases(data)`; it never panics and every returned `ConflictMarker.Line` is between 1 and the line count of `data`, its `Text` equal to that line with any trailing `\r` trimmed.
**Approach (assumed at the header base):** seeds via `f.Add`: every file under `internal/issue/testdata/`, a minimal Note, and hand cases — unclosed fence, CRLF, NUL and `\x1b` bytes, a repeated key, a `---` inside a fenced code block, conflict markers in frontmatter and body, an unclosed code fence above markers. Read testdata with `os.ReadFile` at seed time.
**Files:** internal/frontmatter/fuzz_test.go, internal/rawfm/fuzz_test.go
**Tests:** the two fuzz targets; their seeds run under plain `go test`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/frontmatter/ ./internal/rawfm/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzDecode$' -fuzztime=2m -parallel=2 ./internal/frontmatter/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzConflictMarkers$' -fuzztime=2m -parallel=2 ./internal/rawfm/`
**Commit:** `test(frontmatter): fuzz Decode and the conflict marker scan`

## 5. Fuzz issue parsing and MergeFile

**What:** add `internal/issue/fuzz_test.go` with `FuzzParseIssue` and `FuzzMergeFile`.
**Goal:** `FuzzParseIssue`: `issue.Parse("x.md", data)` never panics; when it succeeds, `s1 := i.Serialize()` parses again and `Parse(s1).Serialize()` equals `s1` byte for byte (canonical form is a fixed point). `FuzzMergeFile` takes three byte slices and calls `issue.MergeFile(base, ours, theirs, fakeTextMerge)`; it never panics and never returns an error (`fakeTextMerge` never fails).
**Approach (assumed at the header base):** reuse `fakeTextMerge`, `issueFile` and `crlf` from `merge_test.go` (same package). Seeds: every `testdata/*.md`; for `FuzzMergeFile`, triples built with `issueFile` (both sides append, one side edits the title, CRLF ours, a version holding the Log placeholder).
**Files:** internal/issue/fuzz_test.go
**Tests:** the two fuzz targets.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/issue/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzParseIssue$' -fuzztime=2m -parallel=2 ./internal/issue/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzMergeFile$' -fuzztime=2m -parallel=2 ./internal/issue/`
**Commit:** `test(issue): fuzz Parse round trips and MergeFile`

## 6. Fuzz Note parsing

**What:** add `internal/knowledge/fuzz_test.go` with `FuzzParseNote`.
**Goal:** `knowledge.Parse("x.md", data)` never panics; when it succeeds, `s1 := n.Serialize()` parses again and re-serializes to `s1` byte for byte.
**Approach (assumed at the header base):** seeds: every `.capcom/knowledge/**/*.md` of this repository found by walking `../../.capcom/knowledge` at seed time (skip silently when absent), plus a Note with `retired`, `superseded-by`, `aliases`, `refs`, CRLF and a conflict marker.
**Files:** internal/knowledge/fuzz_test.go
**Tests:** the fuzz target.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/knowledge/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzParseNote$' -fuzztime=2m -parallel=2 ./internal/knowledge/`
**Commit:** `test(knowledge): fuzz Note parse round trips`

## 7. Fuzz config and apply-file parsing

**What:** add `internal/config/fuzz_test.go` with `FuzzParseConfig` and `internal/batch/fuzz_test.go` with `FuzzParseApply`.
**Goal:** `config.Parse("config.yaml", data)` and `batch.Parse("batch.yaml", data)` never panic, and every error's text contains the name passed.
**Approach (assumed at the header base):** config seeds: the default file `config.Default` is written from (the one `TestParse_DefaultFile_EqualsDefault` reads), this repo's `.capcom/config.yaml`, `format: 7`, a repeated top-level key, `ready:\n  hide: wip`. Apply seeds: the five ops of `docs/site/reference/apply-file.md`'s worked example, an empty list, two documents, `body-file`.
**Files:** internal/config/fuzz_test.go, internal/batch/fuzz_test.go
**Tests:** the two fuzz targets.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/config/ ./internal/batch/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzParseConfig$' -fuzztime=2m -parallel=2 ./internal/config/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzParseApply$' -fuzztime=2m -parallel=2 ./internal/batch/`
**Commit:** `test(config): fuzz config and apply file parsing`

## 8. Run every fuzz target in CI

**What:** add a `fuzz` job to `.github/workflows/ci.yml`: ubuntu-latest, `fail-fast: false`, a matrix `include` of `{package, target}` for the seven targets of items 4–7, each step `go test -run='^$' -fuzz='^${{ matrix.target }}$' -fuzztime=30s ./${{ matrix.package }}`. On failure, upload `${{ matrix.package }}/testdata/fuzz/` as an artifact so the crasher can be committed.
**Goal:** a push or pull request runs all seven targets for 30s each and fails on a crash; `CONTRIBUTING.md` says how to run one target locally and that a crasher is committed under `testdata/fuzz/<Target>/` with its fix.
**Approach (assumed at the header base):** a job beside `lint`; setup as in the `test` job.
**Files:** .github/workflows/ci.yml, CONTRIBUTING.md
**Tests:** a 10-minute soak of each target locally, recorded as a NOTES line (targets and outcome).
**Acceptance:**
- `grep -c 'Fuzz' .github/workflows/ci.yml` counts seven targets
- `for t in ...` each target runs `-fuzztime=10m -parallel=2` with no failure
**Depends on items 3, 4, 5, 6, 7.**
**Closes:** fuzz-parsers
**Commit:** `ci: fuzz the parsers on every push`

## 9. Property-test MergeFile against git merge-file

**What:** add `internal/issue/merge_property_test.go` (issue `merge-driver-property-test`, unit layer).
**Goal:** `TestMergeFile_RandomEdits_KeepEveryLogEntry` runs N iterations (`CAPCOM_PROPERTY_ITERATIONS`, default 200), each seeded from `math/rand/v2` with the seed in the failure message along with the three inputs. Each builds a base issue file, applies two independent random edit sequences (Log entries with distinct dates and texts; title, priority, status, labels, deps, body paragraph) giving ours and theirs, merges with `MergeFile` and a test-local textMerge running `git merge-file -p`, and asserts: without conflict, the result passes `issue.Parse`, every Log entry of ours and theirs appears once and in date order, and a field changed on one side only has that side's value; with conflict, `rawfm.ConflictMarkers` finds a marker. Skips when `git` is not on PATH.
**Approach (assumed at the header base):** the textMerge helper writes the three versions to `t.TempDir()` and runs `git merge-file -p ours base theirs` (exit 1 = conflict, >1 = error); keep it test-local, do not move `cli.gitTextMerge`.
**Files:** internal/issue/merge_property_test.go
**Tests:** the property test; check once by hand that breaking `MergeFile` fails it with the seed printed (not committed).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run RandomEdits ./internal/issue/`
- `CAPCOM_PROPERTY_ITERATIONS=5000 GOMEMLIMIT=2GiB go test -count=1 -run RandomEdits ./internal/issue/`
**Depends on item 5.**
**Commit:** `test(issue): property-test the Log merge against git merge-file`

## 10. Property-test the merge driver through git merge

**What:** add `internal/cli/merge_property_test.go` (issue `merge-driver-property-test`, end-to-end layer).
**Goal:** `TestMergeDriver_RandomBranches_KeepEveryLogEntry` runs N iterations (`CAPCOM_PROPERTY_ITERATIONS`, default 20 here, since each is a git repo): a repo from `newGitRepo` with `capcom init --merge-driver`, the two `git config merge.capcom-log.*` commands init prints, a few issues committed on `main`, two branches each running random `capcom log`, `update --priority/--title`, `dep add`, `close` through `runCLI` and committing, then `git merge` of both into `main`. It asserts `capcom check` exits 0 or reports only `conflict` problems, and `capcom show <id> --json` lists every Log entry either branch added. Seed and failing repo path in the failure message.
**Approach (assumed at the header base):** git runs the driver as `capcom merge-driver …`, so the test builds `./cmd/capcom` once (`go build -o <tmp>/capcom`, as `cmd/agenteval/run.go` builds its build under test) and prepends its directory to `PATH` with `t.Setenv`; skip under `testing.Short()`. Use `runGit` from `lock_test.go`.
**Files:** internal/cli/merge_property_test.go
**Tests:** the property test.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run RandomBranches ./internal/cli/`
- the default run finishes in under 20s on CI (`go test -v` timing)
**Depends on item 9.**
**Closes:** merge-driver-property-test
**Commit:** `test(cli): property-test the merge driver through git merge`
