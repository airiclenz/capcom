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
- **Go toolchain:** pin `toolchain go1.26.<latest patch>` in `go.mod` (Go 1.25 is end of life since 2026-08-19), the patch read when the item runs, the `go 1.25.0` line unchanged so 1.25 users build via GOTOOLCHAIN=auto. (Airic, 2026-11-04, superseding the earlier 1.25 call)
- **Linters:** staticcheck and govulncheck at pinned versions via `go run <module>@<version>`, never `@latest`; no golangci-lint. (issue `ci-static-analysis`)
- **Fuzz budget:** 30s per target in CI, on every push and pull request, one matrix job per target. (issue `fuzz-parsers`)
- **Crashers:** a crasher a fuzz target finds is fixed in the item that found it, its input committed under `testdata/fuzz/<Target>/`, with a `fix(…)` CHANGELOG entry in the sidecar.
- **Property iterations:** 200 by default, `CAPCOM_PROPERTY_ITERATIONS` overrides. (issue `merge-driver-property-test`)
- **Release gate:** lint runs in ci.yml and gates releases, fuzz runs in its own workflow and never gates a release. (Airic, 2026-11-04)

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

**Regression check (2026-11-04, ae5ea40):**
- 1: recast (toolchain pin moves to go1.26.N, ratified); guard folded (setup-go@v5 reads only the `go` line; `go 1.25.0` kept and checked)
- 2: guard folded (staticcheck at item 3's pin; `formValid` stays the zero value; drop only `m = `)
- 3: guard folded (newest pins under go1.26; a stdlib vuln is fixed by raising the toolchain); yields to ADR-0008's release gate
- 4: recast (owns the crlfToLF fix); guard folded (marker line count and `Text` defined)
- 5: guard folded (after item 4; `package issue_test`; timestamp year range)
- 6: guard folded (after item 4)
- 7: guard folded (errors assert the path, not a line)
- 8: recast (fuzz jobs move to `.github/workflows/fuzz.yml`, off the release gate); guard folded (Acceptance greps)
- 8: rejected — the CI step drops `-run='^$'` and the upload needs `if: failure()`; the item's What already writes `go test -run='^$' -fuzz=…` and uploads "On failure"
- 9: guard folded (git merge-file exits with the conflict count; Log dates ascend per side)
- 10: guard folded (seed-determined texts; acyclic deps; a merge runner that accepts exit 1; raw Log for conflicted issues; build by import path)

## 1. Pin a patched Go toolchain in go.mod — ✅ DONE (2026-11-04)

NOTES (2026-11-04): re-derived from the assumption that actions/setup-go@v5 reads the go.mod toolchain line; per the DECISION, both setup-go steps in .github/workflows/ci.yml (test and cross, the only workflow using setup-go) move to actions/setup-go@v6.
NOTES (2026-11-04): newest Go 1.26 patch at run time is go1.26.10 (go.dev/dl JSON and go list -m -versions golang.org/toolchain agree); README and install.md "Go 1.25 or later" left as is, still true via GOTOOLCHAIN=auto.

**What:** Recast at the regression check (2026-11-04). `go.mod` declares `go 1.25.0` and no `toolchain`, so setup-go and the release host build with Go 1.25.0 and its unpatched standard library.
**Goal:** `go.mod` carries a `toolchain go1.26.N` line naming the newest Go 1.26 patch release published at run time (read from `https://go.dev/dl/?mode=json&include=all` or `go list -m -versions golang.org/toolchain`), and the `go` line is unchanged.
**Regression guard.** Ratified design call, replacing the header's Go toolchain line: "**Go toolchain:** pin `toolchain go1.26.<latest patch>` in `go.mod` (Go 1.25 is end of life since 2026-08-19), the patch read when the item runs, the `go 1.25.0` line unchanged so 1.25 users build via GOTOOLCHAIN=auto. (Airic, 2026-11-04, superseding the earlier 1.25 call)". It supersedes the header's earlier `toolchain go1.25.<latest patch>` call. Item 1's Goal, Acceptance grep and commit message name go1.26.N accordingly, and CONTRIBUTING notes that a local build auto-switches up to the pin.
`actions/setup-go@v5` reads only the `go` line of `go.mod` (v6 reads `toolchain`), so the Approach's "needs no change" is wrong: either bump `actions/setup-go` to v6 in every job (test, cross, and the new lint and fuzz jobs) and list ci.yml in Files, or reword the Approach to say the go command's auto-switch fetches the pin in each job.
**Approach (assumed at the header base):** `go mod edit -toolchain=go1.26.N`; `actions/setup-go@v5` with `go-version-file: go.mod` reads the `toolchain` directive first, so `ci.yml` needs no change. Note the patch in `CONTRIBUTING.md` where it says how to build (one sentence: the toolchain line pins the patched Go; `GOTOOLCHAIN=auto` fetches it).
**Files:** go.mod, CONTRIBUTING.md, .github/workflows/ci.yml (only when setup-go moves to v6)
**Read first:** go.mod — go directive (no toolchain line); .github/workflows/ci.yml — test, cross (setup-go@v5 go-version-file); CONTRIBUTING.md — "Build and test";
cmd/release/dist.go — buildArchive; README.md — "From source (Go 1.25 or later)"; docs/site/getting-started/install.md — go install section
**Tests:** none new; the build and vet prove the toolchain resolves.
**Acceptance:**
- `grep -E '^toolchain go1\.26\.[0-9]+$' go.mod && grep -qx 'go 1.25.0' go.mod`, the pin compared against the newest go1.26 patch on go.dev/dl
- `go version` (run in the repo) reports the pinned patch
- `go build ./cmd/capcom && go vet ./internal/config/`
**Commit:** `chore(build): pin the patched Go 1.26 toolchain`

## 2. Fix the staticcheck findings in test files — ✅ DONE (2026-11-04)

NOTES (2026-11-04): staticcheck run at 2026.2.2 (module v0.8.2, the newest honnef.co/go/tools release, built under go1.26.10); item 3 has not yet set STATICCHECK_VERSION in the Makefile, so the pin was read from `go list -m -versions honnef.co/go/tools`. It reported the U1000 plus four SA4006 (not write_close_test.go:223); :223 had its dead `m = ` dropped anyway, as the item's guard names it.
NOTES (2026-11-04): formValid kept and named in agreementKind.content (`case formValid:` sets k.valid, `fields` now starts empty); every SA4006 fixed by dropping only `m = ` and keeping the press call.

**What:** `staticcheck@2025.1.1 ./...` at the base reports five findings, all in tests: `internal/index/agreement_test.go` unused const `formValid` (U1000); `internal/ui/closed_test.go`, `internal/ui/mouse_test.go`, `internal/ui/uistate_test.go`, `internal/ui/write_close_test.go` a value of `m` never used (SA4006).
**Goal:** `staticcheck ./...` at the version item 3 pins exits 0, with no `//lint:ignore` and no `staticcheck.conf`.
**Regression guard.** staticcheck runs at the pin item 3 chooses (the newest staticcheck release that builds under the go1.26 toolchain, 2026.2.x at write time), not 2025.1.1. Acceptance names that pin as read from the Makefile variable, and any extra finding the newer version adds is fixed in this item.
Never delete the `formValid` line: it is the zero value, so every fixture with no `form:` field would turn invalid; name it in `agreementKind.content` (`case formValid:` → `k.valid`) or rename it `_ fileForm = iota` with its comment. Fix every SA4006 in the listed files (six: `write_close_test.go` has two, :223 and :250): drop only `m = ` and keep the `press(...)` call, or assert on the returned model.
**Approach (assumed at the header base):** delete the unused const; for each SA4006, either assert on the reassigned model (when the test meant to check it) or drop the dead assignment (`_ = ` is not a fix). Re-run staticcheck to catch any finding the pinned version adds.
**Files:** internal/index/agreement_test.go, internal/ui/closed_test.go, internal/ui/mouse_test.go, internal/ui/uistate_test.go, internal/ui/write_close_test.go
**Read first:** internal/index/agreement_test.go — fileForm consts, agreementKind.content, agreementKind.write; internal/ui/write_close_test.go — TestWriteClose_L_FileChangedOnDisk_AsksAndYRunsAgain, TestWriteClose_L_HeldLock_Asks;
internal/ui/closed_test.go — TestClosedTab_TabChange_SavesTheTabAndItsSelection; internal/ui/mouse_test.go — TestMouse_ClickDuringReasonPrompt_EnterClosesOnlyThePromptedIssue; internal/ui/uistate_test.go — TestWorktreeState_TabChange_SavesAtOnce
**Tests:** the touched tests still pass.
**Acceptance:**
- `go run honnef.co/go/tools/cmd/staticcheck@<STATICCHECK_VERSION, as the Makefile sets it> ./...`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/`
**Depends on item 1.**
**Commit:** `test: drop dead assignments staticcheck reports`

## 3. Run staticcheck and govulncheck in CI and make lint — ✅ DONE (2026-11-04)

NOTES (2026-11-04): pins are staticcheck 2026.2.2 (honnef.co/go/tools v0.8.2, the newest on proxy.golang.org) and govulncheck v1.9.0 (the newest golang.org/x/vuln tag); the lint job uses actions/setup-go@v6 per item 1's decision, not the @v5 the item text names.
NOTES (2026-11-04): `make lint` exited 0 locally under go1.26.10: staticcheck clean, govulncheck "No vulnerabilities found."; go.mod and go.sum unchanged (`go run pkg@version` does not touch them).
NOTES (2026-11-04): CONTRIBUTING.md's "every target is one of them" sentence now names `make lint` as the two `go run` lines in the Makefile, since its comment lists no single go command.
NOTES (2026-11-04): ADR-0008 and its Makefile amendment were left unedited: the gate reads `ci.yml`'s overall run status (cmd/release/repo.go HasGreenRun), which now includes the lint job with no code change, and the amendment's "every target stays a plain go command" still holds.

**What:** add a `lint` job and a `make lint` target (issue `ci-static-analysis`). The job is ubuntu-latest only, `actions/checkout@v4`, `actions/setup-go@v5` with `go-version-file: go.mod`, then `go run honnef.co/go/tools/cmd/staticcheck@<pin> ./...` and `go run golang.org/x/vuln/cmd/govulncheck@<pin> ./...`; pins are the newest tagged releases at run time that build under the item-1 go1.26 toolchain (staticcheck 2026.2.x and govulncheck v1.9.x at write time), written once as job `env` values and once as Makefile variables. The lint job lives in ci.yml and so joins the ADR-0008 release gate (green CI on the gated commit).
**Goal:** CI fails on any staticcheck finding or any govulncheck-reachable vulnerability; `make lint` runs the same two commands at the same pins; `CONTRIBUTING.md` lists both among the CI checks.
**Regression guard.** Pins are the newest tagged releases at run time that build under the item-1 go1.26 toolchain (staticcheck 2026.2.x and govulncheck v1.9.x at write time), and the stale 2025.1.1 / v1.1.4 examples are dropped. A reachable stdlib vulnerability is fixed by raising the toolchain line to a patched go1.26 release, never excused. The item yields to ADR-0008 (`.capcom/knowledge/decisions/adr-0008/adr-0008-decision.md`): its "green CI on the gated commit" now includes lint, per the header's Release gate call.
**Approach (assumed at the header base):** a third job beside `test` and `cross` in `.github/workflows/ci.yml`; `Makefile` gains `STATICCHECK_VERSION`, `GOVULNCHECK_VERSION`, a `lint` target and `lint` in `.PHONY`. If govulncheck reports a reachable module vulnerability, `go get <module>@<fixed>` and `go mod tidy` in this item, then run the affected package's tests. `vuln.go.dev` may be unreachable from a sandbox: then the local govulncheck run is recorded as a NOTES line and the CI run is the check.
**Files:** .github/workflows/ci.yml, Makefile, CONTRIBUTING.md, go.mod, go.sum
**Read first:** .github/workflows/ci.yml — test, cross jobs; Makefile — .PHONY, vet, test targets; CONTRIBUTING.md — "Build and test" CI paragraph; cmd/agenteval/agent.go — agentLoop.post;
cmd/release/gate.go — checkGreenRun; cmd/release/repo.go — HasGreenRun; .capcom/knowledge/decisions/adr-0008/adr-0008-amend-makefile.md — Makefile rule
**Tests:** none new.
**Acceptance:**
- `make lint` exits 0 (govulncheck may be NOTES-excused only for an unreachable `vuln.go.dev`)
- `grep -n 'staticcheck@\|govulncheck@' .github/workflows/ci.yml Makefile` shows no `@latest`
- `grep -n 'staticcheck' CONTRIBUTING.md && grep -n 'govulncheck' CONTRIBUTING.md`
**Depends on item 2.**
**Closes:** ci-static-analysis
**Commit:** `ci: run staticcheck and govulncheck`

## 4. Fuzz frontmatter.Decode and the rawfm scanners — ✅ DONE (2026-11-04)

NOTES (2026-11-04): re-derived from the assumption that internal/frontmatter/frontmatter_test.go exists: the package's Split tests live in internal/frontmatter/decode_test.go (TestSplit_Fences), so the `\r\r\n` Split case is a new row of that table.
NOTES (2026-11-04): consequential edit — docs/site/reference/file-format.md: made necessary by crlfToLF reading a run of carriage returns before a line feed as one line ending (the line-endings bullet now says so).
NOTES (2026-11-04): FuzzDecode reuses decode_test.go's issueKind, noteKind and listKeys (same package frontmatter_test) and also checks Split's front and body hold no `\r\n`; FuzzConflictMarkers additionally asserts markers in strictly ascending line order and every alias non-empty and listed once, as their doc comments state. No crasher was found in 45s runs of each target, so nothing is committed under testdata/fuzz/.

**What:** Recast at the regression check (2026-11-04). Add `internal/frontmatter/fuzz_test.go` with `FuzzDecode` and `internal/rawfm/fuzz_test.go` with `FuzzConflictMarkers` (issue `fuzz-parsers`).
**Goal:** `FuzzDecode` calls `frontmatter.Decode("x.md", data, kind, apply)` for an issue-like and a Note-like `Kind` (built in the test from a fixed key list) with an `apply` that reads each `Field` through its helpers; it never panics and every error's text contains `x.md`. `FuzzConflictMarkers` calls `rawfm.ConflictMarkers(data)` and `rawfm.Aliases(data)`; it never panics and every returned `ConflictMarker.Line` is between 1 and the line count of `data` (`len(strings.Split(string(data), "\n"))`), its `Text` equal to that line with one trailing `\r` removed (`strings.TrimSuffix`), as `ConflictMarkers`' doc comment states.
**Regression guard.** Item 4 owns the CRLF normalisation fix the reviewers found (crlfToLF in internal/frontmatter/frontmatter.go does one ReplaceAll pass, so `\r\r\n` reads as `\r\n` and Parse/Serialize is not a fixed point for issues and Notes). Add internal/frontmatter/frontmatter.go and its test file to Files, make crlfToLF's output contain no `\r\n` (e.g. collapse every run of `\r` immediately before `\n`), add a unit test for Split on `\r\r\n` input plus a FuzzDecode or Split-level invariant that the normalised text holds no `\r\n`, and carry a fix(frontmatter) commit message and CHANGELOG sidecar entry.
**Approach (assumed at the header base):** seeds via `f.Add`: every file under `internal/issue/testdata/`, a minimal Note, and hand cases — unclosed fence, CRLF, NUL and `\x1b` bytes, a repeated key, a `---` inside a fenced code block, conflict markers in frontmatter and body, an unclosed code fence above markers. Read testdata with `os.ReadFile` at seed time.
**Files:** internal/frontmatter/fuzz_test.go, internal/rawfm/fuzz_test.go, internal/frontmatter/frontmatter.go, internal/frontmatter/frontmatter_test.go
**Read first:** internal/rawfm/conflict.go — ConflictMarkers, bodyStartIndex; internal/rawfm/aliases.go — Aliases, frontmatterEndIndex; internal/rawfm/fence.go — ClosedCodeLines;
internal/frontmatter/decode.go — Decode, Kind, Field.Text, Field.List, decoder.yamlError; internal/frontmatter/frontmatter.go — Split, Error.Error; internal/frontmatter/decode_test.go — package frontmatter_test;
internal/issue/testdata — full.md, fence-in-code-block.md
**Tests:** the two fuzz targets; their seeds run under plain `go test`; a `Split` unit test on `\r\r\n` input; the no-`\r\n` invariant on the normalised text (in `FuzzDecode` or at `Split` level).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/frontmatter/ ./internal/rawfm/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzDecode$' -fuzztime=2m -parallel=2 ./internal/frontmatter/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzConflictMarkers$' -fuzztime=2m -parallel=2 ./internal/rawfm/`
**Commit:** `fix(frontmatter): read a run of \r before \n as one line ending; fuzz Decode and the conflict marker scan`

## 5. Fuzz issue parsing and MergeFile — ✅ DONE (2026-11-04)

NOTES (2026-11-04): the out-of-range timestamp error has its own message, `<key>: the timestamp falls outside the years 0000 to 9999 in UTC`, since the value is valid RFC3339 and the existing "expected an RFC3339 timestamp" text would misdescribe it; file_test.go covers both ends (a `created` before 0000 and a `closed` after 9999) in TestParse_Failures_CiteFileLine.
NOTES (2026-11-04): FuzzParseIssue seeds add hand cases beside testdata/*.md (`\r\r\n`, the 0000/9999 UTC boundaries, the +01:00 year-0000 value, fractional seconds, quoted text, NUL and `\x1b` bytes); FuzzMergeFile seeds also include a fenced Log example, three empty versions and a Log-less side. No crasher was found in 2m runs of each target, so nothing is committed under testdata/fuzz/.
NOTES (2026-11-04): the commit message leads with fix(issue) instead of the planned test(issue), since the item carries the timestamp fix (as item 4 led with fix(frontmatter)).

**What:** add `internal/issue/fuzz_test.go` with `FuzzParseIssue` and `FuzzMergeFile`.
**Goal:** `FuzzParseIssue`: `issue.Parse("x.md", data)` never panics; when it succeeds, `s1 := i.Serialize()` parses again and `Parse(s1).Serialize()` equals `s1` byte for byte (canonical form is a fixed point). `FuzzMergeFile` takes three byte slices and calls `issue.MergeFile(base, ours, theirs, fakeTextMerge)`; it never panics and never returns an error (`fakeTextMerge` never fails).
**Regression guard.** Depends on item 4 (its crlfToLF fix makes the serialize fixed point hold). fuzz_test.go declares `package issue_test` so it can reuse merge_test.go's helpers.
`created: 0000-01-01T00:00:00+01:00` parses but serializes as `-0001-12-31T23:00:00Z`, which Parse refuses: `timestamp()` (internal/issue/file.go, the `time.Parse(time.RFC3339, …)` line) refuses a value whose UTC year falls outside 0000–9999, for `created` and `closed` alike, with an `internal/issue/file_test.go` case; a file holding such a value stops loading, and the fix carries a `fix(issue)` CHANGELOG sidecar entry.
**Approach (assumed at the header base):** reuse `fakeTextMerge`, `issueFile` and `crlf` from `merge_test.go` (same package). Seeds: every `testdata/*.md`; for `FuzzMergeFile`, triples built with `issueFile` (both sides append, one side edits the title, CRLF ours, a version holding the Log placeholder).
**Files:** internal/issue/fuzz_test.go, internal/issue/file.go, internal/issue/file_test.go
**Read first:** internal/issue/file.go — Parse, Serialize, timestamp, timeField; internal/frontmatter/frontmatter.go — Split, crlfToLF; internal/frontmatter/encode.go — Encoder.Text, isPlainSafe;
internal/issue/merge.go — MergeFile, parseLog, logSection.withPlaceholder; internal/issue/merge_test.go — fakeTextMerge, issueFile, crlf, logMergePlaceholder; internal/issue/file_test.go — TestParse_CRLF_SerializesAsLF
**Tests:** the two fuzz targets; a `file_test.go` case for a timestamp whose UTC year leaves 0000–9999.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/issue/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzParseIssue$' -fuzztime=2m -parallel=2 ./internal/issue/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzMergeFile$' -fuzztime=2m -parallel=2 ./internal/issue/`
**Depends on item 4.**
**Commit:** `test(issue): fuzz Parse round trips and MergeFile`

## 6. Fuzz Note parsing — ✅ DONE (2026-11-04)

NOTES (2026-11-04): fuzz_test.go declares `package knowledge`, as knowledge_test.go does; seeds are the 109 committed Notes walked from ../../.capcom/knowledge (skipped when absent) plus hand cases: every key with retired true, CRLF with a `\r\r\n` body, a conflict marker in the frontmatter and in the body, quoted and empty values, NUL and `\x1b` bytes, an empty frontmatter. No crasher was found in a 2m run, so nothing is committed under testdata/fuzz/.

**What:** add `internal/knowledge/fuzz_test.go` with `FuzzParseNote`.
**Goal:** `knowledge.Parse("x.md", data)` never panics; when it succeeds, `s1 := n.Serialize()` parses again and re-serializes to `s1` byte for byte.
**Regression guard.** Depends on item 4 (same crlfToLF fix).
**Approach (assumed at the header base):** seeds: every `.capcom/knowledge/**/*.md` of this repository found by walking `../../.capcom/knowledge` at seed time (skip silently when absent), plus a Note with `retired`, `superseded-by`, `aliases`, `refs`, CRLF and a conflict marker.
**Files:** internal/knowledge/fuzz_test.go
**Read first:** internal/knowledge/note.go — Parse, Serialize, noteSchema, retired; internal/frontmatter/frontmatter.go — Split, crlfToLF; internal/frontmatter/schema.go — Schema.Decode, Schema.Encode;
internal/frontmatter/encode.go — Encoder.List, scalar; internal/clidoc/reference_test.go — committedReferenceDir (precedent for a repo-relative read); internal/knowledge/knowledge_test.go — package knowledge
**Tests:** the fuzz target.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/knowledge/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzParseNote$' -fuzztime=2m -parallel=2 ./internal/knowledge/`
**Depends on item 4.**
**Commit:** `test(knowledge): fuzz Note parse round trips`

## 7. Fuzz config and apply-file parsing

**What:** add `internal/config/fuzz_test.go` with `FuzzParseConfig` and `internal/batch/fuzz_test.go` with `FuzzParseApply`.
**Goal:** `config.Parse("config.yaml", data)` and `batch.Parse("batch.yaml", data)` never panic, and every error's text contains the name passed.
**Regression guard.** The Goal's invariant is: never panics, and every error's text contains the name passed. The issue's "an error names path and a line" holds only where the parser knows a line (yamlError keeps yaml.v3's line inside its message, some yaml.v3 errors carry none), so the target asserts the path always and does not assert a line. The implementer records this narrowing as a dated NOTES line under the item, and item 8's close of fuzz-parsers names it.
**Approach (assumed at the header base):** config seeds: the default file `config.Default` is written from (the one `TestParse_DefaultFile_EqualsDefault` reads), this repo's `.capcom/config.yaml`, `format: 7`, a repeated top-level key, `ready:\n  hide: wip`. Apply seeds: the five ops of `docs/site/reference/apply-file.md`'s worked example, an empty list, two documents, `body-file`.
**Files:** internal/config/fuzz_test.go, internal/batch/fuzz_test.go
**Read first:** internal/config/parse.go — Parse, parser.document, parser.yamlError; internal/config/config.go — DefaultFile, Error.Error; internal/config/config_test.go — TestParse_DefaultFile_EqualsDefault;
internal/batch/parse.go — Parse, parser.list, parser.yamlError, Error.Error; docs/site/reference/apply-file.md — worked example (5 ops); internal/batch/parse_test.go — package batch_test
**Tests:** the two fuzz targets.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/config/ ./internal/batch/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzParseConfig$' -fuzztime=2m -parallel=2 ./internal/config/`
- `GOMEMLIMIT=2GiB go test -run='^$' -fuzz='^FuzzParseApply$' -fuzztime=2m -parallel=2 ./internal/batch/`
**Commit:** `test(config): fuzz config and apply file parsing`

## 8. Run every fuzz target in CI

**What:** Recast at the regression check (2026-11-04). Add a `fuzz` job to `.github/workflows/ci.yml`: ubuntu-latest, `fail-fast: false`, a matrix `include` of `{package, target}` for the seven targets of items 4–7, each step `go test -run='^$' -fuzz='^${{ matrix.target }}$' -fuzztime=30s ./${{ matrix.package }}`. On failure, upload `${{ matrix.package }}/testdata/fuzz/` as an artifact so the crasher can be committed.
**Goal:** a push or pull request runs all seven targets for 30s each and fails on a crash; `CONTRIBUTING.md` says how to run one target locally and that a crasher is committed under `testdata/fuzz/<Target>/` with its fix.
**Regression guard.** Fuzz jobs go in a new workflow file `.github/workflows/fuzz.yml` (push and pull request, one matrix job per target, 30s each, crasher artifact upload), not in ci.yml, so a fuzz failure never blocks the ADR-0008 release gate (ratified, see the Release gate call). Files becomes .github/workflows/fuzz.yml, CONTRIBUTING.md.
The Acceptance checks the targets, the budget and the docs, not lines holding "Fuzz": `grep -cE '^\s*target: Fuzz[A-Za-z]+$'` = 7, `grep -c -- "-fuzztime=30s"` ≥ 1 (both on fuzz.yml), and `grep -n 'testdata/fuzz' CONTRIBUTING.md`.
**Approach (assumed at the header base):** a job beside `lint`; setup as in the `test` job.
**Files:** .github/workflows/fuzz.yml, CONTRIBUTING.md
**Read first:** .github/workflows/ci.yml — test job (setup-go steps), cross job matrix shape; CONTRIBUTING.md — "Build and test", "Conventions and tests";
.capcom/issues/code-maturity/fuzz-parsers.md — CI job spec, Acceptance; cmd/release/repo.go — HasGreenRun
**Tests:** a 10-minute soak of each target locally, recorded as a NOTES line (targets and outcome).
**Acceptance:**
- `grep -cE '^\s*target: Fuzz[A-Za-z]+$' .github/workflows/fuzz.yml` prints 7
- `grep -c -- "-fuzztime=30s" .github/workflows/fuzz.yml` prints at least 1
- `grep -n 'testdata/fuzz' CONTRIBUTING.md`
- `for t in ...` each target runs `-fuzztime=10m -parallel=2` with no failure
**Depends on items 3, 4, 5, 6, 7.**
**Closes:** fuzz-parsers
**Commit:** `ci: fuzz the parsers on every push`

## 9. Property-test MergeFile against git merge-file

**What:** add `internal/issue/merge_property_test.go` (issue `merge-driver-property-test`, unit layer).
**Goal:** `TestMergeFile_RandomEdits_KeepEveryLogEntry` runs N iterations (`CAPCOM_PROPERTY_ITERATIONS`, default 200), each seeded from `math/rand/v2` with the seed in the failure message along with the three inputs. Each builds a base issue file, applies two independent random edit sequences (Log entries with distinct dates and texts; title, priority, status, labels, deps, body paragraph) giving ours and theirs, merges with `MergeFile` and a test-local textMerge running `git merge-file -p`, and asserts: without conflict, the result passes `issue.Parse`, every Log entry of ours and theirs appears once and in date order, and a field changed on one side only has that side's value; with conflict, `rawfm.ConflictMarkers` finds a marker. Skips when `git` is not on PATH.
**Regression guard.** git merge-file exits with the number of conflicts (1..127; 255 when the merge fails), so the textMerge helper calls `gitx.MergeFile` with fixed `gitx.MergeLabels` (importable from `package issue_test`), or reads 1..127 as a conflict and anything else as an error; `cli.gitTextMerge` stays where it is. The generator gives each side's appended Log entries strictly ascending dates, all later than the last base entry, since `MergeFile` never reorders existing entries; "in date order" is asserted only under that premise.
**Approach (assumed at the header base):** the textMerge helper writes the three versions to `t.TempDir()` and runs `git merge-file -p ours base theirs` through `gitx.MergeFile` (exit 1..127 = conflict, other non-zero = error); keep it test-local, do not move `cli.gitTextMerge`.
**Files:** internal/issue/merge_property_test.go
**Read first:** internal/issue/merge.go — MergeFile, TextMerge, mergeEntries, mergeLogs; internal/gitx/merge.go — MergeFile, maxConflictExit, MergeLabels;
internal/issue/merge_test.go — issueFile, fakeTextMerge, conflict, crlf; internal/rawfm/conflict.go — ConflictMarkers; internal/issue/file.go — Parse, issueSchema
**Tests:** the property test; check once by hand that breaking `MergeFile` fails it with the seed printed (not committed).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run RandomEdits ./internal/issue/`
- `CAPCOM_PROPERTY_ITERATIONS=5000 GOMEMLIMIT=2GiB go test -count=1 -run RandomEdits ./internal/issue/`
**Depends on item 5.**
**Commit:** `test(issue): property-test the Log merge against git merge-file`

## 10. Property-test the merge driver through git merge

**What:** add `internal/cli/merge_property_test.go` (issue `merge-driver-property-test`, end-to-end layer).
**Goal:** `TestMergeDriver_RandomBranches_KeepEveryLogEntry` runs N iterations (`CAPCOM_PROPERTY_ITERATIONS`, default 20 here, since each is a git repo): a repo from `newGitRepo` with `capcom init --merge-driver`, the two `git config merge.capcom-log.*` commands init prints, a few issues committed on `main`, two branches each running random `capcom log`, `update --priority/--title`, `dep add`, `close` through `runCLI` and committing, then `git merge` of both into `main`. It asserts `capcom check` exits 0 or reports only `conflict` problems, and `capcom show <id> --json` lists every Log entry either branch added. Seed and failing repo path in the failure message.
**Regression guard.** capcom writes stamp time.Now() with no clock override, so the property test gives each branch distinct reasons, titles and log texts so every merge outcome is determined by the seed, not by the second the commands ran in.
`dep add` edges stay acyclic across branches (add x->y only where x sorts after y), since ops refuses a cycle only within one branch and check would report `cycle` after the merge. `git merge --no-edit` runs through a test-local runner that accepts exit 1 (`runGit` fails the test on it); the conflict is read from `git diff --name-only --diff-filter=U` and the check results, and for an issue check reports as `conflict` the Log is read from the raw file with `issue.LogEntries`, `show --json` only for issues that parse.
The test builds `github.com/airiclenz/capcom/cmd/capcom` once, before any `runCLI` (whose `t.Chdir` breaks a relative path), into `capcom` plus `.exe` when GOOS is windows, and prepends that directory to `PATH` with `t.Setenv`.
**Approach (assumed at the header base):** git runs the driver as `capcom merge-driver …`, so the test builds `./cmd/capcom` once (`go build -o <tmp>/capcom`, as `cmd/agenteval/run.go` builds its build under test) and prepends its directory to `PATH` with `t.Setenv`; skip under `testing.Short()`. Use `runGit` from `lock_test.go` for every git command but `git merge`.
**Files:** internal/cli/merge_property_test.go
**Read first:** internal/cli/init.go — mergeDriverConfig, installMergeDriver; internal/cli/testharness_test.go — runCLI, setHermeticGitEnv; internal/cli/init_test.go — newGitRepo; internal/cli/lock_test.go — runGit;
internal/check/issues.go — checkCycles; internal/cli/show.go — parseNodeFile in show; cmd/agenteval/run.go — resolveCapcom, capcomPackage; internal/cli/merge_driver.go — gitTextMerge
**Tests:** the property test.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run RandomBranches ./internal/cli/`
- the default run finishes in under 20s on CI (`go test -v` timing)
**Depends on item 9.**
**Closes:** merge-driver-property-test
**Commit:** `test(cli): property-test the merge driver through git merge`
