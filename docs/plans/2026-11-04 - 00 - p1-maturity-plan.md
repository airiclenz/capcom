# P1 code maturity: static analysis, fuzzing, merge-driver properties

**Goal:** Close the three P1 issues under `code-maturity`: CI gains staticcheck and govulncheck on a patched, pinned Go toolchain; the hostile-input parsers gain native fuzz targets run in CI; the merge driver gains seeded property tests at the unit and git level.
**Date:** 2026-11-04
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** ae5ea40

**Sources:**
- `.capcom/issues/code-maturity/ci-static-analysis.md`, `fuzz-parsers.md`, `merge-driver-property-test.md` (each item's Acceptance)
- `.github/workflows/ci.yml`, `Makefile`, `CONTRIBUTING.md`
- Notes `cli-test-harness`, `coding-conventions`, `store-layout`

**Ratified design calls:**
- **Go toolchain:** pin `toolchain go1.25.<latest patch>` in `go.mod`, the patch read when the item runs from proxy.golang.org (`go list -m -versions golang.org/toolchain` or the proxy's `@v/list`; go.dev and vuln.go.dev may be unreachable from a sandbox). The line is a minimum, not a pin of every build: CI (setup-go reading `go.mod`) builds and vulnerability-checks exactly the pinned patch, and a host with a newer local Go keeps its own newer toolchain. (Airic, 2026-11-04)
- **Linters:** staticcheck and govulncheck at pinned versions via `go run <module>@<version>`, never `@latest`; no golangci-lint. (issue `ci-static-analysis`)
- **Fuzz budget:** 30s per target in CI, on every push and pull request, one matrix job per target. (issue `fuzz-parsers`)
- **Crashers:** a crasher a fuzz target finds is fixed in the item that found it, its input committed under `testdata/fuzz/<Target>/`, with a `fix(…)` CHANGELOG entry in the sidecar.
- **Property iterations:** 200 by default, `CAPCOM_PROPERTY_ITERATIONS` overrides. (issue `merge-driver-property-test`)
- **CRLF read:** any run of \r directly before \n reads as one LF. (Airic, 2026-11-04)

**Standing requirements:**
- skills: coding-standards
- Tests on the Pi: single packages with `GOMEMLIMIT=2GiB`, never `-race` on `./...`, never two test commands at once; fuzz with `-parallel=2`.
- CLI tests follow Note `cli-test-harness`: `runCLI`, real temp git repo, never `t.Parallel()`.
- Fuzz and property tests use only the standard library (`testing.F`, `math/rand/v2`); no new module.
- Every authorised deviation from item text lands as a dated NOTES line under the item.

**Regression check (2026-11-04, ae5ea40):**
- 1: guard folded (decision: the toolchain line is a minimum; patch read from proxy.golang.org)
- 2: guard folded (zero value stays valid; staticcheck run at 2026.1)
- 3: guard folded (pins staticcheck 2026.1, govulncheck v1.7.0; supersedes in part Note `adr-0008-amend-makefile`, "every target stays a plain `go` command")
- 4: recast (decision: fixes the CRLF fixed-point bug in `frontmatter.crlfToLF` before fuzzing; marker `Text` trims one `\r`)
- 4: guard folded (re-check: read-side comments and `docs/site/reference/file-format.md` state the `\r…\r\n` run rule; write side stays one pass)
- 5: guard folded (decision: depends on item 4; `package issue_test`, `logMergePlaceholder` seed)
- 6: guard folded (decision: depends on item 4; no second fix in `internal/knowledge`)
- 7: guard folded (config errors carry `Line > 0` except YAML syntax errors, recorded as a NOTES deviation)
- 8: guard folded (per-target artifact name, `target: Fuzz` count, CONTRIBUTING check; decision: seven explicit soak commands)
- 9: guard folded (decision: textMerge via `gitx.MergeFile`; each side's entries in ascending date order)
- 10: guard folded (re-exec the test binary as `capcom`, tolerate a conflicted `git merge`, read the Log with `issue.LogEntries`, acyclic deps, explicit `main`)

**Out of scope:**
- golangci-lint, `staticcheck.conf` beyond a check a finding shows is deliberate
- The other `code-maturity` issues (`yaml-module-swap`, golden tests, fixtures, benchmarks, stress)
- Fuzzing `internal/userconfig`, `internal/hooks`, `cmd/release`

## 1. Pin a patched Go toolchain in go.mod

**What:** `go.mod` declares `go 1.25.0` and no `toolchain`, so setup-go and the release host build with Go 1.25.0 and its unpatched standard library.
**Goal:** `go.mod` carries a `toolchain go1.25.N` line naming the newest Go 1.25 patch release published at run time (read from proxy.golang.org: `go list -m -versions golang.org/toolchain` or `https://proxy.golang.org/golang.org/toolchain/@v/list`), and the `go` line is unchanged.
**Regression guard.** The toolchain line is a minimum, not a pin of every build: CI (setup-go reading `go.mod`) builds and vulnerability-checks exactly the pinned patch, and a host with a newer local Go keeps its own newer toolchain. Read the newest 1.25 patch at run time from proxy.golang.org (`go list -m -versions golang.org/toolchain` or the proxy's `@v/list`), since go.dev and vuln.go.dev may be unreachable from a sandbox.
**Approach (assumed at the header base):** `go mod edit -toolchain=go1.25.N`; `actions/setup-go@v5` with `go-version-file: go.mod` reads `go.mod` and `GOTOOLCHAIN=auto` switches each job to the pinned patch, so `ci.yml` needs no change. Note the patch in `CONTRIBUTING.md` where it says how to build (one sentence: the toolchain line is the minimum Go and CI builds with exactly that patch; a newer local Go keeps its own).
**Files:** go.mod, CONTRIBUTING.md
**Read first:** go.mod — go, toolchain directives; .github/workflows/ci.yml — test, cross (setup-go go-version-file); CONTRIBUTING.md — "Build and test";
cmd/release/dist.go — buildArchive; README.md — "From source (Go 1.25 or later)"
**Tests:** none new; the build and vet prove the toolchain resolves.
**Acceptance:**
- `grep -E '^toolchain go1\.25\.[0-9]+$' go.mod`
- `go version` (run in the repo) reports the pinned patch, or the host's own newer Go (the line is a minimum)
- `go build ./cmd/capcom && go vet ./internal/config/`
**Commit:** `chore(build): pin the patched Go 1.25 toolchain`

## 2. Fix the staticcheck findings in test files

**What:** `staticcheck@2025.1.1 ./...` at the base reports five findings, all in tests: `internal/index/agreement_test.go` unused const `formValid` (U1000); `internal/ui/closed_test.go`, `internal/ui/mouse_test.go`, `internal/ui/uistate_test.go`, `internal/ui/write_close_test.go` a value of `m` never used (SA4006).
**Goal:** `staticcheck ./...` at the version item 3 pins exits 0, with no `//lint:ignore` and no `staticcheck.conf`.
**Regression guard.** Keep the zero value meaning valid: drop the `formValid` line by writing `formInvalid fileForm = iota + 1`, and move its "parses and passes validation" comment to the `fileForm` doc, so fixtures that leave `form` unset (`{path: "foo-bar.md"}`) stay valid. Run staticcheck at item 3's pin, 2026.1 (its go.mod says go 1.25.0); 2025.1.1 (go 1.23) refuses this module when `go run` builds it with an older Go.
**Approach (assumed at the header base):** delete the unused const; for each SA4006, either assert on the reassigned model (when the test meant to check it) or drop the dead assignment (`_ = ` is not a fix). Re-run staticcheck to catch any finding the pinned version adds.
**Files:** internal/index/agreement_test.go, internal/ui/closed_test.go, internal/ui/mouse_test.go, internal/ui/uistate_test.go, internal/ui/write_close_test.go
**Read first:** internal/index/agreement_test.go — fileForm, formValid, content; internal/ui/write_close_test.go — TestWriteClose_L_HeldLock_Asks, submitPrompt;
internal/ui/closed_test.go — TestClosedTab_TabChange_SavesTheTabAndItsSelection; internal/ui/mouse_test.go — TestMouse_ClickDuringReasonPrompt_EnterClosesOnlyThePromptedIssue;
internal/ui/uistate_test.go — TestWorktreeState_TabChange_SavesAtOnce; internal/ui/harness_test.go — press, startModel
**Tests:** the touched tests still pass.
**Acceptance:**
- `go run honnef.co/go/tools/cmd/staticcheck@2026.1 ./...`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
**Depends on item 1.**
**Commit:** `test: drop dead assignments staticcheck reports`

## 3. Run staticcheck and govulncheck in CI and make lint

**What:** add a `lint` job and a `make lint` target (issue `ci-static-analysis`). The job is ubuntu-latest only, `actions/checkout@v4`, `actions/setup-go@v5` with `go-version-file: go.mod`, then `go run honnef.co/go/tools/cmd/staticcheck@<pin> ./...` and `go run golang.org/x/vuln/cmd/govulncheck@<pin> ./...`; pins are the newest releases at run time that build under the item-1 toolchain (staticcheck `2026.1` and govulncheck `v1.7.0` at the base), written once as job `env` values and once as Makefile variables.
**Goal:** CI fails on any staticcheck finding or any govulncheck-reachable vulnerability; `make lint` runs the same two commands at the same pins; `CONTRIBUTING.md` lists both among the CI checks.
**Regression guard.** Pin staticcheck `2026.1` and govulncheck `v1.7.0`; "builds under the item-1 toolchain" means the tool's go.mod `go` line is 1.25 or lower (2026.2.x and v1.8.0+ need go 1.26.0, and `go run` would switch to go1.26.x). In `CONTRIBUTING.md`, give the `make lint` line both commands in its comment and fix the "every target is one of them" sentence. This supersedes in part Note `adr-0008-amend-makefile` ("every target stays a plain `go` command"): add an amendment line there naming `lint` and its two commands.
**Approach (assumed at the header base):** a third job beside `test` and `cross` in `.github/workflows/ci.yml`; `Makefile` gains `STATICCHECK_VERSION`, `GOVULNCHECK_VERSION`, a `lint` target and `lint` in `.PHONY`. If govulncheck reports a reachable module vulnerability, `go get <module>@<fixed>` and `go mod tidy` in this item, then run the affected package's tests. `vuln.go.dev` may be unreachable from a sandbox: then the local govulncheck run is recorded as a NOTES line and the CI run is the check.
**Files:** .github/workflows/ci.yml, Makefile, CONTRIBUTING.md, go.mod, go.sum, .capcom/knowledge/decisions/adr-0008/adr-0008-amend-makefile.md
**Read first:** .github/workflows/ci.yml — jobs test, cross, env COVERAGE_FLOOR; Makefile — .PHONY, vet, test; CONTRIBUTING.md — "Build and test", CI paragraph;
.capcom/knowledge/decisions/adr-0008/adr-0008-amend-makefile.md; go.mod — go line (decides which toolchain `go run tool@v` uses)
**Tests:** none new.
**Acceptance:**
- `make lint` exits 0 (govulncheck may be NOTES-excused only for an unreachable `vuln.go.dev`)
- `grep -n 'staticcheck@\|govulncheck@' .github/workflows/ci.yml Makefile` shows no `@latest`
- `grep -n 'staticcheck' CONTRIBUTING.md && grep -n 'govulncheck' CONTRIBUTING.md`
- `grep -n 'lint' .capcom/knowledge/decisions/adr-0008/adr-0008-amend-makefile.md && capcom check`
**Depends on item 2.**
**Closes:** ci-static-analysis
**Commit:** `ci: run staticcheck and govulncheck`

## 4. Fuzz frontmatter.Decode and the rawfm scanners

**What:** Recast at the regression check (2026-11-04). First fix `crlfToLF` in `internal/frontmatter/frontmatter.go`, which reads CRLF as LF in one pass, so a body line ending `"\r\r\n"` reads as `"\r\n"`, serializes as such and re-reads as `"\n"`, breaking the canonical-form fixed point for issues and Notes; then add `internal/frontmatter/fuzz_test.go` with `FuzzDecode` and `internal/rawfm/fuzz_test.go` with `FuzzConflictMarkers` (issue `fuzz-parsers`).
**Goal:** `FuzzDecode` calls `frontmatter.Decode("x.md", data, kind, apply)` for an issue-like and a Note-like `Kind` (built in the test from a fixed key list) with an `apply` that reads each `Field` through its helpers; it never panics and every error's text contains `x.md`. `FuzzConflictMarkers` calls `rawfm.ConflictMarkers(data)` and `rawfm.Aliases(data)`; it never panics and every returned `ConflictMarker.Line` is between 1 and the line count of `data`, its `Text` equal to that line with one trailing `\r` trimmed (`strings.TrimSuffix`), the line count being `len(strings.Split(string(data), "\n"))` so a final unterminated marker line counts. `frontmatter.Split` reads any run of one or more `\r` directly before `\n` as one LF and keeps a lone `\r`, so parse, serialize, parse is a fixed point for the read.
**Regression guard.** Binding (ratified by Airic, 2026-11-04): any run of one or more `\r` immediately followed by `\n` reads as a single `\n`; a lone `\r` not before `\n` is kept. Add a unit test in `internal/frontmatter` for `"x\r\r\ny"` -> `"x\ny"` and for a lone `"\r"` kept, plus a round-trip test through `issue.Parse`/`Serialize` of a body holding `"\r\r\n"`. The commit is a `fix(frontmatter)` commit and its sidecar carries a fix CHANGELOG entry. Marker `Text` trims exactly one trailing `\r`, as `ConflictMarkers` does.
Every read-side comment or doc line that describes how `Split` or `Decode` handles line endings states the run rule (find them with `grep -rn 'CRLF\|byte for byte' internal/frontmatter docs/site/reference/file-format.md`; at least `file-format.md:297`, `frontmatter.go` `Split` and `crlfToLF` comments, `decode.go` `Document.Body`). The write side (`nodestore/write.go:49`, `edit.NormalizeNewlines`) stays one pass, unchanged.
**Approach (assumed at the header base):** seeds via `f.Add`: every file under `internal/issue/testdata/`, a minimal Note, and hand cases — unclosed fence, CRLF, NUL and `\x1b` bytes, a repeated key, a `---` inside a fenced code block, conflict markers in frontmatter and body, an unclosed code fence above markers. Read testdata with `os.ReadFile` at seed time.
**Files:** internal/frontmatter/frontmatter.go, internal/frontmatter/frontmatter_test.go (new), internal/issue/file_test.go, internal/frontmatter/fuzz_test.go, internal/rawfm/fuzz_test.go, internal/frontmatter/decode.go, docs/site/reference/file-format.md
**Read first:** internal/frontmatter/frontmatter.go — Split, splitAtFence, crlfToLF; internal/frontmatter/decode.go — Decode, decoder.document, decoder.conflictError, Field.Text, Field.List, Field.Scalar;
internal/rawfm/conflict.go — ConflictMarkers, bodyStartIndex; internal/rawfm/aliases.go — Aliases, IsFrontmatterFence; internal/issue/file_test.go — mustParse, readGolden, TestParse_CRLF_SerializesAsLF;
internal/nodestore/write.go — Kind.encode; internal/nodestore/format.go — Kind.Format
**Tests:** the CRLF read unit tests (`"x\r\r\ny"` -> `"x\ny"`, lone `"\r"` kept) and the issue round trip of a `"\r\r\n"` body; the two fuzz targets, whose seeds run under plain `go test`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/frontmatter/ ./internal/rawfm/ ./internal/issue/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzDecode$' -fuzztime=2m -parallel=2 ./internal/frontmatter/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzConflictMarkers$' -fuzztime=2m -parallel=2 ./internal/rawfm/`
**Commit:** `fix(frontmatter): read any run of \r before \n as one LF, and fuzz Decode and the conflict marker scan`

## 5. Fuzz issue parsing and MergeFile

**What:** add `internal/issue/fuzz_test.go` with `FuzzParseIssue` and `FuzzMergeFile`.
**Goal:** `FuzzParseIssue`: `issue.Parse("x.md", data)` never panics; when it succeeds, `s1 := i.Serialize()` parses again and `Parse(s1).Serialize()` equals `s1` byte for byte (canonical form is a fixed point). `FuzzMergeFile` takes three byte slices and calls `issue.MergeFile(base, ours, theirs, fakeTextMerge)`; it never panics and never returns an error (`fakeTextMerge` never fails).
**Regression guard.** Depends on item 4 (the CRLF fix makes the Serialize fixed point hold): the known crasher body `"x\r\r\ny"` is fixed there, in `internal/frontmatter`, not here. `fuzz_test.go` is `package issue_test`, calls `issue.Parse` / `issue.MergeFile`, and seeds the placeholder case with the test const `logMergePlaceholder`.
**Approach (assumed at the header base):** reuse `fakeTextMerge`, `issueFile` and `crlf` from `merge_test.go` (same package, `issue_test`). Seeds: every `testdata/*.md`; for `FuzzMergeFile`, triples built with `issueFile` (both sides append, one side edits the title, CRLF ours, a version holding the Log placeholder).
**Files:** internal/issue/fuzz_test.go
**Read first:** internal/issue/file.go — Parse, Issue.Serialize; internal/issue/merge.go — MergeFile, TextMerge, logPlaceholder;
internal/issue/merge_test.go — fakeTextMerge, issueFile, crlf, logMergePlaceholder, entryOctFirst; internal/frontmatter/frontmatter.go — Split, crlfToLF
**Tests:** the two fuzz targets.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/issue/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzParseIssue$' -fuzztime=2m -parallel=2 ./internal/issue/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzMergeFile$' -fuzztime=2m -parallel=2 ./internal/issue/`
**Depends on item 4.**
**Commit:** `test(issue): fuzz Parse round trips and MergeFile`

## 6. Fuzz Note parsing

**What:** add `internal/knowledge/fuzz_test.go` with `FuzzParseNote`.
**Goal:** `knowledge.Parse("x.md", data)` never panics; when it succeeds, `s1 := n.Serialize()` parses again and re-serializes to `s1` byte for byte.
**Regression guard.** Depends on item 4 (the CRLF fix makes the Serialize fixed point hold): a Note body `"x\r\r\ny"` round-trips through `frontmatter.Split`, fixed once in `internal/frontmatter` by item 4; add no second fix in `internal/knowledge`.
**Approach (assumed at the header base):** seeds: every `.capcom/knowledge/**/*.md` of this repository found by walking `../../.capcom/knowledge` at seed time (skip silently when absent), plus a Note with `retired`, `superseded-by`, `aliases`, `refs`, CRLF and a conflict marker.
**Files:** internal/knowledge/fuzz_test.go
**Read first:** internal/knowledge/note.go — Parse, Note.Serialize, noteSchema; internal/frontmatter/frontmatter.go — Split, crlfToLF;
internal/knowledge/knowledge_test.go — package knowledge (internal tests); .capcom/knowledge — seed walk via ../../.capcom/knowledge
**Tests:** the fuzz target.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/knowledge/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzParseNote$' -fuzztime=2m -parallel=2 ./internal/knowledge/`
**Depends on item 4.**
**Commit:** `test(knowledge): fuzz Note parse round trips`

## 7. Fuzz config and apply-file parsing

**What:** add `internal/config/fuzz_test.go` with `FuzzParseConfig` and `internal/batch/fuzz_test.go` with `FuzzParseApply`.
**Goal:** `config.Parse("config.yaml", data)` and `batch.Parse("batch.yaml", data)` never panic, and every error's text contains the name passed.
**Regression guard.** The issue's `FuzzParseConfig` invariant says an error names path and a line: assert `Line > 0` for every config error except a YAML syntax error, whose line `parser.yamlError` keeps inside the message (`Line` 0), and record that exception as a dated NOTES deviation under this item.
**Approach (assumed at the header base):** config seeds: the default file `config.Default` is written from (the one `TestParse_DefaultFile_EqualsDefault` reads), this repo's `.capcom/config.yaml`, `format: 7`, a repeated top-level key, `ready:\n  hide: wip`. Apply seeds: the five ops of `docs/site/reference/apply-file.md`'s worked example, an empty list, two documents, `body-file`.
**Files:** internal/config/fuzz_test.go, internal/batch/fuzz_test.go
**Read first:** internal/config/parse.go — Parse, parser.yamlError, parser.errorAt; internal/config/config.go — DefaultFile, Error.Error;
internal/config/config_test.go — TestParse_DefaultFile_EqualsDefault (package config); internal/batch/parse.go — Parse, Error.Error;
internal/batch/parse_test.go — package batch_test; docs/site/reference/apply-file.md — worked example
**Tests:** the two fuzz targets.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/config/ ./internal/batch/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzParseConfig$' -fuzztime=2m -parallel=2 ./internal/config/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzParseApply$' -fuzztime=2m -parallel=2 ./internal/batch/`
**Commit:** `test(config): fuzz config and apply file parsing`

## 8. Run every fuzz target in CI

**What:** add a `fuzz` job to `.github/workflows/ci.yml`: ubuntu-latest, `fail-fast: false`, a matrix `include` of `{package, target}` for the seven targets of items 4–7, each step `go test -run='^$' -fuzz='^${{ matrix.target }}$' -fuzztime=30s ./${{ matrix.package }}`. On failure, upload `${{ matrix.package }}/testdata/fuzz/` as an artifact so the crasher can be committed.
**Regression guard.** Name the upload `fuzz-${{ matrix.target }}` with `if: failure()`, path `${{ matrix.package }}/testdata/fuzz/`, so two crashing targets in one run do not collide on one artifact name (upload-artifact@v4 refuses a name that exists). Count targets with `grep -c 'target: Fuzz'`, not `'Fuzz'`, which also counts step names.
**Goal:** a push or pull request runs all seven targets for 30s each and fails on a crash; `CONTRIBUTING.md` says how to run one target locally and that a crasher is committed under `testdata/fuzz/<Target>/` with its fix.
**Approach (assumed at the header base):** a job beside `lint`; setup as in the `test` job.
**Files:** .github/workflows/ci.yml, CONTRIBUTING.md
**Read first:** .github/workflows/ci.yml — jobs test, cross (setup-go go-version-file); CONTRIBUTING.md — "Build and test" CI paragraph;
.gitignore — testdata/ not ignored (crashers committable)
**Tests:** a 10-minute soak of each target locally, recorded as a NOTES line (targets and outcome).
**Acceptance:**
- `grep -c 'target: Fuzz' .github/workflows/ci.yml` prints 7
- `grep -n 'testdata/fuzz' CONTRIBUTING.md`
- `go test -run='^$' -fuzz='^FuzzDecode$' -fuzztime=10m -parallel=2 ./internal/frontmatter/`
- `go test -run='^$' -fuzz='^FuzzConflictMarkers$' -fuzztime=10m -parallel=2 ./internal/rawfm/`
- `go test -run='^$' -fuzz='^FuzzParseIssue$' -fuzztime=10m -parallel=2 ./internal/issue/`
- `go test -run='^$' -fuzz='^FuzzMergeFile$' -fuzztime=10m -parallel=2 ./internal/issue/`
- `go test -run='^$' -fuzz='^FuzzParseNote$' -fuzztime=10m -parallel=2 ./internal/knowledge/`
- `go test -run='^$' -fuzz='^FuzzParseConfig$' -fuzztime=10m -parallel=2 ./internal/config/`
- `go test -run='^$' -fuzz='^FuzzParseApply$' -fuzztime=10m -parallel=2 ./internal/batch/`
**Depends on items 3, 4, 5, 6, 7.**
**Closes:** fuzz-parsers
**Commit:** `ci: fuzz the parsers on every push`

## 9. Property-test MergeFile against git merge-file

**What:** add `internal/issue/merge_property_test.go` (issue `merge-driver-property-test`, unit layer).
**Goal:** `TestMergeFile_RandomEdits_KeepEveryLogEntry` runs N iterations (`CAPCOM_PROPERTY_ITERATIONS`, default 200), each seeded from `math/rand/v2` with the seed in the failure message along with the three inputs. Each builds a base issue file, applies two independent random edit sequences (Log entries with distinct dates and texts, each side's new entries in ascending date order; title, priority, status, labels, deps, body paragraph) giving ours and theirs, merges with `MergeFile` and a test-local textMerge running `git merge-file -p`, and asserts: without conflict, the result passes `issue.Parse`, every Log entry of ours and theirs appears once and in date order, and a field changed on one side only has that side's value; with conflict, `rawfm.ConflictMarkers` finds a marker. Skips when `git` is not on PATH.
**Regression guard.** Use `gitx.MergeFile` (internal/gitx/merge.go) as the textMerge, through a small test-local adapter that writes the three versions to `t.TempDir()` and calls `gitx.MergeFile` with `gitx.MergeLabels` — no hand-written exec of `git merge-file`; `internal/gitx` does not import `internal/issue`, so there is no import cycle. Generate each side's new Log entries in ascending date order, all later than base's last entry, dates distinct across sides (`mergeEntries` never reorders a side).
**Approach (assumed at the header base):** the textMerge adapter writes the three versions to `t.TempDir()` and calls `gitx.MergeFile` (a conflict is its `true` result, never an error); keep it test-local, do not move `cli.gitTextMerge`.
**Files:** internal/issue/merge_property_test.go
**Read first:** internal/issue/merge.go — MergeFile, TextMerge, mergeLogs, mergeEntries, entryDate; internal/gitx/merge.go — MergeFile, maxConflictExit;
internal/issue/merge_test.go — issueFile, fakeTextMerge (package issue_test); internal/rawfm/conflict.go — ConflictMarkers; internal/issue/file.go — Parse, Serialize
**Tests:** the property test; check once by hand that breaking `MergeFile` fails it with the seed printed (not committed).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run RandomEdits ./internal/issue/`
- `CAPCOM_PROPERTY_ITERATIONS=5000 GOMEMLIMIT=2GiB go test -count=1 -run RandomEdits ./internal/issue/`
**Depends on item 5.**
**Commit:** `test(issue): property-test the Log merge against git merge-file`

## 10. Property-test the merge driver through git merge

**What:** add `internal/cli/merge_property_test.go` (issue `merge-driver-property-test`, end-to-end layer).
**Goal:** `TestMergeDriver_RandomBranches_KeepEveryLogEntry` runs N iterations (`CAPCOM_PROPERTY_ITERATIONS`, default 20 here, since each is a git repo): a repo from `newGitRepo` with `capcom init --merge-driver`, the two `git config merge.capcom-log.*` commands init prints, a few issues committed on `main` (created with `git checkout -q -b main` after init, since `newGitRepo` starts on `master`), two branches each running random `capcom log`, `update --priority/--title`, `dep add` (only from a lower- to a higher-indexed issue), `close` through `runCLI` and committing, then `git merge` of both into `main`. It asserts `capcom check` exits 0 or reports only `conflict` problems, and `issue.LogEntries` on each issue's merged file bytes lists every Log entry either branch added. Seed and failing repo path in the failure message.
**Regression guard.** Do not build `./cmd/capcom`: re-exec the test binary as `clipboard_test.go`'s `TestMain` does — copy `os.Executable()` to `<tmp>/capcom` (`capcom.exe` on Windows), set an env var that makes a new `internal/cli` `TestMain` run `Run(os.Args[1:], ...)` and exit, and prepend `<tmp>` to `PATH`. Run `git merge` with `exec.Command` tolerating exit 1 (as `TestCheck_LogMergeConflict_ReportsEachMarkerAndExitsOne` does), then `git commit -a` the conflicted state before the next merge. Read each issue's entries with `issue.LogEntries` on the merged file bytes, not `show --json` (a conflicted issue does not parse, and `issueData` has no Log field). Add deps only from a lower- to a higher-indexed issue, so no `cycle` problem arises. Create `main` explicitly with `git checkout -q -b main` after init.
**Approach (assumed at the header base):** git runs the driver as `capcom merge-driver …`, so the test puts the re-executed test binary on `PATH` as `capcom` with `t.Setenv`; skip under `testing.Short()`. Use `runGit` from `lock_test.go` for every git command except `git merge`.
**Files:** internal/cli/merge_property_test.go (with the package's new `TestMain`)
**Read first:** internal/cli/merge_driver.go — newMergeDriverCommand, mergeIssueFile; internal/cli/check_test.go — TestCheck_LogMergeConflict_ReportsEachMarkerAndExitsOne;
internal/cli/testharness_test.go — runCLI, setHermeticGitEnv; internal/cli/init_test.go — newGitRepo; internal/cli/lock_test.go — runGit;
internal/clipboard/clipboard_test.go — TestMain, testBinary; internal/issue/merge.go — LogEntries
**Tests:** the property test.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run RandomBranches ./internal/cli/`
- the default run finishes in under 20s on CI (`go test -v` timing)
**Depends on item 9.**
**Closes:** merge-driver-property-test
**Commit:** `test(cli): property-test the merge driver through git merge`
