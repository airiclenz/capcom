# Code and architecture review 2026-10-13: fixes, splits and refactors

**Goal:** Fix every verified defect, close every test gap, split the large files and land the deepening refactors from the 2026-10-13 code and architecture review. Order: hostile-repository reads first, then local fixes, pure moves, refactors (which absorb their linked fixes), remaining tests.
**Date:** 2026-10-13
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** d192883

**Sources:**
- `docs/reviews/code-and-architecture-review-2026-10-13.md` (ground truth; item ids F-, U-, T-, A-, P2- are its ids; section 7 holds the capcom ids)
- `AGENTS.md`; Notes `coding-conventions`, `error-messages`, `cli-test-harness`, `issue-workflow`, `store-layout`, `release-process`
- ADR Notes `adr-0002`, `adr-0005`, `adr-0008`, `adr-0009`, `adr-0010` (`capcom know tree decisions --depth 3`)
- Raw audit evidence (local only, gitignored): `docs/skill-runs/code-audit/2026-10-13/`

**Ratified design calls (user, 2026-10-13):**
- **F-07 replacement:** visible escape (`\x1b` form) in text output; `--json` and `--kv` unchanged.
- **F-10 duplicate id:** `tree.Load` drops both files; their subtrees are reported as orphans; check's duplicate wording unchanged.
- **F-11 scope:** local compare-and-swap only ((a)-(c), plus (d)'s close re-check via A-01); no store-wide write lock; the dep-cycle race stays, reported by `check`.
- **F-12 gate:** keep the CI half when the tag exists and the release is pending; bind the resolved SHA; amend the ADR 0008 Note to match.
- **F-15 VERSION shape:** bare `X.Y.Z` everywhere; the tagger exits non-zero on refusal.
- **F-16 init:** on an existing store without `config.yaml`, write the template with `format:` commented out; a new store keeps `format: 3`.
- **cli packages:** extract `internal/cli/output` only; no `issuecmd`/`knowcmd` split.
- **Issues:** filed before any code item, by item P0.0, under parent `code-review-1013`.
- **P0.4 outside AGENTS.md link:** refused only by `capcom agents` (write mode); check, prime and Note writes treat it as having no block — no read, no write, no new problem or warning.
- **P1.9 index and dropped subtrees:** the index leaves out a repeated id's subtree like an orphan directory, so every verb agrees.

**Closes:** code-review-1013

**Standing requirements:**
- skills: coding-standards
- Raspberry Pi 5: one package per test command with `GOMEMLIMIT=2GiB`; never `go test -race ./...`; never two test commands at once. `go vet ./...` is fine.
- Commit to `main` (the run branch fast-forwards to it); one commit per item; no AI attribution anywhere.
- Never tag, bump `VERSION` or edit a CHANGELOG release heading.
- `fix(...)` items: the new test fails on the pre-item tree (bite check).
- After any hand edit under `.capcom/`, run `capcom check`.
- Every context, the master's and each sub-agent's, stays under 200k tokens: pass file paths, not file contents.

**Out of scope:**
- P2-12 (`split-init-go`) and P2-13 (`split-release-files`): A-09 and A-11 replace them; not filed.
- A store-wide write lock and a new ADR for it (F-11 call).
- Splitting `internal/cli` into `issuecmd`/`knowcmd`.
- The review's refuted claims (Appendix A).

**Regression check (2026-10-13, d192883):**
- P0.0: guard folded (decision)
- P0.3: recast (decision)
- P0.4: recast (decision)
- P0.5: guard folded
- P0.6: guard folded
- P0.7: guard folded
- P0.9: guard folded
- P0.10: guard folded (decision)
- P1.3: guard folded
- P1.4: guard folded
- P1.6: guard folded
- P1.7: guard folded
- P1.8: recast (decision)
- P1.9: new item from P1.8's regression (decision); takes `duplicate-id-subtree` from P1.8; not yet regression-checked
- P1.20: guard folded
- P1.21: guard folded (decision)
- P1.23: guard folded
- P1.24: guard folded
- P1.28: guard folded (decision)
- P1.29: recast (decision)
- P1.30: guard folded
- P1.31: guard folded
- P1.40: guard folded
- P1.41: guard folded
- P1.42: guard folded
- P1.43: guard folded (decision)
- P1.44: guard folded
- P1.45: recast (decision)
- P2.1: move-only check replaced (decision)
- P2.2: recast (decision); move-only check replaced (decision)
- P2.3: guard folded (decision); move-only check replaced (decision)
- P2.4: move-only check replaced (decision)
- P2.5: guard folded; move-only check replaced (decision)
- P2.6: guard folded; move-only check replaced (decision)
- P2.7: guard folded; move-only check replaced (decision)
- P2.8: guard folded; move-only check replaced (decision)
- P2.9: guard folded; move-only check replaced (decision)
- P2.11: guard folded; move-only check replaced (decision)
- P2.13: move-only check replaced (decision)
- P2.15: move-only check replaced (decision)
- P2.17: move-only check replaced (decision)
- P2.19: guard folded; move-only check replaced (decision)
- P2.21: guard folded; move-only check replaced (decision)
- P2.23: move-only check replaced (decision)
- P2.25: guard folded; move-only check replaced (decision)
- P2.27: guard folded; move-only check replaced (decision)
- P2.29: move-only check replaced (decision)
- P2.30: recast (decision); move-only check replaced (decision)
- P2.31: guard folded; move-only check replaced (decision)
- P2.32: guard folded; move-only check replaced (decision)
- P2.33: guard folded; move-only check replaced (decision)
- P2.34: guard folded; move-only check replaced (decision)
- P2.35: guard folded; move-only check replaced (decision)
- P3.1: guard folded
- P3.3: guard folded
- P3.4: guard folded
- P3.5: guard folded (decision)
- P3.6: guard folded
- P3.8: recast (decision)
- P3.9: guard folded
- P3.11: guard folded
- P3.15: guard folded (decision)
- P3.16: guard folded
- P3.17: guard folded
- P3.18: guard folded
- P3.19: guard folded (decision)
- P3.21: guard folded
- P3.41: guard folded
- P3.42: guard folded
- P3.43: guard folded
- P3.44: guard folded
- P3.46: guard folded (decision)
- P3.47: guard folded
- P3.48: guard folded
- P3.50: guard folded
- P3.51: guard folded
- P3.55: guard folded
- P3.56: guard folded
- P3.62: guard folded
- P3.63: guard folded
- P3.71: guard folded (decision)
- P3.72: move-only check replaced (decision)
- P3.73: guard folded (decision); move-only check replaced (decision)
- P3.74: guard folded; move-only check replaced (decision)
- P3.80: guard folded (decision)
- P3.81: guard folded
- P3.82: guard folded
- P3.83: guard folded
- P3.84: guard folded (decision)
- P3.90: guard folded (decision)
- P3.93: guard folded (decision)
- P3.94: guard folded
- P3.95: guard folded
- P3.97: guard folded (decision)
- P3.98: guard folded (decision)
- P3.99: guard folded (decision)
- P4.1: guard folded
- P3.5: rejected — feed U+009B to the JSON case (TestJSONAndKVRenderers_ControlCharacters_PassedVerbatim cannot carry a lone byte 0x9b verbatim); P3.5's JSON/KV test names no lone-byte input, and P0.11's JSON-checked input decodes `\x9b` from YAML to the valid rune U+009B
- P3.62: rejected — add internal/check/check.go to Files because `addParseFailure` needs a `*frontmatter.Error`; `addParseFailure` takes any error, using `errors.As` with a fallback to line 0 and `err.Error()`
- P3.98: rejected — Files gains the nodestore ports and Acceptance tests internal/nodestore, where `.Dir()` callers would now live; no nodestore test calls `.Dir()` at the base, and P3.13/P3.14 port no `successorsIn` or `Dir()` helper
- Round 2 (re-check of P0.3, P0.4, P1.8, P1.9, P1.29, P1.45, P2.2, P2.30, P3.8; SAFE items not listed):
- P0.4: guard folded (round 2)
- P1.8: guard folded (decision, round 2)
- P1.9: guard folded (decision, round 2); now regression-checked
- P1.29: guard folded (round 2)
- P3.8: Read first anchors replaced (decision, round 2)
- P3.97: guard folded (decision, round 2)
- P3.5, P3.62, P3.98: the round-1 UNCONFIRMED findings above were verified by the writer and stay rejected

## P0.0 File the review issues, part 1: parent, test gaps and defects — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the review file and this plan file were already tracked before the item ran (committed in 6f48479 and b02ae12), so they are not in FILES and the commit adds only the 36 issue files; both `git ls-files --error-unmatch` acceptance checks pass.
NOTES (2026-10-13): bodies for agents-symlink-escape, terminal-control-escape, duplicate-id-subtree, stale-rewrite-cas, release-gate-binding and tagger-silent-pass state the plan header's ratified design calls (2026-10-13) instead of the review's open design-call options; agents-symlink-escape's acceptance follows the P0.4 call (refused only by `capcom agents`, refresh leaves an outside link alone) rather than the review's ModeRefresh-error acceptance.


**What:** File review section 7 in capcom, first half: the parent `code-review-1013` and, under it, every "Test gaps" row and every "Defects" row except the six P0.1 files (`dep-close-race`, `winget-probe-search`, `missing-config-format`, `check-related-format`, `note-self-supersede`, `know-rm-report-drops`). The same commit adds the untracked review file.
**Regression guard.** The plan file itself is committed in P0.0's commit, since every filed issue's `--plan` names it and `capcom check` needs it tracked.
**Goal:** `code-review-1013` is titled "Code and architecture review 2026-10-13" and has 35 children, the rows above, each with its section 7 id, title and deps and a body that ends in a `## Acceptance` section; the review file is tracked; `capcom check` is clean.
**Approach (assumed at the header base):**
- Command shape (flags checked with `capcom create --help` and `capcom dep --help`): `capcom create --id <id> --title '<section 7 title>' --parent code-review-1013 --type <type> --priority <n> --plan '<this plan's basename without .md>' --body-file <tmpdir>/<id>.md [--dep <a>,<b>]`. `--dep` must name an issue that already exists, so create in the order below. `capcom dep add <id> <dep-id>` adds an edge afterwards. The parent takes no `--parent` and no `--dep`.
- Bodies follow Note `issue-workflow` and draw on the review item: its Problem and Trigger, its Fix Goal and its Acceptance (a table row draws on its cells). Each body says what is wrong or wanted and why (severity, trigger), then gives `## Acceptance` bullets. Name symbols, never the review's line numbers. The last line is `Source: docs/reviews/code-and-architecture-review-2026-10-13.md, <review id>.` Body files go in a `mktemp -d` directory outside the repository. The parent's body names the review and its phases, and its `## Acceptance` reads "every child issue is closed".
- Type and priority (binding): `F-` rows are `bug`; `T-` rows and the parent are `task`. Priority is 1 for High, 2 for Medium and 3 for Low, from the severity the review gives the row's item or sub-point; the parent gets 2.
- Order: the parent; the "Test gaps" rows in table order; then the "Defects" rows in table order (`guarded-file-reader` first).
- Deps: the row's `--dep` column, plus the one review 6.3 edge the column lacks: `agents-guide-read-guard` also depends on `briefing-panic-test`.
**Files:** .capcom/issues/code-review-1013.md; .capcom/issues/code-review-1013/<id>.md for each of the 35 ids above; docs/reviews/code-and-architecture-review-2026-10-13.md; docs/plans/2026-10-13 - 00 - code-and-architecture-review-fixes-plan.md
**Read first:** docs/reviews/code-and-architecture-review-2026-10-13.md — 7 Suggested capcom issues (Defects, Test gaps tables), 6.3 Dependency summary, 3 Findings: test gaps (severity column); internal/check/check.go — isPlanFound, CodeMissingPlan; .capcom/config.yaml — types, plans
**Tests:** none (no code).
**Acceptance:**
- `capcom check` exits 0
- `capcom list --tree --depth 3` shows `code-review-1013` with its 35 children
- `ls .capcom/issues/code-review-1013/*.md | wc -l` prints 35
- `capcom dep list config-read-guard` lists `guarded-file-reader` and `store-init-error-tests`; `capcom dep list agents-guide-read-guard` lists `guarded-file-reader` and `briefing-panic-test`
- `grep -L '^## Acceptance' .capcom/issues/code-review-1013.md .capcom/issues/code-review-1013/*.md` prints nothing
- `git ls-files --error-unmatch docs/reviews/code-and-architecture-review-2026-10-13.md` succeeds
- `git ls-files --error-unmatch 'docs/plans/2026-10-13 - 00 - code-and-architecture-review-fixes-plan.md'` succeeds
**Commit:** `chore(issues): file the code and architecture review 2026-10-13 issues`

## P0.1 File the review issues, part 2: pure moves, refactors and the defects they absorb — ✅ DONE (2026-10-13)

NOTES (2026-10-13): bodies for missing-config-format, dep-close-race and cli-output-package state the plan header's ratified design calls (init writes the template with `format:` commented out on an existing store; local compare-and-swap only, dep-cycle race stays reported by `check`; extract `internal/cli/output` only) instead of the review's open options.
NOTES (2026-10-13): `ready-import-alias` depends on its sibling parent `split-large-files` per the review's `--dep` column, and the plan closes both in P2.37; `cli-output-package` (a `split-large-files` child) is closed only later, at P3.6, so `split-large-files` will still have an open child when P2.37 closes it.


**What:** File the rest of review section 7 under `code-review-1013`: the parent `split-large-files` with the "Pure moves" rows below it, except `split-init-go` and `split-release-files`, which are out of scope; every "Deepening refactors" row; the six "Defects" rows P0.0 left out; and both rows of "Defects from the architecture exploration".
**Goal:** `split-large-files` sits under `code-review-1013` and has 12 children. `code-review-1013` has 58 direct children. Every new issue has its section 7 id, title and deps and a body that ends in a `## Acceptance` section. Neither `split-init-go` nor `split-release-files` exists, and `capcom check` is clean.
**Approach (assumed at the header base):**
- The command shape, body rules and `--plan` are P0.0's. The "Pure moves" rows take `--parent split-large-files`, `cli-output-package` (the P2-14/A-14 row) included. Every other row, `split-large-files` included, takes `--parent code-review-1013`.
- Where each body comes from: a P2 row from review 5.2 (the split and its move-only acceptance); an A row from its section 4 item (Goal, approach, the fixes it absorbs); an F row from section 2; a U row from section 2.4.
- Type and priority (binding): `F-` and `U-` rows are `bug`, with priority from severity as in P0.0. P2 rows, A rows and `split-large-files` are `task` with priority 3.
- Order (each dep exists before it is named): `split-large-files`; the "Pure moves" rows in table order; `config-owns-format`, `reference-rewrite-engine`, `nodestore-post-write`, `issue-lifecycle-module`, `generic-node-verbs`, `node-layout-claims`, `shared-diagnostics`, `outline-walker`, `hooks-package`, `store-entry-points`, `release-step-table`, `channel-repo-seam`, `frontmatter-schema`, `trim-shallow-interfaces`; then `dep-close-race`, `winget-probe-search`, `missing-config-format`, `check-related-format`, `note-self-supersede`, `know-rm-report-drops`, `rename-dup-deps`, `ready-import-alias`.
- Deps: the `--dep` column ("(or none)" means use the dep), plus the review 6.3 edges the column lacks. `node-layout-claims` also depends on `split-nodestore-create`, `replace-missing-target` and `rename-no-replace`; `shared-diagnostics` on `duplicate-id-subtree`; `nodestore-post-write` on `reference-rewrite-engine`; `release-step-table` on `release-sums-check`.
Depends on P0.0.
**Files:** .capcom/issues/code-review-1013/<id>.md for each of the 23 ids filed directly under it here; .capcom/issues/code-review-1013/split-large-files/<id>.md for its 12 children
**Read first:** docs/reviews/code-and-architecture-review-2026-10-13.md — 7 (Pure moves, Deepening refactors, Defects from the architecture exploration), 6.3 Dependency summary, 5.2 Pure moves, 2.4 U-02/U-09; internal/check/check.go — isPlanFound
**Tests:** none (no code).
**Acceptance:**
- `capcom check` exits 0
- `capcom list --tree --depth 3` shows `split-large-files` under `code-review-1013` with its 12 children
- `ls .capcom/issues/code-review-1013/*.md | wc -l` prints 58; `ls .capcom/issues/code-review-1013/split-large-files/*.md | wc -l` prints 12
- `capcom dep list node-layout-claims` lists `nodefile-package`, `duplicate-id-subtree`, `split-nodestore-create`, `replace-missing-target`, `rename-no-replace`; `capcom dep list nodefile-package` lists `split-tree-go` and `guarded-file-reader`
- `find .capcom/issues -name 'split-init-go.md' -o -name 'split-release-files.md'` prints nothing
- `grep -rL '^## Acceptance' .capcom/issues/code-review-1013/` prints nothing
**Commit:** `chore(issues): file the code and architecture review 2026-10-13 refactor issues`

## P0.2 Pin Compose's panic recovery — ✅ DONE (2026-10-13)

**What:** Closes test gap T-02 (`briefing-panic-test`). In `internal/briefing/briefing.go`, add a source seam: an unexported package variable `runCheck = check.Run`, which `readHealth` calls in place of `check.Run`. Nothing else changes. A new test makes the seam panic.
Depends on P0.0.
**Files:** internal/briefing/briefing.go; internal/briefing/briefing_test.go
**Read first:** internal/briefing/briefing.go — Compose, readHealth, Briefing; internal/briefing/briefing_test.go — newBriefingRepo, briefingStore, failedSections, TestCompose_BrokenIndexAndLocks_FailInOrderAndKeepTheRest; internal/check/check.go — Run
**Closes:** briefing-panic-test
**Tests:** add `TestCompose_PanickingSource_IsOneInternalFailureAndNoSection` to `briefing_test.go`. It sets `runCheck` to a func that panics, restores it with `t.Cleanup` and never runs in parallel. It composes `newBriefingRepo(t, briefingStore())` and asserts:
- exactly one Failure, of `SectionInternal`, whose error reads `internal error: <panic value>`;
- `Work`, `Locks`, `Health` and `Elsewhere` are all nil.

Bite check: the test panics when `Compose`'s deferred `recover` is removed.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/briefing/ -run 'TestCompose'`
- `go vet ./internal/briefing/`
**Commit:** `test(briefing): pin Compose's panic recovery`

## P0.3 One guarded reader for repository-controlled files — ✅ DONE (2026-10-13)

NOTES (2026-10-13): openNoFollow takes the caller's Lstat info as a second parameter, which the unix opener ignores and the non-unix opener uses for its os.SameFile identity refusal (the regression guard keeps that check in the non-unix opener only); a SetNonblock failure after the open is returned as an *fs.PathError naming path.

NOTES (2026-10-13): the ReadNodeFile and ReadRegularFile tests in read_test.go share their case tables through helpers taking the reader, instead of duplicating them; the swap test is TestReadNodeFile_SwappedBeforeOpen_RefusedNotRegular (not parallel, as it sets the beforeOpen hook).

NOTES (2026-10-13): only ELOOP is mapped to ErrNotRegular at the O_NOFOLLOW open, as the plan says; FreeBSD (EMLINK) and NetBSD (EFTYPE) report a refused link differently and would surface the os error instead (not CI targets).

**What:** Recast at the regression check (2026-10-13). Fixes the F-02 shared fix and F-02(e) (`guarded-file-reader`). No read site outside node files has a guarded reader to call. `tree.ReadNodeFile` itself checks the entry with `Lstat` and then opens it in a way that follows a symlink and blocks on a FIFO, so an entry swapped between the two steps is read through (review F-02(e)).
**Regression guard.** On unix accept the opened handle when `checkReadable` passes on its `Stat` (`O_NOFOLLOW` already rules out a followed link, and a FIFO shows in the handle's mode); keep the `os.SameFile` identity refusal only in the non-unix opener. Step 4 returns `checkReadable`'s own error (`ErrTooLarge` for size, `ErrNotRegular` for mode), and `ErrNotRegular` alone for an identity mismatch. Prose guard rule: every comment in `internal/tree` naming `ErrNotRegular`, `ErrTooLarge`, `maxNodeFileSize` or `readOp` as a node-file-only limit; find them with `grep -n 'ErrNotRegular\|ErrTooLarge\|maxNodeFileSize\|readOp' internal/tree/*.go`.
**Goal:** `tree.ReadRegularFile(path string) ([]byte, error)` in `internal/tree/read.go` is the one guarded reader. It never follows a symlink and never blocks on a FIFO. It refuses a non-regular entry with `tree.ErrNotRegular` and one over 1 MiB with `tree.ErrTooLarge`, each inside an `*fs.PathError` naming path, and that holds for an entry swapped after the first check. `tree.ReadNodeFile` returns exactly what `ReadRegularFile` returns.
**Approach (assumed at the header base):**
- `ReadRegularFile` takes no limit (binding): every caller caps at 1 MiB, the cap `maxNodeFileSize` holds and `ErrTooLarge`'s text names. Steps, in order:
  1. `Lstat` and `checkReadable`.
  2. Call the hook `beforeOpen` when it is set.
  3. Open with `openNoFollow(path)`.
  4. `Stat` the handle. Refuse it with `ErrNotRegular` unless `checkReadable` passes and `os.SameFile(lstatInfo, openedInfo)` holds.
  5. Read through `io.LimitReader(maxNodeFileSize+1)`.
- `openNoFollow` lives in a new `internal/tree/open_unix.go` (`//go:build unix`). It calls `os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)`, using `golang.org/x/sys/unix`, which go.mod already requires. An `ELOOP` from the open becomes `&fs.PathError{Op: readOp, Path: path, Err: ErrNotRegular}`. After the open it clears the flag with `unix.SetNonblock(int(file.Fd()), false)`. A new `internal/tree/open_other.go` (`//go:build !unix`) uses `os.Open`.
- `ReadNodeFile` becomes a one-line wrapper. Its doc comment says so and keeps the contract text. `beforeOpen` is an unexported `func(path string)` variable, nil in production. `SetBeforeOpen(t, fn)` in `export_test.go` sets it and restores it on cleanup, as `SetLink` does.
- Prose guard: any comment that calls `ErrTooLarge` or `maxNodeFileSize` a node-file-only limit must now say "any file read through the guarded reader". Find them with `grep -n 'ErrTooLarge\|maxNodeFileSize' internal/tree/*.go`.
Depends on P0.0.
**Files:** internal/tree/read.go; internal/tree/open_unix.go; internal/tree/open_other.go; internal/tree/tree.go; internal/tree/export_test.go; internal/tree/read_test.go; internal/tree/read_unix_test.go
**Read first:** internal/tree/read.go — ReadNodeFile, checkReadable, readOp, maxNodeFileSize, isNotRegular; internal/tree/tree.go — ErrNotRegular, ErrTooLarge, leftOutFileOf; internal/tree/export_test.go — SetLink; internal/tree/read_test.go — TestReadNodeFile_Refused_SentinelInPathError, TestReadNodeFile_RegularFile_ReadsUpToTheLimit; internal/tree/fifo_unix_test.go — makeFIFO; internal/tree/tree_test.go — makeSymlink, writeSizedFile; internal/lock/oslock_unix.go — golang.org/x/sys/unix import
**Closes:** guarded-file-reader
**Tests:**
- `read_test.go`: `TestReadRegularFile_RegularFile_ReadsUpToTheLimit` and `TestReadRegularFile_Refused_SentinelInPathError` mirror the `ReadNodeFile` tests: symlink, directory, 1 MiB + 1 byte, and a FIFO made with `makeFIFO`.
- New `read_unix_test.go` (`//go:build unix`, package `tree_test`). Through `SetBeforeOpen`, swap the regular file between the `Lstat` and the open for (1) a symlink to another regular file and (2) a FIFO. `ReadNodeFile` must return an error wrapping `ErrNotRegular` in both cases. Each call runs in a goroutine and the test fails after 5 s, so the pre-item tree fails instead of hanging.
- Bite check: both swap tests fail on the pre-item tree, which has no seam and follows the link.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `go vet ./internal/tree/`; `GOOS=windows go vet ./internal/tree/`; `GOOS=darwin go vet ./internal/tree/`
**Commit:** `fix(tree): read repository files through one guarded reader that never follows a swapped-in link`

## P0.4 Keep the AGENTS.md block inside the working tree — ✅ DONE (2026-10-13)

NOTES (2026-10-13): consequential edit — internal/cli/agents.go: made necessary by agents.Error now also carrying the outside-link refusal; only agentsFailure's doc comment changed, no code.

NOTES (2026-10-13): resolveGuide returns an unexported guideFile struct (resolved path, permissions, isMissing, isOutside) rather than three values, and the outside test is a separate isInsideTree helper; reconcile now resolves first and wraps a non-ENOENT EvalSymlinks error as "AGENTS.md: <err>" in every mode, where before only the write step resolved.

NOTES (2026-10-13): docs/format.md gains one bullet covering both the in-tree link that keeps working and the outside link that is refused, with the plan's sentence verbatim.

**What:** Recast at the regression check (2026-10-13). Fixes F-01 (`agents-symlink-escape`). `resolveGuide` follows an `AGENTS.md` symlink to any target, and `reconcile` reads that target and atomically replaces it. A committed `AGENTS.md -> ~/.bashrc` therefore gets the block appended by `capcom agents` or by a Note write.
**Regression guard.** An `AGENTS.md` link outside the working tree is refused only by `capcom agents` (write mode); check, prime and Note writes treat it as having no block: no read, no write, no new problem or warning. Binding: fail only in ModeWrite; in ModeInspect and ModeRefresh an outside link is unread and StateAbsent. Outside-ness is `!filepath.IsLocal(rel)` (a `filepath.Rel` error counts as outside). Both new tests skip on Windows and as root via the makeSymlink-style guard. docs/format.md's opt-in sentence ("the block is opt-in") stays true; the item's CHANGELOG entry (via its sidecar) states the in-tree limit, superseding the earlier CHANGELOG entry that kept a symlinked AGENTS.md a link with no in-tree limit; docs/format.md gains one sentence on that limit.
**Regression guard (round 2).** Only ModeWrite (`Write`, `Sync(..., ModeWrite)`) returns the `*Error`; `Inspect` and `Sync` in ModeInspect or ModeRefresh return StateAbsent and nil and read nothing (check.agentsBlock and `agents --check` run ModeInspect). The docs/format.md bullet reads: "`capcom agents` fails on it; `capcom check`, `agents --check` and Note writes treat it as having no block." Only `agentsFailure` sees the error. The ModeInspect test case gives the outside file a current or stale block, so StateAbsent can only mean it was not read.
**Goal:** the Managed block is read from and written to the guide's resolved target only when that target lies inside the resolved worktree root. Otherwise `Write` and `Sync(..., ModeWrite)` return an `*agents.Error` with Path `AGENTS.md` and the message `links to <resolved target>, outside the working tree; the capcom block is not read or written`; `Inspect` and `Sync` in ModeInspect or ModeRefresh return StateAbsent and nil. The outside file is never read or written. An in-tree link (`AGENTS.md -> CLAUDE.md`) keeps working.
**Approach (assumed at the header base):**
- `reconcile` resolves, then reads, then writes the same target. `resolveGuide(worktreeRoot)` returns the `EvalSymlinks` target, its permissions and whether the guide is missing. A missing guide or a dangling link reads as no content and is written at `Path(worktreeRoot)`, as it is today.
- The target is checked against `filepath.EvalSymlinks(worktreeRoot)` with `filepath.Rel` on cleaned paths: a result of `..` or one starting `../` is outside. Never use a string prefix. `readGuide` reads the resolved target.
- `Error.Error()` renders `path: message` when Line is 0 and `path:line: message` otherwise. Callers need no change: only `cli.agentsFailure` sees the error, as `CodeInvalid`.
- In docs/format.md, "The Managed block in AGENTS.md", add a bullet: an `AGENTS.md` link that resolves outside the working tree is neither read nor written. `capcom agents` fails on it; `capcom check`, `agents --check` and Note writes treat it as having no block.
Depends on P0.0, P0.3.
**Files:** internal/agents/block.go; internal/agents/block_test.go; internal/cli/agents_test.go; docs/format.md
**Read first:** internal/agents/block.go — reconcile, resolveGuide, readGuide, Error.Error, Mode.writes, markerError; internal/check/check.go — agentsBlock; internal/cli/agents.go — writeAgentsBlock, checkAgentsBlock, agentsFailure; internal/cli/know.go — refreshAgentsAfterWrite; internal/agents/block_test.go — TestWrite_SymlinkedGuide_StaysALinkAndItsTargetGetsTheBlock, TestSync_EachModeAndState_ReportsTheStateFoundAndWritesOnlyWhatTheModeAllows, writeKnowledge; internal/cli/errors.go — Error.Error; docs/format.md — The Managed block in AGENTS.md
**Closes:** agents-symlink-escape
**Tests:**
- `block_test.go`: `TestSync_GuideLinkedOutsideTheTree_FailsAndLeavesTheTargetAlone`, one case per mode: ModeWrite with an outside file that has no markers, ModeRefresh with an outside file that holds a stale block, and ModeInspect with an outside file that holds a current or stale block. ModeWrite returns an `*Error` with the message above; ModeRefresh and ModeInspect return no error and `StateAbsent` without reading the outside file. The outside bytes are unchanged in every case. The worktree and the outside file are sibling directories of `t.TempDir()`. Skip on Windows, as root and where `os.Symlink` fails, as `TestWrite_SymlinkedGuide_StaysALinkAndItsTargetGetsTheBlock` does; that test still passes.
- `internal/cli/agents_test.go`: `TestAgents_GuideLinkedOutsideTheTree_FailsNamingTheLink`. `capcom agents` exits 1 with stderr exactly `capcom: AGENTS.md: links to <resolved outside path>, outside the working tree; the capcom block is not read or written\n`, and the outside bytes are unchanged.
- Bite check: both tests fail on the pre-item tree, which appends the block outside.
- `TestAgents_GuideLinkedOutsideTheTree_FailsNamingTheLink` skips on Windows and as root, as the block test does.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/agents/`
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Agents|Know'`
- `go vet ./internal/agents/ ./internal/cli/`
**Commit:** `fix(agents): refuse an AGENTS.md link that leaves the working tree`

## P0.5 Read AGENTS.md through the guarded reader — ✅ DONE (2026-10-13)

NOTES (2026-10-13): consequential edit — internal/check/check.go: made necessary by agents.Error now also carrying an unreadable guide; only the package doc and the CodeAgentsBlock doc comment changed, no code.

NOTES (2026-10-13): docs/format.md extends P0.4's bullet with its own sentence rather than "treated the same way": an outside link reads as no block in check, while a guide that is not a regular file or is over 1 MiB is an error in every mode (an agents-block problem in check), as the Goal line binds.

NOTES (2026-10-13): the refusal is built by a small unreadableError helper beside outsideError; block_test.go gains skipSymlinkTest, linkGuideTo, writeSizedFile, syncWithin (the 5 s goroutine guard, applied to every case) and assertDanglingGuideRead for the dangling-link rows, which run in every mode (ModeWrite writes the block; the others leave the link alone).

**What:** Fixes F-02(a) (`agents-guide-read-guard`). `readGuide` reads `AGENTS.md` with `os.ReadFile`. An in-tree FIFO or a huge file behind it therefore hangs, or runs out of memory in, `capcom prime`, `check`, `agents` and every Note write. A directory in its place fails `check.Run` as a whole.
**Regression guard.** A dangling `AGENTS.md` link still reads as no content: `readGuide` reads nothing when `resolveGuide` reports the guide missing (dangling link included), so ModeWrite still writes the block. The two link cases (`AGENTS.md ->` an in-tree FIFO, `->` a file of 1 MiB + 1 byte) skip on Windows and as root, as `TestWrite_SymlinkedGuide_StaysALinkAndItsTargetGetsTheBlock` does; the FIFO cases skip through `makeFIFO`. The `Error` doc in block.go and `agentsFailure`'s doc in cli/agents.go, which describe the error as tangled markers only, are superseded: widen both to a guide that cannot be read.
**Goal:** the guide's resolved target is read only through `tree.ReadRegularFile`. When the target is not a regular file, every `agents` mode promptly returns an `*agents.Error` with Path `AGENTS.md` and the message `not a regular file`; when it is over 1 MiB, the message is `file is larger than 1 MiB`. `capcom check` reports it as an `agents-block` problem, and `capcom prime` exits 0 and counts it.
**Approach (assumed at the header base):**
- `readGuide` calls `tree.ReadRegularFile` on the target that P0.4's `resolveGuide` returned. An in-tree link is resolved first, so `AGENTS.md -> CLAUDE.md` still reads.
- An error that `errors.Is` `tree.ErrNotRegular` or `tree.ErrTooLarge` becomes `&Error{Path: FileName, Message: <the sentinel's text>}`. A missing file is still no content.
- In docs/format.md, "The Managed block in AGENTS.md", extend P0.4's bullet: an `AGENTS.md` that is not a regular file, or is larger than 1 MiB, is treated the same way.
Depends on P0.2, P0.3, P0.4.
**Files:** internal/agents/block.go; internal/agents/block_test.go; internal/agents/fifo_unix_test.go; internal/agents/fifo_windows_test.go; internal/cli/agents_test.go; internal/cli/prime_test.go; docs/format.md; internal/cli/agents.go
**Read first:** internal/agents/block.go — readGuide, reconcile, resolveGuide, Error, markerError; internal/tree/read.go — ReadNodeFile, checkReadable; internal/check/check.go — agentsBlock, Problem.String; internal/cli/agents.go — agentsFailure; internal/cli/know.go — refreshAgentsAfterWrite; internal/cli/prime.go — addHealth; internal/cli/prime_test.go — TestPrime_UnparseableIssue_CountsItInCheckWithoutAWarningLine; internal/agents/block_test.go — TestWrite_SymlinkedGuide_StaysALinkAndItsTargetGetsTheBlock, TestSync_TangledMarkers_FailsAtTheMarkerLineWithoutWriting
**Closes:** agents-guide-read-guard
**Tests:**
- New `internal/agents/fifo_unix_test.go` and `fifo_windows_test.go`: `makeFIFO`, copied from the `internal/tree` pair (package `agents`).
- `block_test.go`: `TestSync_IrregularGuide_FailsAsAnAgentsError` covers every mode against each of four guides: `AGENTS.md` that is a FIFO; `AGENTS.md -> ` an in-tree FIFO; `AGENTS.md -> ` an in-tree file of 1 MiB + 1 byte; `AGENTS.md` that is a directory. Each returns an `*Error` with the message above. FIFO cases run in a goroutine and fail after 5 s.
- `internal/cli/agents_test.go`: `TestCheck_OversizedAgentsGuide_ReportsAnAgentsBlockProblem`. The output of `capcom check` contains the line `AGENTS.md: file is larger than 1 MiB`.
- `internal/cli/prime_test.go`: `TestPrime_OversizedAgentsGuide_ExitsZeroCountingIt`. `capcom prime` exits 0, and stdout contains `check: 1 problem; run capcom check`.
- Bite check: every new case fails or times out on the pre-item tree.
- `block_test.go`: `TestSync_IrregularGuide_FailsAsAnAgentsError` gains a dangling-link case: ModeWrite still writes the block. The link cases skip on Windows and as root.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/agents/`
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Agents|Check|Prime|Know'`
- `go vet ./internal/agents/ ./internal/cli/`
**Commit:** `fix(agents): read AGENTS.md through the guarded reader`

## P0.6 Pin the store's init and config-read error branches — ✅ DONE (2026-10-13)

NOTES (2026-10-13): added test helper replaceConfigWithDir, shared by the Open and RaiseFormat config-is-a-directory tests; bite-checked all three tests against createDir accepting a file, loadConfig treating every read error as missing, and RaiseFormat returning (false, nil) on a read error — each fails, then store.go was restored

**What:** Closes test gap T-09 (`store-init-error-tests`). Adds characterisation tests for the error branches of `createDir`, `loadConfig` and `RaiseFormat` in `store.go`, before P0.7 changes how config is read.
**Regression guard.** The bite mutation for `TestRaiseFormat_ConfigIsADirectory_FailsAndLeavesIt` is `RaiseFormat` returning `(false, nil)` on any read error; treating every read error as a missing file does not bite, because the later rename onto the directory still fails. The test does not pin the read error's wording, which P0.7 changes to `.capcom/config.yaml: not a regular file`.
Depends on P0.0.
**Files:** internal/store/store_test.go
**Read first:** internal/store/store.go — Init, createDir, loadConfig, RaiseFormat, relative; internal/store/store_test.go — newRepo, initStore, writeConfig, TestOpen_InvalidConfig_FailsWithPathAndLine; internal/tree/atomic.go — WriteFileAtomicVerbatim, writeFileAtomic
**Closes:** store-init-error-tests
**Tests:** in `store_test.go`:
- `TestInit_StoreDirIsAFile_FailsNamingItAndCreatesNothing`: `.capcom` is a regular file. `Init` fails with an error reading `.capcom: exists and is not a directory`, the file's bytes are unchanged, and nothing else is created.
- `TestRaiseFormat_ConfigIsADirectory_FailsAndLeavesIt`: after `Init`, replace `config.yaml` with a directory and call `RaiseFormat(3)` on the store `Init` returned. It fails, and the path is still an empty directory.
- `TestOpen_ConfigIsADirectory_FailsNamingTheConfig`: `Open` fails with an error whose text contains `.capcom/config.yaml`.

Bite check: each test fails against a deliberately broken implementation, for example `createDir` accepting a file, or `RaiseFormat` treating every read error as a missing file.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/store/`
- `go vet ./internal/store/`
**Commit:** `test(store): pin the init and config-read error branches`

## P0.7 Read config.yaml through the guarded reader — ✅ DONE (2026-10-13)

NOTES (2026-10-13): readConfig wraps every non-missing read failure with the store-relative path, so a RaiseFormat read failure other than the two refusals (for example permission denied) now leads with `.capcom/config.yaml: ` as Open's already did; raiseStoreFormat strips only the two refusals, per the regression guard.
NOTES (2026-10-13): added TestInit_SymlinkedConfig_FailsNamingIt (a dangling config.yaml link: Init fails with the refusal and leaves the link) and a "symlink to a regular file" row in TestOpen_IrregularConfig_FailsNamingIt, to cover the Goal's Init and plain-symlink clauses; the oversized comment-only content is built inline in the cli test rather than shared, as store and cli are separate test packages.

**What:** Fixes F-02(b) (`config-read-guard`). `loadConfig` and `RaiseFormat` read `.capcom/config.yaml` with `os.ReadFile`. `Open` reads it first, so a `config.yaml` linked to `/dev/zero` or a FIFO hangs every command. A link to a sensitive file gets that file parsed, and fragments of it can be echoed in parse errors.
**Regression guard.** `cli/related.go` `raiseStoreFormat` wraps any `RaiseFormat` error with the config path as Path. When the error `errors.Is` `ErrNotRegular` or `ErrTooLarge`, it uses the sentinel's text (the error unwrapped of its path) as Message, so `related add` and `know update` print the path once.
**Goal:** `config.yaml` is read only through `tree.ReadRegularFile`. When `config.yaml` is a symlink, FIFO, directory or other non-regular entry, `store.Open`, `store.Init` and `RaiseFormat` fail promptly with an error reading `.capcom/config.yaml: not a regular file`. When it is over 1 MiB they fail with `.capcom/config.yaml: file is larger than 1 MiB`. Each error wraps the tree sentinel. A missing `config.yaml` behaves as before.
**Approach (assumed at the header base):**
- `loadConfig` and `RaiseFormat` share one private method in `store.go`, `(*Store).readConfig() ([]byte, error)`, which calls `tree.ReadRegularFile(s.ConfigPath())`.
- `fs.ErrNotExist` passes through to each caller's missing-file branch. `tree.ErrNotRegular` and `tree.ErrTooLarge` become `fmt.Errorf("%s: %w", s.relative(configFile), sentinel)`.
- A symlinked `config.yaml` is refused like any other non-regular entry (binding, per review F-02(b)). `cli.storeFailure` already reports the error as `CodeInvalid` with its text.
- In docs/format.md, "Config", add that a `config.yaml` that is not a regular file, or is larger than 1 MiB, is an error naming it.
Depends on P0.0, P0.3, P0.6.
**Files:** internal/store/store.go; internal/store/store_test.go; internal/store/fifo_unix_test.go; internal/store/fifo_windows_test.go; internal/cli/read_test.go; docs/format.md; internal/cli/related.go
**Read first:** internal/store/store.go — loadConfig, RaiseFormat, Open, Init, createConfig, relative; internal/cli/related.go — raiseStoreFormat; internal/cli/init.go — storeFailure; internal/check/check.go — Run, declaredFormat; internal/store/store_test.go — initStore, writeConfig, newRepo; internal/cli/read_test.go — TestList_NewerStoreFormat_FailsNamingTheFormatLine, newReadStore; internal/tree/fifo_unix_test.go — makeFIFO
**Closes:** config-read-guard
**Tests:**
- New `internal/store/fifo_unix_test.go` and `fifo_windows_test.go`: `makeFIFO`, copied from the `internal/tree` pair (package `store`).
- `store_test.go`, `TestOpen_IrregularConfig_FailsNamingIt`: `config.yaml` is a FIFO, a symlink to a FIFO, or a regular file of 1 MiB + 1 byte made of `#` comment lines. `Open` returns the exact error text above. FIFO cases run in a goroutine and fail after 5 s.
- `store_test.go`, `TestRaiseFormat_OversizedConfig_FailsAndLeavesIt`: the error names `.capcom/config.yaml`, and the file's bytes are unchanged. P0.6's directory tests keep passing.
- `internal/cli/read_test.go`, `TestList_OversizedConfig_FailsNamingIt`: `capcom list` exits 1 with stderr `capcom: .capcom/config.yaml: file is larger than 1 MiB\n`.
- Bite check: every new case fails or times out on the pre-item tree, where the comment-only file parses.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/store/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Config|Init|Prime|List'`
- `go vet ./internal/store/ ./internal/cli/`
**Commit:** `fix(store): read config.yaml through the guarded reader`

## P0.8 Read a Ref's symbol file through the guarded reader — ✅ DONE (2026-10-13)

NOTES (2026-10-13): Verify's doc comment now lists a symbol file that is not a regular file of at most 1 MiB among the Findings, alongside the checkSymbol change.

**What:** Fixes F-02(c) (`ref-symbol-read-guard`). `knowledge.checkSymbol` reads the file of a `path#symbol` Ref with `os.ReadFile` and copies it into a string. A Ref naming a huge file is therefore held in memory twice on every `prime`, and a Ref naming a local FIFO blocks `prime`, `check` and `know verify`.
**Goal:** a symbol search reads its file only through `tree.ReadRegularFile` and searches the bytes. A Ref whose resolved file is not a regular file gives the Finding `is not a regular file; symbol "<s>" needs a file`. A Ref whose file is over 1 MiB gives `is larger than 1 MiB; symbol "<s>" is not searched`. Neither blocks. A directory keeps its existing Finding.
**Approach (assumed at the header base):**
- In `checkSymbol`, `path` is the file `EvalSymlinks` resolved. Keep the `os.Stat` directory branch, then call `tree.ReadRegularFile(path)`.
- Map `tree.ErrNotRegular` and `tree.ErrTooLarge` to the two messages above. Any other error goes through `unresolvable`. Search with `bytes.Contains(data, []byte(symbol))`.
- The 1 MiB cap is the guarded reader's; do not add a second constant.
- In docs/format.md, "Refs", add that the symbol's file must be a regular file of at most 1 MiB.
Depends on P0.0, P0.3.
**Files:** internal/knowledge/verify.go; internal/knowledge/knowledge_test.go; internal/cli/know_test.go; docs/format.md
**Read first:** internal/knowledge/verify.go — checkSymbol, checkRef, unresolvable, Verify; internal/knowledge/knowledge_test.go — TestVerify_UnreadableSymbolFile_IsAFindingNotAnError, TestVerify_Refs_FlagOnlyTheBrokenOnes, newWorktree, parseNote; internal/knowledge/fifo_unix_test.go — makeFIFO; internal/cli/know.go — newKnowVerifyCommand, verifyNotes; internal/cli/know_test.go — TestKnowVerify_BrokenRefs_ListsEachWithItsLineAndFails, newKnowStore, writeNoteFiles
**Closes:** ref-symbol-read-guard
**Tests:**
- `knowledge_test.go`: `TestVerify_FIFORef_IsAFindingNotAHang` uses the existing `makeFIFO` on unix, runs in a goroutine and fails after 5 s. `TestVerify_OversizedRef_IsAFinding` uses a 1 MiB + 1 byte file that does contain the symbol. Each yields one Finding with the message above.
- `internal/cli/know_test.go`, `TestKnowVerify_OversizedRef_IsListedAsBroken`: `capcom know verify` fails and prints exactly `.capcom/knowledge/<id>.md:<line>: ref "big.txt#needle": is larger than 1 MiB; symbol "needle" is not searched`.
- Bite check: both fail or time out on the pre-item tree.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/knowledge/`
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'KnowVerify'`
- `go vet ./internal/knowledge/ ./internal/cli/`
**Commit:** `fix(knowledge): read a Ref's symbol file through the guarded reader`

## P0.9 Pin the Elsewhere scan's failure warnings and file filter — ✅ DONE (2026-10-13)

**What:** Closes test gap T-07 (`elsewhere-failure-tests`). Adds characterisation tests for the "never fails" contract of `elsewhere.Scan` and for the filter in `issueID`, before P0.10 changes the branch read.
**Regression guard.** Name the branches so the one with the missing tree sorts first in refname order (e.g. `a-broken`, `b-healthy`), so the bite mutation "Scan returns at the first branch warning" turns the test red. The not-a-repository test sets `GIT_CEILING_DIRECTORIES` as `newRepo` does.
Depends on P0.0.
**Files:** internal/elsewhere/elsewhere_test.go
**Read first:** internal/elsewhere/elsewhere.go — Scan, scanBranch, issueID, warn; internal/gitx/branches.go — Branches, ChangedFiles; internal/elsewhere/elsewhere_test.go — newRepo, commitIssues, onBranch, scan, TestScan_UnparsableCopy_ReportsModifiedWithAWarning
**Closes:** elsewhere-failure-tests
**Tests:** in `elsewhere_test.go`:
- `TestScan_NotARepository_WarnsListBranchesOnce`: `Scan` at a `t.TempDir()` outside git returns no entries and one warning starting `elsewhere: list branches: `.
- `TestScan_BranchWithAMissingTree_WarnsForItAndReportsTheOthers`: two side branches change different issues. Delete the loose object of one branch tip's root tree (`git rev-parse <branch>^{tree}`); chmod it 0644 before removing it, for Windows. The scan warns `elsewhere: branch <that branch>: ...` once and still reports the healthy branch's entry. This is not the case of a ref naming a missing commit: that fails `for-each-ref` as a whole.
- `TestScan_HiddenAndNonMarkdownFiles_AreNotReported`: a side branch adds `.hidden.md`, `notes.txt` and one real issue file under the pathspec, and only the real issue is reported.

Bite check: each test fails against a deliberately broken implementation, for example a `Scan` that returns at the first branch warning, or an `issueID` that accepts every name.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/elsewhere/`
- `go vet ./internal/elsewhere/`
**Commit:** `test(elsewhere): pin the scan's failure warnings and file filter`

## P0.10 Cap the size of other-branch issue copies — ✅ DONE (2026-10-13)

**What:** Fixes F-02(d) (`branch-blob-size-cap`). `elsewhere.branchStatus` reads a branch's copy of a modified issue with `gitx.ShowFile`, which buffers the whole blob, and `gitx.run` then copies it twice more. A fetched branch that commits a multi-GB issue file makes every reading command and `prime` run out of memory.
**Regression guard.** Drop the premise that `internal/tree` imports only the standard library (P0.3 adds `golang.org/x/sys/unix`, already in go.mod); the approach is otherwise unchanged. Accepted cost: each Modified entry of a recent unmerged branch now spawns two git processes (`cat-file -s`, then `cat-file blob`) on every list, ready, show and prime that scans Elsewhere. Prose guard: every comment in internal/elsewhere/elsewhere.go stating that each changed issue file yields an Entry names the oversized-copy exception (`grep -n 'Entry per\|still reported\|branchStatus reads' internal/elsewhere/elsewhere.go`).
**Goal:**
- `gitx.ShowFile(dir, revision, path string, limit int64)` reads a blob only after `git cat-file -s` reports it at most `limit` bytes; otherwise it fails with an error wrapping the new `gitx.ErrBlobTooLarge`.
- The Elsewhere scan passes 1 MiB. When a modified issue's branch copy is larger, the scan records exactly one warning, `elsewhere: branch <branch>: <path>: file is larger than 1 MiB`, and no entry for that file.
**Approach (assumed at the header base):**
- `gitx` gains `ErrBlobTooLarge = errors.New("blob is larger than the size limit")`. `ShowFile` runs `cat-file -s <revision>:<path>`, parses the size with `strconv.ParseInt` and refuses a blob above `limit` with `fmt.Errorf("%s:%s: %w", revision, path, ErrBlobTooLarge)`. It then runs `cat-file blob` as it does today.
- `elsewhere` gains a constant `maxBranchCopySize int64 = 1 << 20`. `branchStatus` reads from `branch.Commit` instead of `branch.Ref`: that is the tip hash `Branches` listed, so the size probe and the read name the same blob. `branchStatus` also returns whether to keep the entry.
- On `errors.Is(err, gitx.ErrBlobTooLarge)`, `branchStatus` warns with `r.warn("branch %s: %s: %v", branch.Name, filePath, tree.ErrTooLarge)`. `elsewhere` imports `tree`. `scanBranch` drops the entry when it is not kept.
- An unparsable copy keeps its entry and its warning as before. `ChangedFiles` is unchanged.
- Producer and consumers of the new `ShowFile` signature, found with `grep -rn 'ShowFile(' internal/`: the producer is `gitx.ShowFile`; the consumers are `elsewhere.branchStatus` and `TestShowFile_ReadsTheBranchCopy`. The `ShowFile` method in `cmd/release` is a different method and stays untouched.
- In docs/format.md, at the `elsewhere.days` bullet, add that a branch copy larger than 1 MiB is not read and a warning names it.
Depends on P0.0, P0.9.
**Files:** internal/gitx/branches.go; internal/gitx/branches_test.go; internal/elsewhere/elsewhere.go; internal/elsewhere/elsewhere_test.go; docs/format.md
**Read first:** internal/elsewhere/elsewhere.go — branchStatus, scanBranch, Scan, Report.warn; internal/gitx/branches.go — ShowFile, Branch, ChangedFiles; internal/gitx/gitx.go — run, Error; internal/gitx/branches_test.go — TestShowFile_ReadsTheBranchCopy, commitFile; internal/elsewhere/elsewhere_test.go — onBranch, newIssueRepo, scan, TestScan_UnparsableCopy_ReportsModifiedWithAWarning; internal/tree/tree.go — ErrTooLarge; internal/cli/output.go — addElsewhere
**Closes:** branch-blob-size-cap
**Tests:**
- `branches_test.go`: `TestShowFile_ReadsTheBranchCopy` passes a 1 MiB limit. The new `TestShowFile_BlobOverTheLimit_FailsWithErrBlobTooLarge` uses a limit of 8 and a longer blob.
- `elsewhere_test.go`, `TestScan_OversizedBranchCopy_WarnsOnceAndReportsNoEntry`: a side branch grows an existing issue to 1 MiB + 1 byte. `Scan` returns exactly that warning and no entry for the file, and a second issue changed on the same branch is still reported.
- Bite check: the new tests fail on the pre-item tree, which reads and reports the copy.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/gitx/`
- `GOMEMLIMIT=2GiB go test ./internal/elsewhere/`
- `GOMEMLIMIT=2GiB go test ./internal/briefing/`
- `go vet ./internal/gitx/ ./internal/elsewhere/`
**Commit:** `fix(elsewhere): refuse other-branch issue copies larger than 1 MiB`

## P0.11 Escape terminal control characters in text output — ✅ DONE (2026-10-13)

**What:** Fixes F-07 (`terminal-control-escape`). Text output writes titles, field values and bodies from repository files to the terminal unfiltered. An Issue titled `"\e[2J\e[31mx"`, or a body that carries OSC sequences, can clear or rewrite the screen, set the window title, or write to the clipboard.
**Goal:** in text mode, stdout and stderr carry no C0 control other than TAB and LF, no DEL, no C1 control and no invalid UTF-8 byte. Each is written as a visible `\xHH` escape (ratified design call). `--json` and `--kv` output is byte-for-byte unchanged.
**Approach (assumed at the header base):**
- Add one function, `sanitizeTerminal(text string) string`, in `internal/cli/output.go` (binding: one function, which A-14 later moves into `cli/output`). It decodes with `utf8.DecodeRuneInString`:
  - a rune below 0x20 other than `\t` and `\n`, the rune 0x7f, or a rune in 0x80–0x9f becomes `\x` and two lower-case hex digits of the code point;
  - an invalid byte (`utf8.RuneError` of width 1) becomes `\x` and that byte's two hex digits;
  - everything else is unchanged.
- Apply it at the one text sink. `textRenderer.Render` writes `sanitizeTerminal(out.String())`, which covers Text, Elsewhere lines and warnings. `textRenderer.RenderError` sanitises `failure.Error()`. `jsonRenderer`, `kvRenderer`, `flatten` and `kvLineBreaks` stay untouched.
- Prose guard: in the `Result` doc comment, "Text is written verbatim in text mode" becomes "written in text mode with control characters escaped by sanitizeTerminal". Find it with `grep -n 'verbatim' internal/cli/output.go`.
Depends on P0.0.
**Files:** internal/cli/output.go; internal/cli/terminal_test.go
**Read first:** internal/cli/output.go — textRenderer.Render, textRenderer.RenderError, Result, kvRenderer.Render, writeKVLine, jsonRenderer.write; internal/cli/root.go — Run, selectRenderer; internal/cli/issue_render.go — issueText, issueLines, flatten, issueKV; internal/cli/know.go — noteText; internal/cli/testharness_test.go — runCLI; internal/cli/read_test.go — writeIssueFiles; internal/cli/know_test.go — writeNoteFiles
**Closes:** terminal-control-escape
**Tests:** in a new `internal/cli/terminal_test.go`:
- `TestSanitizeTerminal_EscapesControlsOnly`, a table: ESC, BEL, CR, DEL, U+009B and a lone byte 0x9b become `\x1b`, `\x07`, `\x0d`, `\x7f`, `\x9b` and `\x9b`. TAB, LF, `é` and plain text are kept.
- `TestText_ControlCharactersFromFiles_AreEscapedAndJSONKeepsThem`: write an Issue file and a Note file by hand. Each frontmatter title is the YAML `"a\e[2Jb\x9bc\ad"`, and each body holds the same characters. The text stdout of `show`, `list`, `know show` and `know tree` contains `a\x1b[2Jb\x9bc\x07d` literally and no raw control byte. `show --json` decodes the raw title and body, and the `TITLE=` line of `show --kv` carries the raw ESC.
- Bite check: the CLI test fails on the pre-item tree.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Sanitize|Text_|Show|List|Know'`
- `go vet ./internal/cli/`
**Commit:** `fix(cli): escape terminal control characters in text output`

## P1.1 show and know show render from one read of the file — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the parse-and-rebuild sits in two helpers, `issueFromFile` (show.go) and `noteFromFile` (know.go), both methods that parse and fail through `frontmatterFailure`; know.go's loop variable is renamed `indexed` so the rebuilt `entry` does not shadow it.

**What:** Fixes F-05 (`show-etag-snapshot`): `show` and `know show` print fields and body from the index but the etag from a later disk read, so `--if-match` can pass against content the caller never saw.
**Goal:** every field, the body and the etag that `show` and `know show` print (text, `--json`, `--kv`) derive from one read of the node file's bytes. Readiness, dependents, child progress and reverse Related links stay index data. A file that no longer parses fails as invalid at its file and line.
**Approach (assumed at the header base):** in `showResult` (`internal/cli/show.go`) and `noteShowResult` (`internal/cli/know.go`), drop the `issueIndex.Body` call; read the file once through `readIssueFile`, parse it with `issue.Parse` / `knowledge.Parse` (as `verifyNotes` does), and fail through `a.frontmatterFailure` on a parse error. Build the rendered entry from the index entry's `ID`, `Path` and `ParentID` plus every frontmatter field of the parse (`Category` from `a.store.Config.CategoryOf(parsed.Status)`), and take the body from `parsed.Body`; pass that entry to `showLinks`, `issueText`, `newShowData`, `issueKV`, `noteText` and `newNoteData`. `Index.Body` (`internal/index/query.go`) is then unused: delete it and its test in `query_test.go`.
**Files:** internal/cli/show.go; internal/cli/know.go; internal/cli/read_test.go; internal/cli/know_test.go; internal/index/query.go; internal/index/query_test.go
**Read first:** internal/cli/show.go — showResult, showLinks, loadShowGraph; internal/cli/know.go — noteShowResult, verifyNotes, newNoteData, noteText; internal/cli/update.go — readIssueFile, frontmatterFailure; internal/cli/issue_render.go — newIssueData, newShowData, issueText, issueKV; internal/index/index.go — Entry, Index.Issue; internal/index/notes.go — NoteEntry, insertNote; internal/index/refresh.go — recordedFile.isDirty, insertIssue; internal/index/query.go — Index.Body
**Closes:** show-etag-snapshot
**Tests:**
- `read_test.go`: `TestShow_FileRewrittenAtSameSizeAndMtime_PrintsTheBytesItsEtagNames`. Write an issue, `os.Chtimes` it an hour into the past, run `show` (the index records it clean), rewrite it with a same-length title and body, restore the same mtime, run `show` and `show --json`. Assert the printed title and body are the new ones and the etag equals `edit.ETag` of the new bytes.
- `know_test.go`: `TestKnowShow_FileRewrittenAtSameSizeAndMtime_PrintsTheBytesItsEtagNames`, the same steps for a Note.
- Bite check: both fail on the pre-item tree, which prints the index's old title and body beside the new etag.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Show'`
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `go vet ./internal/cli/ ./internal/index/`
**Commit:** `fix(cli): render show and know show from the bytes their etag names`

## P1.2 dep rm and related rm remove a dangling entry by its text before prefix matching — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the text test (new isEntryText in internal/cli/dep.go) matches an entry equal to the argument as typed or as tree.NormalizeArg spells it, per the Goal line, and is used in the not-found path too, where the old test matched the argument as typed only; the plan's Approach said "keeps today's exact-text test" there.
NOTES (2026-10-13): the three branches the Approach describes live in one shared helper, depGraph.isRemovedByText (internal/cli/dep.go), called by both depRemoval and relatedRemoval; docs/format.md has no dep rm paragraph, so only the related rm paragraph changed.

**What:** Fixes F-06 (`dangling-ref-removal`): a dangling `deps` or `related` entry whose text is a prefix of a live id resolves to that Issue, so the dangling entry cannot be removed (or a valid entry is removed instead).
**Goal:** when the holder's list has an entry equal to the argument (as typed or as `tree.NormalizeArg` spells it) and the argument is not exactly the id or an Alias of a usable Issue, that entry is removed by its text, whether the argument would otherwise prefix-match one Issue, several, or none. An argument that is exactly a usable Issue's id or Alias, and a name a duplicate leaves out, behave as before.
**Approach (assumed at the header base):** `depRemoval` (`internal/cli/dep.go`) also takes the dependent's `Deps`; `relatedRemoval` (`internal/cli/related.go`) loads the dep graph before resolving. Order in both: (1) `issueIndex.Resolve(arg)`; `tree.ErrNotFound` keeps today's exact-text test; a `tree.ErrDuplicate` failure keeps failing. (2) On success, when `graph.graph.Resolve(tree.NormalizeArg(arg))` finds no Issue (the match was a prefix) and an entry of the holder's list equals the text, return the exact-text test with `first` as holder. (3) On `tree.ErrAmbiguous`, the same exact-text test before failing. Otherwise keep today's `graph.names` tests. Update the doc comments of `newDepRmCommand`, `depRemoval`, `newRelatedRmCommand` and `relatedRemoval`, and the `related rm` paragraph of `docs/format.md`. Prose guard: every sentence saying an entry is matched by its exact text only when the argument names no issue (`grep -n 'exact text\|names no issue' internal/cli/dep.go internal/cli/related.go docs/format.md`).
**Files:** internal/cli/dep.go; internal/cli/related.go; internal/cli/dep_test.go; internal/cli/related_test.go; docs/format.md
**Read first:** internal/cli/dep.go — depRemoval, newDepRmCommand, loadDepGraph, depGraph.names; internal/cli/related.go — relatedRemoval, newRelatedRmCommand; internal/index/query.go — Index.Resolve, resolveID, unusableClaim; internal/issue/deps.go — DepGraph.Resolve; internal/issue/ready.go — readyTree.resolve; internal/tree/resolve.go — NormalizeArg; internal/cli/dep_test.go — newDepStore, TestDepRm_DanglingDep_RemovesItByItsText, TestDepRm_IDOfInvalidFile_RemovesTheExactEntry; internal/cli/related_test.go — newRelatedStore, TestRelatedRm_DanglingEntry_RemovesItByItsText
**Closes:** dangling-ref-removal
**Tests:** in `dep_test.go` and `related_test.go` (same four cases each, beside `TestDepRm_DanglingDep_RemovesItByItsText` and `TestRelatedRm_DanglingEntry_RemovesItByItsText`): a dangling entry that prefixes one live Issue; one that prefixes two; a dangling entry beside a valid entry naming the prefixed Issue (only the dangling one is removed); the exact-id and Alias paths unchanged (`TestDepRm_DepWrittenAsAlias_RemovesIt`, `TestDepRm_DuplicatedDep_FailsNamingBothFilesAndWritesNothing` stay green). Bite check: the first three cases fail on the pre-item tree.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'DepRm|RelatedRm'`
- `go vet ./internal/cli/`
**Commit:** `fix(cli): remove a dangling dep or related entry by its text before prefix matching`

## P1.3 A second YAML document in front matter or config.yaml is an error — ✅ DONE (2026-10-13)

NOTES (2026-10-13): config gains a one-line parser.yamlError helper so both Decode calls share the existing `invalid YAML: …` wording; RaiseFormat's inserted line goes through a new insertLine helper, which also gives a `---` last line without a newline its line ending.
NOTES (2026-10-13): added TestParse_ContentAfterDocumentEnd_FailsAsInvalidYAMLNamingItsLine (config, not in the plan's test list) to pin the guard's `...` path; the RaiseFormat table also gained `--- # mine` and `# c\n---` (no newline) cases, and every case now checks the statuses re-parse unchanged.
NOTES (2026-10-13): pre-existing, not changed: RaiseFormat still prepends `format:` to a config whose top-level mapping is flow-style (`{...}`) or indented, or whose `---` line carries content, which leaves it unparseable; that predates this item.

**What:** Fixes F-08 (`yaml-multi-document`): both YAML decoders read only the first document, so anything after a `...` or a second `---` escapes the strict checks, and `config.RaiseFormat` prepends `format:` above a leading `---`, making a two-document config.
**Regression guard.** A first `Decode` that returns `io.EOF` is the no-key frontmatter (return nil, as config's `parser.document` does); only the second `Decode` must return `io.EOF`. For config.yaml the Goal's "at a line" covers the `---` case only: content after `...` fails through config's `invalid YAML: …` path, with the line only in the message. `RaiseFormat`'s commented-key branch follows the same document-start rule: a commented format line before the first `---` counts as absent (insert after `---`, leave the comment).
**Goal:** front matter and `config.yaml` holding more than one YAML document fail with the package's `*Error` at a line; `RaiseFormat` output always re-parses to the same settings at the new format.
**Approach (assumed at the header base):** `decoder.document` (`internal/frontmatter/decode.go`) decodes with `yaml.NewDecoder` instead of `yaml.Unmarshal`, and `parser.document` (`internal/config/config.go`) already does. After the first `Decode`, and before any empty-document early return, a second `Decode` must return `io.EOF`. A second `Decode` error goes through the existing YAML-error path (`decoder.yamlError`; config's `invalid YAML: …`). yaml.v3 itself rejects content after a `...` line, citing that line. A second document fails as `only one YAML document is allowed` at that document node's `Line` (front matter adds `frontmatterLineOffset`). `RaiseFormat` without a format line inserts `format: N` directly after the first line that is exactly `---` (trailing blanks or a `#` comment allowed) when only blank lines, comments and `%` directives precede it; otherwise it is the first line, as now. yaml.v3 refuses `%YAML 1.2`, so tests use `%YAML 1.1`.
**Files:** internal/frontmatter/decode.go; internal/frontmatter/decode_test.go; internal/config/config.go; internal/config/config_test.go; internal/cli/check_test.go; internal/cli/related_test.go
**Read first:** internal/frontmatter/decode.go — decoder.document, decoder.yamlError, Decode, conflictError; internal/config/config.go — parser.document, RaiseFormat, findFormatLine, trimLineEnding, Parse; internal/check/check.go — Run; internal/frontmatter/decode_test.go — TestDecode_NoKeys_CallsNothing, TestDecode_YAMLSyntaxError_CitesFrontmatterLine; internal/issue/file_test.go — TestParse_EmptyFrontmatterAndBody; internal/config/config_test.go — TestRaiseFormat_RewritesOrInsertsOneFormatKey, TestParse_MalformedYAML_NamesFile; internal/cli/related_test.go — newRelatedStore
**Closes:** yaml-multi-document
**Tests:**
- `decode_test.go`: `"---\ntitle: a\n...\nbogus: 1\n---\n"` fails at file line 3 (the `...` line). Front matter whose second document follows a `--- ` line (with a trailing blank, which `Split` does not take as the fence) fails with `only one YAML document is allowed`.
- `config_test.go`: `"format: 2\n---\nstatuses: …"` fails at line 2. `TestRaiseFormat_RewritesOrInsertsOneFormatKey` gains `"---\nstatuses: …"`, `"# c\n---\nstatuses: …"` and `"%YAML 1.1\n---\nstatuses: …"`; each result re-parses with the same statuses at the new format.
- `internal/cli/check_test.go`: `TestCheck_ConfigWithSecondDocument_ReportsItsLine` asserts the exact output line `.capcom/config.yaml:<n>: only one YAML document is allowed`. `TestCheck_FrontmatterWithSecondDocument_ReportsItsLine` does the same for an issue file.
- `related_test.go`: `TestRelatedAdd_ConfigOpeningWithDocumentMarker_RaisesInsideIt`: a format-1 config starting `---` with a custom status; after `related add`, `list` still lists an issue in that status.
- Bite check: all but the unchanged RaiseFormat cases fail on the pre-item tree.
- `config_test.go`: `TestRaiseFormat_RewritesOrInsertsOneFormatKey` gains `"# format: 1\n---\nstatuses: …"`, which re-parses with the same statuses at the new format. `TestDecode_NoKeys_CallsNothing` and issue `TestParse_EmptyFrontmatterAndBody` stay green.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/frontmatter/`
- `GOMEMLIMIT=2GiB go test ./internal/config/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Check|RelatedAdd'`
- `go vet ./internal/frontmatter/ ./internal/config/ ./internal/cli/`
**Commit:** `fix(config): refuse a second YAML document in config.yaml and front matter`

## P1.4 Index refresh: cover files changed under the walk — ✅ DONE (2026-10-13)

NOTES (2026-10-13): indexFile's new result is `isGone bool` (true only on the fs.ErrNotExist branch) rather than "whether it read the file", so a file refused unread (WarningNotRegular) stays in Parsed as the regression guard requires; no CHANGELOG entry, since no command prints Report.Parsed or Report.Removed.

**What:** Closes test gap T-04 (`refresh-race-tests`). It also fixes the drift it exposes: a file gone between the walk and its read is listed in `Report.Parsed`, which documents "the files read and parsed again".
**Regression guard.** `TestIndexFile_FileGrownPastTheLimitSinceTheWalk_WarnsAndKeepsItOut` (`err = r.indexFile(file)`) is updated to `indexFile`'s new signature. A file refused unread at read time (WarningNotRegular, files row written) stays in `Parsed`; only the `fs.ErrNotExist` branch leaves `Parsed`.
**Goal:** a file removed after the walk lists it, whether before its stat or before its read, never fails `Refresh`. Its rows are gone, it is not in `Parsed`, and it is in `Removed` when the index held rows for it. A clean file whose stored `files.problems` is not valid JSON fails `Refresh` with an error naming that file.
**Approach (assumed at the header base):** add two package-level seams in the `commitTx` style: `var readNodeFile = tree.ReadNodeFile` used by `refresher.indexFile` (`internal/index/refresh.go`), and `var lstat = os.Lstat` used by `walkIssues` (`internal/index/walk.go`). `indexFile` reports whether it read the file. `sync` appends to `Parsed` only then, and appends to `Removed` a known file that vanished before its read.
**Files:** internal/index/refresh.go; internal/index/walk.go; internal/index/index_test.go
**Read first:** internal/index/refresh.go — refresher.sync, refresher.indexFile, readRefusal, commitTx, refresher.rewarn; internal/index/walk.go — walkIssues; internal/index/index_test.go — TestRefresh_CommitFails_ReportsOnlyTheCommitError, TestIndexFile_FileGrownPastTheLimitSinceTheWalk_WarnsAndKeepsItOut, TestRefresh_IndexFromBeforeRelatedLinks_RebuildsWithThem, laterClock, writeIssue; internal/index/index.go — Report
**Closes:** refresh-race-tests
**Tests:** in `index_test.go`. The seam-swapping tests never call `t.Parallel()` and restore the seam in `t.Cleanup`, as `TestRefresh_CommitFails_ReportsOnlyTheCommitError` does.
- `TestRefresh_FileGoneBeforeItsRead_IsDroppedAndNotParsed`: index a file and change its size. The swapped `readNodeFile` removes the file, then calls the real one. `Refresh` succeeds, the issue is not indexed or found by `Search`, it is absent from `Parsed` and present in `Removed`.
- `TestRefresh_FileGoneBeforeItsStat_IsRemoved`: the swapped `lstat` removes the file first. `Refresh` succeeds and `Removed` lists it.
- `TestRefresh_BadStoredProblems_FailsNamingTheFile`: index an invalid file under `laterClock()` and overwrite its `files.problems` with `{` through SQL. The next `Refresh` fails, and the error names `.capcom/issues/<file>.md`.
- Bite check: the `Parsed`/`Removed` assertions of the first test fail on the pre-item tree.
- `TestIndexFile_FileGrownPastTheLimitSinceTheWalk_WarnsAndKeepsItOut` takes `indexFile`'s new result and still passes.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `go vet ./internal/index/`
**Commit:** `fix(index): keep a file gone before its read out of the refresh's parsed list`

## P1.5 Index records the names a file with problems claims — ✅ DONE (2026-10-13)

NOTES (2026-10-13): consequential edit — docs/format.md: made necessary by unusableClaim now refusing an alias of a file with problems as well as its id ("Naming a node" said "the id of a file with problems"); P1.6 widens the sentence further.
NOTES (2026-10-13): a frontmatter that does not parse is detected by a WarningFrontmatter problem (claimedNames), the only kind parseIssue/parseNote emit on a parse failure; index's nodeFileExtension constant is now unused and left in place for P3.11, which removes it.

**What:** Fixes F-09(a) (`left-out-name-claims`): an Alias of a file the index leaves out for problems is never stored, so `Resolve` of that Alias falls through to a unique-prefix match and a write lands on the wrong Issue.
**Goal:** every name a node file with problems claims (its id, and the Aliases its parse gives, or `rawfm.Aliases` reads when it does not parse) makes `Index.Resolve` and `Index.ResolveNote` fail with `tree.ErrNotFound` as `id "<arg>" not found: <file> is invalid`, never a prefix match. A file refused unread at read time claims its id only.
**Approach (assumed at the header base):** in `internal/index/schema.go`, add a `claims (path, kind, name)` table with an index on `(kind, name)`, add `claims` to `rowTables`, extend the `schemaStatements` comment, and bump `schemaVersion` by one (the existing rebuild-on-version rule drops and rebuilds the disposable index). In `refresher.indexFile` (`refresh.go`), a file with problems also gets one `claims` row per claimed name. `unusableClaim` (`query.go`) looks the name up in `claims` for the kind, first path in path order, instead of comparing `path.Base` of every `files` row with problems; the error wording stays. Update `unusableClaim`'s and `Resolve`'s doc comments.
Depends on P1.4.
**Files:** internal/index/schema.go; internal/index/refresh.go; internal/index/query.go; internal/index/query_test.go; internal/index/index_test.go; internal/cli/close_test.go
**Read first:** internal/index/query.go — unusableClaim, resolveID, duplicateClaim, Resolve; internal/index/refresh.go — refresher.indexFile, parseIssue, refresher.forget; internal/index/notes.go — parseNote, ResolveNote; internal/index/schema.go — schemaVersion, schemaStatements, rowTables; internal/rawfm/aliases.go — Aliases; internal/index/query_test.go — TestResolve_NameOfExcludedNode_RefusesInsteadOfPrefixMatching; internal/cli/close_test.go — newInvalidFooStore, TestClose_IDOfInvalidFile_FailsNotFoundAndLeavesThePrefixMatchOpen
**Tests:**
- `query_test.go`: `TestResolve_NameOfExcludedNode_RefusesInsteadOfPrefixMatching` gains two cases beside `foo-bar.md`: "alias of an invalid file" (`foo-rework.md`, invalid status, `aliases: [foo]`) and "alias of an unparseable file". Both make `Resolve("Foo")` fail with `ErrNotFound` naming that file. `Resolve("fo")` still finds `foo-bar`.
- `index_test.go`: `TestRefresh_IndexFromBeforeClaims_RebuildsWithThem`, modelled on `TestRefresh_IndexFromBeforeRelatedLinks_RebuildsWithThem`.
- `close_test.go`: `TestClose_AliasOfInvalidFile_FailsNotFoundAndLeavesThePrefixMatchOpen`: `auth-rework.md` (invalid, `aliases: [auth]`) beside `auth-tests.md`. `close auth` exits not_found with stderr containing `id "auth" not found: .capcom/issues/auth-rework.md is invalid`, and every issue file is unchanged.
- Bite check: the new cases fail on the pre-item tree.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Close|Resolve|DepRm'`
- `go vet ./internal/index/ ./internal/cli/`
**Commit:** `fix(index): refuse the aliases of a left-out file instead of prefix matching`

## P1.6 Index records the names of files the walk leaves out — ✅ DONE (2026-10-13)

NOTES (2026-10-13): walkIssues returns the walk's claims (reusing the existing `claim` type) and walkStore tags them with the tree's kind and keeps them on the refresher as `walkClaims`, rather than walkIssues writing kind-less claims onto the refresher itself; refresh then runs `replaceWalkClaims` after sync. The claims INSERT is now a shared `insertClaimQuery` constant used by insertClaims too.
NOTES (2026-10-13): under an orphan directory a file the guarded reader refuses unread claims its id only, and any other read failure (gone, unreadable) claims nothing; WalkDir errors skip that entry; neither fails Refresh. index's `nodeFileExtension` constant (P1.5 left it unused for P3.11 to remove) is used again by the new nodeFileID, so P3.11 should keep it.
NOTES (2026-10-13): consequential edit — internal/index/query.go: made necessary by walk claims widening what Resolve, resolveID and unusableClaim refuse (doc comments said "invalid file" / "file with problems")
NOTES (2026-10-13): consequential edit — internal/index/notes.go: made necessary by walk claims widening what ResolveNote refuses (doc comment said "the id of an invalid Note file")
NOTES (2026-10-13): consequential edit — internal/index/schema.go: made necessary by the claims table now also holding the walk's claims (schemaStatements comment said "every name a file with problems claims")

**What:** Fixes F-09(b) (`left-out-name-claims`). A node file the walk refuses unread (a symlink, a FIFO, a file over 1 MiB) gets no row, and neither does a node file under a directory left out with it, so its id falls through to prefix matching. The outcome depends on which phase spots the problem.
**Regression guard.** A `WalkDir` error or a failed read below an orphan directory (unreadable subdirectory, directory removed after `tree.Walk`) adds no claim and never fails `Refresh`; an orphan directory stays a warning only. Hidden names are skipped at every depth (`filepath.SkipDir` on a hidden directory), as nodestore's `findNodeFile` does.
**Goal:** every name a node file left out at walk time claims makes `Index.Resolve` and `Index.ResolveNote` fail as P1.5's claims do. That covers a refused node file's id, and the id and Aliases of every node file at any depth under a directory the walk leaves out. The claims hold exactly as long as the walk keeps reporting the file.
**Approach (assumed at the header base):** `walkIssues` (`internal/index/walk.go`) turns `tree.Walk`'s problems into claims, kept on the `refresher`:
- a problem wrapping `tree.ErrNotRegular` or `tree.ErrTooLarge` at a `.md` name with a valid id claims that id, never read;
- a problem wrapping `tree.ErrOrphanDirectory` claims, for every non-hidden `.md` file with a valid id below that directory (`filepath.WalkDir`), its id, plus `rawfm.Aliases` of its bytes read through `tree.ReadNodeFile` (a refused read adds none).

After `sync`, `refresh` (`refresh.go`) deletes every `claims` row whose path has no `files` row and inserts the walk's claims. No schema bump beyond P1.5's: walk claims are rebuilt on every refresh. Update `docs/format.md` "Naming a node": a name that a file with problems, a file left out unread, or a node file under a directory left out claims (its id or an alias) is not found, never a prefix. Prose guard: every sentence limiting that rule to "the id of a file with problems" (`grep -rn 'file with problems\|invalid file' docs/format.md internal/index/*.go`).
Depends on P1.5.
**Files:** internal/index/walk.go; internal/index/refresh.go; internal/index/query_test.go; internal/index/index_test.go; internal/cli/close_test.go; docs/format.md
**Read first:** internal/index/walk.go — walkIssues, walkWarningKind, worktreeRelative; internal/index/refresh.go — refresh, walkStore, refresher; internal/tree/tree.go — Walk, treeWalker.walk, leftOutFileOf, invalidIDDirectoryError; internal/nodestore/create.go — findNodeFile; internal/index/index_test.go — oversizeIssueText, makeSymlink, TestRefresh_NonRegularNodeFiles_WarnOnceEachAndIndexTheRest; docs/format.md — "Naming a node"
**Closes:** left-out-name-claims
**Tests:**
- `query_test.go`, same table: "file over 1 MiB" (`oversizeIssueText`), "symlinked file" (`makeSymlink`, which skips where it must), "file inside an orphan directory" (`orphan-dir/foo.md`) and "alias of a file inside an orphan directory".
- `index_test.go`: `TestRefresh_WalkRefusedFileFixed_DropsItsClaim`: once the oversize `foo.md` is replaced by a valid one, `Resolve("foo")` finds it.
- `close_test.go`: `TestClose_IDOfOversizeFile_FailsNotFoundAndLeavesThePrefixMatchOpen`, an oversize `foo.md` beside `foo-bar.md`. Stderr contains `id "foo" not found: .capcom/issues/foo.md is invalid`, and `foo-bar.md` is unchanged.
- Bite check: the new cases fail on the pre-item tree.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Close'`
- `go vet ./internal/index/ ./internal/cli/`
**Commit:** `fix(index): refuse the names of files the walk leaves out instead of prefix matching`

## P1.7 Create and rename refuse an id a dropped duplicate-id file claims — ✅ DONE (2026-10-13)

NOTES (2026-10-13): duplicateClaimFailure shares a `claimOf` helper (named id → "taken by", ReadAliases lists id → "an alias of") and a `duplicateIDFiles` helper (repeated-id problems: ErrDuplicate with an empty Alias; Path then Other, each once) that findUnparsedAliasHolder also scans with RecoverAliases; the repeated id in the message is the base name of the problem's Path.
NOTES (2026-10-13): Move's doc comment gains the duplicate refusal as a new step 5 (steps 5-8 renumbered 6-9); Create's step 3 lists it between the orphan and unparsed refusals.
NOTES (2026-10-13): move_test.go's existing "duplicate of the moved issue" case already writes its second file below session-work/, whose session-work.md newMoveIssues writes, so it needed no extra parent file; the new create/move fixtures write sub.md beside sub/foo.md via writeDuplicateIDFiles.
NOTES (2026-10-13): added a create_test case beyond the plan's list, "id maybe an alias of an unparseable dropped duplicate-id file", pinning the findUnparsedAliasHolder scan the Approach adds; it bites on the pre-item tree.

**What:** Fixes F-10(b) (`duplicate-id-subtree`): `create --id` and a renaming `mv`/`know mv` read no Aliases of a file left out for a repeated id, so they write a second holder of a name (ADR 0002). This lands before P1.8, so a duplicated id stays refused once `tree.Load` drops both of its files.
**Regression guard.** The create and move fixtures write `sub.md` beside `sub/foo.md`, as `newDuplicateIssueStore` writes `sub-issue.md`; without it `sub/` is an orphan directory, `tree.Walk` never reaches `sub/foo.md`, and the orphan refusal fires first. The item's CHANGELOG entry (via its sidecar) says create and mv now refuse an alias only a left-out file lists, superseding the CHANGELOG entry "An alias only a left-out file lists is unaffected".
**Goal:** a new id is refused with an `*ExistsError` at the file when a file named by a repeated-id problem of the load is named for that id or records it among its Aliases, as far as they can be read. The message is `id "<id>" is already taken by this file, left out for the duplicate id "<dup>"`, or `… an alias of this file …`.
**Approach (assumed at the header base):** in `internal/nodestore/create.go`, add `duplicateClaimFailure(problems, id)` beside `orphanClaimFailure`. It covers every problem wrapping `tree.ErrDuplicate` with an empty `Alias` (a repeated id), and the files it names (`Path`, then `Other`, each once). A file named `<id>.md` is "taken by"; `ReadAliases` listing id is "an alias of". `claimFailure` calls it after `orphanClaimFailure`. `findUnparsedAliasHolder` also scans those files with `RecoverAliases`. In `internal/nodestore/move.go`, `Move` calls it on a rename right after `orphanClaimFailure`; renumber its doc comment's step 4. Add the rule to the `docs/format.md` paragraph that names the orphan-directory refusal.
**Files:** internal/nodestore/create.go; internal/nodestore/move.go; internal/nodestore/create_test.go; internal/nodestore/move_test.go; internal/cli/write_test.go; docs/format.md
**Read first:** internal/nodestore/create.go — Create, claimFailure, orphanClaimFailure, findUnparsedAliasHolder, findNodeFile; internal/nodestore/move.go — Move; internal/nodestore/kind.go — ReadAliases, RecoverAliases; internal/nodestore/errors.go — ExistsError, existsError; internal/tree/tree.go — indexIDs, checkDroppedAliases; internal/nodestore/create_test.go — TestCreate_Refusals_WriteNothing; internal/nodestore/move_test.go — TestMove_Refusals_MoveNothing, newMoveIssues; internal/cli/write_test.go — TestCreate_AliasInsideOrphanDirectory_FailsNamingTheFile
**Tests:**
- `create_test.go`, `TestCreate_Refusals_WriteNothing`: "id an alias of a dropped duplicate-id file" (`sub/foo.md` beside `foo.md` and `sub.md`, listing `old-bar`) is refused with the message naming `sub/foo.md`. "duplicated id" (`create --id foo`) stays refused with an `*ExistsError` naming either file.
- `move_test.go`, `TestMove_Refusals_MoveNothing`: "new id an alias of a dropped duplicate-id file".
- `write_test.go`: `TestCreate_AliasOfDroppedDuplicateIDFile_FailsNamingTheFile` asserts stderr contains `.capcom/issues/sub-issue/foo.md: id "old-bar" is already an alias of this file, left out for the duplicate id "foo"`, and nothing is written.
- Bite check: the alias cases fail on the pre-item tree.
- `move_test.go`'s duplicate case writes `sub.md` too.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Create|Mv'`
- `go vet ./internal/nodestore/ ./internal/cli/`
**Commit:** `fix(nodestore): refuse an id a dropped duplicate-id file claims`

## P1.8 tree.Load drops both files of a repeated id, with their subtrees — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the directory problem's reason constant is named `duplicateReason` (the plan's approach called it `duplicateIDReason`); Tree gains a `LeftOut()` accessor that the issue/knowledge remove planners and check use to see the left-out files.
NOTES (2026-10-13): the nodestore "dependent left out for a repeated id" case puts `deps: [fix-parser]` on `sub-issue/foo.md` (the file the base already dropped) rather than on `foo.md`, so the case bites on the pre-item tree; with `foo.md` as the dependent it passes at the base too.
NOTES (2026-10-13): the "duplicate of the parent" case of TestRemove_Refusals_RemoveNothing and the nodestore left-out case are regression pins; the tree `fo` prefix case pins base behaviour, as the plan says.
NOTES (2026-10-13): the index still keeps the first file of a repeated id until P1.9 lands, so index-resolved verbs and tree-resolved verbs can disagree about a repeated id's subtree until then.

**What:** Recast at the regression check (2026-10-13). Fixes F-10(a) and F-10(c) (`duplicate-id-subtree`). `tree.Load` keeps the first file of a repeated id, so the dropped file's children join it and a move silently re-parents them. The `AGENTS.md` block lists a top-level Note the index refuses. Ratified: `tree.Load` drops BOTH files of a repeated id, and their subtrees are reported as orphans. Check's duplicate wording stays (`TestRun_DirectoryProblems_WordedAsTheIndexWarnsThem`).
**Regression guard.** (1) tree exposes the left-out node files (both files of the id and their subtrees); issue and knowledge remove planners also scan them, so `rm target` is still refused while a left-out file depends on it; pin in internal/nodestore/remove_test.go. (2) `recordDuplicate` every alias of the first file of a repeated id and of each left-out subtree node with that id's duplicate problem, so those names never fall to prefix matching; add a resolve_test case; `TestResolve_AliasOnlyADroppedFileLists_IsNotADuplicate` stays green. (3) Add internal/nodestore/remove_test.go to Files; its "duplicate of the parent" case expects the `*tree.Error` wrapping `ErrDuplicate` naming both files. (4) check builds its dep graph from the loaded nodes plus the left-out ones, so it reports no new missing-dep/missing-related lines. (5) Goal wording: a node under a repeated id fails with "the duplicate problem of the repeated id it lies under, naming both files". The index half moves to P1.9, which now closes `duplicate-id-subtree`.
**Regression guard (round 2).** The ids of left-out nodes stay in the unique-prefix candidate sets (tree Resolve; index resolveID via P1.6's claims), so a prefix ambiguous at the base stays ambiguous and never resolves to a different node: `Resolve` counts the repeated ids and the ids of left-out subtree nodes (the `duplicates` keys that are ids) as prefix candidates; with `newDuplicateIssueStore` (`foo.md`, `sub-issue/foo.md`, `foo-bar.md`) `rm fo` stays refused as ambiguous. Guard (4) extends to `checkSupersession`: a `superseded-by` that names a left-out Note is found. Guard (4)'s rule: the left-out nodes join `entries` and `checkCycles`' `byID`, so a cycle below a repeated id is reported at its file's path; `checkIssue` runs only on loaded nodes. The docs/format.md AGENTS-block sentence and the item's CHANGELOG entry (via its sidecar) say that any load problem naming the Note's file (a duplicate id or alias, an invalid or repeated alias) leaves it out. `create --id c2` for a node below a repeated id is now refused by `orphanClaimFailure`'s orphan-directory wording instead of `id "c2" is already taken by issue c2`; accept that wording and name it in the CHANGELOG entry.
**Goal:** for an id two or more files share, no file of the id is a Node. Every node file under a directory beside one of them is left out too. Each such directory is one `ErrOrphanDirectory` problem worded `directory left out: its node file <id>.md has a duplicate id`. `Resolve` of a left-out subtree node's id fails with the duplicate problem of the repeated id it lies under, naming both files. The duplicate problems themselves are worded and placed as before. `agents.TopNotes` leaves out every top-level Note whose file a load problem names.
**Approach (assumed at the header base):** in `internal/tree/tree.go`, `indexIDs` drops every file of a repeated id. It still adds one problem per file after the first, naming the first, and records it. Load then leaves out every node whose path lies under a dropped file's directory. It adds the directory problem through `invalidIDDirectoryError` with a new `duplicateIDReason`, and records each left-out node's id in `duplicates` via `recordDuplicate`. All dropped files go through `checkDroppedAliases`, so the AliasFunc still sees every node file. In `internal/agents/block.go`, `TopNotes` passes an AliasFunc that parses each Note once (`tree.ReadNodeFile`, `knowledge.Parse`; refused or unparseable gives none) and reuses that parse. It skips a top-level node whose path is the `Path` or `Other` of any `tree.Errors` entry. In `docs/format.md`, the tree-rules bullet gains the duplicate-id directory, and the AGENTS block paragraph gains the left-out Note. Prose guard: every comment or doc saying Load keeps the first file of a repeated id (`grep -rn -i 'first file\|keeps the first\|after its first' internal/tree/ docs/format.md`).
Depends on P1.7.
**Files:** internal/tree/tree.go; internal/tree/resolve.go; internal/tree/tree_test.go; internal/tree/resolve_test.go; internal/agents/block.go; internal/agents/block_test.go; internal/cli/check_test.go; docs/format.md; internal/issue/remove.go; internal/knowledge/remove.go; internal/check/check.go; internal/check/check_test.go; internal/nodestore/remove_test.go
**Read first:** internal/tree/tree.go — Load, indexIDs, checkDroppedAliases, recordDuplicate, invalidIDDirectoryError, treeWalker.walk; internal/tree/resolve.go — Resolve, Lookup; internal/check/check.go — issues, graphNodes, checkCycles, checkSupersession, loadTree; internal/agents/block.go — TopNotes; internal/issue/remove.go — PlanRemove, planDrops; internal/nodestore/create.go — claimFailure, orphanClaimFailure, isBlockingProblem; internal/tree/resolve_test.go — TestResolve_DuplicatedName_LeavesOtherNamesAndLookupAlone, TestResolve_AliasOnlyADroppedFileLists_IsNotADuplicate; internal/nodestore/remove_test.go — TestRemove_Refusals_RemoveNothing
**Tests:**
- `tree_test.go`: `TestLoad_DuplicateIDWithChildren_LeavesBothFilesAndTheSubtreeOut` (`a.md`, `a/x.md`, `b.md`, `b/x.md`, `b/x/c2.md`): neither `x` nor `c2` is in `Nodes()`, `ChildrenOf("x")` is empty, and the `b/x` directory problem is worded as above.
- `resolve_test.go`: `TestResolve_NodeBelowDuplicatedID_FailsNamingBothFiles`, which covers the review's "a move of `c2` is refused". `TestResolve_DuplicatedName_LeavesOtherNamesAndLookupAlone` now expects `Lookup("foo")` to find nothing.
- `block_test.go`: `TestTopNotes_DuplicatedTopLevelID_IsLeftOut` (`zeta.md`, `zoo.md`, `zoo/zeta.md`) and `TestTopNotes_SharedAlias_LeavesBothOut`.
- `internal/cli/check_test.go`: `TestCheck_DuplicateIDWithChildren_ReportsTheDirectoryLeftOut` asserts the exact `capcom check` line for the child directory.
- Bite check: all new tests fail on the pre-item tree.
- `nodestore/remove_test.go`: `rm target` stays refused while a left-out file (`foo.md` beside `sub-issue/foo.md`) has `deps: [target]`; the "duplicate of the parent" case of `TestRemove_Refusals_RemoveNothing` expects the `*tree.Error` wrapping `ErrDuplicate` naming both files.
- `resolve_test.go`: an alias of the first file of a repeated id (`foo.md` `aliases: [auth]` beside `auth-tests.md`) fails with the duplicate problem instead of prefix-matching `auth-tests`.
- `TestCheck_DuplicateIDWithChildren_ReportsTheDirectoryLeftOut` also asserts no `missing-dep` line for an issue whose `deps` names a left-out node.
- `resolve_test.go`: prefix case "fo" over `foo.md`, `sub-issue/foo.md` and `foo-bar.md` stays ambiguous (it fails at the base too, so it pins the base behaviour, not a bite).
- `internal/check/check_test.go`: a Note whose `superseded-by` names a left-out Note (a repeated-id Note or one below it) gets no `missing-superseded-by` line.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/agents/`
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `GOMEMLIMIT=2GiB go test ./internal/issue/`
- `GOMEMLIMIT=2GiB go test ./internal/knowledge/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go vet ./internal/tree/ ./internal/agents/ ./internal/cli/`
**Commit:** `fix(tree): drop both files of a repeated id and the subtrees beside them`

## P1.9 Index leaves out the subtree of a repeated id — ✅ DONE (2026-10-13)

NOTES (2026-10-13): re-derived from "the subtree's names reach unusableClaim through the claims table alone": the claims table holds no link from a claim to its repeated id and the item allows no schema change, so the refresh keeps the left-out walk nodes (id, path, kind and the repeated id's duplicate problem) on the Index (`leftOut`, set after commit like `report`), which unusableClaim and resolveID's prefix candidates read; internal/index/index.go holds the Index struct.
NOTES (2026-10-13): a file of a repeated id that itself lies below another repeated id's directory keeps its (excluded) rows, as tree.Load treats it as a file of its repeated id first; walk claims for kept files are dropped (`withoutKeptFiles`) so no claim row outlives its file. TestRefresh_RepeatedIDBelowARepeatedID_StaysADuplicate and TestRefresh_RepeatedIDFixed_IndexesTheSubtreeAgain pin base behaviour (they pass on the pre-item tree).
NOTES (2026-10-13): the index mirrors tree.Load's leave-out rule (repeated ids over every walk node, innermost enclosing directory, Lstat'ed directory problems) in walk.go rather than sharing it; tree exports only the wording, `DuplicateIDError` and `DuplicateIDDirectoryError`, which tree's own repeatedIDs and addRepeatedIDDirs now use too. The duplicate problem for a left-out node is tree's wording (`<second>: duplicate id "x", also at <first>`), so `show c2` and `mv c2` print the same line.
NOTES (2026-10-13): the plan named internal/index/refresh_test.go, which did not exist; it is new, holding the index tests. A prefix only a left-out node matches fails with its duplicate problem, as tree.Resolve does.

**What:** Added at the regression check (2026-10-13): the index half of F-10 (`duplicate-id-subtree`). After P1.8, `tree.Load` drops both files of a repeated id with their subtrees, so the tree verbs (`mv`, `rm`, `create --parent`) refuse a node below one of them. The index walks with `tree.Walk`, not `tree.Load`, so it still holds rows for that node, and `show`, `close` and `update` keep serving it. Ratified: the index leaves out a repeated id's subtree like an orphan directory, so every verb agrees.
**Regression guard.** The ids of left-out nodes stay in the unique-prefix candidate sets (tree Resolve; index resolveID via P1.6's claims), so a prefix ambiguous at the base stays ambiguous and never resolves to a different node. Claims under a duplicate-id directory make `unusableClaim` return a `*tree.Error` wrapping `ErrDuplicate` that names both repeated-id files, so `show c2` gives the Goal's duplicate problem (not P1.6's not-found wording). `walkIssues` appends the duplicate-id directory problem (wrapping `ErrOrphanDirectory`) to the problems P1.6 turns into claims, before the claims are built, so `c2` has a claim and `show c` never prefix-matches it. The wording comes from a new exported constructor in `internal/tree` for the duplicate-id directory error (index cannot build tree's unexported `invalidIDDirectoryError`/`duplicateIDReason`); the index warns with kind `WarningOrphanDirectory` at the directory path, so cli `isIndexWarnedAbout` de-duplicates the tree problem. Rule: the repeated ids come from every walk node of the tree (ids only, not aliases), as P1.8's `indexIDs` counts them, before any subtree is left out.
**Goal:** after a refresh, no index row exists for a node file under the directory of a repeated id (both files' directories). `capcom show <child>` fails with the same duplicate problem the tree verbs give. Refresh reports each such directory as left out, with the wording P1.8 uses (`directory left out: its node file <id>.md has a duplicate id`).
**Approach (assumed at the header base):** in the index refresh walk (the `tree.Walk` consumer in `internal/index`), when an id is repeated, drop the rows of the nodes under either file's directory and record a left-out-directory problem. P1.6's claims for left-out files cover these files' names too. The index schema version does not rise again: P1.5 already raises it once in this plan, and this item changes no row shape.
Depends on P1.8.
**Files:** internal/index/refresh.go; internal/index/walk.go; internal/index/query.go; internal/index/refresh_test.go; internal/tree/tree.go; internal/cli/read_test.go
**Read first:** internal/index/walk.go — walkIssues, walkWarningKind; internal/index/refresh.go — refresh, refresher.sync, excludeDuplicatesOf, excludeClaims; internal/index/query.go — resolveID, unusableClaim, duplicateClaim; internal/tree/tree.go — Walk, invalidIDDirectoryError, indexIDs; internal/cli/errors.go — isIndexWarnedAbout, treeProblemKindOf; internal/cli/read_test.go — show tests (no show_test.go exists); internal/cli/rm_test.go — newDuplicateIssueStore, assertDuplicateRefusal
**Closes:** duplicate-id-subtree
**Tests:**
- index test with `a.md`, `a/x.md`, `b.md`, `b/x.md` and `b/x/c2.md` (`a.md` and `b.md` so `a/` and `b/` are no orphan directories): no `c2` row after refresh, and a left-out problem for the directory.
- CLI test (`internal/cli/read_test.go`): `capcom show c2` exits with the duplicate error naming both files.
- index prefix test: a prefix shared by `c2` and a loaded id stays ambiguous after refresh (it pins the base behaviour, not a bite).
- The index row test and the CLI test fail on the pre-item tree (bite check).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Show'`
- `go vet ./internal/index/ ./internal/cli/`
**Commit:** `fix(index): leave out the subtree of a repeated id`

## P1.20 Plan rewrites: compare-and-swap in apply and undo — ✅ DONE (2026-10-13)

**What:** Fixes the tree half of F-11(a) (`stale-rewrite-cas`): `tree.Plan.AddRewrite` overwrites another Node's file from the planner's snapshot and restores `Original` blindly on rollback (per review F-11(a)). Ratified: local compare-and-swap only, no store-wide write lock.
**Regression guard.** Prose rule: every doc comment that says what Apply does with a rewrite or its rollback gets the compare-and-swap clause (`grep -n 'rewrite' internal/tree/*.go`), `Move.Apply`'s (move.go) and `Removal`'s (remove.go) included.
**Goal:** A rewrite lands only on a file whose bytes still equal its `Original`. Otherwise `Apply` fails with an error wrapping the new sentinel `tree.ErrChanged`, as an `*fs.PathError` naming the file, and the rewrites already made are taken back. Undo restores `Original` only over a file that still holds the bytes the rewrite wrote. Otherwise it leaves the file as it is and adds a rollback failure naming the file. That failure does not wrap `ErrChanged`.
**Approach (assumed at the header base):** Add `ErrChanged = errors.New("changed since it was read")` to the sentinel block in `tree.go`. Do not call it "stale": the glossary keeps Stale for the Tree lock. In `AddRewrite`'s apply, after the `os.Stat` for the mode, re-read the file with `ReadNodeFile` (the guarded reader, a thin wrapper over it once P0.3 lands). A read failure is returned as it is. Bytes that differ from `rewrite.Original` fail with `&fs.PathError{Op: "rewrite", Path: rewrite.Path, Err: ErrChanged}` before any write. The apply remembers the bytes it wrote: `Content` with every CRLF turned into LF, the conversion `WriteFileAtomic` makes. Undo re-reads with `ReadNodeFile` and writes `Original` back (`WriteFileAtomicVerbatim`, the remembered mode) only when the bytes equal those written bytes. Otherwise it returns `fmt.Errorf("%s: changed since the rewrite; left as it is", path)`, which `rollBack` appends as a "rolling back:" failure. Update the doc comments of `AddRewrite` and `Plan.Apply`. Existing tests whose `Original` is `"x\n"` match `writeNodes`' bytes and stay green unchanged.
Depends on P0.3.
**Files:** internal/tree/tree.go; internal/tree/plan.go; internal/tree/move_test.go; internal/tree/move.go; internal/tree/remove.go
**Read first:** internal/tree/plan.go — Plan.AddRewrite, Plan.Apply, rollBack; internal/tree/read.go — ReadNodeFile; internal/tree/tree.go — sentinel var block (ErrNotRegular, ErrTooLarge); internal/tree/move_test.go — TestMoveApply_RenameFails_RestoresRewriteBytesAndMode, TestPlanApply_RewriteAddedAfterStep_AppliesFirst, recordStep, errInjected, snapshotDir; internal/tree/tree_test.go — writeNodes; internal/tree/move.go — Move.Apply, Rewrite; internal/tree/remove.go — Removal, removeFileStep
**Tests:** add to `move_test.go`, beside the `TestPlanApply_*` tests. (1) `TestPlanApply_RewrittenFileChangedBeforeApply_FailsChangedAndTakesBackEarlierRewrites`: a `tree.Plan` holds rewrites of A and then B. The test writes other bytes to B after the plan is built. `Apply` fails `errors.Is(err, tree.ErrChanged)` with B's path in the text, A holds its `Original` and B holds the concurrent bytes. (2) `TestPlanApply_RewrittenFileChangedBeforeRollback_LeftAsItIsAndNamed`: a rewrite of A, then an `AddStep` whose apply writes other bytes into A and returns `errInjected`. `Apply` fails `errors.Is(err, errInjected)` and `!errors.Is(err, tree.ErrChanged)`, the text names A after "rolling back", and A keeps the concurrent bytes. Both tests must fail on the pre-item tree (bite check).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/issue/`
- `GOMEMLIMIT=2GiB go test ./internal/knowledge/`
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `go vet ./internal/tree/`
**Commit:** `fix(tree): compare-and-swap every plan rewrite and its rollback`

## P1.21 mv, rm and fmt: a file changed under the command is a conflict — ✅ DONE (2026-10-13)

NOTES (2026-10-13): Move and Remove share one helper, `Kind.applyChangedFailure` (errors.go), which splits the joined Apply error, checks the first failure for `tree.ErrChanged`, `errors.As`es its `*fs.PathError` and builds `Kind.changedError` with the rest as rollback failures (newlines flattened, paths relativized, as `removalApplyFailure` does); Format's re-read lives in a new `Kind.formatRecheck` (format.go) rather than inline.
NOTES (2026-10-13): Format refuses a changed file with `k.changedError` as the regression guard names it; the approach text's `k.staleError` is that same helper under the name the guard rejects.
NOTES (2026-10-13): consequential edit — internal/cli/mv.go: made necessary by mv now refusing with conflict when a rewritten file changed (prose guard)
NOTES (2026-10-13): consequential edit — internal/cli/know_mv.go: made necessary by know mv now refusing with conflict when a rewritten file changed (prose guard)
NOTES (2026-10-13): consequential edit — internal/cli/rm.go: made necessary by rm now refusing with conflict, taking the removal back, when a rewritten file changed (prose guard)
NOTES (2026-10-13): consequential edit — internal/cli/know_rm.go: made necessary by know rm now refusing with conflict, taking the removal back, when a rewritten file changed (prose guard)

**What:** Fixes the nodestore half of F-11(a) (`stale-rewrite-cas`). `nodestore.Move` and `nodestore.Remove` turn a rewrite refused with `tree.ErrChanged` into the conflict `--if-match` already reports. `nodestore.Format` gets the same compare-and-swap for each file. No sentinel exists for `--if-match`: it is refused with the typed `*nodestore.ConflictError` (built by `Kind.conflictError`), which `cli.kindFailure` maps to `CodeConflict`. `tree` cannot import `nodestore`, so `nodestore` makes the conversion.
**Regression guard.** Name the helper `Kind.changedError` (not `staleError`): the glossary reserves "stale" for the Tree lock. Prose rule: every comment or doc passage that says when mv, rm, know mv, know rm, fmt or `kindFailure` yield a conflict, or what `beforeRecheck` serves (`grep -rn -i 'conflict\|beforeRecheck' internal/cli/*.go internal/nodestore/*.go docs/format.md`), the `capcom fmt` passage of docs/format.md included.
**Goal:** Suppose a file that `mv`, `rm`, `know mv`, `know rm` or `fmt` rewrites changes between the command's read and its write. The command then fails with a `*nodestore.ConflictError` at that file (relative path) with Message exactly `changed since this command read it; run the command again`. Each rollback failure is appended after `; `. `IfMatch` and `ETag` are empty. The concurrent bytes survive.
**Approach (assumed at the header base):** Add `Kind.changedError(path, rollbackFailures)` in `errors.go`. Widen the `ConflictError` doc to cover this second cause. In `Move`, before `k.fileError(node.Path, err)` on an `applyMove` failure, check `errors.Is(err, tree.ErrChanged)`. Then `errors.As` the `*fs.PathError` for the file and return the conflict. The rollback failures are the joined errors after the first, flattened as `removalApplyFailure` does. Do the same in `Remove` before `removalApplyFailure`. In `Format`, right before the `k.write` of a file, call `beforeRecheck(node.Path)` (Edit's existing test seam). Then re-read the file with `tree.ReadNodeFile` (a read failure is a `*FileError`, as in its first read). Refuse bytes that differ from `data` with `k.staleError`. Files already rewritten stay rewritten, as `Format`'s doc already says. Update the step lists in the doc comments of `Move` (step 8), `Remove` (step 7) and `Format`. Prose guard: every `docs/format.md` passage that says what `--if-match` refuses gets one clause saying the command also refuses with a conflict when a file it rewrites changed while it ran (`grep -n 'if-match' docs/format.md`).
Depends on P1.20.
**Files:** internal/nodestore/errors.go; internal/nodestore/move.go; internal/nodestore/remove.go; internal/nodestore/format.go; internal/nodestore/move_test.go; internal/nodestore/remove_test.go; internal/nodestore/format_test.go; docs/format.md; internal/cli/errors.go; internal/nodestore/edit.go
**Read first:** internal/nodestore/move.go — Kind.Move; internal/nodestore/remove.go — Kind.Remove, Kind.removalApplyFailure; internal/nodestore/format.go — Kind.Format; internal/nodestore/errors.go — ConflictError, Kind.conflictError, Kind.fileError; internal/nodestore/edit.go — beforeRecheck, Kind.recheck; internal/nodestore/edit_test.go — setBeforeRecheck; internal/nodestore/move_test.go — newMoveIssues, TestMove_ApplyFails_NamesTheFilesRelativeToTheWorktree, treeContent; internal/nodestore/remove_test.go — newRemoveIssues, TestRemove_ApplyFails_NamesTheFailedFileRelativeToTheWorktree
**Closes:** stale-rewrite-cas
**Tests:** (1) `move_test.go`: `issues.applyMove` is swapped for a func that writes other bytes to a dependent issue's file and then calls `move.Apply()`. Assert a `*ConflictError` at the dependent's relative path with the exact Message, the dependent holding the concurrent bytes, and the moved file still at its old path. (2) `remove_test.go`: the same through `issues.applyRemoval` on a `--drop-deps` removal. (3) `format_test.go`: a non-parallel test sets `beforeRecheck` (`setBeforeRecheck`) to change the second unformatted file. Assert the `*ConflictError` at it and that the file keeps the concurrent bytes. `cli.kindFailure`'s `*ConflictError` → `conflict` mapping is unchanged. All three must fail on the pre-item tree (bite check).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Mv|Rm|Fmt|KnowMv|KnowRm'`
- `go vet ./internal/nodestore/`
**Commit:** `fix(nodestore): refuse a file changed under mv, rm or fmt as a conflict`

## P1.22 Replace-mode writes refuse a missing target; Move checks its new parent — ✅ DONE (2026-10-13)

NOTES (2026-10-13): `writeFileAtomic` and `writeTemp` each gained a trailing bool (`isReplacing`, `isCreatingDir`); the replace path's Lstat lives in a new helper `checkReplaceTarget`, which returns any non-missing Lstat failure as it is. A missing directory fails at `os.CreateTemp` (an error naming path that wraps fs.ErrNotExist), not at the Lstat.
NOTES (2026-10-13): no CLOSES: `replace-missing-target` also wants the Create/Edit/Format half, which P1.23 owns and closes.
NOTES (2026-10-13): Bite check: with ReplaceFileAtomic routed through the create-directory path and the pre-item move.go restored, TestReplaceFileAtomic_MissingPath_FailsNotExistCreatingNothing and TestMoveApply_NewParentRemovedBeforeApply_FailsCreatingNoDirectory both fail.

**What:** Fixes the tree half of F-11(b) (`replace-missing-target`): a replace-mode write recreates a node file, or its directory, that another session moved or removed. The Move `Mkdir` step can also recreate the children directory of a removed parent (per review F-11(b)).
**Goal:** `tree.ReplaceFileAtomic(path, content)` writes like `WriteFileAtomic` (CRLF to LF, mode 0644, atomic). It never creates a directory, and it fails with an error wrapping `fs.ErrNotExist` and naming `path` when `path` is missing just before the rename, leaving no temp file. `AddRewrite`'s apply writes through it. A Move that creates the new parent's children directory first checks that the parent's node file still exists. If it does not, the move fails, creates no directory and leaves the tree as it was.
**Approach (assumed at the header base):** In `atomic.go`, add `ReplaceFileAtomic`, built on the shared `writeFileAtomic` path. `writeTemp` gets a way to skip its `os.MkdirAll`. The replace path `os.Lstat`s `path` right before `os.Rename`. On a missing path it removes the temp file and returns `&fs.PathError{Op: "replace", Path: path, Err: fs.ErrNotExist}`. `WriteFileAtomic`, `WriteFileAtomicVerbatim` and `ReplaceFileVerbatim` keep their behaviour. In `plan.go`, `AddRewrite`'s apply calls `ReplaceFileAtomic` in place of `WriteFileAtomic`. In `move.go`, the `Mkdir` step that `planRenames` adds `os.Lstat`s `newDir + nodeExtension` before `os.Mkdir`. That step exists only when the new parent has no children directory, so it always has a parent node file. A failed `Lstat` is returned as it is.
Depends on P1.20.
**Files:** internal/tree/atomic.go; internal/tree/atomic_test.go; internal/tree/plan.go; internal/tree/move.go; internal/tree/move_test.go
**Read first:** internal/tree/atomic.go — writeFileAtomic, writeTemp, WriteFileAtomic, ReplaceFileVerbatim, WriteFileExclusive; internal/tree/plan.go — Plan.AddRewrite; internal/tree/move.go — Move.planRenames, nodeExtension; internal/tree/tree_test.go — dirEntries, TestWriteFileAtomic_Success_ReplacesFileAndLeavesNoTempFile; internal/tree/atomic_test.go — TestWriteFileExclusive_PathExists_FailsWithErrExistAndLeavesFileAndNoTempFile; internal/tree/move_test.go — snapshotDir, TestPlanMove_ReparentWithChildren_MovesSubtreeAndRemovesEmptyDir
**Tests:** (1) `atomic_test.go`: `TestReplaceFileAtomic_ExistingFile_ReplacesAsLFAndLeavesNoTempFile`. (2) `TestReplaceFileAtomic_MissingPath_FailsNotExistCreatingNothing`, with two cases, a missing file in an existing directory and a missing directory. Each asserts `errors.Is(err, fs.ErrNotExist)`, the path in the text, no file, no directory and no `.tmp-` entry. (3) `move_test.go`: `TestMoveApply_NewParentRemovedBeforeApply_FailsCreatingNoDirectory`, which re-parents below a childless node, removes that node's file after `PlanMove` and asserts `Apply` fails with the tree unchanged (`snapshotDir`). (2) and (3) must fail on the pre-item tree (bite check).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `go vet ./internal/tree/`
**Commit:** `fix(tree): refuse to recreate a removed node file or parent directory`

## P1.23 Edit, Format and Create never recreate a removed node or parent — ✅ DONE (2026-10-13)

NOTES (2026-10-13): `place` turns any replace-mode failure wrapping `fs.ErrNotExist` into `k.fileError(path, err)`; other write failures are still returned as they are. The `writeMode` doc for `replaceFile` and `write`'s doc now say replace mode never creates a missing file or directory.
NOTES (2026-10-13): Bite check: with create.go, edit.go and format.go restored to the pre-item tree, TestCreate_ParentRemovedBeforeTheWrite_FailsAtTheParentAndCreatesNoDirectory and both cases of TestPlace_ReplaceMissingTarget_FailsAtTheFileAndCreatesNothing fail.

**What:** Fixes the nodestore half of F-11(b) (`replace-missing-target`). `nodestore.Edit` and `Format` replace through `tree.ReplaceFileAtomic`. `nodestore.Create` checks right before its exclusive write that the parent's node file is still there, because otherwise its write recreates `<parent>/` with no node file.
**Regression guard.** The Edit/Format half is pinned in nodestore: a `create_test.go` test (package nodestore) calls `k.place(path, id, data, replaceFile)` on a missing file and on a missing directory and asserts a `*FileError` at the relative path with no file or directory created.
**Goal:** Suppose another session removes the node file between an `Edit` or `Format` read and the write. The write then fails with a `*FileError` at that file and creates nothing. Suppose a `Create` below a parent finds the parent's node file gone just before its write. It then fails with a `*FileError` at the parent's file (relative path), and neither the parent's children directory nor the new file exists afterwards.
**Approach (assumed at the header base):** In `create.go`, `place` uses `tree.ReplaceFileAtomic` under `replaceFile` (`tree.WriteFileExclusive` stays under `createFile`). A replace failure wrapping `fs.ErrNotExist` becomes `k.fileError(path, err)`. In `Create`, keep the parent's absolute file from `newNodePath`. When it is not "", `os.Lstat` it right before `k.write(path, id, node, createFile)`, and on failure return `k.fileError(parentPath, err)`. Update step 8 of `Edit`'s doc comment in `edit.go`: a node file removed since the re-check fails with a `*FileError` and is not recreated. Update `Create`'s step 8 and `Format`'s doc to match.
Depends on P1.22.
**Files:** internal/nodestore/create.go; internal/nodestore/edit.go; internal/nodestore/format.go; internal/nodestore/create_test.go
**Read first:** internal/nodestore/create.go — Kind.place, Kind.write, Kind.Create, newNodePath; internal/nodestore/edit.go — Kind.Edit, Kind.recheck; internal/nodestore/format.go — Kind.Format; internal/nodestore/errors.go — Kind.fileError; internal/nodestore/create_test.go — TestCreate_FilePlantedAfterTheClaimCheck_FailsAsTakenAndKeepsIt, TestCreate_ParentByAlias_WritesIntoItsChildrenDirectory
**Closes:** replace-missing-target
**Tests:** `create_test.go`: `TestCreate_ParentRemovedBeforeTheWrite_FailsAtTheParentAndCreatesNoDirectory`. The parent has no children yet. The `build` callback removes the parent's node file, and the test asserts a `*FileError` at the parent's relative path, no `<parent>/` directory and no new file. It must fail on the pre-item tree (bite check). `Edit` and `Format` rely on P1.22's `ReplaceFileAtomic` tests for the missing-target refusal. `create_test.go` (package nodestore): `TestPlace_ReplaceMissingTarget_FailsAtTheFileAndCreatesNothing` calls `k.place(path, id, data, replaceFile)` on a missing file and on a missing directory; each fails with a `*FileError` at the relative path and creates nothing. Bite: today's `WriteFileAtomic` creates both.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Create|Update|Fmt|KnowAdd|KnowUpdate'`
- `go vet ./internal/nodestore/`
**Commit:** `fix(nodestore): never recreate a node or parent another session removed`

## P1.24 Node-file rename never replaces a file created meanwhile — ✅ DONE (2026-10-13)

NOTES (2026-10-13): The node-file step (`Plan.fileRenameStep` / `Plan.moveFile`) hard-links through the package `link` var, then removes the old name through a new package var `removeFile` (seam `tree.SetRemoveFile` in export_test.go, used by test (3)); undo is the same move reversed, so an undo onto a path that appeared meanwhile is refused too instead of replaced. The directory step and the link-refused fallback share a new `Plan.checkedRename` (today's `Lstat` + `doRename`); the directory undo still renames back unchecked as before.
NOTES (2026-10-13): Bite check: with plan.go and move.go restored to the pre-item tree (plus a stub `removeFile` so the tests build), all three new tests fail; TestMoveApply_FileCreatedAtTargetDuringRename_RefusedKeepingBoth fails with `Apply err = <nil>`.

**What:** Fixes F-11(c) (`rename-no-replace`). `Plan.renameStep` checks the target with `os.Lstat` and then calls `os.Rename`, so a file created in between is silently replaced (per review F-11(c)).
**Regression guard.** When removing the old path fails after the link landed, the file step removes the new path before returning the error, so no duplicate node file is left. The Goal's "at any moment" holds on a file system with hard links; without them the fallback keeps today's `Lstat` check.
**Goal:** A Move's node-file rename refuses a target that appears at any moment before it lands. The rename fails with an error wrapping `tree.ErrMoveTarget` that names the target, and leaves both files intact. On a file system that refuses hard links, the rename still moves the file. Directory renames keep their `Lstat` and `os.Rename`.
**Approach (assumed at the header base):** Split `renameStep` into a directory step (unchanged) and a node-file step. Unless a test swapped `p.rename` (`SetMoveRename`), the file step names the new path with the package's `link` var (`os.Link`, atomic `EEXIST`) and then `os.Remove`s the old path. `EEXIST` becomes `fmt.Errorf("%s: %w", newPath, ErrMoveTarget)`. Any other link refusal falls back to the existing `Lstat` and rename, mirroring `placeExclusive`'s fallback. Undo is the same pair reversed (link new→old, then remove new). `planRenames` uses the file step for `node.Path` and the directory step for the children directory. Update `SetLink`'s doc in `export_test.go`, which also names the Move rename.
Depends on P1.22.
**Files:** internal/tree/plan.go; internal/tree/move.go; internal/tree/export_test.go; internal/tree/move_test.go
**Read first:** internal/tree/plan.go — Plan.renameStep, Plan.doRename, Plan.Apply, rollBack; internal/tree/move.go — Move.planRenames, ErrMoveTarget; internal/tree/atomic.go — link, placeExclusive; internal/tree/export_test.go — SetLink, SetMoveRename; internal/tree/move_test.go — TestMoveApply_FileAtTarget_RefusedLeavingBothInPlace, TestMoveApply_SecondRenameFails_LeavesTreeUnchanged, TestMoveApply_ChildrenDirAtTarget_RefusedRenamingNodeFileBack; internal/tree/atomic_test.go — refuseHardLinks
**Closes:** rename-no-replace
**Tests:** add to `move_test.go`: (1) `TestMoveApply_FileCreatedAtTargetDuringRename_RefusedKeepingBoth`: `tree.SetLink` writes a file at the new path and then calls `os.Link`. Assert `errors.Is(err, tree.ErrMoveTarget)`, the planted bytes intact and the node file at its old path with its bytes. (2) `TestMoveApply_NoHardLinks_RenamesThroughTheFallback`: `SetLink` returns an `*os.LinkError` wrapping `errors.ErrUnsupported`. Assert the move lands. (1) must fail on the pre-item tree (bite check). The existing `SetMoveRename` tests stay green. (3) `TestMoveApply_OldPathUnremovableAfterLink_RemovesTheNewName`: with a remove seam (or a `SetLink` that also makes the old path unremovable), the apply fails and only the old file remains.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `go vet ./internal/tree/`
**Commit:** `fix(tree): rename a node file without replacing one created meanwhile`

## P1.25 Elsewhere scan is off when elsewhere.days is 0 — ✅ DONE (2026-10-13)

NOTES (2026-10-13): docs/format.md — the `elsewhere.days` entry now says 0 turns the scan off (user-facing doc for the changed behaviour; the paragraph was rewrapped to the file's line width). The `Scan` doc comment also names the 0-days early return, beside the `Options.Days` comment the plan names.
NOTES (2026-10-13): Bite check: with elsewhere.go restored to the pre-item tree, both new tests fail: the future-dated branch is reported as `session-refresh: done on fix-login (unmerged)`, and the plain temp directory warns `elsewhere: list branches: ...`. Tests use a helper `scanZeroDays` beside `scan`.

**What:** Fixes F-18 (`elsewhere-days-zero`): with `Days: 0`, a branch whose tip is dated in the future still passes the cutoff (per review F-18). Following the review's Fix, `Scan` returns before any git call. No upper bound on tip dates is added.
**Goal:** `elsewhere.Scan` with `Options.Days == 0` returns an empty `Report`, with no entries and no warnings, and runs no git command, whatever the branches and their dates.
**Approach (assumed at the header base):** At the top of `Scan`, before `gitx.Branches`, return the zero `Report` when `options.Days == 0`. Extend the `Options.Days` doc comment: 0 skips the scan before any git call.
Depends on P0.9.
**Files:** internal/elsewhere/elsewhere.go; internal/elsewhere/elsewhere_test.go
**Read first:** internal/elsewhere/elsewhere.go — Scan, Options, otherRecentBranches; internal/elsewhere/elsewhere_test.go — newRepo, onBranch, commitIssues, scan, TestScan_FreshRepository_ReportsNothing; internal/cli/output.go — addElsewhere; internal/briefing/briefing.go — scanElsewhere
**Closes:** elsewhere-days-zero
**Tests:** `elsewhere_test.go`: (1) `TestScan_ZeroDays_FutureDatedBranch_ReportsNothing` uses a side branch committed with `GIT_COMMITTER_DATE` and `GIT_AUTHOR_DATE` a year ahead, set through `t.Setenv` around `onBranch`. (2) `TestScan_ZeroDays_NotARepository_RunsNoGit` points `WorktreeRoot` at a plain temp directory. With no early return, `gitx.Branches` fails there and warns "list branches", so an empty `Warnings` proves no git ran. Both must fail on the pre-item tree (bite check).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/elsewhere/`
- `go vet ./internal/elsewhere/`
**Commit:** `fix(elsewhere): skip the scan when elsewhere.days is 0`

## P1.26 gitx: HasCommit returns fatal git failures; pin IsAncestor's error path — ✅ DONE (2026-10-13)

**What:** Fixes F-19 (`hascommit-fatal-exit`): `gitx.HasCommit` maps every git exit to "no such commit". The item also adds T-08's (`isancestor-error-test`) `IsAncestor` error test.
**Goal:** `HasCommit` returns `(false, nil)` only when `rev-parse --verify --quiet` exits 1. Any other git failure, such as exit 128 outside a repository, comes back as `(false, err)`. `IsAncestor` with a ref that does not exist returns `(false, err)` with a non-nil error.
**Approach (assumed at the header base):** In `branches.go`, add a named constant beside `notAncestorExit` for `rev-parse`'s "names no commit" exit (1). In `HasCommit`, check `errors.As(err, &exitFailure)` against it as `IsAncestor` does, and return every other error unchanged. The `*Error` `run` returns unwraps to the `*exec.ExitError`. The caller `elsewhere.Scan` already turns the error into a "resolve HEAD:" warning.
**Files:** internal/gitx/branches.go; internal/gitx/branches_test.go
**Read first:** internal/gitx/branches.go — HasCommit, IsAncestor, notAncestorExit; internal/gitx/gitx.go — run, Error.Unwrap; internal/gitx/gitx_test.go — setHermeticGitEnv, newRepo, git; internal/gitx/branches_test.go — TestBranches_UnbornRepository_ListsNone, TestIsAncestor_MergedAndDivergedBranches; internal/elsewhere/elsewhere.go — Scan
**Closes:** hascommit-fatal-exit; isancestor-error-test
**Tests:** `branches_test.go`: (1) `TestHasCommit_NotARepository_ReturnsTheGitError`, in a temp dir with `setHermeticGitEnv` (ceiling set) and no `git init`. It must fail on the pre-item tree (bite check). (2) `TestHasCommit_UnbornHead_IsFalseWithoutError` is a regression guard and passes before and after. (3) `TestIsAncestor_MissingRef_ReturnsAnError` covers T-08. It pins existing behaviour, so it passes on the pre-item tree.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/gitx/`
- `GOMEMLIMIT=2GiB go test ./internal/elsewhere/`
- `go vet ./internal/gitx/`
**Commit:** `fix(gitx): report a fatal rev-parse failure from HasCommit`

## P1.27 Index listings read from one snapshot — ✅ DONE (2026-10-13)

NOTES (2026-10-13): SearchIssues reads its ids through a new unexported searchIDs(q, text), and List through listIDs(q, filter), so Search and ListIDs stay single statements on x.db while the snapshot readers never call an exported method; Issue and Note likewise wrap new readIssue/readNote readers.
NOTES (2026-10-13): the List test indexes its own three open issues instead of reusing listFixture, and both tests take the ids to expect from ListIDs/Notes before swapping the seam rather than hard-coding the last id.

**What:** Fixes F-20 (`index-read-snapshot`). `List`, `SearchIssues`, `Notes` and `SearchNotes` read their ids and then each entry in separate statements. A refresh committed by another command in between fails the whole read with `id "x" not found`, or mixes two versions in one Entry (per review F-20).
**Goal:** `Index.List`, `SearchIssues`, `Resolve`, `Issue`, `Notes`, `SearchNotes`, `ResolveNote` and `Note` each read all their rows from one SQLite read snapshot. Rows that another connection deletes or changes partway through one call do not affect its result.
**Approach (assumed at the header base):** Add an unexported `x.snapshot(func(q querier) error) error`. It takes a `*sql.Conn` from `x.db`, runs the raw statements `BEGIN DEFERRED` … `COMMIT` on it (`ROLLBACK` on failure), and passes the conn as the querier. It never uses `BeginTx`, because the DSN's `_txlock=immediate` would take the write lock. `querier` is the `QueryContext`/`QueryRowContext` subset that `*sql.DB` and `*sql.Conn` share. `MaxOpenConns` is 1, so every statement inside must use `q`, never `x.db`. Make `strings`, the issue row reader behind `Issue`, the Note row reader behind `Note`, `resolveID` and its claim helpers, `issues` and `noteList` take a querier. Each exported method opens one snapshot and calls only these unexported readers inside it, never another exported method. Add a test seam `var beforeEntryRead = func(id string) {}`, called in `issues` and `noteList` before each entry read. Unchanged: `Refresh`, `Body`, `RelatedBy`, `Search` and `ListIDs`, which are each one statement.
Depends on P1.4.
**Files:** internal/index/index.go; internal/index/query.go; internal/index/notes.go; internal/index/query_test.go; internal/index/notes_test.go
**Read first:** internal/index/query.go — List, ListIDs, SearchIssues, Resolve, resolveID, unusableClaim, duplicateClaim, issues, resolved; internal/index/index.go — Open (SetMaxOpenConns), dsnOptions, Issue, strings, Search; internal/index/notes.go — Note, ResolveNote, Notes, SearchNotes, noteList; internal/index/query_test.go — listFixture; internal/index/index_test.go — TestRefresh_CommitFails_ReportsOnlyTheCommitError (seam swap pattern), newStore, openIndex
**Closes:** index-read-snapshot
**Tests:** (1) `query_test.go` (`package index`): `TestList_RowsDeletedMidRead_ReturnsOneSnapshot`. `beforeEntryRead`, on its first call, opens a second connection (`sql.Open(driverName, Path(store)+dsnOptions)`) and deletes the last listed issue's `issues` row. `List` then returns every issue with no error. (2) `notes_test.go`: the same for `Notes`. Restore the seam in `t.Cleanup` and do not run these tests in parallel. Both must fail on the pre-item tree (bite check).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'List|Ready|Search|Close|KnowTree|KnowSearch'`
- `go vet ./internal/index/`
**Commit:** `fix(index): read each query's rows from one snapshot`

## P1.28 Walk treats a child directory that vanished as empty — ✅ DONE (2026-10-13)

**What:** Fixes F-21 (`walk-vanished-dir`): `treeWalker.walk` returns the raw `os.ReadDir` error, so a child directory moved or removed mid-walk fails `tree.Walk`, the index refresh and `Open` (per review F-21).
**Regression guard.** This item amends the `Load` doc comment as P1.8 left it, not the base wording. Rule: every comment saying an unreadable directory stops the walk or the load (`grep -n 'cannot be read' internal/tree/*.go`) gains "a child directory gone since its parent was read walks as empty".
**Goal:** `tree.Walk` and `tree.Load` succeed without a child directory that is gone by the time it is read, and return every other node. A missing top directory still walks empty, and any other `ReadDir` failure still stops the walk.
**Approach (assumed at the header base):** In `tree.go`, add `var readDir = os.ReadDir` and use it in `treeWalker.walk`. When `errors.Is(err, fs.ErrNotExist)` and `parentID != rootParentID`, return nil (an empty directory). Update the doc comments of `Walk` and `walk`: a child directory gone since its parent was read is walked as empty. Add `SetReadDir(t *testing.T, fn func(string) ([]fs.DirEntry, error))` to `export_test.go`, restoring the variable in `t.Cleanup`, following `SetLink`.
**Files:** internal/tree/tree.go; internal/tree/export_test.go; internal/tree/tree_test.go
**Read first:** internal/tree/tree.go — Walk, treeWalker.walk, Load, leftOutFileOf, rootParentID; internal/tree/export_test.go — SetLink; internal/tree/tree_test.go — writeNodes, TestWalk_MissingDirectory_WalksEmpty, TestWalk_DirectoryProblems_WordedAndLeftOut; internal/index/walk.go — walkIssues
**Closes:** walk-vanished-dir
**Tests:** `tree_test.go`: `TestWalk_ChildDirectoryGoneBeforeItsRead_WalksTheRest`. The nodes are `a.md`, `a/b.md` and `c.md`. A `SetReadDir` hook `os.RemoveAll`s `a/` when asked for it, then calls `os.ReadDir`. Assert no error, nodes `a` and `c`, no `b` and no problems. It must fail on the pre-item tree (bite check). The test is not parallel.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `go vet ./internal/tree/`
**Commit:** `fix(tree): walk a child directory that vanished mid-walk as empty`

## P1.29 Log section lookup skips fenced code blocks — ✅ DONE (2026-10-13)

NOTES (2026-10-13): CodeFence also exports IsOpen(), which logBounds needs to tell that its scan ended inside a never-closed fence (regression guard 1); the redo marks each such opener as plain text and scans again until no fence is left open.
NOTES (2026-10-13): the shared helper is logBounds (with its one-pass scanLogBounds) in mutate.go, taking lines without their line endings; appendLogEntry trims only "\n" and parseLog uses trimLineEnd, so each keeps its own line-ending handling, and the heading test is now TrimRight(" \t") on those trimmed lines.
NOTES (2026-10-13): the "unclosed fence above the Log appends to it" case passes on the pre-item tree by design (it guards against a regression of the fence-aware lookup); the other two new cases fail there (bite-checked). The rawfm package doc now also names body start and code fences.

**What:** Recast at the regression check (2026-10-13). Fixes F-22 (`log-fence-aware`). `appendLogEntry` and `parseLog` take the first `## Log` line even when it sits inside a fenced code block, and their section-end scan does the same (per review F-22).
**Regression guard.** (1) A fence still open at the end of the lines hides nothing: when the Log-bounds scan ends inside a fence, redo the start/end search treating that opener as plain text (`ConflictMarkers` keeps treating it as code); add mutate_test case "unclosed fence above the Log appends to it". (2) `parseLog` starts its `CodeFence` scan at the body (after the closing `---`), reusing rawfm's body-start rule (export it from rawfm).
**Regression guard (round 2).** Export the body-start rule through an exported wrapper and keep the unexported `bodyStartIndex`, whose second caller `frontmatterEndIndex` (internal/rawfm/aliases.go) stays unchanged.
**Goal:** In an issue body or file, the Log section opens at the first `## Log` line outside a fenced code block and ends at the first level-one or level-two heading outside one. `capcom log` and `reopen` append to the real Log. `MergeFile` merges the real Log entry by entry. `rawfm.ConflictMarkers` keeps its behaviour.
**Approach (assumed at the header base):** There is one fence rule, and it lives in `rawfm`, which `issue` already imports. Move `codeFenceOpener`, `closesCodeFence`, `trimFenceIndent` and the fence constants into a new `internal/rawfm/fence.go`. Export from it a stateful `CodeFence` (zero value outside any block) with `IsCode(line string) bool`, which reports whether a line, without its line ending, opens, lies inside or closes a fenced block. `ConflictMarkers` uses it for body lines. In `issue`, one unexported helper finds the Log's start and end over a line slice with a `CodeFence`, and `appendLogEntry` (`mutate.go`) and `parseLog` (`merge.go`) both use it. `isSectionBoundary` stays the heading test. Keep each caller's own line-ending trimming.
**Files:** internal/rawfm/fence.go; internal/rawfm/conflict.go; internal/issue/mutate.go; internal/issue/merge.go; internal/issue/mutate_test.go; internal/issue/merge_test.go
**Read first:** internal/issue/mutate.go — appendLogEntry, isSectionBoundary, LogHeading, AppendLog; internal/issue/merge.go — parseLog, MergeFile, trimLineEnd, parseLogContent; internal/rawfm/conflict.go — ConflictMarkers, codeFenceOpener, closesCodeFence, trimFenceIndent, bodyStartIndex; internal/rawfm/aliases.go — frontmatterEndIndex; internal/issue/mutate_test.go — TestIssue_AppendLog_AddsEntryToLogSection; internal/issue/merge_test.go — issueFile, fakeTextMerge, TestMergeFile_ThreeVersions_MergesLogByEntry
**Closes:** log-fence-aware
**Tests:** (1) `mutate_test.go`: a case for `AppendLog` with a fenced block quoting `## Log` and an entry above the real `## Log`. The new entry lands in the real Log, and the fenced example is byte-identical. (2) `merge_test.go`: `MergeFile` on three versions with that fenced example, where ours and theirs each append one entry. The merge has no conflict and holds both entries in the real Log. Both must fail on the pre-item tree (bite check). The existing `rawfm` conflict tests pin the move. (3) `mutate_test.go`: "unclosed fence above the Log appends to it": a body with a never-closed fence above `## Log` gets the entry in that Log and no second `## Log` heading.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/rawfm/`
- `GOMEMLIMIT=2GiB go test ./internal/issue/`
- `go vet ./internal/rawfm/ ./internal/issue/`
**Commit:** `fix(issue): find the Log section outside fenced code blocks`

## P1.30 Move removes the emptied old parent as a cleanup — ✅ DONE (2026-10-13)

NOTES (2026-10-13): The `remove` seam and `doRemove` moved from `Removal` to `Plan`, next to `rename`; `Removal`'s steps, its cleanup and `SetRemovalRemove` still work through the embedded Plan. `Move` type's doc also says the emptied old parent's removal is its Plan's cleanup.
NOTES (2026-10-13): Bite check: with the pre-item move.go restored (seam kept so the test builds), TestMoveApply_EmptiedOldDirRemovalFails_WarnsAndKeepsTheMove fails: the old directory is removed through os.Remove and Plan.Apply returns no warning.
NOTES (2026-10-13): The command-level warning (`mv`, `know mv`) is P1.31's, which also closes `move-dir-cleanup`; until then `Move.Apply` drops the warning.

**What:** Fixes the tree half of F-25 (`move-dir-cleanup`): `planRenames` appends `removeEmptyDirStep` as a hard step, so a failed removal of an empty directory rolls back a finished move (per review F-25).
**Regression guard.** `CleanupError`'s doc in remove.go is widened to "a best-effort cleanup of a Plan (a removal or a move)". The Goal's "`removeEmptyDirStep` no longer exists" is checked by an Acceptance grep.
**Goal:** Suppose the removal of a move's emptied old parent directory fails. The move's renames and rewrites stay landed, and `Move.Plan.Apply` returns no error and one `*tree.CleanupError` warning naming that directory. `removeEmptyDirStep` no longer exists.
**Approach (assumed at the header base):** Move the `remove` seam field and `doRemove` from `Removal` to `Plan`, next to `rename`. `Removal`'s uses and `SetRemovalRemove` keep compiling through field promotion. In `planRenames`, replace the step with `m.AddCleanup(func() error { return removeEmptyDir(oldDir, m.doRemove) })` under the same condition, and delete `removeEmptyDirStep`. `Move.Apply` keeps returning only the error. Its doc and `PlanMove`'s say that `PlanMove` adds this cleanup and that a caller wanting its warning calls `Plan.Apply`. Update `planRenames`' doc. Add `SetMoveRemove(move *Move, remove func(string) error)` to `export_test.go`.
**Files:** internal/tree/plan.go; internal/tree/move.go; internal/tree/remove.go; internal/tree/export_test.go; internal/tree/move_test.go
**Read first:** internal/tree/move.go — planRenames, removeEmptyDirStep, Move.Apply, PlanMove; internal/tree/remove.go — Removal, doRemove, removeEmptyDir, CleanupError, PlanRemove; internal/tree/plan.go — Plan, AddCleanup, Plan.Apply, doRename; internal/tree/export_test.go — SetRemovalRemove, SetMoveRename; internal/tree/move_test.go — errInjected, snapshotDir, TestPlanMove_ReparentWithChildren_MovesSubtreeAndRemovesEmptyDir; internal/tree/remove_test.go — TestRemovalApply_EmptiedParentDirRemovalFails_WarnsAndKeepsTheRemoval
**Tests:** `move_test.go`: `TestMoveApply_EmptiedOldDirRemovalFails_WarnsAndKeepsTheMove`. It re-parents the only child of `old-parent`, and `SetMoveRemove` returns `errInjected` for the old directory. `move.Plan.Apply()` returns a nil error and one warning, a `*tree.CleanupError` whose `Path` is the old directory. The node file sits at its new path. It must fail on the pre-item tree (bite check). `TestPlanMove_ReparentWithChildren_MovesSubtreeAndRemovesEmptyDir` stays green.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/issue/`
- `GOMEMLIMIT=2GiB go test ./internal/knowledge/`
- `go vet ./internal/tree/`
- `grep -rn removeEmptyDirStep internal/` prints nothing
**Commit:** `fix(tree): make the emptied old parent's removal a move cleanup`

## P1.31 mv and know mv warn about the old parent left in place — ✅ DONE (2026-10-13)

NOTES (2026-10-13): re-derived from "step 8 of Move's doc" — the renames-and-rewrites step is now step 9 (P1.21 added a step); its text gains the LeftInPlace sentence.
NOTES (2026-10-13): The default `applyMove` is a package-level func `applyMove` in kind.go (calls `move.Plan.Apply()`), used by both `Issues` and `Notes`. `movedEqual` in nodestore/move_test.go now also compares `LeftInPlace`, so the existing Move tests assert no directory was left in place. The nodestore test is `TestMove_CleanupFails_ReturnsTheDirectoryLeftInPlace`.
NOTES (2026-10-13): consequential edit — internal/cli/know_rm.go: made necessary by `cleanupWarnings` now rendering a move's cleanup failures too (its doc said removal only).
NOTES (2026-10-13): Bite check: with the two `cleanupWarnings(moved.LeftInPlace)` lines removed from cli/mv.go and cli/know_mv.go, both new CLI tests fail; the new nodestore test does not compile against the pre-item `applyMove` signature.

**What:** Fixes the rest of F-25 (`move-dir-cleanup`). `nodestore.Move` collects the cleanup warnings of P1.30's move, and `mv` and `know mv` print them the way `rm` and `know rm` do.
**Regression guard.** Rule: every `applyMove =` assignment in internal/nodestore/*_test.go (`grep -n 'applyMove =' internal/nodestore/`), P1.21's new swap included, takes the new signature and calls `move.Plan.Apply()`. `cleanupFailure`'s doc in nodestore/remove.go is widened to cover a move.
**Goal:** Suppose `capcom mv` or `capcom know mv` empties the old parent directory and cannot remove it. The command still exits 0 with the node moved, and warns `<dir>: <os error>; the empty directory was not removed`, with every path relative to the working tree root.
**Approach (assumed at the header base):** In `kind.go`, `applyMove` becomes `func(move *tree.Move) ([]error, error)` and defaults to a func that calls `move.Plan.Apply()`. `Moved` gains `LeftInPlace []*FileError`, documented as on `Removed`. `Move` fills it through `k.cleanupFailure(failure, node)` and updates step 8 of its doc. In `cli/mv.go` (`app.move`) and `cli/know_mv.go`, append `cleanupWarnings(moved.LeftInPlace)` to `result.Warnings`. Producers and consumers of `applyMove`: `Issues` and `Notes` in `kind.go`, `Move` in `move.go`, and the swap in `TestMove_ApplyFails_NamesTheFilesRelativeToTheWorktree` (`move_test.go`).
Depends on P1.30; P1.21.
**Files:** internal/nodestore/kind.go; internal/nodestore/move.go; internal/nodestore/move_test.go; internal/cli/mv.go; internal/cli/know_mv.go; internal/cli/mv_test.go; internal/cli/know_mv_test.go; internal/nodestore/remove.go
**Read first:** internal/nodestore/move.go — Kind.Move, Moved; internal/nodestore/kind.go — Kind.applyMove, Issues, Notes, relativeText; internal/nodestore/remove.go — Removed.LeftInPlace, cleanupFailure; internal/nodestore/move_test.go — TestMove_ApplyFails_NamesTheFilesRelativeToTheWorktree, newMoveIssues, movedEqual; internal/nodestore/remove_test.go — applyRemoval swap returning *tree.CleanupError; internal/cli/mv.go — app.move; internal/cli/know_mv.go — app.moveNote; internal/cli/know_rm.go — cleanupWarnings
**Closes:** move-dir-cleanup
**Tests:** (1) `cli/mv_test.go`: `TestMv_EmptiedOldParentNotRemovable_MovesAndWarns` moves the only child of `old-parent` below a `new-parent` that already has a child. `.capcom/issues` is made `0o555` and restored in `t.Cleanup`, skipped on Windows and as root as in `TestKnowRm_DependentWriteFails_RollsBackNamingTheDependentFile`. Assert exit 0, the file at its new path, and the exact warning `.capcom/issues/old-parent: remove .capcom/issues/old-parent: permission denied; the empty directory was not removed`. (2) `cli/know_mv_test.go`: the same for `know mv` under `.capcom/knowledge`. (3) `nodestore/move_test.go`: `applyMove` returns a `*tree.CleanupError`, and `Moved.LeftInPlace` names the directory relatively. All must fail on the pre-item tree (bite check).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Mv|KnowMv'`
- `go vet ./internal/nodestore/ ./internal/cli/`
**Commit:** `fix(nodestore): warn about the old parent directory a move left in place`

## P1.40 Release: never downgrade a channel pinned to a newer version — ✅ DONE (2026-10-13)

NOTES (2026-10-13): smoke's checkHomebrew skips only when the latest release is published and is another tag; when the latest release cannot be read or none is published it runs brew upgrade as before, so the existing "latest release cannot be read" and "no release is published" smoke cases keep their tallies.
NOTES (2026-10-13): TestSmokeChecks "the bucket has moved past an older release" (latest v0.24.12) now also skips Homebrew, so its tally moves from 6 passed/1 skipped to 5 passed/2 skipped, as the item's checkHomebrew rule requires.
NOTES (2026-10-13): smoke's `Long` help text now says the brew upgrade check applies when the version is the latest release (paragraph rewrapped); docs/releasing.md's Homebrew line stays true and is unchanged.

**What:** Fixes F-03 (`release-no-downgrade`): a rerun of `publish` for an older version rewrites the tap's formula and the bucket's manifest when they pin a newer release.
**Regression guard.** A newer pin also reads done, superseding adr-0008-decision's done test ("the formula pins the version; the bucket manifest is at the version"): this item edits that "publish converges" paragraph through `capcom know update` (P1.43 then rewrites it as this item leaves it) and the doc comments of `probeHomebrew`, `probeScoop` and `pinnedVersionStep`. Rule: every comment or document that defines when the homebrew or scoop step is done (`grep -rn 'pins the version\|at the version\|describes the version' cmd/release .capcom/knowledge docs`). `smoke.go` `checkHomebrew` skips when the version is not the latest release, as `checkStaleBucket` does for Scoop, so a rerun for an older version never `brew upgrade`s onto the newer formula.
**Goal:** a tap or bucket whose file pins a version newer than the one being published probes `done` with the detail `<file> pins newer <X.Y.Z>`, and `publish` issues no clone, commit or push to it.
**Approach (assumed at the header base):** in `pinnedVersionStep` (`cmd/release/probes.go`), parse `pinned` with `parseReleaseVersion` and compare it with `version` through `releaseVersion.compare`. Equal: done, as now. Newer: `stateDone`, detail `"%s pins newer %s"` (file, pinned). Lower, or unparseable: pending, as now. `probeHomebrew` and `probeScoop` keep calling it; converge runs only pending steps, so nothing else changes.
**Files:** cmd/release/probes.go; cmd/release/probes_test.go; cmd/release/channels_test.go; cmd/release/smoke.go; cmd/release/smoke_test.go; .capcom/knowledge/decisions/adr-0008/adr-0008-decision.md
**Read first:** cmd/release/probes.go — pinnedVersionStep, probeHomebrew, probeScoop; cmd/release/version.go — parseReleaseVersion, releaseVersion.compare; cmd/release/probes_test.go — TestProbesReportEachState, fakeRemote; cmd/release/channels_test.go — releasedFake, fakeChannels, runChannelPublish; cmd/release/smoke.go — checkHomebrew, checkStaleBucket
**Closes:** release-no-downgrade
**Tests:**
- `probes_test.go`: `TestProbesReportEachState` gains a formula and a manifest pinning `0.24.12` for `v0.24.11`, each `done` with `pins newer 0.24.12`.
- `channels_test.go`: new `TestPublishLeavesAChannelPinnedToANewerVersionAlone`: `releasedFake` with the tap and the bucket at `0.24.12` (remote and clone); no `gh repo clone`, `git commit` or `git push` is recorded, and the step table shows both rows `done` with `pins newer 0.24.12`.
- Both fail on the pre-item tree (bite check).
- `smoke_test.go`: `TestSmokeChecks` gains a case where Homebrew is skipped because the version is not the latest release.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'Probes|PinnedToANewer|Publish'`
- `go vet ./cmd/release/`
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'Smoke'`
- `capcom check`
**Commit:** `fix(release): never downgrade a tap or bucket pinned to a newer version`

## P1.41 Release: a draft release is pending until publish un-drafts it — ✅ DONE (2026-10-13)

NOTES (2026-10-13): re-derived from repo.ReleaseAssets returning a bare name slice: at the tree it returns ([]string, bool, error), so the new githubReleaseAssets{names, isDraft} struct replaces only the slice and the found flag stays its own return value.
NOTES (2026-10-13): consequential edit — .capcom/knowledge/decisions/adr-0008/adr-0008-decision.md: made necessary by probeRelease no longer reading a draft with all seven assets as done ("the release is published, not a draft, and has all seven assets").
NOTES (2026-10-13): publish's Long help text and describeRelease's dry-run line now say the release step publishes a draft; the memRepo publish-log comments name UndraftRelease (one wrapped comment reflowed).
NOTES (2026-10-13): the two new probe cases sit at the end of TestProbesReportEachState's table so gofmt does not realign the existing release/tap rows.

**What:** Fixes F-04 (`release-draft-probe`): a draft left by an interrupted `gh release create` probes `done`, and the channel steps then push download URLs that 404.
**Regression guard.** `publish_test.go`'s `mutatingCommands` gains `repoCallName + " " + methodUndraftRelease`, so the no-change tests fail when a run un-drafts a release it should leave alone.
**Goal:** `probeRelease` never reports a draft release `done`; `publish` over a draft uploads any missing assets, then un-drafts it exactly once, after which the release step probes `done`. Every `gh release` call names `--repo airiclenz/capcom` (ADR 0008).
**Approach (assumed at the header base):** `repo.ReleaseAssets` returns a struct `githubReleaseAssets{names []string; isDraft bool}` instead of the bare name slice. Producers: `execRepo.ReleaseAssets` (`--json assets,isDraft`) and `memRepo.ReleaseAssets` (new `drafts map[string]bool` field). Consumers: `probeRelease`, `convergeRelease`, the three `ReleaseAssets` cases in `repo_test.go`. New `repo` method `UndraftRelease(ctx, tag)`: `execRepo` runs `gh release edit <tag> --repo airiclenz/capcom --draft=false --latest` with `nonInteractiveEnv`; `memRepo` records `repo UndraftRelease <tag>` in its publish log, clears the draft and has a `methodUndraftRelease` failure constant. `probeRelease`: a draft is `pending`, detail naming `is a draft` (with the missing assets when any). `convergeRelease`: skip `UploadAssets` when nothing is missing, and call `UndraftRelease` after the upload when the release is a draft.
**Files:** cmd/release/repo.go; cmd/release/repo_test.go; cmd/release/memrepo_test.go; cmd/release/probes.go; cmd/release/probes_test.go; cmd/release/publish.go; cmd/release/publish_test.go
**Read first:** cmd/release/repo.go — repo.ReleaseAssets, execRepo.ReleaseAssets, execRepo.CreateRelease; cmd/release/memrepo_test.go — memRepo.ReleaseAssets, memRepo.UploadAssets, memPublish.note; cmd/release/probes.go — probeRelease, missingAssets; cmd/release/publish.go — convergeRelease, checkPublishedSums; cmd/release/publish_test.go — mutatingCommands, TestPublishPartialReleaseUploadsOnlyTheMissingAssets (a draft that already has SHA256SUMS needs assetContent[SHA256SUMS] = rebuiltSums, or checkPublishedSums refuses); cmd/release/repo_test.go — TestExecRepoRunsEachMethodsArgv ("ReleaseAssets keeps only the uploaded ones"), "ReleaseAssets release not found is no release"
**Closes:** release-draft-probe
**Tests:**
- `probes_test.go`: a draft with all seven assets reads `pending` naming the draft.
- `publish_test.go`: new `TestPublishUndraftsADraftReleaseOnce`: a draft with all assets gets exactly one `repo UndraftRelease v0.24.11`, no `CreateRelease` or `UploadAssets`, and the release row reads `done`; a draft missing two archives uploads them before the un-draft.
- `repo_test.go`: `TestExecRepoRunsEachMethodsArgv` pins the `UndraftRelease` argv and the `--json assets,isDraft` view.
- The probe and publish tests fail on the pre-item tree (bite check).
- `publish_test.go`: `mutatingCommands` lists `repo UndraftRelease`; `TestPublishChangesNothingWhenItHasNothingToDo` and the "nothing was changed" refusal tests stay green.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/`
- `go vet ./cmd/release/`
**Commit:** `fix(release): treat a draft GitHub release as pending and un-draft it`

## P1.42 Release: bind publish to the commit the gate judged — ✅ DONE (2026-10-13)

NOTES (2026-10-13): consequential edit — docs/releasing.md: made necessary by publish pushing the gated commit instead of `HEAD` (its `publish` bullet said it "pushes `HEAD` to `main`").
NOTES (2026-10-13): checkPushable skips the VERSION read when the gate could not resolve HEAD (gateReport.Head empty): the gate's "could not resolve HEAD" refusal already blocks the run and there is no commit to read at; the "VERSION at HEAD reads ..." refusal wording is unchanged (pinned by TestPublishRefusesBeforeChangingAnything).
NOTES (2026-10-13): memRepo.PushMain now records the pushed commit and resolves origin/main to it (not to memRepo.head); memPublish.pushed() returns (commit, bool); the HEAD-moved hook is memPublish.headAfterGate.
NOTES (2026-10-13): release-gate-binding is not listed under CLOSES: its part (a), the kept CI half, and the ADR 0008 amendment belong to P1.43.

**What:** Fixes F-12(b) (`release-gate-binding`): the gate judges a resolved SHA, but `publish` reads VERSION, checks ancestry, verifies the tag and pushes through the symbolic `HEAD`.
**Regression guard.** `checkGate` passes the resolved `head` to its `IsAncestor(ctx, remoteMain, …)` call (memRepo.resolve returns a commit id unchanged, so gate_test holds). `describeTag` keeps its `push HEAD to origin/main` wording, which `TestPublishChangesNothingWhenItHasNothingToDo` and `TestPublishDryRunWithABlockedGateFailsButPrintsThePlan` pin. The other HEAD-push prose (repo.go's two `PushMain` comments; publish.go's comment and error text around `checkPushable` and `convergeTag`) names the gated commit.
**Goal:** `publish` resolves HEAD once, in the gate; `VERSION`, the ancestry check, the tag's bump-commit check and the push all use that SHA; the push is `git push origin <sha>:refs/heads/main`; HEAD resolving to another commit before the push refuses the run with `HEAD moved from <short> to <short> after the gate cleared it: run publish again` and pushes nothing.
**Approach (assumed at the header base):** `gateReport` gains `Head string` tagged `json:"-"` (the status JSON keys stay as `TestStatusJSONKeysArePinned` pins them); `checkGate` sets it from its one `Resolve(HEAD)`. `publishRun` gains `head`, set by `checkPushable`, which reads `ShowFile(ctx, head, versionFile)`. `convergeTag` re-resolves `HEAD`, refuses on a mismatch, then `IsAncestor(head, remoteMain)` and `PushMain(ctx, head)`. `verifyTag` uses `LastCommitTouching(ctx, head, versionFile)`. `repo.PushMain` takes the commit: producers `execRepo.PushMain` (argv above), `memRepo.PushMain` (logs `repo PushMain <sha>`); consumer `convergeTag`. `memPublish` gains a hook that makes `Resolve("HEAD")` answer another commit after its first call. The publish verb's Long text says it pushes the gated commit, refusing if HEAD moved.
**Files:** cmd/release/gate.go; cmd/release/publish.go; cmd/release/repo.go; cmd/release/repo_test.go; cmd/release/memrepo_test.go; cmd/release/publish_test.go
**Read first:** cmd/release/gate.go — checkGate, gateReport; cmd/release/publish.go — runPublish, checkPushable, convergeTag, verifyTag, describeTag, newPublishCommand (Long); cmd/release/repo.go — repo.PushMain, execRepo.PushMain; cmd/release/memrepo_test.go — memRepo.Resolve, memRepo.resolve, memRepo.ShowFile, memRepo.PushMain, memPublish; cmd/release/publish_test.go — newPublishFake, TestPublishFirstRunConvergesTheTagThenTheRelease, TestPublishTagStepFailures; cmd/release/status_test.go — TestStatusJSONKeysArePinned, TestStatusReportsTheGateBaselineAndHighestTag (compares the JSON, so the field tagged `json:"-"` is safe); cmd/release/repo_test.go — TestExecRepoRunsEachMethodsArgv ("PushMain")
**Tests:**
- `publish_test.go`: new `TestPublishPushesTheGatedCommit` (the log holds `repo PushMain <bumpCommit>`) and `TestPublishRefusesWhenHeadMovesAfterTheGate` (error with the exact text above, no `repo PushMain`, the step table printed).
- `repo_test.go`: the `PushMain` argv case becomes `git push origin <sha>:refs/heads/main`.
- Both publish tests fail on the pre-item tree (bite check).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'Publish|Gate|ExecRepo|Status'`
- `go vet ./cmd/release/`
**Commit:** `fix(release): push the commit the gate judged, and refuse when HEAD moved`

## P1.43 Release: keep the gate's CI half once the tag exists — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the CI half runs whenever the tag is done and the release step is not done (pending, and also unknown or setup-pending), not only when it is pending; an unknown release already refuses the run, and the extra check only adds refusals.
NOTES (2026-10-13): the existing `not checked` gate row now reads `not checked: <version> is on origin and released, so the rest converges from remote state`, since it shows only once the release is done too; TestPublishChangesNothingWhenItHasNothingToDo's finished-release case now asserts it.
NOTES (2026-10-13): checkTagCI resolves the tag after FetchMain, which already fetches origin's tags (`git fetch origin main --tags`), so no separate FetchTag (a mutating call in the tests) is made.
NOTES (2026-10-13): the gate-row and refusal formatting checkPushable did inline moved into helpers gateRow and gateRefusals, shared with the new CI-only branch (runPublish now calls publishRun.checkGateFor, which finds the tag and release steps by name through stepStateOf).

**What:** Fixes F-12(a) (`release-gate-binding`): once the version's tag is on origin, `publish` skips the gate entirely, so a release can be built from a tag whose commit never passed CI.
**Regression guard.** Fix `publishFake.tagged()` once so it sets `tagCommits` (the tag's commit with a green CI run), so every pending-release test, including those P1.41 and P1.44 add, keeps passing. P1.43 rewrites the adr-0008-decision "publish converges" paragraph as P1.40 left it, not the base wording. The prose grep widens to `grep -rn -i 'HEAD.\{0,3\}to.\{0,3\}\(origin/\)\?main\|HEAD:main\|pushes .HEAD\|converges from remote state\|no bypass\|gate is blocked or VERSION' docs .agents .capcom/knowledge cmd/release`, leaving out channels.go's channel push (`git push --quiet origin HEAD:main`), which stays.
**Goal:** when the tag step is `done` and the release step is `pending`, `publish` refuses unless `ci.yml` has a successful run for the tag's commit or a peeled bump `origin/main` contains; the clean tree, HEAD containing `origin/main` and VERSION at HEAD are not consulted. With the release `done` too, the gate is not consulted. The ADR 0008 Note and the release docs say so.
**Approach (assumed at the header base):** binding call: keep the CI half of the gate when the tag exists and the release is pending. `gate.go` gains `checkTagCI(ctx, checkout, tag) gateReport`: `FetchMain`, `Resolve(tag)`, `peelBumps` from that commit, `checkGreenRun`; failures named as `checkGate` names them. `runPublish` finds the tag and release steps by name (not by index): tag pending → `checkPushable` as now; tag done and release pending → `checkTagCI`, gate row `CI only, the tag is on origin: cleared at <short>` or `... BLOCKED at <short>`, each failure a `gate:` refusal; both done → the existing `not checked` row. The verb's Long text says so.
Prose guard: every prose site saying when `publish` runs the gate, or that it pushes `HEAD`, matches this item and P1.42; find them with `grep -rn 'converges from remote state\|no bypass\|pushes .HEAD\|HEAD. to .main\|gate is blocked or VERSION' docs .agents .capcom/knowledge cmd/release`. Edit `adr-0008-decision` in place with `capcom know update adr-0008-decision --body-replace-old … --body-replace-new …` (the "publish converges" and "The gate is green CI" paragraphs: CI half once the tag exists, SHA binding, `<sha>:refs/heads/main`, HEAD re-check), and the `adr-0008` Note's status line to `Accepted; amended 2026-10-12 and 2026-10-13.`
Depends on P1.42.
**Files:** cmd/release/gate.go; cmd/release/publish.go; cmd/release/publish_test.go; docs/releasing.md; .agents/skills/release-capcom/SKILL.md; .capcom/knowledge/decisions/adr-0008/adr-0008-decision.md; .capcom/knowledge/decisions/adr-0008.md
**Read first:** cmd/release/publish.go — runPublish, checkPushable, printPublish, newPublishCommand (Long); cmd/release/gate.go — checkGate, peelBumps, checkGreenRun; cmd/release/memrepo_test.go — memRepo.Resolve, memRepo.resolve, memRepo.FetchMain (does nothing); cmd/release/publish_test.go — publishFake.tagged, TestPublishConvergesFromRemoteStateOnceTheTagExists, TestPublishPartialReleaseUploadsOnlyTheMissingAssets; cmd/release/gate_test.go — linearHistory; .capcom/knowledge/decisions/adr-0008/adr-0008-decision.md — "publish converges", "The gate is green CI"; docs/releasing.md — The gate, The verbs
**Closes:** release-gate-binding
**Tests:**
- `publish_test.go`: new `TestPublishRefusesARedCIOnTheTagCommitWhileTheReleaseIsPending` (tag on origin at the bump, no green run: refused naming `ci.yml`, no `repo CloneAt`, gate row `CI only`). `TestPublishConvergesFromRemoteStateOnceTheTagExists` keeps the dirty tree and the later VERSION, gains a green run for the tag commit's gate commit, and its comment says the CI half is checked.
- The new test fails on the pre-item tree (bite check).
- `publishFake.tagged()` also sets `f.repo.tagCommits`; `TestPublishPartialReleaseUploadsOnlyTheMissingAssets`, `TestPublishRefusesARebuildThatDoesNotMatchThePublishedSums` and the P1.41/P1.44 tests keep passing.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'Publish|Gate|Status'`
- `go vet ./cmd/release/`
- `capcom check`
**Commit:** `fix(release): require green CI on the tag commit while the release is pending`

## P1.44 Release: never upload SHA256SUMS that the uploaded archives do not match — ✅ DONE (2026-10-13)

NOTES (2026-10-13): checkPublishedSums's way out now lists SHA256SUMS plus only the mismatched archives the release already carries, comma-separated (`(SHA256SUMS, <name>, ...)` instead of `(SHA256SUMS and <names>)`), so a mismatched archive the release lacks is named as mismatched but never as an asset to delete.
NOTES (2026-10-13): the rebuilt-SHA256SUMS read both checks share moved into a helper readRebuiltSums; the tests gained helpers rebuiltAssetContent (every rebuilt asset's bytes, beside rebuiltSums) and downloadedAssets (the names DownloadAsset fetched).
NOTES (2026-10-13): uploaded archives download into `<buildDir>/uploaded`, removed with the build directory when the run ends.

**What:** Fixes F-13 (`release-sums-check`): a release left holding archives but no SHA256SUMS gets the rebuild's sums uploaded unchecked, and archives uploaded beside an existing SHA256SUMS are never checked against it.
**Regression guard.** `checkUploadedArchives` words its own refusal: the uploaded archive does not match the rebuilt one, and the way out names only the archives (`gh release delete-asset <tag> <name>` for <names>); it does not reuse `checkPublishedSums`'s SHA256SUMS wording, which would name an asset the release lacks.
**Goal:** on a resumed release, `publish` uploads nothing unless every archive already on the release hashes to the rebuild's value and every archive about to be uploaded matches the release's SHA256SUMS when it has one; a mismatch refuses naming each archive and the `gh release delete-asset <tag> <name>` way out.
**Approach (assumed at the header base):** in `convergeRelease`: with SHA256SUMS uploaded, `checkPublishedSums` checks every archive of `releaseAssetNames` (uploaded or missing), not only the uploaded ones. With SHA256SUMS absent and archives uploaded, a new `checkUploadedArchives` downloads each uploaded archive with `repo.DownloadAsset` into a directory under `run.buildDir`, hashes it with `fileSHA256` and compares it with `checksumFor(rebuilt sums, name)`; a mismatch refuses with the same way-out wording `checkPublishedSums` uses. No `go.mod` toolchain line.
Depends on P1.41.
**Files:** cmd/release/publish.go; cmd/release/publish_test.go
**Read first:** cmd/release/publish.go — convergeRelease, checkPublishedSums, buildReleaseAssets, publishRun.buildDir; cmd/release/dist.go — fileSHA256; cmd/release/render.go — checksumFor; cmd/release/memrepo_test.go — memRepo.DownloadAsset (writes into an existing dir, so MkdirAll it first); cmd/release/publish_test.go — rebuiltSums, TestPublishPartialReleaseUploadsOnlyTheMissingAssets, TestPublishRefusesARebuildThatDoesNotMatchThePublishedSums; cmd/release/dist_test.go — runTestDist
**Closes:** release-sums-check
**Tests:**
- `publish_test.go`: new `TestPublishRefusesUploadedArchivesTheRebuildDoesNotMatch` (two archives uploaded, no sums, one archive's content differs: refused naming it, no `repo UploadAssets`) and `TestPublishRefusesAMissingArchiveThePublishedSumsDoNotMatch`. The `SHA256SUMS missing` case of `TestPublishPartialReleaseUploadsOnlyTheMissingAssets` holds the rebuilt archives' bytes in `assetContent` (a helper beside `rebuiltSums`) and expects one `DownloadAsset` per archive.
- The two new tests fail on the pre-item tree (bite check).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run Publish`
- `go vet ./cmd/release/`
**Commit:** `fix(release): check uploaded archives before uploading SHA256SUMS beside them`

## P1.45 Release: bare X.Y.Z everywhere, and the tagger fails loudly — ✅ DONE (2026-10-13)

NOTES (2026-10-13): TestVersionBumpExitsNonZeroOnALeadingV also commits a valid bump (0.5.1) before the `v0.6.0` one, pinning that the valid bump is still listed in `bumps` when the script exits 1.
NOTES (2026-10-13): channels_test.go's TestPublishWingetConvergesFromRemoteStateOnceTheTagExists still sets VERSION to "v0.24.12" as a moved-on value publish never reads once the tag exists; left untouched since it passes and is out of this item's Files.

**What:** Recast at the regression check (2026-10-13). Fixes F-15 (`tagger-silent-pass`): the tagger misses a bump buried under later commits when `before` is unusable, and refuses a `v`-prefixed VERSION while exiting 0, which `publish` and `dist` accept.
**Regression guard.** Files adds .github/workflows/tag-on-version-bump.yml; the tag step becomes `if: ${{ !cancelled() && steps.detect.outputs.bumps != '' }}` so valid bumps are still tagged while the job goes red. Tests: dist_test.go `testVersion` becomes `"1.2.3\n"` (and its comment); the new leading-v refusal test uses `testSourceTreeAt(t, "v1.2.3\n")`.
**Goal:** `version-bump.sh` with an unusable `before` scans from the newest release tag reachable from `after` (else `after^`), so a buried bump is tagged; after writing `bumps` it exits 1 when it refused any commit. `checkPushable` and `dist` refuse a VERSION with a leading `v`, naming the VERSION file.
**Approach (assumed at the header base):** binding call: bare `X.Y.Z` everywhere; the tagger exits non-zero on refusal; the fallback range starts at the newest reachable release tag. In the script, the fallback becomes `before="$(git describe --tags --abbrev=0 --match 'v[0-9]*' "$after" 2>/dev/null || git rev-parse --verify --quiet "${after}^" || true)"`, its comment updated; the loop records a refusal, and the script exits 1 after the `bumps=` line. `checkPushable` refuses `VERSION reads "vX.Y.Z": VERSION holds a bare X.Y.Z, with no leading v` (its "tolerated" comment goes). `readBareVersion` (`dist.go`; consumers `runDist`, `buildReleaseAssets`) refuses `<path>:1: "vX.Y.Z" has a leading v; VERSION holds a bare X.Y.Z`. `prepare` already writes bare.
Depends on P1.42.
**Files:** .github/scripts/version-bump.sh; cmd/release/versionbump_test.go; cmd/release/publish.go; cmd/release/publish_test.go; cmd/release/dist.go; cmd/release/dist_test.go; .github/workflows/tag-on-version-bump.yml
**Read first:** .github/scripts/version-bump.sh — before fallback, bumps loop, read_version; .github/workflows/tag-on-version-bump.yml — detect step, Create the annotated tags; cmd/release/versionbump_test.go — runVersionBump, newBumpRepo, TestVersionBumpRefusesAVersionThatIsNotBareXYZ, TestVersionBumpFallsBackToTheParentOfAfterForAZeroBefore; cmd/release/publish.go — checkPushable, buildReleaseAssets, runPublish; cmd/release/dist.go — readBareVersion, runDist; cmd/release/dist_test.go — testVersion, testSourceTreeAt, TestDistNamesArchivesAfterTheBareVersion; cmd/release/publish_test.go — TestPublishAcceptsAVersionFileWithALeadingV, newPublishFake; cmd/release/smoke_test.go — writeReleaseTree
**Closes:** tagger-silent-pass
**Tests:**
- `versionbump_test.go`: a runner variant that returns the exit status; new `TestVersionBumpFallsBackToTheNewestReleaseTag` (`before` absent from the clone, a tag on an older commit, the bump under a later commit: the bump is listed) and `TestVersionBumpExitsNonZeroOnALeadingV` (VERSION `v0.6.0`). `TestVersionBumpRefusesAVersionThatIsNotBareXYZ` expects the non-zero exit. Real-git, skipped on Windows (ADR 0008 bumptest amendment).
- `publish_test.go`: `TestPublishAcceptsAVersionFileWithALeadingV` becomes `TestPublishRefusesAVersionFileWithALeadingV`. `dist_test.go`: new `TestDistRefusesAVersionWithALeadingV`.
- All fail on the pre-item tree (bite check).
- `dist_test.go`: `testVersion` becomes `"1.2.3\n"` (and its comment); `TestDistRefusesAVersionWithALeadingV` uses `testSourceTreeAt(t, "v1.2.3\n")`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'VersionBump|Dist|Publish'`
- `go vet ./cmd/release/`
**Commit:** `fix(release): require a bare VERSION and fail the tagger on a refusal`

## P2.1 Split tree/tree.go into errors, ids, walk and claims — ✅ DONE (2026-10-13)

NOTES (2026-10-13): placement of unnamed symbols by the only-user rule — maxIDLength and idPattern/aliasPattern to ids.go; readDir to walk.go; idClaim/aliasClaim, DuplicateIDError, DuplicateIDDirectoryError, droppedNode, repeatedID, repeatedIDDir, repeatedIDs, enclosingDir and addRepeatedIDDirs to claims.go; nodeExtension, hiddenPrefix, rootParentID, LeftOut and the left-out reasons const block (used by walk.go and claims.go) stay in tree.go.

**What:** Pure move per review P2-01. `internal/tree/tree.go` is split into four files, and the package gains a `doc.go` file map, pinned by a structural test.
**Goal:** `internal/tree` holds `errors.go`, `ids.go`, `walk.go`, `claims.go` and `doc.go` beside `tree.go`, which keeps `Tree`, `Load`, the accessors and `PathFor`. `doc.go` maps every non-test file, and `TestDocFileMap_MatchesDirectory` passes. Every existing test passes unchanged.
**Approach (assumed at the header base):**
- `errors.go`: the sentinel `var` block, plus `Error`, `Errors`, `invalidIDDirectoryError` and `repeatedAliasError`, each with its methods.
- `ids.go`: `ValidateID`, `ValidateAlias`, `CheckAliases` and `checkAlias`, plus `RenameAliases` from `move.go`.
- `walk.go`: `Walk`, `treeWalker` with its `walk`, `leftOutFile` and `leftOutFileOf`.
- `claims.go`: `indexIDs`, `indexAliases`, `indexAlias`, `checkDroppedAliases`, `claimedAlias`, `newDuplicateAliasError` and `recordDuplicate`.
- `tree.go` keeps `AliasFunc`, `Node`, `Tree`, `Load`, `Dir`, `Nodes`, `ChildrenOf` and `PathFor`.
- Placement rule for every const, var and helper not named here, including code that Phase 0/1 added: it goes to the file of its only user. One used by two or more of these files stays in `tree.go`.
- `doc.go` takes the package comment from `tree.go` and adds a `# Files` map in the form of `internal/cli/doc.go`. `doc_test.go` is `internal/cli/doc_test.go` copied under `package tree`.
- Only moves: no renames and no logic edits.
Depends on P1.8; P1.28.
**Files:** internal/tree/tree.go; internal/tree/move.go; internal/tree/errors.go; internal/tree/ids.go; internal/tree/walk.go; internal/tree/claims.go; internal/tree/doc.go; internal/tree/doc_test.go
**Read first:** internal/tree/tree.go — Load, Walk, treeWalker.walk, leftOutFileOf, indexIDs, checkDroppedAliases, recordDuplicate; internal/tree/move.go — RenameAliases; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory, mappedFilePattern; internal/cli/doc.go — the # Files map; internal/tree/resolve.go — Resolve (reads duplicates); internal/tree/atomic.go — tempPattern (second user of hiddenPrefix, so it stays in tree.go)
**Closes:** split-tree-go
**Tests:** add `internal/tree/doc_test.go` (`TestDocFileMap_MatchesDirectory`). Existing tests stay unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers and the new `doc.go`/`doc_test.go`. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(tree): split tree.go into errors, ids, walk and claims`

## P2.2 nodefile package, part 1: move the guarded reads out of tree — ✅ DONE (2026-10-13)

NOTES (2026-10-13): tree/nodefile.go forwards both exported readers, ReadRegularFile and ReadNodeFile (the plan names ReadNodeFile "and the same for each other exported function"); tree keeps its fifo_*_test.go files because tree_test.go's walk tests still call makeFIFO, so nodefile gets copies.

**What:** Recast at the regression check (2026-10-13). Part 1 of review P2-03 (`nodefile-package`). `ReadNodeFile`, the guarded reader P0.3 and the sentinels `ErrNotRegular`/`ErrTooLarge` move from `internal/tree` to a new leaf package, `internal/nodefile`. Until P2.9, `tree` forwards the moved functions so that callers compile unchanged.
**Regression guard.** Also moves P0.3's unix/other open files (`openNoFollow`), `read_unix_test.go` and `SetBeforeOpen` from export_test.go into internal/nodefile; P2.9's tree deletion rests on that. The move-only check also allows the `nodefile.` qualifiers in the moved tests.
**Goal:** `internal/nodefile` holds the code of `read.go` and its tests, and imports nothing from `internal/`. `tree.ErrNotRegular` and `tree.ErrTooLarge` hold the same error values as `nodefile.ErrNotRegular` and `nodefile.ErrTooLarge`, so every `errors.Is` site still matches. Every package outside `tree` compiles unchanged.
**Approach (assumed at the header base):**
- `git mv` `read.go` and `read_test.go` to `internal/nodefile/` (`package nodefile`, with tests in `package nodefile_test`). This includes the reader P0.3, and any test that calls only moved code moves too.
- `ErrNotRegular` and `ErrTooLarge` move, with their comments, to `nodefile/read.go`. Tree's sentinel block in `errors.go` declares `ErrNotRegular = nodefile.ErrNotRegular`, and the same for `ErrTooLarge`. This re-export is permanent.
- Every unexported name of `read.go` that tree still uses is capitalised and not otherwise renamed. At the base those are `isNotRegular` → `IsNotRegular` and `maxNodeFileSize` → `MaxNodeFileSize`, called from `walk.go`.
- New `internal/tree/nodefile.go` holds temporary forwarders: `var ReadNodeFile = nodefile.ReadNodeFile`, and the same for each other exported function moved here. P2.9 deletes the file.
- The nodefile test files get copies of the helpers the moved tests need: `makeFIFO` per OS from `tree/fifo_*_test.go`, plus `makeSymlink` and `writeSizedFile` from `tree_test.go`, in `helpers_test.go`.
- `nodefile/doc.go` holds only the package comment, because the package stays under 10 files. Update `tree/doc.go`'s map.
Depends on P2.1; P0.3.
**Files:** internal/tree/read.go → internal/nodefile/read.go; internal/tree/read_test.go → internal/nodefile/read_test.go; internal/nodefile/doc.go; internal/nodefile/fifo_unix_test.go; internal/nodefile/fifo_windows_test.go; internal/nodefile/helpers_test.go; internal/tree/nodefile.go; internal/tree/errors.go; internal/tree/walk.go; internal/tree/doc.go; internal/tree/open_unix.go → internal/nodefile/open_unix.go; internal/tree/open_other.go → internal/nodefile/open_other.go; internal/tree/read_unix_test.go → internal/nodefile/read_unix_test.go; internal/tree/export_test.go; internal/nodefile/export_test.go
**Read first:** internal/tree/read.go — ReadNodeFile, checkReadable, isNotRegular, maxNodeFileSize, readOp, nonRegularModes; internal/tree/walk.go (P2.1) — treeWalker.walk, leftOutFileOf; internal/tree/errors.go (P2.1) — sentinel var block (ErrNotRegular, ErrTooLarge); internal/tree/remove.go — removeFileStep; internal/tree/export_test.go — SetLink; internal/tree/read_test.go — TestReadNodeFile_Refused_SentinelInPathError; internal/tree/tree_test.go — makeSymlink, writeSizedFile; internal/tree/fifo_unix_test.go — makeFIFO
**Tests:** moved tests only.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodefile/`
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `go list -f '{{join .Imports "\n"}}' ./internal/nodefile/ | grep internal/` prints nothing
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers, the capitalised names, the re-exported sentinels, the forwarders and the doc maps. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(tree): move the guarded node-file reads to internal/nodefile`

## P2.3 nodefile package, part 2: move the atomic writes out of tree — ✅ DONE (2026-10-13)

NOTES (2026-10-13): nodefile's export_test.go is invisible to tree's tests, so the regression guard's "nodefile-side test hook" is two new exports in nodefile/atomic.go: `Link` (the hard link through the moved `link` seam, which Plan.moveFile now calls) and `SwapLink(fn) (restore func())`; tree.SetLink in tree/export_test.go stays (move_test.go compiles unchanged) and swaps nodefile's seam through `t.Cleanup(nodefile.SwapLink(fn))`; nodefile/export_test.go gets its own SetLink for the moved tests.
NOTES (2026-10-13): `joinNonNil` moved with atomic.go, and Plan.moveFile still needs it, so tree/plan.go keeps an unexported copy; the `removeFile` seam and tree.SetRemoveFile stay in tree (code with a *Plan receiver stays in plan.go).
NOTES (2026-10-13): `refuseHardLinks` moved with atomic_test.go and is copied into tree/move_test.go for TestMoveApply_NoHardLinks_RenamesThroughTheFallback; `writeNodes` is copied into nodefile/helpers_test.go; `failingReader`, `errReaderFailed` and `dirEntries` moved with the TestWriteFileAtomic_* tests, as tree has no other user; tree_test.go drops its now unused `io` import.
NOTES (2026-10-13): plan.go and remove.go also call `nodefile.ReadNodeFile` directly instead of the P2.2 forwarder, so tree's own files name no forwarder (the Goal's "Tree's own files call nodefile. directly").
NOTES (2026-10-13): consequential edit — internal/tree/tree.go: made necessary by moving WriteFileAtomic (hiddenPrefix's comment names it, now `nodefile.WriteFileAtomic`).
NOTES (2026-10-13): consequential edit — internal/nodefile/doc.go: made necessary by moving the atomic writes into nodefile (the package comment described the reader only).

**What:** Part 2 of review P2-03. The atomic writes, `lfWriter`, `syncDir`, the refuse-missing replace `ReplaceFileAtomic` (P1.22) and the no-replace rename P1.24 move from `internal/tree` to `internal/nodefile`.
**Regression guard.** P1.24's no-replace rename (tree/plan.go) uses the `link` seam that P2.3 moves to nodefile; P2.3 makes tree's Plan call the nodefile primitive and tree's move tests reach the seam through nodefile's `SetLink` (export_test re-export or a nodefile-side test hook), so tree/move_test.go keeps compiling; list tree/move_test.go in Files if it changes. The refuse-missing replace is P1.22's `ReplaceFileAtomic` (atomic.go, with its `TestReplaceFileAtomic_*` tests); it gets a forwarder, and the item depends on P1.22.
**Goal:** `internal/nodefile` holds the code of `atomic.go`, `syncdir_other.go` and `syncdir_windows.go`, together with their tests and the `SetLink` seam. Tree's own files call `nodefile.` directly. Every package outside `tree` compiles unchanged.
**Approach (assumed at the header base):**
- `git mv` the three files and `atomic_test.go` to `internal/nodefile/`, keeping the build tags. The primitives P1.23 and P1.24 move wherever those items put them. Code with a `*Plan` or `step` receiver stays in `tree/plan.go`. Any test that calls only moved code moves too, including `TestWriteFileAtomic_*` and `failingReader` from `tree_test.go`.
- `nodefile/atomic.go` declares its own unexported `hiddenPrefix = "."` for `tempPattern`. Tree keeps its own copy for `Walk`.
- `SetLink`, and any seam that swaps a moved package variable, moves from `tree/export_test.go` to `nodefile/export_test.go`.
- `tree/nodefile.go` gains forwarders for `WriteFileAtomic`, `WriteFileAtomicVerbatim`, `ReplaceFileVerbatim`, `ReplaceFileAtomic`, `WriteFileExclusive` and every other exported moved function.
- Every call that tree's own files make to a forwarded name becomes `nodefile.<Name>`, comments included. At the base those files are `plan.go` and `remove.go`.
- `nodefile/helpers_test.go` gains the helpers the moved tests need (`writeNodes`, `dirEntries`). Update `tree/doc.go`'s map.
Depends on P2.2; P1.21; P1.22; P1.23; P1.24.
**Files:** internal/tree/atomic.go → internal/nodefile/atomic.go; internal/tree/syncdir_other.go → internal/nodefile/syncdir_other.go; internal/tree/syncdir_windows.go → internal/nodefile/syncdir_windows.go; internal/tree/atomic_test.go → internal/nodefile/atomic_test.go; internal/tree/tree_test.go; internal/tree/export_test.go; internal/nodefile/export_test.go; internal/nodefile/helpers_test.go; internal/tree/nodefile.go; internal/tree/plan.go; internal/tree/remove.go; internal/tree/doc.go; internal/tree/move_test.go
**Read first:** internal/tree/atomic.go — link, placeExclusive, WriteFileExclusive, writeFileAtomic, writeTemp, tempPattern; internal/tree/plan.go — AddRewrite, renameStep, doRename; internal/tree/export_test.go — SetLink, SetMoveRename; internal/tree/atomic_test.go — refuseHardLinks; internal/tree/tree_test.go — failingReader, errReaderFailed, dirEntries, TestWriteFileAtomic_ReaderFails_OldFileIntactNoTempFile; internal/tree/remove.go — removeFileStep; internal/tree/syncdir_other.go — syncDir
**Tests:** moved tests only. Tests that call `SetLink` stay non-parallel.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodefile/`
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `go list -f '{{join .Imports "\n"}}' ./internal/nodefile/ | grep internal/` prints nothing
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers, the `nodefile.` qualifiers, `hiddenPrefix`, the forwarders and the doc map. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(tree): move the atomic node-file writes to internal/nodefile`

## P2.4 nodefile package, part 3: move edit.ETag — ✅ DONE (2026-10-13)

NOTES (2026-10-13): consequential edit — internal/nodefile/doc.go: made necessary by moving ETag into nodefile (the package comment listed only the reader and the writes).

**What:** Part 3 of review P2-03. `edit.ETag` moves to `internal/nodefile`. Until P2.9, `edit` forwards it.
**Goal:** `nodefile.ETag` returns the same etags and is pinned by the moved `TestETag_HashesContent`. `internal/nodefile` still imports nothing from `internal/`. Every caller compiles unchanged.
**Approach (assumed at the header base):** move `ETag`, with its comment, from `internal/edit/edit.go` to `internal/nodefile/etag.go`. Move `TestETag_HashesContent` from `edit_test.go` to `nodefile/etag_test.go`. `edit.go` keeps the temporary forwarder `var ETag = nodefile.ETag`, which P2.9 deletes.
Depends on P2.3.
**Files:** internal/edit/edit.go; internal/edit/edit_test.go; internal/nodefile/etag.go; internal/nodefile/etag_test.go
**Read first:** internal/edit/edit.go — ETag (hash/fnv import leaves with it); internal/edit/edit_test.go — TestETag_HashesContent (regexp/slices imports); internal/nodestore/create.go — place (edit.ETag, writeFile := tree.WriteFileAtomic func value)
**Tests:** moved test only.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodefile/`
- `GOMEMLIMIT=2GiB go test ./internal/edit/`
- `go list -f '{{join .Imports "\n"}}' ./internal/nodefile/ | grep internal/` prints nothing
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers and the forwarder. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(edit): move ETag to internal/nodefile`

## P2.5 nodefile package, part 4: lock, store, agents, check and index import nodefile — ✅ DONE (2026-10-13)

NOTES (2026-10-13): internal/index/index_test.go is unchanged: P1.4's readNodeFile seam is the package variable in index/refresh.go (now `= nodefile.ReadNodeFile`), and the test only swaps that variable, naming no forwarder.
NOTES (2026-10-13): comments that gained the longer `nodefile.` qualifier were not rewrapped, to keep the diff to qualifiers and import lines; store.go's Open comment now runs to 106 columns.

**What:** Part 4 of review P2-03. The `lock`, `store`, `agents`, `check` and `index` packages call the moved functions through `nodefile`.
**Regression guard.** The Goal is checked by an Acceptance grep. P1.6's `tree.ReadNodeFile` call in index/walk.go and P1.4's `readNodeFile` seam in index_test.go switch too.
**Goal:** none of these packages names a forwarder from `tree/nodefile.go` or `edit.ETag`. `lock` and `store` no longer import `internal/tree`.
**Approach (assumed at the header base):** in every file of these packages that names a forwarded function, the reference becomes `nodefile.<Name>`, in code and comments alike. At the base those files are `lock/lock.go`, `store/store.go`, `agents/block.go`, `check/check.go` and `index/refresh.go`, and the P0.4, P0.5 and P0.7 call sites are in the same files. Sentinel references stay as `tree.Err…` in files that still import `tree`. `lock` and `store` move every tree reference to `nodefile`, sentinels included, and drop the import. If a matching call site sits in a file this list does not name, switch it too and leave a NOTES line.
Depends on P2.4; P0.4; P0.5; P0.7.
**Files:** internal/lock/lock.go; internal/store/store.go; internal/agents/block.go; internal/check/check.go; internal/index/refresh.go; internal/index/walk.go; internal/index/index_test.go
**Read first:** internal/lock/lock.go — the record write (tree.WriteFileAtomic); internal/store/store.go — RaiseFormat (WriteFileAtomicVerbatim, ReplaceFileVerbatim); internal/agents/block.go — TopNotes, reconcile; internal/check/check.go — readFile, the problem-code switch; internal/index/refresh.go — indexFile, readRefusal; internal/index/walk.go — walkIssues, walkWarningKind
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/lock/`
- `GOMEMLIMIT=2GiB go test ./internal/store/`
- `GOMEMLIMIT=2GiB go test ./internal/agents/`
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `go list -f '{{join .Imports "\n"}}' ./internal/lock/ ./internal/store/ | grep internal/tree` prints nothing
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes nothing beyond the qualifiers and the import lines. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
- `grep -rnE '(tree|edit)\.(ReadNodeFile|ReadRegularFile|WriteFileAtomic\w*|ReplaceFile\w*|WriteFileExclusive|ETag)\b' internal/lock internal/store internal/agents internal/check internal/index` prints nothing
**Commit:** `refactor(store): import the node-file IO from internal/nodefile in lock, store, agents, check and index`

## P2.6 nodefile package, part 5: issue and knowledge import nodefile — ✅ DONE (2026-10-13)

NOTES (2026-10-13): comments that gained the longer `nodefile.` qualifier were not rewrapped, to keep the diff to qualifiers and import lines (as in P2.5); four comment lines in issue/move.go, issue/remove.go and knowledge/remove.go now run to 101-102 columns.
NOTES (2026-10-13): issue/remove.go and issue/remove_test.go drop the `edit` import (ETag was their only `edit` use); move_test.go in both packages had comment-only sites and gains no import.

**What:** Part 5 of review P2-03. The `issue` and `knowledge` packages, their tests included, call the moved functions through `nodefile`.
**Regression guard.** The Goal (code and comments) is checked by an Acceptance grep; comment-only sites compile either way.
**Goal:** no file of `internal/issue` or `internal/knowledge` names a forwarder from `tree/nodefile.go` or `edit.ETag`, in code or in comments.
**Approach (assumed at the header base):** the same switch as P2.5. At the base the files are `issue/move.go`, `issue/move_test.go`, `issue/remove.go` and `issue/remove_test.go`, plus `knowledge/move.go`, `knowledge/move_test.go` and `knowledge/remove.go`. `knowledge/verify.go` is switched if the P0.8 reader call there is a forwarded name. Sentinel references stay as `tree.Err…`. `edit` stays imported wherever other `edit` functions are still used. If a matching call site sits in a file this list does not name, switch it too and leave a NOTES line.
Depends on P2.5; P0.8.
**Files:** internal/issue/move.go; internal/issue/move_test.go; internal/issue/remove.go; internal/issue/remove_test.go; internal/knowledge/move.go; internal/knowledge/move_test.go; internal/knowledge/remove.go; internal/knowledge/verify.go
**Read first:** internal/issue/remove.go — PlanRemove, planDrops, isRefusedRead, RemoveOptions.IfMatch; internal/issue/move.go — planRewrite, Move.Unread; internal/knowledge/move.go — PlanMove, planSuccessorRewrites; internal/knowledge/remove.go — PlanRemove, planDrops, isRefusedRead; internal/knowledge/verify.go — checkSymbol; internal/issue/remove_test.go — ETagMismatchError check; internal/issue/move_test.go — unreadSwaps
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/issue/`
- `GOMEMLIMIT=2GiB go test ./internal/knowledge/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes nothing beyond the qualifiers and the import lines. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
- `grep -rnE '\b(tree|edit)\.(ReadNodeFile|ReadRegularFile|WriteFileAtomic|WriteFileAtomicVerbatim|ReplaceFileAtomic|ReplaceFileVerbatim|WriteFileExclusive|ETag)\b' --include=*.go internal/issue internal/knowledge` prints nothing
**Commit:** `refactor(issue): import the node-file IO from internal/nodefile in issue and knowledge`

## P2.7 nodefile package, part 6: nodestore imports nodefile — ✅ DONE (2026-10-13)

NOTES (2026-10-13): lines that gained the longer `nodefile.` qualifier were not rewrapped, to keep the diff to qualifiers and import lines (as in P2.5 and P2.6); four comment lines in nodestore/edit.go and nodestore/kind.go and one code line in nodestore/move.go now run to 101-104 columns.
NOTES (2026-10-13): edit.go drops both the `edit` and `tree` imports and format.go drops `tree` (their only uses were the forwarded names; format.go keeps a comment-only `*tree.Error` mention); create.go, move.go and remove.go drop `edit` (ETag was its only use).

**What:** Part 6 of review P2-03. The non-test files of `nodestore` call the moved functions through `nodefile`.
**Regression guard.** The Goal (code and comments) is checked by an Acceptance grep; comment-only sites compile either way.
**Goal:** no non-test file of `internal/nodestore` names a forwarder from `tree/nodefile.go` or `edit.ETag`, in code or in comments.
**Approach (assumed at the header base):** the same switch as P2.5. At the base the files are `create.go`, `edit.go`, `format.go`, `kind.go`, `move.go` and `remove.go`. Sentinel references stay as `tree.Err…`.
Depends on P2.6.
**Files:** internal/nodestore/create.go; internal/nodestore/edit.go; internal/nodestore/format.go; internal/nodestore/kind.go; internal/nodestore/move.go; internal/nodestore/remove.go
**Read first:** internal/nodestore/create.go — place, writeMode; internal/nodestore/edit.go — Edit, recheck; internal/nodestore/kind.go — ReadAliases, RecoverAliases, isRefusedRead; internal/nodestore/format.go — Format; internal/nodestore/move.go — Move; internal/nodestore/remove.go — Remove
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes nothing beyond the qualifiers and the import lines. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
- `grep -rnE '\b(tree|edit)\.(ReadNodeFile|ReadRegularFile|WriteFileAtomic|WriteFileAtomicVerbatim|ReplaceFileAtomic|ReplaceFileVerbatim|WriteFileExclusive|ETag)\b' --include=*.go --exclude='*_test.go' internal/nodestore` prints nothing
**Commit:** `refactor(nodestore): import the node-file IO from internal/nodefile`

## P2.8 nodefile package, part 7: cli imports nodefile — ✅ DONE (2026-10-13)

NOTES (2026-10-13): lines that gained the longer `nodefile.` qualifier were not rewrapped, to keep the diff to qualifiers and import lines (as in P2.5 to P2.7); two comment lines (init.go, update.go) and three code lines (init.go) now run to 103-110 columns.
NOTES (2026-10-13): update.go, init.go and merge_driver.go drop the `tree` import, and close.go, show.go, know_test.go and read_test.go drop `edit` (the forwarded names were their only uses); know.go keeps `edit` for Labels, NormalizeNewlines and WithFinalNewline, and update.go keeps it for Labels, ReplaceBody and AppendBody.

**What:** Part 7 of review P2-03. `internal/cli` calls the moved functions through `nodefile`.
**Regression guard.** The Goal (code and comments) is checked by an Acceptance grep; comment-only sites such as init.go's compile either way. errors.go names only the re-exported sentinels, which the regex leaves out.
**Goal:** no file of `internal/cli` names a forwarder from `tree/nodefile.go` or `edit.ETag`, in code or in comments.
**Approach (assumed at the header base):** the same switch as P2.5. At the base the files are `close.go`, `init.go`, `know.go`, `merge_driver.go`, `show.go`, `update.go`, `know_test.go` and `read_test.go`. Any call site that P1.1 added in `show.go` or `know.go` is switched too.
Depends on P2.7; P1.1.
**Files:** internal/cli/close.go; internal/cli/init.go; internal/cli/know.go; internal/cli/merge_driver.go; internal/cli/show.go; internal/cli/update.go; internal/cli/know_test.go; internal/cli/read_test.go
**Read first:** internal/cli/update.go — readIssueFile, absoluteIssuePath; internal/cli/show.go — showResult; internal/cli/know.go — noteShowResult; internal/cli/close.go — closeOne; internal/cli/init.go — addMergeAttribute, mergeHooks; internal/cli/merge_driver.go — mergeIssueFile; internal/cli/read_test.go — fileETag; internal/cli/know_test.go — noteFileETag
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes nothing beyond the qualifiers and the import lines. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
- `grep -rnE '\b(tree|edit)\.(ReadNodeFile|ReadRegularFile|WriteFileAtomic|WriteFileAtomicVerbatim|ReplaceFileAtomic|ReplaceFileVerbatim|WriteFileExclusive|ETag)\b' --include=*.go internal/cli` prints nothing
**Commit:** `refactor(cli): import the node-file IO from internal/nodefile`

## P2.9 nodefile package, part 8: drop the forwarders — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the edit package comment no longer names the entity tag, which left the package with the ETag forwarder (same file as the forwarder, listed in Files).

**What:** The last part of review P2-03. The `nodestore` tests switch over, and the temporary forwarders in `tree` and `edit` are deleted.
**Regression guard.** The Acceptance regex names every former forwarder in full, `ReadRegularFile` (P0.3) and `ReplaceFileAtomic` (P1.22) included.
**Goal:** `internal/tree/nodefile.go` no longer exists, and `edit` declares no `ETag`. No Go file names `tree.ReadNodeFile`, `tree.WriteFile…`, `tree.ReplaceFileVerbatim`, `edit.ETag` or any other former forwarder. `tree.ErrNotRegular` and `tree.ErrTooLarge` remain as re-exports. `nodefile` imports nothing from `internal/`, and neither `lock` nor `store` imports `tree`.
**Approach (assumed at the header base):** switch `edit.ETag` in `nodestore`'s `create_test.go`, `edit_test.go`, `move_test.go` and `remove_test.go`. Delete `tree/nodefile.go` and drop it from `tree/doc.go`. Delete the `ETag` forwarder in `edit/edit.go`. If `go vet ./...` then names a call site left behind, such as one a Phase 0/1 item added, switch it here and leave a NOTES line.
Depends on P2.8.
**Files:** internal/nodestore/create_test.go; internal/nodestore/edit_test.go; internal/nodestore/move_test.go; internal/nodestore/remove_test.go; internal/tree/nodefile.go; internal/tree/doc.go; internal/edit/edit.go
**Read first:** internal/tree/nodefile.go — the forwarder vars; internal/edit/edit.go — ETag; internal/nodestore/edit_test.go — edit.ETag uses; internal/nodestore/move_test.go — edit.ETag uses; internal/nodestore/create_test.go; internal/nodestore/remove_test.go; internal/tree/doc.go — file map
**Closes:** nodefile-package
**Tests:** existing tests unchanged.
**Acceptance:** every importing package, one command at a time:
- `GOMEMLIMIT=2GiB go test ./internal/nodefile/`, then the same for `./internal/tree/`, `./internal/edit/`, `./internal/lock/`, `./internal/store/`, `./internal/agents/`, `./internal/check/`, `./internal/index/`, `./internal/issue/`, `./internal/knowledge/`, `./internal/nodestore/` and `./internal/cli/`
- `grep -rnE '(tree|edit)\.(ReadNodeFile|ReadRegularFile|WriteFileAtomic|WriteFileAtomicVerbatim|ReplaceFileAtomic|ReplaceFileVerbatim|WriteFileExclusive|ETag)\b' --include=*.go internal cmd` prints nothing
- `go list -f '{{join .Imports "\n"}}' ./internal/nodefile/ | grep internal/` and `go list -f '{{join .Imports "\n"}}' ./internal/lock/ ./internal/store/ | grep internal/tree` print nothing
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes nothing beyond the qualifiers, the import lines and the deletions. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(tree): drop the node-file IO forwarders`

## P2.11 Split nodestore/create.go into create, load, claim and write — ✅ DONE (2026-10-13)

**What:** Pure move per review P2-02. The helpers that Create, Move, Remove, Edit and Format share leave `create.go`, and `nodestore` gains a `doc.go` file map pinned by a structural test.
**Regression guard.** After the split nodestore has 12 non-test files, doc.go included. `duplicateClaimFailure`, which P1.7 added to create.go and which `claimFailure` and `Move` call, goes to claim.go beside `orphanClaimFailure`; the item depends on P1.7, not P1.8.
**Goal:** `create.go` holds `Create` and `newNodePath`. `load.go`, `claim.go` and `write.go` hold the shared helpers. `doc.go` maps all 12 non-test files, doc.go included, and `TestDocFileMap_MatchesDirectory` passes. Every existing test passes unchanged.
**Approach (assumed at the header base):**
- `load.go`: `load`, `triage`, `isBlockingProblem`, `resolveFailure` and `relativeTreeError`.
- `claim.go`: `claimFailure`, `orphanClaimFailure`, `duplicateClaimFailure` (P1.7), `unparsedClaimFailure`, `findUnparsedAliasHolder`, `findNodeFile`, `nodeID`, `nodeFileExtension` and `hiddenNamePrefix`.
- `write.go`: `write`, `encode`, `place`, `validationError`, `writeMode` with `replaceFile` and `createFile`, and `problemSeparator`.
- Every other declaration, including code that P1.7 or P1.23 added to `create.go`, goes to the file of its only user. One used by several of these files stays in `create.go`.
- `doc.go` takes the package comment from `kind.go` and adds a `# Files` map. `doc_test.go` is `internal/cli/doc_test.go` copied under `package nodestore`.
- Only moves: no renames and no logic edits.
Depends on P2.9; P1.7; P1.23.
**Files:** internal/nodestore/create.go; internal/nodestore/load.go; internal/nodestore/claim.go; internal/nodestore/write.go; internal/nodestore/kind.go; internal/nodestore/doc.go; internal/nodestore/doc_test.go
**Read first:** internal/nodestore/create.go — Create, newNodePath, claimFailure, orphanClaimFailure, findNodeFile, triage, place, writeMode; internal/nodestore/move.go — Move (orphanClaimFailure, unparsedClaimFailure, triage, resolveFailure); internal/nodestore/edit.go — Edit (encode, place); internal/nodestore/format.go — Format (write, relativeTreeError); internal/nodestore/remove.go — problemSeparator use; internal/nodestore/kind.go — package comment; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory
**Closes:** split-nodestore-create
**Tests:** add `internal/nodestore/doc_test.go` (`TestDocFileMap_MatchesDirectory`). Existing tests stay unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers and the new `doc.go`/`doc_test.go`. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(nodestore): split create.go into create, load, claim and write`

## P2.13 Split check/check.go into problem, issues, notes and agents — ✅ DONE (2026-10-13)

NOTES (2026-10-13): placed the declarations the Approach does not name by their only user: planExtension in issues.go; refSymbolSeparator, the supersession format const block, the keyRetired/keySupersededBy const block and leftOutNames in notes.go; checkConflictMarkers and addParseFailure stay in check.go (used by issues and notes)

**What:** Pure move per review P2-04.
**Goal:** `internal/check/check.go` keeps `Run` and the `checker` core. `problem.go`, `issues.go`, `notes.go` and `agents.go` hold the rest. Every existing test passes unchanged.
**Approach (assumed at the header base):**
- `problem.go`: the `Code…` constant block, `Problem` with `String`, `loadTree`, `treeCode` and `readFile`.
- `issues.go`: `parsedIssue`, `issues`, `graphNodes`, `checkIssue`, `checkRelated`, `isPlanFound` and `checkCycles`.
- `notes.go`: `notes`, `declaredFormat`, `checkSupersession`, `retiredSubtrees` and `checkNote`.
- `agents.go`: `agentsBlock`.
- `check.go` keeps the package comment, `Run`, `checker`, `add` and `relative`.
- Every declaration not named here goes to the file of its only user. One used by several files stays in `check.go`: at the base, `checkConflictMarkers` and `addParseFailure` do.
- Only moves: no renames and no logic edits.
Depends on P2.5.
**Files:** internal/check/check.go; internal/check/problem.go; internal/check/issues.go; internal/check/notes.go; internal/check/agents.go
**Read first:** internal/check/check.go — Run, checker, add, relative, checkConflictMarkers, addParseFailure; internal/check/check.go — loadTree, treeCode, readFile, Problem; internal/check/check.go — issues, isPlanFound, planExtension; internal/check/check.go — notes, declaredFormat, checkSupersession, checkNote, refSymbolSeparator, supersessionFormatVersion, keyRetired; internal/check/check.go — agentsBlock
**Closes:** split-check-go
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(check): split check.go into problem, issues, notes and agents`

## P2.15 Split config/config.go into config, format and parse — ✅ DONE (2026-10-13)

NOTES (2026-10-13): format.go also takes the regexp var block's documentStartLine and the helpers documentStart and insertLine (format-line rewrite is their only user); parse.go also takes the categories var (only parser.statuses and isCategory use it), per the item's "file of its only user" rule.

**What:** Pure move per review P2-05.
**Goal:** `internal/config/config.go` keeps the types, every constant, `DefaultFile`, `Default` and the lookups. `format.go` holds the format-line rewrite, and `parse.go` holds the strict parser. Note `store-config`'s Ref `internal/config/config.go#FormatVersion` still resolves, and every existing test passes unchanged.
**Approach (assumed at the header base):**
- `format.go`: `realFormatLine`, `commentedFormatLine`, `RaiseFormat`, `findFormatLine`, `trimLineEnding` and `lineEnding`.
- `parse.go`: `Parse`, `parser` and every method on it, and `isCategory` if the parser is its only user.
- Every top-level `const` stays in `config.go`, because `FormatVersion` is a Ref target. Any other declaration not named here, including what P1.3 added, goes to the file of its only user. One used by several files stays in `config.go`.
- Only moves: no renames and no logic edits.
Depends on P1.3.
**Files:** internal/config/config.go; internal/config/format.go; internal/config/parse.go
**Read first:** internal/config/config.go — RaiseFormat, findFormatLine, realFormatLine, trimLineEnding, lineEnding; internal/config/config.go — Parse, parser, isCategory, categories (var; only parser.statuses and isCategory use it); internal/config/config.go — FormatVersion, keylessFormat, keyFormat, formatComment (consts shared by format and parse); .capcom/knowledge/store-layout/store-config.md — refs; internal/knowledge/verify.go — checkSymbol (text-contains match)
**Closes:** split-config-go
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/config/`
- `capcom check`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(config): split config.go into config, format and parse`

## P2.17 Split index/refresh.go into refresh, problems and duplicates — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the claim type moves to duplicates.go as the approach names it, though walk.go, query.go and the refresher struct also use it; claimedNames, insertClaimQuery, insertClaims and replaceWalkClaims (P1.5/P1.6) stay in refresh.go, their only user; statement, sharedListStatements and exec stay in refresh.go (shared with notes.go). internal/index has no doc.go file map, so none is updated.

**What:** Pure move per review P2-06.
**Goal:** `internal/index/refresh.go` keeps the refresh orchestration, `indexFile`, the inserts, `forget` and `rewarn`. `problems.go` maps file problems to Warnings, and `duplicates.go` holds the duplicate and claim exclusion. Every existing test passes unchanged.
**Approach (assumed at the header base):**
- `problems.go`: `readRefusal`, `parseIssue` and `aliasProblems`.
- `duplicates.go`: `claim`, `excludeDuplicates`, `excludeDuplicatesOf` and `excludeClaims`.
- Every other declaration, including what P1.6 added, goes to the file of its only user. One used by several files stays in `refresh.go`.
- Only moves: no renames and no logic edits.
Depends on P2.5; P1.6.
**Files:** internal/index/refresh.go; internal/index/problems.go; internal/index/duplicates.go
**Read first:** internal/index/refresh.go — refresh, refresher, sync, indexFile, rewarn, forget, insertIssue, sharedListStatements; internal/index/refresh.go — readRefusal, parseIssue, aliasProblems; internal/index/refresh.go — claim, excludeDuplicates, excludeDuplicatesOf, excludeClaims; internal/index/notes.go — parseNote, insertNote; internal/index/walk.go — walkIssues
**Closes:** split-index-refresh
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(index): split refresh.go into refresh, problems and duplicates`

## P2.19 Collect the shared cli flag constants in flags.go — ✅ DONE (2026-10-13)

NOTES (2026-10-13): flags.go groups the moved constants by their former file (list, update, create, know blocks, then flagNoElsewhere, flagCheck, unlimitedDepth); the block comments read "Flags of capcom <cmd> that other commands share"; the left-behind block comments in list.go, create.go and know.go are cut to "Flags of capcom list." / "Flags of capcom create." / "Flags of the know subcommands."; update.go's comment now describes stdinPath (readTextInput's stdin path) and kindIssueWrite (the --json kind of every command that writes an issue); flagCheck's comment widened to name capcom fmt --check.

**What:** Pure move per review P2-07. A command no longer takes its flag names from a sibling command's file.
**Regression guard.** `flagCheck` (agents.go, used by agents.go and fmt.go) moves too; its comment, which names only `capcom agents`, is widened to name `fmt --check`. update.go's block comment ("Flags of capcom update; --status, ... shared with list.") is rewritten to describe what remains (`stdinPath`, `kindIssueWrite`). The move-only check allows both comment lines.
**Goal:** `internal/cli/flags.go` declares every flag-name constant used by two or more non-test files of `internal/cli`, plus `unlimitedDepth`. No other file declares any of them. A flag constant used by one file stays in that file. `doc.go` maps `flags.go`.
**Approach (assumed at the header base):**
- Find the constants with this check: for each `flag…` constant, count the non-test files of `internal/cli` that name it (`grep -lw <name> internal/cli/*.go | grep -v _test`).
- At the base the constants that move are:
  - from `list.go`: `flagAll`, `flagLabel`, `flagLimit`, `flagNoLabel`, `flagParent`, `flagPlan`, `flagPlanned`, `flagPriority`, `flagStatus`, `flagType` and `flagUnplanned`
  - from `update.go`: `flagTitle`, `flagAddLabel`, `flagRemoveLabel`, `flagBodyAppend`, `flagBodyAppendFile`, `flagBodyReplaceOld`, `flagBodyReplaceNew` and `flagIfMatch`
  - from `create.go`: `flagID` and `flagBodyFile`
  - from `know.go`: `flagDepth` and `unlimitedDepth`
  - from `output.go`: `flagNoElsewhere`
  - from `agents.go`: `flagCheck`
- Each constant keeps its comment. A block comment that names the flags its block shares with another command is cut down to the flags that remain in the block.
- Only moves: no renames and no value changes.
Depends on P2.8.
**Files:** internal/cli/flags.go; internal/cli/list.go; internal/cli/update.go; internal/cli/create.go; internal/cli/know.go; internal/cli/output.go; internal/cli/doc.go; internal/cli/agents.go
**Read first:** internal/cli/list.go — const block, treeRowFilters; internal/cli/update.go — const block (flagTitle..flagIfMatch, stdinPath, kindIssueWrite); internal/cli/create.go — const block; internal/cli/know.go — const block, unlimitedDepth; internal/cli/output.go — flagNoElsewhere; internal/cli/agents.go — flagCheck; internal/cli/fmt.go — newFmtCommand; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory
**Closes:** cli-flags-file
**Tests:** existing tests unchanged (`TestDocFileMap_MatchesDirectory` pins the map).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers, the trimmed block comments and the doc map. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(cli): collect the shared flag constants in flags.go`

## P2.21 Split cli/know.go per subcommand — ✅ DONE (2026-10-13)

NOTES (2026-10-13): noteFromFile (added by P1.1) went to know_show.go, its only user; resolveNote, noteData/newNoteData, retiredFlagUsage and childrenByParent stay in know.go as shared by two or more subcommands; flagRetired joined flagDepth in flags.go's know block (gofmt realigned the flagDepth line).
NOTES (2026-10-13): the know.go const block ("Flags of the know subcommands.") is gone; its members sit in blocks "Flags of capcom know tree." (flagBackground), "Flags of capcom know show." (flagWithChildren), "Flags of capcom know add." (flagRef), and kindNoteWrite in know_add.go gained its own doc comment ("the --json kind of every command that writes a Note") since it no longer shares a block.
NOTES (2026-10-13): issue bodies know-update-empty-label and split-know-go cite internal/cli/know.go line numbers that are now stale; left untouched (issue register is not this item's to edit).

**What:** Pure move per review P2-08, matching the existing `know_update.go`, `know_mv.go` and `know_rm.go`.
**Regression guard.** `flagRetired`, used by know tree, know show and know search, moves to flags.go here; `flagWithChildren`, `flagRef` and `flagBackground` go to know_show.go, know_add.go and know_tree.go. `childrenByParent`, used by know tree and know show --with-children, stays in know.go. The move-only check allows the comments of the split const block.
**Goal:** `know.go` holds the `know` root command and the Note data that its subcommands share. `know_tree.go`, `know_show.go`, `know_search.go`, `know_add.go` and `know_verify.go` each hold one subcommand. `doc.go` maps every file, and every existing test passes unchanged.
**Approach (assumed at the header base):**
- `know.go`: `newKnowCommand`, `noteData`, `newNoteData`, `resolveNote`, `retiredFlagUsage` and `childrenByParent` (shared by know tree and know show).
- `know_tree.go`: `newKnowTreeCommand`, `outlineData`, `outlineLine`, `noteOutlineLine`, `outline` and `backgroundFlagUsage`.
- `know_show.go`: `newKnowShowCommand`, `noteShowResult`, `noteText` and `retiredText`.
- `know_search.go`: `newKnowSearchCommand`.
- `know_add.go`: `knowAddOptions`, `newKnowAddCommand`, `addNote`, `newNote`, `noteBody`, `noteWriteResult`, `kindNoteWrite` and `refreshAgentsAfterWrite`. These go here per review even though `know mv`, `know rm` and `know update` call some of them.
- `know_verify.go`: `findingData`, `newKnowVerifyCommand` and `verifyNotes`.
- Everything else, including the know flag constants that P2.19 left and code that P1.1 added, goes to the file of its only user. One used by several files stays in `know.go`.
- `doc.go`: narrow `know.go`'s line, and add one line per new file.
- Only moves: no renames and no logic edits.
Depends on P2.19; P1.1.
**Files:** internal/cli/know.go; internal/cli/know_tree.go; internal/cli/know_show.go; internal/cli/know_search.go; internal/cli/know_add.go; internal/cli/know_verify.go; internal/cli/doc.go; internal/cli/flags.go
**Read first:** internal/cli/know.go — newKnowCommand, noteData, newNoteData, resolveNote, retiredFlagUsage, const block; internal/cli/know.go — newKnowTreeCommand, outline, childrenByParent, outlineLine, backgroundFlagUsage; internal/cli/know.go — newKnowShowCommand, noteShowResult, noteText, retiredText; internal/cli/know.go — addNote, newNote, noteBody, noteWriteResult, refreshAgentsAfterWrite, kindNoteWrite; internal/cli/know_update.go — noteBody, noteWriteResult callers; internal/cli/know_mv.go, internal/cli/know_rm.go — refreshAgentsAfterWrite callers; internal/cli/doc.go — file map; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory
**Closes:** split-know-go
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers and the doc map. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(cli): split know.go per subcommand`

## P2.23 Split reopen out of cli/close.go — ✅ DONE (2026-10-13)

**What:** Pure move per review P2-09.
**Goal:** `internal/cli/reopen.go` holds `capcom reopen`, and `close.go` holds `capcom close`. `doc.go` maps both, and every existing test passes unchanged.
**Approach (assumed at the header base):**
- `reopen.go`: `newReopenCommand`, `reopen`, `reopenLogPrefix` and `targetStatus`. `targetStatus` goes here per review even though close also calls it.
- `close.go` keeps everything else: `flagReason`, the outcome constants, `statusOptions`, `closeData`, `newCloseCommand`, `closeResult`, `issueNodes`, `closeOne` and `requireReason`.
- `doc.go`: `close.go` becomes "capcom close", and `reopen.go` gets its own line.
- Only moves: no renames and no logic edits.
Depends on P2.21.
**Files:** internal/cli/close.go; internal/cli/reopen.go; internal/cli/doc.go
**Read first:** internal/cli/close.go — newReopenCommand, reopen, reopenLogPrefix, targetStatus, requireReason, statusOptions, flagReason; internal/cli/doc.go — file map; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory; internal/cli/update.go — isClosedCategory; internal/cli/log.go — appendLog
**Closes:** split-close-go
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers and the doc map. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(cli): split reopen out of close.go`

## P2.25 Split cli/dep.go into dep, dep_list and dep_graph — ✅ DONE (2026-10-13)

NOTES (2026-10-13): isEntryText and isRemovedByText stay in dep.go: their users are dep.go (depRemoval) and related.go, so neither has a single user among the three files.

**What:** Pure move per review P2-10.
**Regression guard.** The "Text of capcom dep list." const block splits: `depsHeading`, `dependentsHeading`, `depMissingLabel` and `depSeeAboveLabel` go to dep_list.go under the block comment; `cycleArrow` (dep add only) stays in dep.go with its own one-line comment. The move-only check allows that comment and the const wrapper lines.
**Goal:** `dep.go` holds the `dep` root, `dep add` and `dep rm`. `dep_list.go` holds `dep list`. `dep_graph.go` holds the index-loaded dependency graph that `related.go` also uses, together with `dep graph`. `doc.go` maps all three, and every existing test passes unchanged.
**Approach (assumed at the header base):**
- `dep_list.go`: `newDepListCommand`, `listResult`, `depBranches`, `depSection`, `newDepSection` with `add`, `depListData`, `depListItem` and `defaultDepListDepth`.
- `dep_graph.go`: `depGraph`, `loadDepGraph`, `names`, `newDepGraphCommand`, `shownBy`, `flowchartResult`, `depGraphData`, `depGraphNode`, `depGraphEdge`, `directDepth` and the `mermaid…` constants.
- `dep.go` keeps `newDepCommand`, `newDepAddCommand`, `newDepRmCommand`, `depRemoval` (with any P1.2 change), `resolveEdge`, which `related.go` also calls, and every declaration used by several of the three files.
- Every other declaration goes to the file of its only user.
- `doc.go`: narrow `dep.go`'s line, and add one line per new file.
- Only moves: no renames and no logic edits.
Depends on P2.23; P1.2.
**Files:** internal/cli/dep.go; internal/cli/dep_list.go; internal/cli/dep_graph.go; internal/cli/doc.go
**Read first:** internal/cli/dep.go — newDepCommand, depRemoval, resolveEdge, loadDepGraph, depGraph.names, depGraph.listResult, depSection, the dep-list/mermaid const blocks; internal/cli/related.go — newRelatedAddCommand, relatedRemoval (loadDepGraph, names, resolveEdge); internal/cli/outline.go — outlineRow, renderOutline; internal/cli/issue_render.go — issueData, newIssueData, issueLines; internal/cli/doc.go — file map
**Closes:** split-dep-go
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers and the doc map. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(cli): split dep.go into dep, dep_list and dep_graph`

## P2.27 Move the shared node-edit helpers out of cli/update.go — ✅ DONE (2026-10-13)

**What:** Pure move per review P2-11. This prepares for A-02.
**Regression guard.** After P2.19 update.go's const block holds only `stdinPath` and `kindIssueWrite`: delete the emptied block, and give each moved const a one-line comment in node_edit.go. The move-only check allows those lines.
**Goal:** `internal/cli/node_edit.go` holds the edit path that every issue and Note edit verb goes through. `update.go` holds the `capcom update` command. `doc.go` maps both, and every existing test passes unchanged.
**Approach (assumed at the header base):**
- `node_edit.go`: `editIssue`, `editNode`, `issueResolver` with `Resolve`, `readTextInput` with `stdinPath`, and `writeResult` with `kindIssueWrite`. Move the last two out of `update.go`'s const block.
- `update.go` keeps everything else: the update flags, `updateOptions`, `newUpdateCommand`, `applyUpdate`, `checkStatusChange`, `isClosedCategory`, `hasAnyFlag`, `readIssueFile`, `absoluteIssuePath` and `frontmatterFailure`.
- `doc.go`: `update.go` becomes "capcom update", and `node_edit.go` gets its own line.
- Only moves: no renames and no logic edits.
Depends on P2.25.
**Files:** internal/cli/update.go; internal/cli/node_edit.go; internal/cli/doc.go
**Read first:** internal/cli/update.go — editIssue, editNode, issueResolver.Resolve, readTextInput, writeResult, stdinPath, kindIssueWrite; internal/cli/know_update.go — editNode caller, readTextInput callers; internal/cli/create.go — readTextInput, writeResult; internal/cli/related_test.go — kindIssueWrite use; internal/cli/doc.go — file map
**Closes:** node-edit-helpers
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers and the doc map. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(cli): move the shared node-edit helpers to node_edit.go`

## P2.29 cli/output package, part 1: the error contract — ✅ DONE (2026-10-13)

NOTES (2026-10-13): internal/cli/output/doc.go added (not in Files) to carry the new package's doc comment, per the Go convention that a package documents itself in doc.go; it holds no code.
NOTES (2026-10-13): consequential edit — internal/cli/doc.go: made necessary by moving Error, Code and the exit codes out of cli/errors.go (its file-map line now says the file holds the temporary aliases, kindFailure and the tree-problem warnings)
NOTES (2026-10-13): consequential edit — .agents/skills/release-notes-capcom/prompts/_capcom.md: made necessary by moving the exit codes to internal/cli/output/errors.go (the "Output modes & exit codes" row named internal/cli/errors.go)

**What:** Part 1 of review P2-14, the pure move half of A-14. The typed error and the exit codes move from `internal/cli/errors.go` to a new package, `internal/cli/output`. Until P2.35, `cli` keeps temporary aliases under the old spellings.
**Goal:** `internal/cli/output/errors.go` declares `Code` and its constants, `ExitOK`, `ExitError`, `ExitUsage`, `ExitNotFound`, `Error` with `Error()` and `ExitCode()`, `Errorf`, `UsageErrorf` and `AsError`. `output` never imports `internal/cli`. Every code, exit code and message is byte-for-byte unchanged. Note `error-messages` names the new file, and `capcom check` passes.
**Approach (assumed at the header base):**
- Move those declarations, with their comments, out of `cli/errors.go`. Names are capitalised and not otherwise renamed: `asError` → `AsError`, and the field `isRendered` → `IsRendered`, whose sites in `fmt.go`, `check.go` and `root.go` follow (`grep -n isRendered`). `isUsage` stays unexported because only `UsageErrorf` sets it.
- Where the declarations stood, `cli/errors.go` gets temporary aliases: `type Code = output.Code`, `type Error = output.Error`, the constants, and one-line wrapper funcs for `Errorf`, `UsageErrorf` and `asError`. The wrapper funcs keep vet's printf check working.
- `nodeFileExtension`, `kindFailure` and every declaration after it stay in `cli/errors.go`.
- `TestError_Error_PrefixesKnownLocation` and `TestError_ExitCode_MapsCodes` move from `cli_test.go` to `output/errors_test.go`.
- Update Note `error-messages`. Its Ref becomes `internal/cli/output/errors.go#type Error struct`, and its body says "In `internal/cli/output`". Use `capcom know update error-messages --remove-ref … --add-ref … --body-replace-old … --body-replace-new …`.
Depends on P2.27; P0.11.
**Files:** internal/cli/output/errors.go; internal/cli/output/errors_test.go; internal/cli/errors.go; internal/cli/fmt.go; internal/cli/check.go; internal/cli/root.go; internal/cli/cli_test.go; .capcom/knowledge/coding-conventions/error-messages.md
**Read first:** internal/cli/errors.go — Code, Exit*, Error, Errorf, UsageErrorf, asError, nodeFileExtension, kindFailure; internal/cli/root.go — Run (isRendered, asError), app.emit; internal/cli/check.go — newCheckCommand (isRendered); internal/cli/fmt.go — formatRun.emit (isRendered); internal/cli/cli_test.go — TestError_Error_PrefixesKnownLocation, TestError_ExitCode_MapsCodes; .capcom/knowledge/coding-conventions/error-messages.md — refs, body
**Tests:** moved tests only.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/output/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go list -f '{{join .Imports "\n"}}' ./internal/cli/output/ | grep 'internal/cli$'` prints nothing
- `capcom check`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers, the capitalised names, the alias block and the Note. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(cli): move the error contract to internal/cli/output`

## P2.30 cli/output package, part 2: the renderers and storeFailure — ✅ DONE (2026-10-13)

NOTES (2026-10-13): kvKeyMessage and kvLineBreaks are capitalised as KVKeyMessage and KVLineBreaks (Go keeps an acronym in one case, matching KVPair and KVRecord), not KvKeyMessage/KvLineBreaks; the other names take a plain capital (OutputFormat, FormatText, FormatJSON, FormatKV, NewRenderer, KindError, StoreFailure).
NOTES (2026-10-13): cli/output.go also aliases the moved exported names SchemaVersion, Envelope, ErrorBody and Renderer beside Result, KVRecord and KVPair, so every name the file declared keeps its old spelling until P2.35.
NOTES (2026-10-13): internal/cli/output/doc.go package comment widened to name the renderers and StoreFailure (not in Files; the package documents itself in doc.go).
NOTES (2026-10-13): consequential edit — internal/cli/doc.go: made necessary by moving the output contract out of cli/output.go (its file-map line now says the file holds the temporary aliases and the Elsewhere scan of a result)
NOTES (2026-10-13): plain `go vet` can replay a cached clean result on this host and miss the composites (unkeyed KVPair) findings; the 46 sites were found and checked with `GOCACHE=<fresh dir> go vet -composites ./internal/cli/`.

**What:** Recast at the regression check (2026-10-13). Part 2 of review P2-14. The output contract (`Result`, the `--kv` records, the `--json` envelope and the renderers) moves from `internal/cli/output.go`, and `storeFailure` moves from `internal/cli/init.go`. Both go to `internal/cli/output`, behind temporary aliases.
**Regression guard.** P0.11 calls the sanitiser only from the text renderer sink, so `sanitizeTerminal` moves into internal/cli/output with the renderer and stays unexported; its unit test moves with it into package output; no alias is made for it. Once `KVPair` aliases `output.KVPair`, `go vet` (composites) flags unkeyed `KVPair` literals; P2.30 keys every unkeyed `KVPair` literal in internal/cli (find with grep; at the base: close, issue_render, know_show, node_edit, rm, show) and lists those files in Files; the move-only check allows those keyed-literal edits. output/output_test.go declares its own `decodeEnvelope` (a copy of cli_test.go's, which other cli tests keep using); the move-only check allows it.
**Goal:** `internal/cli/output/output.go` declares `SchemaVersion`, `Result`, `KVRecord`, `KVPair`, `Envelope`, `ErrorBody`, `Renderer`, the format selector, the constructor, the text, `--json` and `--kv` renderers, and the text sanitiser that P0.11 added. `output/errors.go` gains `StoreFailure`. Output in every mode is byte-for-byte unchanged, and `output` never imports `internal/cli`.
**Approach (assumed at the header base):**
- Move every declaration of `cli/output.go` except `addElsewhere` and `issuesPathspec`, which stay in `cli` because they use `app`.
- Capitalise, and do not otherwise rename, each unexported name that code or tests left in package `cli` use. At the base those are `outputFormat`, `formatText`, `formatJSON`, `formatKV`, `newRenderer`, `kvKeyMessage`, `kvLineBreaks`, `kindError` and `storeFailure`. The P0.11 sanitiser moves unexported (see the guard).
- `output` imports `elsewhere`, `config` and `store`, as the review allows.
- Temporary aliases under the old spellings go where the code stood: in `cli/output.go`, and a wrapper func `storeFailure` in `cli/init.go`.
- Move `TestKVRenderer_Render_ClosesEveryRecordWithStatus`, `TestTextRenderer_Render_WritesWarningsAfterResult` and `TestJSONRenderer_RenderError_CarriesLocation` to `output/output_test.go`. Move `TestStoreFailure_MapsInitAndOpenErrors` to `output/errors_test.go`. `TestApp_Emit_*` and `TestStoreFailure_StoreCommandBeforeInit_IsNotInitialized` stay in `cli`, because they need `app` and `newGitRepo`.
Depends on P2.29.
**Files:** internal/cli/output/output.go; internal/cli/output/output_test.go; internal/cli/output/errors.go; internal/cli/output/errors_test.go; internal/cli/output.go; internal/cli/init.go; internal/cli/cli_test.go; internal/cli/init_test.go; internal/cli/terminal_test.go; internal/cli/close.go; internal/cli/issue_render.go; internal/cli/know_show.go; internal/cli/node_edit.go; internal/cli/rm.go; internal/cli/show.go
**Read first:** internal/cli/output.go — Result, KVPair, newRenderer, textRenderer.Render, kvLineBreaks, kindError, kvKeyMessage, addElsewhere; internal/cli/init.go — storeFailure; internal/cli/root.go — Run (outputFormat, newRenderer); internal/cli/cli_test.go — decodeEnvelope, TestKVRenderer_Render_ClosesEveryRecordWithStatus, TestJSONRenderer_RenderError_CarriesLocation; internal/cli/init_test.go — TestStoreFailure_MapsInitAndOpenErrors, TestStoreFailure_StoreCommandBeforeInit_IsNotInitialized; internal/cli/issue_render.go — issueText, issueKV (unkeyed KVPair); internal/cli/show.go — kvKeyMessage; internal/cli/update.go — writeResult (unkeyed KVPair; node_edit.go after its split)
**Tests:** moved tests only. `TestSanitizeTerminal_EscapesControlsOnly` moves from terminal_test.go to output/output_test.go; the unkeyed `KVPair` literals become keyed (`{Key: "ID", Value: arg}`).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/output/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go list -f '{{join .Imports "\n"}}' ./internal/cli/output/ | grep 'internal/cli$'` prints nothing
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes no lines beyond the package/import headers, the capitalised names and the alias blocks. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(cli): move the renderers and storeFailure to internal/cli/output`

## P2.31 cli/output package, part 3: commands agents.go to fmt.go — ✅ DONE (2026-10-13)

**What:** Part 3 of review P2-14. The first batch of command files names the output contract through package `output`.
**Regression guard.** P2.30 keys every unkeyed `KVPair` literal in `internal/cli`, so `go vet ./...` stays green here. The Goal's "use no temporary alias" is checked by an Acceptance grep.
**Goal:** the non-test files of `internal/cli` from `agents.go` through `fmt.go`, in `ls` order and excluding `errors.go`, use no temporary alias from P2.29 or P2.30. Each reference is `output.<Name>`, with the capitalised spelling.
**Approach (assumed at the header base):** replace each alias use with its `output.` name and add the import. Test files stay unchanged. At the time this runs the batch is `agents.go`, `check.go`, `close.go`, `create.go`, `dep.go`, `dep_graph.go`, `dep_list.go` and `fmt.go`. Any other file in that range that names an alias joins the batch.
Depends on P2.30.
**Files:** internal/cli/agents.go; internal/cli/check.go; internal/cli/close.go; internal/cli/create.go; internal/cli/dep.go; internal/cli/dep_graph.go; internal/cli/dep_list.go; internal/cli/fmt.go
**Read first:** internal/cli/errors.go — Errorf, UsageErrorf, asError, Error; internal/cli/output.go — Result, KVRecord, KVPair; internal/cli/init.go — storeFailure; internal/cli/close.go — closeResult; internal/cli/check.go — newCheckCommand; internal/cli/agents.go — checkAgentsBlock, agentsFailure; internal/cli/fmt.go — newFmtCommand
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes nothing beyond the qualifiers and the import lines. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
- `grep -nE "(^|[^.[:alnum:]_])(($A)([^[:alnum:]_:]|\$)|Error([^[:alnum:]_:(]|\$))" <this item's Go files> | grep -vE '^[^:]+:[0-9]+:\s*//'` prints nothing, where `$A` is the `|`-joined alias names P2.29 and P2.30 declare (at the base its only false hit is init.go's "Claude Code" string)
**Commit:** `refactor(cli): name the output contract through internal/cli/output in agents to fmt`

## P2.32 cli/output package, part 4: commands issue_render.go to know_tree.go — ✅ DONE (2026-10-13)

NOTES (2026-10-13): internal/cli/know.go is in the batch's ls range and in Files but names no temporary alias, so it is left unchanged and is not in FILES.

**What:** Part 4 of review P2-14. The second batch of command files names the output contract through package `output`.
**Regression guard.** P2.30 keys every unkeyed `KVPair` literal in `internal/cli`, so `go vet ./...` stays green here. The Goal's "use no temporary alias" is checked by an Acceptance grep.
**Goal:** the non-test files of `internal/cli` from `issue_render.go` through `know_tree.go`, in `ls` order, use no temporary alias from P2.29 or P2.30. Each reference is `output.<Name>`.
**Approach (assumed at the header base):** the same switch as P2.31. At the time this runs the batch is `issue_render.go`, `know.go`, `know_add.go`, `know_mv.go`, `know_rm.go`, `know_search.go`, `know_show.go` and `know_tree.go`.
Depends on P2.31.
**Files:** internal/cli/issue_render.go; internal/cli/know.go; internal/cli/know_add.go; internal/cli/know_mv.go; internal/cli/know_rm.go; internal/cli/know_search.go; internal/cli/know_show.go; internal/cli/know_tree.go
**Read first:** internal/cli/issue_render.go — issueText, issueKV, flatten; internal/cli/know.go — noteText, noteShowResult, addNote, refreshAgentsAfterWrite; internal/cli/output.go — kvLineBreaks, KVPair, KVRecord; internal/cli/know_mv.go — moveNote; internal/cli/know_rm.go — removeNote
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes nothing beyond the qualifiers and the import lines. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
- `grep -nE "(^|[^.[:alnum:]_])(($A)([^[:alnum:]_:]|\$)|Error([^[:alnum:]_:(]|\$))" <this item's Go files> | grep -vE '^[^:]+:[0-9]+:\s*//'` prints nothing, where `$A` is the `|`-joined alias names P2.29 and P2.30 declare (at the base its only false hit is init.go's "Claude Code" string)
**Commit:** `refactor(cli): name the output contract through internal/cli/output in issue_render to know_tree`

## P2.33 cli/output package, part 5: commands know_update.go to node_edit.go — ✅ DONE (2026-10-13)

**What:** Part 5 of review P2-14. The third batch of command files names the output contract through package `output`.
**Regression guard.** P2.30 keys every unkeyed `KVPair` literal in `internal/cli`, so `go vet ./...` stays green here. The Goal's "use no temporary alias" is checked by an Acceptance grep.
**Goal:** the non-test files of `internal/cli` from `know_update.go` through `node_edit.go`, in `ls` order, use no temporary alias from P2.29 or P2.30. Each reference is `output.<Name>`.
**Approach (assumed at the header base):** the same switch as P2.31. At the time this runs the batch is `know_update.go`, `know_verify.go`, `list.go`, `lock.go`, `log.go`, `merge_driver.go`, `mv.go` and `node_edit.go`.
Depends on P2.32.
**Files:** internal/cli/know_update.go; internal/cli/know_verify.go; internal/cli/list.go; internal/cli/lock.go; internal/cli/log.go; internal/cli/merge_driver.go; internal/cli/mv.go; internal/cli/node_edit.go
**Read first:** internal/cli/update.go — writeResult, editIssue, issueResolver; internal/cli/lock.go — lockPairs, lockTarget; internal/cli/list.go — listResult, issueTreeResult; internal/cli/mv.go — move; internal/cli/know_update.go; internal/cli/merge_driver.go
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes nothing beyond the qualifiers and the import lines. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
- `grep -nE "(^|[^.[:alnum:]_])(($A)([^[:alnum:]_:]|\$)|Error([^[:alnum:]_:(]|\$))" <this item's Go files> | grep -vE '^[^:]+:[0-9]+:\s*//'` prints nothing, where `$A` is the `|`-joined alias names P2.29 and P2.30 declare (at the base its only false hit is init.go's "Claude Code" string)
**Commit:** `refactor(cli): name the output contract through internal/cli/output in know_update to node_edit`

## P2.34 cli/output package, part 6: commands prime.go to search.go — ✅ DONE (2026-10-13)

**What:** Part 6 of review P2-14. The fourth batch of command files names the output contract through package `output`.
**Regression guard.** P2.30 keys every unkeyed `KVPair` literal in `internal/cli`, so `go vet ./...` stays green here. The Goal's "use no temporary alias" is checked by an Acceptance grep.
**Goal:** the non-test files of `internal/cli` from `prime.go` through `search.go`, in `ls` order, use no temporary alias from P2.29 or P2.30. Each reference is `output.<Name>`.
**Approach (assumed at the header base):** the same switch as P2.31. At the time this runs the batch is `prime.go`, `ready.go`, `reindex.go`, `related.go`, `reopen.go`, `rm.go`, `root.go` and `search.go`.
Depends on P2.33.
**Files:** internal/cli/prime.go; internal/cli/ready.go; internal/cli/reindex.go; internal/cli/related.go; internal/cli/reopen.go; internal/cli/rm.go; internal/cli/root.go; internal/cli/search.go
**Read first:** internal/cli/root.go — app, Run, emit, scanFormat; internal/cli/output.go — outputFormat, formatText, newRenderer, Renderer; internal/cli/rm.go — removeIssue; internal/cli/prime.go — renderBriefing; internal/cli/search.go; internal/cli/ready.go
**Tests:** existing tests unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes nothing beyond the qualifiers and the import lines. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
- `grep -nE "(^|[^.[:alnum:]_])(($A)([^[:alnum:]_:]|\$)|Error([^[:alnum:]_:(]|\$))" <this item's Go files> | grep -vE '^[^:]+:[0-9]+:\s*//'` prints nothing, where `$A` is the `|`-joined alias names P2.29 and P2.30 declare (at the base its only false hit is init.go's "Claude Code" string)
**Commit:** `refactor(cli): name the output contract through internal/cli/output in prime to search`

## P2.35 cli/output package, part 7: drop the aliases — ✅ DONE (2026-10-13)

NOTES (2026-10-13): this host's `grep` is a shell function wrapping ugrep, which rejects the `\2` backreference in the alias-assignment Acceptance grep and prints paths without the leading `./`, so the prose-rule grep's `^\./docs/...` filter never matches; both were run with `/usr/bin/grep` (GNU) and print nothing.
NOTES (2026-10-13): the package comment in internal/cli/doc.go still says cli implements "the output contract ... and the mapping from typed errors to exit codes"; P2.29/P2.30 moved that code, not this item, so the sentence is left for the A-14 deepening.

**What:** The last part of review P2-14's pure move. The remaining files switch over, and the temporary aliases are deleted. The deepening A-14 comes later, so this item closes nothing.
**Regression guard.** P2.30 keys every unkeyed `KVPair` literal, so `go vet ./...` stays green here. `go vet` prints only the first type error, so the undefined names come from `go test -gcflags=-e -run '^$' ./internal/cli/ 2>&1 | sed -n 's/.*undefined: //p' | sort -u`. The Acceptance also checks grouped and const/var aliases, the prose rule and kindFailure's comment, which names `asError`.
**Goal:** no non-test file of `internal/cli` declares an alias of an `output` name or a wrapper func for one, and every reference is `output.<Name>`. The cli tests still run unchanged, through `internal/cli/output_alias_test.go`, which declares under the old spellings only the names those tests use. `doc.go`'s lines for `errors.go` and `output.go` describe what those files still hold. No doc outside the archives names `internal/cli/output.go` as the output contract.
**Approach (assumed at the header base):**
- Switch `show.go`, `update.go`, `errors.go` (`kindFailure` and below), `output.go` (`addElsewhere`, `issuesPathspec`) and `init.go`. Then delete the alias blocks in `errors.go` and `output.go`, and the `storeFailure` wrapper in `init.go`.
- `output_alias_test.go` declares exactly the names `go test -gcflags=-e -run '^$' ./internal/cli/ 2>&1 | sed -n 's/.*undefined: //p' | sort -u` lists once the aliases are gone. Review 5.3 keeps the CLI test lines unchanged.
- Prose rule: every doc that names `internal/cli/output.go` or `internal/cli/errors.go` as the home of the output modes or exit codes names `internal/cli/output/` instead. Find them with `grep -rln 'internal/cli/output\.go\|internal/cli/errors\.go' --include=*.md . | grep -v '^\./docs/\(reviews\|skill-runs\|plans/archived\|handoffs\)\|^\./\.capcom/issues/'`. At the base the only hit is the "Output modes & exit codes" row of `.agents/skills/release-notes-capcom/prompts/_capcom.md`.
- Importers: only `internal/cli` names the moved code. `cmd/capcom` reaches it through `cli.Run`, which `go vet ./...` covers.
Depends on P2.34.
**Files:** internal/cli/show.go; internal/cli/update.go; internal/cli/errors.go; internal/cli/output.go; internal/cli/init.go; internal/cli/output_alias_test.go; internal/cli/doc.go; .agents/skills/release-notes-capcom/prompts/_capcom.md
**Read first:** internal/cli/errors.go — kindFailure, asError; internal/cli/output.go — addElsewhere, issuesPathspec; internal/cli/init.go — storeFailure; internal/cli/show.go — showResult, resolveFailure; internal/cli/doc.go — file map; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory; internal/cli/init_test.go — TestStoreFailure_StoreCommandBeforeInit_IsNotInitialized; .agents/skills/release-notes-capcom/prompts/_capcom.md — "Output modes & exit codes" row
**Tests:** add `output_alias_test.go`. Every other test stays unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/output/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `grep -nE '^type (Result|KVRecord|KVPair|Envelope|ErrorBody|Renderer|Code|Error|outputFormat) =|^func (Errorf|UsageErrorf|asError|storeFailure|newRenderer)\(' internal/cli/*.go | grep -v _test.go` prints nothing
- `go list -f '{{join .Imports "\n"}}' ./internal/cli/output/ | grep 'internal/cli$'` prints nothing
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`). The zebra diff changes nothing beyond the qualifiers, the import lines, the deleted aliases, the new test alias file, the doc map and the prose row. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
- `grep -niE '^(type |const |var )?[[:space:]]*([[:alnum:]_]+)( [[:alnum:]_.]+)? += output\.\2$' internal/cli/*.go | grep -v _test.go` prints nothing
- `grep -nw 'asError\|storeFailure\|newRenderer' internal/cli/*.go | grep -v _test` prints nothing
- `grep -rln 'internal/cli/output\.go\|internal/cli/errors\.go' --include=*.md . | grep -v '^\./docs/\(reviews\|skill-runs\|plans\|handoffs\)\|^\./\.capcom/issues/'` prints nothing
**Commit:** `refactor(cli): drop the output aliases from package cli`

## P2.37 Import internal/briefing under its own name in cli/ready.go — ✅ DONE (2026-10-13)

**What:** Fixes U-09 (`ready-import-alias`), a cosmetic issue per review 2.4. Package `cli` imports `internal/briefing` as `briefing` in `prime.go` but as `readylist` in `ready.go`. This is the last item of the Phase 2 moves.
**Goal:** `grep -rn readylist internal/` finds nothing. `ready.go` names `briefing.ReadyList`, `briefing.ReadyOptions` and `briefing.ReadyItem`.
**Approach (assumed at the header base):** drop the alias from the import, and rewrite every `readylist.` qualifier as `briefing.`.
Depends on P2.35.
**Files:** internal/cli/ready.go
**Read first:** internal/cli/ready.go — import block, newReadyCommand RunE (ReadyList), readyOptions.listOptions, readyListData, readyLines, droppedDepWarnings; internal/cli/prime.go — briefing import
**Closes:** ready-import-alias; split-large-files
**Tests:** none new. The existing ready tests stay unchanged.
**Acceptance:**
- `grep -rn readylist internal/` prints nothing
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run Ready`
- `go vet ./internal/cli/`
**Commit:** `refactor(cli): import internal/briefing under its own name in ready.go`

## P3.1 Config owns the store-format constants — ✅ DONE (2026-10-13)

NOTES (2026-10-13): re-derived from the assumption that supersessionFormatVersion, missingConfigFormat, declaredFormat and checkSupersession live in internal/check/check.go; P2.13 moved them to internal/check/notes.go, which is edited instead and check.go is untouched.
NOTES (2026-10-13): with its two format constants gone, cli/related.go's const block held only formatRaisedWarning, so it became a single const declaration whose comment names the two raising writes the removed block comment named.

**What:** First part of A-07 (`config-owns-format`). The format rules move to one owner, `config`, and their copies in `check` and `cli` go. Behaviour is unchanged.
**Regression guard.** `declaredFormat` returns `config.MissingFormat` and `missingConfigFormat` is deleted here, so `check` defines no format constant of its own; P3.2 then deletes only `declaredFormat`.
**Goal:** `config` exports `MissingFormat` (1: the format of a store with no `config.yaml` or no `format` key), `FormatRelated` (2) and `FormatSupersession` (3). Each threshold has exactly one definition in `internal/`. `check` and `cli` read these constants and define none of their own.
**Approach (assumed at the header base):** in `internal/config`, rename `keylessFormat` to the exported `MissingFormat` everywhere it is used (`Parse`, `findFormatLine`). Add `FormatRelated` and `FormatSupersession` beside `FormatVersion`, documented per ADR 0009. Delete `relatedFormatVersion` and `supersessionFormatVersion` from `internal/cli/related.go`, and switch `newRelatedAddCommand` and `know update`'s `raiseStoreFormat` call to the config constants. Delete `supersessionFormatVersion` from `internal/check` and have `checkSupersession` read `config.FormatSupersession`. `declaredFormat` returns `config.MissingFormat`, and `missingConfigFormat` is deleted here; P3.2 then deletes only `declaredFormat`. `config` stays a leaf package.
Depends on P2.13; P2.15.
**Files:** internal/config/config.go; internal/config/format.go; internal/config/parse.go; internal/check/check.go; internal/check/notes.go; internal/cli/related.go; internal/cli/know_update.go
**Read first:** internal/config/config.go — keylessFormat, FormatVersion, Parse, findFormatLine; internal/check/check.go — supersessionFormatVersion, missingConfigFormat, declaredFormat, checkSupersession; internal/cli/related.go — relatedFormatVersion, supersessionFormatVersion, newRelatedAddCommand, raiseStoreFormat; internal/cli/know_update.go — updateNote; internal/cli/related_test.go — TestRelatedAdd_KeylessConfig_RaisesTheFormatOnceWithAWarning; internal/cli/know_update_test.go — TestKnowUpdate_FormatTwoStore_RaisedToThreeOnceWithAWarning
**Tests:** none new; the existing format tests pin the behaviour (`TestRaiseFormat_*`, `TestRun_SupersessionKeysWithoutConfig_ReportFormatTooOldAtEachKey`, `TestRelatedAdd_*`, `TestKnowUpdate_FormatTwoStore_*`).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/config/`
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'RelatedAdd|KnowUpdate'`
- `grep -rnE '\b(keylessFormat|relatedFormatVersion|supersessionFormatVersion|missingConfigFormat)\b' internal/` prints nothing
- `go vet ./internal/config/ ./internal/check/ ./internal/cli/`
**Commit:** `refactor(config): own the store-format constants in config`

## P3.2 A store without config.yaml opens at format 1 — ✅ DONE (2026-10-13)

NOTES (2026-10-13): re-derived from "declaredFormat and missingConfigFormat live in internal/check/check.go": at HEAD declaredFormat lives in internal/check/notes.go and missingConfigFormat no longer exists (P3.1 replaced it with config.MissingFormat), so check.go needed no change and is not in FILES.
NOTES (2026-10-13): no CHANGELOG entry: Config.Format's only production reader is check, which already reported format 1 for such a store; the fix changes no output. missing-config-format is not listed under CLOSES because its second half (init keeping an existing store at its format) is P3.3.

**What:** Fixes the first half of F-16 (`missing-config-format`). `store.loadConfig` stands `config.Default()`, at format 3, in for a missing file, and `check` compensates by statting the file again.
**Goal:** opening a store with no `config.yaml` gives `Config.Format == config.MissingFormat` and every other key at its default. `check` takes the declared format from `Config.Format` alone and never stats `config.yaml` for it. Its `format-too-old` problems in such a store still say "the store declares format 1".
**Approach (assumed at the header base):** in the missing-file branch of `loadConfig` (with whatever guarded read P0.7 gave it), set `Format` to `config.MissingFormat` on the default config. In `internal/check`, delete `declaredFormat` and `missingConfigFormat`, and have `notes` pass `c.store.Config.Format`. Drop the imports this leaves unused. Value shape: `Config.Format` is produced by `config.Parse` (`parser.format`), `config.Default` and `store.loadConfig`. Its only production reader is `check` (`notes`/`checkSupersession`). Its test readers are `cli/init_test.go` (`TestInit_WrittenConfig_DeclaresCurrentFormat`) and `config_test.go`; grep `Config.Format\|\.Format !=` at write time.
Depends on P3.1; P0.7; P0.6.
**Files:** internal/store/store.go; internal/store/store_test.go; internal/check/notes.go; internal/check/check.go
**Read first:** internal/store/store.go — loadConfig, Open, Init; internal/check/check.go — notes, declaredFormat, checkSupersession, Run; internal/config/config.go — Default, Config.Format; internal/store/store_test.go — newRepo, initStore, TestRaiseFormat_MissingConfig_CreatesItHoldingTheFormatKey; internal/check/check_test.go — newRepo, TestRun_SupersessionKeysWithoutConfig_ReportFormatTooOldAtEachKey; internal/cli/init_test.go — TestInit_WrittenConfig_DeclaresCurrentFormat
**Tests:** add `TestOpen_NoConfig_DeclaresFormatOneWithDefaults` to `store_test.go`. It must fail on the pre-item tree, which gives format 3 (the bite check). `TestRun_SupersessionKeysWithoutConfig_ReportFormatTooOldAtEachKey` stays green, now through `Config.Format`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/store/`
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `grep -rn 'missingConfigFormat\|declaredFormat()' internal/check/` prints nothing
- `go vet ./internal/store/ ./internal/check/`
**Commit:** `fix(store): open a store without config.yaml at format 1`

## P3.3 init keeps an existing store at its format — ✅ DONE (2026-10-13)

NOTES (2026-10-13): config_test.go's TestRaiseFormat_FormatOneTemplate_BecomesCurrentTemplate is renamed TestRaiseFormat_KeylessFile_BecomesDefaultFile, since the formatOneTemplate helper it named is replaced by config.KeylessFile; the new parse test is TestKeylessFile_Parse_DefaultsAtFormatOne.
NOTES (2026-10-13): Read-first anchor internal/check/check.go — checkSupersession now lives in internal/check/notes.go; nothing there needed changing (the format-1 problem line comes out as the plan states).
NOTES (2026-10-13): docs/format.md line 505 ("the file `capcom init` writes, except `format`") and the store-config / adr-0009 summary Notes were left as they are: they stay true for a store init creates and claim nothing about an existing one.

**What:** Fixes F-16 (`missing-config-format`). `capcom init --merge-driver` or `--claude-hooks` on an existing store with no `config.yaml` writes `format: 3`. Ratified F-16 (binding): `init` on an existing store with no `config.yaml` writes the template with the `format:` line commented out, so the store stays at format 1. A new store still gets `format: 3`.
**Regression guard.** The prose rule covers all live prose: `grep -rni 'init.*format\|format.*init' docs/*.md README.md .capcom/knowledge`, docs/migrating-from-beads.md and the adr-0009-decision Note (edited through `capcom know update`, then `capcom check`) included; `createConfig`'s doc changes with `Init`'s. For an existing store this supersedes adr-0009-decision's "from then on `init` writes the current format"; new stores keep it.
**Goal:** `config` exports `KeylessFile`: `DefaultFile` with its format comment and key replaced by `keylessFormatComment` and `# format: 1`, the only difference between the two. `store.Init` writes `DefaultFile` when the same call created the store root, and `KeylessFile` when the root already existed. `KeylessFile` parses to `Default()` at format 1, and `RaiseFormat(KeylessFile, FormatVersion)` returns exactly `DefaultFile`. init's text output is unchanged (`created .capcom/config.yaml`).
**Approach (assumed at the header base):** build `DefaultFile` and `KeylessFile` from shared constant pieces in `config.go`, so they differ only in the two format lines. `createConfig` takes `isNewStore`, the `isNew` that `createDir("")` returned in `Init`. Replace `config_test.go`'s `formatOneTemplate` helper with `KeylessFile`. Update `Init`'s doc comment.
**Prose guard:** fix every sentence in the docs that states what `init` writes for `format`. Find them with `grep -rni 'init.*format\|format.*init' docs/*.md README.md .capcom/knowledge`; `docs/format.md` Config ("`capcom init` writes `format: 3`") is one.
Depends on P3.2.
**Files:** internal/config/config.go; internal/config/config_test.go; internal/store/store.go; internal/store/store_test.go; internal/cli/init_test.go; docs/format.md; docs/migrating-from-beads.md; .capcom/knowledge/decisions/adr-0009/adr-0009-decision.md
**Read first:** internal/store/store.go — Init, createDir, createConfig, loadConfig; internal/config/config.go — DefaultFile, formatComment, keylessFormatComment, RaiseFormat, Parse; internal/config/config_test.go — formatOneTemplate, TestRaiseFormat_FormatOneTemplate_BecomesCurrentTemplate; internal/cli/init_test.go — newGitRepo, TestInit_WrittenConfig_DeclaresCurrentFormat; internal/cli/init.go — initText; internal/check/check.go — checkSupersession; internal/store/store_test.go — TestInit_ReRun_IsNoOpAndKeepsConfig
**Tests:** each fails on the pre-item tree:
- `store_test.go`: `TestInit_ExistingStoreWithoutConfig_WritesKeylessTemplateAtFormatOne`.
- `cli/init_test.go`: `TestInit_ExistingStoreWithoutConfig_StaysAtFormatOne`. It removes `config.yaml` from an initialised store, runs `init --merge-driver`, and asserts `config.yaml` equals `config.KeylessFile`. Then, with a Note carrying `retired: true`, `capcom check` prints the exact line `.capcom/knowledge/<note>.md:3: retired needs store format 3, but the store declares format 1: set format: 3 in .capcom/config.yaml, or run a capcom write verb that raises it`.
- `config_test.go`: `KeylessFile` parses to format 1 and raises to `DefaultFile`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/config/`
- `GOMEMLIMIT=2GiB go test ./internal/store/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Init'`
- `go vet ./internal/config/ ./internal/store/ ./internal/cli/`
**Closes:** missing-config-format
- `capcom check`
**Commit:** `fix(store): keep an existing store at its format when init writes config.yaml`

## P3.4 check gates the Issue related key on the store format — ✅ DONE (2026-10-13)

NOTES (2026-10-13): re-derived from "checkIssue, checkRelated, checkSupersession and CodeFormatTooOld live in internal/check/check.go" — after P2.13 they sit in issues.go, notes.go and problem.go, the files the item already lists; the shared message helper is checker.addFormatTooOld in problem.go.

**What:** Fixes F-17 (`check-related-format`), and completes A-07. A `related` key in a store declaring format 1 passes `check`, although every format-1 binary rejects it. This item emits the existing code `format-too-old` in one more place. No `check` JSON code changes.
**Regression guard.** The prose rule is every sentence listing what `capcom check` reports at an issue or Note key, the related section's sentence (which never says "format-too-old") included.
**Goal:** `check` reports `format-too-old` at an Issue's `related` key line (`RelatedLine()`) when the declared format is below `config.FormatRelated`. The message has the same shape as the Note keys': `related needs store format 2, but the store declares format <n>: set format: 2 in .capcom/config.yaml, or run a capcom write verb that raises it`. Both messages come from one helper.
**Approach (assumed at the header base):** extract the message in `checkSupersession` into a helper taking the key, the needed format and the declared format, and call it from `checkIssue` (issues.go) when `entry.issue.RelatedLine() > 0`. Update the `CodeFormatTooOld` doc comment, which names Note keys only.
**Prose guard:** update every sentence listing what `capcom check` reports at an issue or Note key. Find them with `grep -rn 'capcom check.*report\|format-too-old' docs/format.md README.md internal/check/*.go`; the `related` section's `capcom check` sentence and the Supersession one are among them.
Depends on P3.2.
**Files:** internal/check/issues.go; internal/check/notes.go; internal/check/problem.go; internal/check/check_test.go; internal/cli/check_test.go; docs/format.md
**Read first:** internal/check/check.go — checkIssue, checkRelated, checkSupersession, CodeFormatTooOld, issues; internal/issue/issue.go — RelatedLine; internal/check/check_test.go — TestRun_OneFixturePerProblemClass_ReportsPathLineAndCode, TestRun_RelatedNamingAnAlias_ReportsNothing, writeFiles, issueFile; internal/cli/check.go — newCheckCommand; internal/cli/related_test.go — writeStoreConfig; internal/cli/reindex_test.go — newIssueStore; internal/briefing/briefing.go — readHealth
**Tests:**
- `check_test.go`: `TestRun_RelatedBelowFormatTwo_ReportsFormatTooOldAtTheRelatedLine` covers a store with no config and a keyless one, and asserts the exact message. It fails on the pre-item tree (the bite check). The fixtures that carry `related:` without a config (the three "related …" cases of `TestRun_OneFixturePerProblemClass_ReportsPathLineAndCode` and `TestRun_RelatedNamingAnAlias_ReportsNothing`) gain `.capcom/config.yaml: "format: 2\n"`.
- `cli/check_test.go`: `TestCheck_RelatedInFormatOneStore_ReportsFormatTooOld` asserts the exact stdout line and exit 1.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Check|Prime|Related'`
- `go vet ./internal/check/ ./internal/cli/`
**Closes:** check-related-format; config-owns-format
**Commit:** `fix(check): report a related key in a store below format 2`

## P3.5 The text renderer sanitises everything it writes — ✅ DONE (2026-10-13)

NOTES (2026-10-13): re-derived from the assumption that P0.11 left sanitiser calls in flatten (issue_render.go) and the body writes of show and know show — P0.11 landed the single call at textRenderer.Render/RenderError and P2.30 moved it, unexported, into internal/cli/output, so no call site existed to remove and output.go, issue_render.go, show.go and know_show.go needed no change; the item reduces to its two named tests.

NOTES (2026-10-13): the text test feeds both a lone byte 0x9b and the rune U+009B; the JSON/KV test feeds only U+009B (valid UTF-8, so a JSON round trip can carry it), per the plan's rejected regression finding. Removing sanitizeTerminal from Render/RenderError makes all six text subtests fail.

**What:** First part of A-14 (`cli-output-package`). After P0.11 and P2.35, the terminal sanitiser lives in `internal/cli/output`, but command files call it site by site. This item makes sanitising part of the text renderer's interface. Ratified F-07 (binding): text output replaces a control character with a visible escape (the `\x1b` form); `--json` and `--kv` stay byte-for-byte unchanged.
**Regression guard.** There is no sanitiser alias to remove (see P2.30); the item's remaining scope stands.
**Goal:** in text mode, every string the renderer writes passes through the sanitiser: `Result.Text`, each Elsewhere line, each warning, and the failure line on stderr. The sanitiser is unexported in `internal/cli/output`, and no file of `internal/cli` outside that package calls it. `--json` and `--kv` output is unchanged, and `internal/cli/output` imports no `internal/cli` package.
**Approach (assumed at the header base):** in `textRenderer.Render` and `textRenderer.RenderError`, sanitise each string before it is written. Remove every sanitiser call from `internal/cli` itself, which P0.11 put in `flatten` (issue_render.go) and the body writes of `show` and `know show` (`grep -rn 'anitize' internal/cli --include=*.go`). Then unexport the sanitiser. The sanitiser is idempotent, because escaped text is printable, so moving the call does not change bytes. `flatten` keeps only its line-break folding.
Depends on P2.35; P0.11.
**Files:** internal/cli/output/output.go; internal/cli/output/output_test.go; internal/cli/issue_render.go; internal/cli/show.go; internal/cli/know_show.go
**Read first:** internal/cli/output.go — textRenderer.Render, textRenderer.RenderError, jsonRenderer.write, kvRenderer.Render, writeKVLine, Result; internal/cli/issue_render.go — flatten; internal/cli/terminal_test.go — TestSanitizeTerminal_EscapesControlsOnly, TestText_ControlCharactersFromFiles_AreEscapedAndJSONKeepsThem; internal/cli/output_alias_test.go — sanitiser alias; internal/cli/cli_test.go — TestTextRenderer_Render_WritesWarningsAfterResult
**Tests:**
- `output_test.go`: `TestTextRenderer_ControlCharacters_EscapedInEveryStream`. Its table feeds `\x1b[2J`, `\x9b` and `\x07` through Text, an Elsewhere entry, a warning and a failure, and asserts that TAB and LF are kept.
- `output_test.go`: `TestJSONAndKVRenderers_ControlCharacters_PassedVerbatim`.
- P0.11's CLI test stays green.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/output/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Show|List|Know|Dep'`
- `grep -rn 'anitize' internal/cli/*.go` prints nothing
- `go list -deps ./internal/cli/output | grep -x 'github.com/airiclenz/capcom/internal/cli'` prints nothing
- `go vet ./internal/cli/ ./internal/cli/output/`
**Commit:** `refactor(cli): sanitise text output in the renderer, not at each call site`

## P3.6 The exit-code decision lives in cli/output — ✅ DONE (2026-10-13)

NOTES (2026-10-13): added TestRendered_MarksACopy beside the planned TestExit_MapsEachOutcome, pinning that Rendered marks a distinct copy and leaves its argument unmarked

**What:** Completes A-14 (`cli-output-package`). `cli.Run` still decides the exit code itself, and `emit`, `check` and `fmt` set the rendered mark on a failure directly.
**Regression guard.** The Goal sentence and its grep cover non-test files; cli_test.go and init_test.go keep their `ExitCode()` assertions. The package comment in internal/cli/doc.go says output decides the exit code and `Run` only wires cobra and calls `output.Exit`.
**Goal:** `internal/cli/output` exports one function that turns a command's final error into the process exit code, given the Renderer:
- `nil` gives 0.
- A failure already rendered with its partial result keeps its code and is not written again.
- An error that is not an `*Error` becomes a usage failure (exit 2) and is rendered.
- Any other `*Error` is rendered and maps to 1, or 3 for `not_found` and `locked`.
- A render failure gives 1.
`cli.Run` calls this function once. The rendered mark can only be set through an `output` function. No file of `internal/cli` outside `output` calls `ExitCode()` or touches the mark. Exit codes and output are unchanged.
**Approach (assumed at the header base):** add `output.Exit(renderer Renderer, err error) int` beside the `Exit*` constants, and move into it the part of `Run` after `closeIndex`. `Run` keeps the cobra wiring, `closeIndex` and the `scanFormat` fallback for a nil renderer. Add `output.Rendered(failure *Error) *Error`, which returns a marked copy, and use it in `emit` (root.go), `newCheckCommand` (check.go) and the fmt runner (fmt.go). Unexport the mark field P2.35 exported. `cmd/capcom` is unchanged.
Depends on P3.5.
**Files:** internal/cli/output/errors.go; internal/cli/output/errors_test.go; internal/cli/root.go; internal/cli/check.go; internal/cli/fmt.go; internal/cli/doc.go
**Read first:** internal/cli/root.go — Run, emit, scanFormat, closeIndex; internal/cli/errors.go — Error, ExitCode, isRendered, asError (output/errors.go after P2.29); internal/cli/check.go — newCheckCommand; internal/cli/fmt.go — formatRun.emit; internal/cli/output.go — Renderer, newRenderer, textRenderer; internal/cli/cli_test.go — TestApp_Emit_ResultWithoutKVRecordsRejectsKV, TestError_ExitCode_MapsCodes; internal/cli/init_test.go — TestStoreFailure_StoreCommandBeforeInit_IsNotInitialized; internal/cli/doc.go — package comment
**Tests:** `errors_test.go`: `TestExit_MapsEachOutcome`, a table of nil, rendered not_found, untyped, invalid, locked, usage, and a failing renderer, asserting the code and what was written. The whole `cli` package stays green.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/output/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/`
- `grep -rnE '\.ExitCode\(\)|[iI]sRendered' internal/cli/*.go | grep -v _test.go` prints nothing
- `go vet ./internal/cli/ ./internal/cli/output/ ./cmd/capcom/`
**Closes:** cli-output-package
**Commit:** `refactor(cli): decide the exit code behind the cli/output interface`

## P3.8 Tree.Claimant names any file holding a name — ✅ DONE (2026-10-13)

NOTES (2026-10-13): consequential edit — internal/tree/doc.go: made necessary by claims.go now holding Claimant; its file-map line names it.

NOTES (2026-10-13): where orphan directories nest (an orphan inside a repeated id's directory), Claim.Dir is the outermost reported one, matching the first problem nodestore's orphanClaimFailure names today; an orphan directory gone since Load lists as empty instead of failing as findNodeFile's WalkDir does.

NOTES (2026-10-13): ClaimKind constants are LiveClaim, OrphanClaim, DuplicateClaim and UnparsedClaim, starting at 1 so the zero Claim has no kind; repeated-id files are tried in LeftOut order (id, then path) rather than nodestore's problem order.

**What:** Recast at the regression check (2026-10-13). First part of A-05 (`node-layout-claims`). Per review A-05, the claim concept moves into `tree`, generalising F-10(b). A name may be held by a live Node, a dropped duplicate, a file inside an orphan directory or an unparseable file, and today each nodestore verb scans for these itself.
**Regression guard.** `except` names the moved node's file path (not its id), so an orphan or unparseable file with the same id still claims; add a claims_test case for it. Precedence lists orphan before duplicate, matching P1.7's `claimFailure` order (message unchanged for users). `Claim.Dir` is the Path of the `ErrOrphanDirectory` problem the file lies under; add a nested-orphan claims_test case. `tree.Load`'s signature stays unchanged; `Claimant` takes the recovering AliasFunc.
**Goal:** `func (t *Tree) Claimant(name string, except string, recover AliasFunc) (Claim, bool, error)` returns the first file holding `name`, by its file name or as an alias, in this binding precedence:
1. a live Node (id or alias);
2. a node file anywhere below an orphan directory, the dropped duplicates' subtrees included;
3. a file `Load` dropped as a repeated id (its aliases read with Load's AliasFunc);
4. a node file among all of the above from which `recover` reads `name`.
A holder whose file path is `except` (the moved node's file path) is skipped; `""` skips none. `Claim` carries `Path`, `Holder` (the file's id), `IsAlias`, `Kind` (live, duplicate, orphan or unparsed) and, for an orphan, `Dir`, the Path of the `ErrOrphanDirectory` problem the file lies under. Orphan directories are listed with the same entry rules `Walk` applies, from one shared helper: hidden names skipped, only `.md` regular files.
**Approach (assumed at the header base):** `Load` keeps its AliasFunc, its dropped nodes (P1.8 drops both files of a repeated id) and the orphan directories among its problems on `Tree`, as unexported fields in tree.go. `Claimant` and `Claim` go in claims.go (P2.1). Extract the walker's skip test into a helper in walk.go that both `treeWalker.walk` and the orphan listing call. `tree.Load`'s signature stays unchanged; `Claimant` takes the recovering AliasFunc.
Depends on P2.1; P1.8.
**Files:** internal/tree/tree.go; internal/tree/claims.go; internal/tree/walk.go; internal/tree/claims_test.go
**Read first:** internal/nodestore/create.go — claimFailure, orphanClaimFailure, unparsedClaimFailure, findUnparsedAliasHolder, findNodeFile; internal/tree/tree.go — Load, Tree; internal/tree/claims.go (P2.1) — indexIDs, checkDroppedAliases, recordDuplicate; internal/tree/walk.go (P2.1) — treeWalker.walk, leftOutFileOf; internal/tree/resolve.go — Lookup, Resolve; internal/nodestore/kind.go — ReadAliases, RecoverAliases; internal/nodestore/move.go — Move (rename claim steps); internal/nodefile/read.go (P2.2) — IsNotRegular
**Tests:** `claims_test.go` covers each Kind:
- by name and by alias;
- precedence when two holders exist;
- `except` skipping the moved node's own alias;
- a free name giving `false`;
- an AliasFunc error returned unchanged;
- hidden and non-`.md` entries inside an orphan directory never claiming.
- an orphan or unparseable file with the excepted id still claims (`except` is the moved node's path);
- a nested orphan (`stray/sub.md`, `stray/sub/auth-flow.md`) claims with `Dir` the reported orphan directory `stray`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `go vet ./internal/tree/`
**Commit:** `refactor(tree): add Tree.Claimant for every file that holds a name`

## P3.9 nodestore claims names through Tree.Claimant — ✅ DONE (2026-10-13)

NOTES (2026-10-13): claimRefusal takes the refused id as well as the Claim (`claimRefusal(id string, claim tree.Claim) *ExistsError`), since a Claim does not carry the name it holds; Create and Move each call nodes.Claimant inline, with no claimFailure wrapper left.

NOTES (2026-10-13): the acceptance grep `grep -rn 'filepath.WalkDir' internal/nodestore/` still prints one line, the pre-existing test helper `nodeFiles` in internal/nodestore/create_test.go (it snapshots a directory for the write-nothing tests); production code has none (`grep -rn --exclude='*_test.go' 'filepath.WalkDir' internal/nodestore/` prints nothing). The helper is outside the item's Files and was left as it is.

NOTES (2026-10-13): Move now refuses a rename onto an id an unparseable file holds before triage, so such a holder in a tree whose triage would block is reported as the *ExistsError rather than the blocking tree.Errors, as the regression guard states; Move's doc step list says so and is renumbered (the claim is step 4, triage 5).

**What:** Second part of A-05. Create and Move shrink to load, resolve, claim, triage and write.
**Regression guard.** `claimRefusal` keeps four wordings: the duplicate Kind uses P1.7's "left out for the duplicate id" text with Holder the duplicate id, so P1.7's `TestCreate_AliasOfDroppedDuplicateIDFile_FailsNamingTheFile` stays green. The dropped-duplicate scan and its test are P1.7's (`duplicateClaimFailure`). Move's one Claimant call runs before triage, as stated: an unparsed holder in a tree with triage problems is now refused as `ExistsError` before triage's tree.Errors; Move's doc step list says so. Per P3.8, `except` is the moved node's file path.
**Goal:** `Kind.Create` and a renaming `Kind.Move` refuse a taken id through one `Tree.Claimant` call each. `nodestore` keeps no walk of its own and no per-holder claim scan. Every refusal keeps its `*ExistsError` path, holder and message text, mapped from `Claim.Kind`. Create's and Move's doc comments list the claim as one numbered step.
**Approach (assumed at the header base):**
- Delete `claimFailure`, `orphanClaimFailure`, `unparsedClaimFailure`, `findUnparsedAliasHolder`, `findNodeFile`, `hiddenNamePrefix` and the dropped-duplicate scan P1.7 added (`duplicateClaimFailure`) (claim.go after P2.11).
- Add one `claimRefusal(claim tree.Claim) *ExistsError` that keeps today's four wordings: `existsError` for a live holder, P1.7's "left out for the duplicate id" for a duplicate holder, "inside the orphan directory", and "whose file does not parse: fix it first". It is fed `k.RecoverAliases`.
- Create calls Claimant with except `""` where `claimFailure` ran.
- Move calls it on a rename with except the moved node's file path (P3.8), before triage. `planMove`'s `TakenError` still refuses another live holder first.
- **Prose guard:** fix every numbered step list in the doc comments that names the claim refusals (`grep -n 'orphan directory\|does not parse' internal/nodestore/*.go`).
Depends on P3.8; P2.11; P2.9; P1.23; P1.24.
**Files:** internal/nodestore/claim.go; internal/nodestore/create.go; internal/nodestore/move.go; internal/nodestore/doc.go
**Read first:** internal/nodestore/create.go — Create, claimFailure, orphanClaimFailure, unparsedClaimFailure, findNodeFile, nodeID; internal/nodestore/move.go — Move, planFailure; internal/nodestore/errors.go — existsError, ExistsError; internal/nodestore/create_test.go — TestCreate_Refusals_WriteNothing; internal/nodestore/move_test.go — TestMove_Refusals_MoveNothing; internal/cli/mv_test.go — TestMv_OntoIDInsideOrphanDirectory_FailsNamingTheFile, TestMv_OntoAliasOfUnparseableIssue_FailsExistsNamingTheFile; internal/cli/write_test.go — TestCreate_AliasOfUnparseableIssue_FailsExistsNamingTheFile
**Tests:** none new. These pin the behaviour:
- `create_test.go`'s `TestCreate_Refusals_WriteNothing` and `move_test.go`'s `TestMove_Refusals_MoveNothing`;
- P1.7's dropped-duplicate refusal test (`TestCreate_AliasOfDroppedDuplicateIDFile_FailsNamingTheFile`);
- the CLI tests `TestMv_Onto*`, `TestKnowMv_Onto*`, `TestKnowAdd_*Orphan*`, `TestKnowAdd_AliasOfUnparseableNote_*` and `TestCreate_AliasOfUnparseableIssue_*`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Mv|KnowAdd|Create'`
- `grep -rn 'filepath.WalkDir' internal/nodestore/` prints nothing
- `go vet ./internal/nodestore/`
**Commit:** `refactor(nodestore): refuse taken ids through Tree.Claimant`

## P3.10 tree exports the node-file layout — ✅ DONE (2026-10-13)

NOTES (2026-10-13): re-derived from the assumption that tree's id-from-path and children-directory derivations sit only in tree.go, move.go and remove.go: P3.8 added claims.go with its own nodeIDOf and indexIDs' TrimSuffix, so nodeIDOf is replaced by IDOf (and deleted) and indexIDs uses ChildrenDir; nodestore's nodeID is already gone after P3.9, leaving isBlockingProblem as its only use.

**What:** Third part of A-05. The node-file layout rules ("id = base name without `.md`", "children directory = path without `.md`") move behind `tree` exports, and tree and nodestore use them.
**Goal:** `tree.IDOf(path string) string` and `tree.ChildrenDir(path string) string` are defined beside `PathFor` in tree.go and work on OS and forward-slash paths alike. `tree` (`PathFor`, `PlanMove`'s renames, `PlanRemove`) and `nodestore` (`isBlockingProblem` and any other id-from-path) derive the layout only through them. `nodestore` defines no `.md` constant.
**Approach (assumed at the header base):** replace each `strings.TrimSuffix(…, nodeExtension)` in tree/move.go and tree/remove.go, and nodestore's `nodeFileExtension` uses after P3.9, by symbol. tree's own `nodeExtension` stays as the single definition.
Depends on P3.9.
**Files:** internal/tree/tree.go; internal/tree/move.go; internal/tree/remove.go; internal/tree/tree_test.go; internal/nodestore/load.go; internal/nodestore/claim.go
**Read first:** internal/tree/tree.go — PathFor, nodeExtension; internal/tree/move.go — Move.planRenames; internal/tree/remove.go — PlanRemove; internal/nodestore/create.go — isBlockingProblem, nodeID (load.go/claim.go after P2.11); internal/tree/tree_test.go — package tree_test
**Tests:** `tree_test.go`: `TestLayout_IDOfAndChildrenDir`, covering an absolute path, a relative forward-slash path and a nested path.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `grep -rn '= "\.md"' internal/nodestore/` prints nothing
- `go vet ./internal/tree/ ./internal/nodestore/`
**Commit:** `refactor(tree): export the node-file layout as IDOf and ChildrenDir`

## P3.11 cli, index and knowledge use the tree layout — ✅ DONE (2026-10-13)

NOTES (2026-10-13): index's leaveOutRepeatedIDSubtrees derives a children directory, not an id, so it uses tree.ChildrenDir rather than the regression guard's tree.IDOf; nodeFileID keeps its "is a node file" test by comparing tree.IDOf of the base name with the base name itself. internal/index/refresh.go and problems.go held no hand derivation and are unchanged.

**What:** Completes A-05 (`node-layout-claims`). Its nodefile half, the guarded reader and the P1.23/P1.24 primitives, landed with P2.9.
**Regression guard.** P1.5 replaced `unusableClaim`'s file-name test with a claims lookup, so the rule is: every use of index's `nodeFileExtension` (`grep -rn nodeFileExtension internal/index`), P1.6's walk.go claims and P1.5's refused-file id included, goes through `tree.IDOf`.
**Goal:** no package outside `tree` defines a `.md` constant or derives an id or children directory from a path by hand.
- `cli`'s `movedWarnings` uses `tree.ChildrenDir`, and `cli`'s `nodeFileExtension` (errors.go, or wherever P2.35 left it) is gone.
- every use of `index`'s `nodeFileExtension` (`grep -rn nodeFileExtension internal/index`) goes through `tree.IDOf`, and the constant is gone.
- `knowledge`'s `validateSupersededBy` uses `tree.IDOf(path)`, and `noteExtension` is gone.
**Approach (assumed at the header base):** a mechanical replacement by symbol. The `index` paths are forward-slash, which `IDOf` handles. `elsewhere`'s `issueExtension` reads git paths and stays.
Depends on P3.10; P2.35.
**Files:** internal/cli/errors.go; internal/cli/mv.go; internal/index/query.go; internal/knowledge/note.go; internal/index/walk.go; internal/index/refresh.go; internal/index/problems.go
**Read first:** internal/cli/mv.go — app.movedWarnings, movedPath; internal/cli/errors.go — nodeFileExtension; internal/index/query.go — unusableClaim, nodeFileExtension; internal/index/walk.go — walkIssues; internal/knowledge/note.go — validateSupersededBy, noteExtension; internal/cli/know_mv.go — movedWarnings caller
**Tests:** none new: `TestMv_*` (moved warnings), `index`'s resolve tests and `knowledge`'s `TestValidate_SupersededBy_ReportsBadEntriesAtTheirLines` pin it.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Mv'`
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `GOMEMLIMIT=2GiB go test ./internal/knowledge/`
- `grep -rn '= "\.md"' internal/nodestore internal/cli internal/index internal/knowledge --include=*.go` prints nothing
- `go vet ./internal/cli/ ./internal/index/ ./internal/knowledge/`
**Closes:** node-layout-claims
**Commit:** `refactor(cli): derive node-file paths through the tree layout`

## P3.13 Characterise Move's reference rewrites through Kind — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the ported alias `apogee-kk0.4` is replaced by kebab aliases (`old-login`, `legacy-name`) so the tests do not lean on a legacy id form; the alias-chain test checks the first id still resolves through a further Kind.Move (OldID third-name), since Kind has no resolver-free lookup.

NOTES (2026-10-13): the unparseable-moved-issue case asserts Kind-level types (invalid id and own subtree a lineless *FileError, taken id an *ExistsError, valid rename a *FileError at the parse line) and that the tree is unchanged; the knowledge taken-id-first and swapped-after-Load cases are left to P3.15's refs_test.go as planned; makeFIFO is unused until P3.15.

**What:** First part of A-04 (`reference-rewrite-engine`), as the characterisation tests the review asks for. These tests pin, at `Kind.Move` level, what `issue.PlanMove` and `knowledge.PlanMove`'s own tests pin today, so those planners can be deleted later. They pass on the pre-item tree. The U-02 and F-23 cases are not here; they are P3.15's bite tests.
**Tests:** new `internal/nodestore/move_refs_test.go` (`package nodestore`), ported from `internal/issue/move_test.go`, `internal/knowledge/move_test.go` and `knowledge_test.go`'s `TestPlanMove_*`.
- Issues: a rename rewrites other issues' `deps` and `related` entries naming the old id or an alias. A second rename keeps the alias chain. A rename back to the issue's own alias swaps it. A re-parent rewrites no other file. An unparseable moved issue fails after the destination check (taken id first).
- Notes: a rename records the old id as an alias and rewrites other Notes' `superseded-by`, with `[x, y]` renamed from x to y giving `[y]`. A re-parent leaves the bytes. An unparseable other Note joins `Moved.Unread`.
- Also add `internal/nodestore/fifo_unix_test.go` and `fifo_windows_test.go`, copied from `internal/issue`, for P3.15.
Depends on P3.11; P2.11; P1.21.
**Files:** internal/nodestore/move_refs_test.go; internal/nodestore/fifo_unix_test.go; internal/nodestore/fifo_windows_test.go
**Read first:** internal/nodestore/move.go — Kind.Move, planFailure; internal/nodestore/move_test.go — newMoveIssues, treeContent, movedEqual; internal/nodestore/kind_test.go — newStore, writeNodeFile, unparseable; internal/nodestore/edit_test.go — readFile; internal/issue/move_test.go — TestPlanMove_UnparseableIssueFile_DestinationCheckedFirst, TestPlanMove_RenameBackToOwnAlias_SwapsIt, moveFile; internal/knowledge/knowledge_test.go — TestPlanMove_Rename_RewritesOtherNotesSuccessors, TestPlanMove_UnparseableOtherNote_JoinsUnread; internal/knowledge/move_test.go — TestPlanMove_ReparentOnly_LeavesTheFileBytes; internal/issue/fifo_unix_test.go — makeFIFO (package issue_test: the copy takes package nodestore)
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `go vet ./internal/nodestore/`
**Commit:** `test(nodestore): characterise move rewrites of both tree kinds`

## P3.14 Characterise Remove's reference drops through Kind — ✅ DONE (2026-10-13)

NOTES (2026-10-13): Removed exposes no superseded-by drops (P3.17 adds that), so the Note drop test asserts exact file bytes of the holders and the untouched bystander instead of the ported SupersededByDropped list; issue files reuse move_refs_test.go's refIssue in place of the ported removeFile helper.

**What:** Second part of A-04, again characterisation only. These tests pin `issue.PlanRemove`'s and `knowledge.PlanRemove`'s behaviour at `Kind.Remove` level, and pass on the pre-item tree.
**Tests:** new `internal/nodestore/remove_refs_test.go`, ported from `internal/issue/remove_test.go`, `internal/knowledge/remove_test.go` and `knowledge_test.go`'s `TestPlanRemove_*`.
- Issues: `--drop-deps` drops `deps` and `related` entries naming the id or an alias, with the holders in tree order. Dependents are refused in path order, with the exact `DependentsError` message `issue <id> is a dependency of <ids>`. A stale `ifMatch` is a `*ConflictError` before the dependents refusal. An unparseable own file is removed, and its rawfm-recovered alias still drops dependents' entries. Unparseable other files join `Removed.Unread`.
- Notes: other Notes' `superseded-by` entries naming the id or an alias are dropped, asserted on the file bytes. An unparseable other Note joins `Unread`. An unparseable own Note is removed.
Depends on P3.13.
**Files:** internal/nodestore/remove_refs_test.go
**Read first:** internal/nodestore/remove.go — Kind.Remove, removalFailure, planIssueRemove, planNoteRemove; internal/nodestore/remove_test.go — newRemoveIssues, assertDependents, assertRemoveConflict, TestRemove_Refusals_RemoveNothing; internal/issue/remove_test.go — TestPlanRemove_DropDeps_DropsDepsAndRelatedNamingIdOrAlias, TestPlanRemove_Dependents_RefusedInPathOrderAndNothingChanges, TestPlanRemove_UnparseableFiles_RemovesOwnAndListsOthersAsUnread; internal/knowledge/knowledge_test.go — TestPlanRemove_DropsSuccessorEntriesNamingTheNote, TestPlanRemove_UnparseableOtherNote_JoinsUnread; internal/knowledge/remove_test.go — TestPlanRemove_UnparseableNote_RemovesItsFile
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `go vet ./internal/nodestore/`
**Commit:** `test(nodestore): characterise remove drops of both tree kinds`

## P3.15 One rename engine over every reference field — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the engine is the Kind method `planNodeMove` (with `renameOwnRefs`, `planRefRewrite` and `renameRefs`) in refs.go, returning a `nodeMove` that replaces `plannedMove`; it calls `nodes.PlanMove(…, k.codec.nounPhrase)` itself. `refField` carries `key`, `get`, `set` and `isSelfRefused` only: `blocksRemoval` is left for P3.16, which first reads it.

NOTES (2026-10-13): the moved node's own file is now read and parsed before any other node file on both kinds (the issue planner used to read in tree order), so an unparseable moved issue fails before a hard read failure of an earlier other issue file; no test pins the old order.

NOTES (2026-10-13): beyond the planned tests, added `TestMove_RenameIssueOntoItsOwnEntry_KeepsTheSelfEntry` (issue self-entries kept), `TestRenameRefs` and `TestCodecs_ListTheirReferenceFields` (the Codecs' reference fields); `issue.PlanMove` and `knowledge.PlanMove` stay in their packages, now unused by nodestore.

**What:** Third part of A-04. It fixes U-02 (`rename-dup-deps`): `capcom mv x y` turns `deps: [x, y]` into `[y, y]`, and `related` the same way. It also fixes F-23 (`note-self-supersede`): `know mv auth-v1 --id auth` on a Note with `superseded-by: [auth]` writes a Note that names itself, which then drops out of the index. Following review A-04, the moved node's own fields go through the same rename op. A self-entry is dropped, not refused: F-23 is fixed by dropping the moved Note's own superseded-by entry that names its new id; Issue deps/related keep self-entries as today.
**Regression guard.** F-23 is fixed by dropping the moved Note's own superseded-by entry that names its new id (not by refusing the move); Issue deps/related keep self-entries as today. A file (other or own) is rewritten, and its field deduped, only when one of its entries names the old id or an alias. The Goal's "never calls" clause is checked by an Acceptance grep.
**Goal:**
- Each Codec lists its reference fields: issues `deps` and `related`, Notes `superseded-by`.
- `Kind.Move` plans the moved node's own rewrite (aliases through `tree.RenameAliases`, plus its reference fields) and, on a rename, every other node's, through one engine in `nodestore`. It never calls `issue.PlanMove` or `knowledge.PlanMove`.
- A file (other or own) is rewritten only when one of its entries names the old id or an alias; the rename replaces each such entry with the new id and keeps only the first entry equal to it.
- On the moved node's own self-refused fields (`superseded-by`), an entry naming the new id is dropped. Issue `deps` and `related` keep self-entries, as today.
**Approach (assumed at the header base):**
- New `refs.go` holds an unexported `refField[T]{key, get, set, blocksRemoval, isSelfRefused}`, `renameRefs` (dedupe on output, as `renameSuccessors` does) and the move planner.
- The move planner reads each file with `nodefile.ReadNodeFile`. A refused or unparseable other file joins Unread; the moved node's own file fails. Each change becomes a `tree.Rewrite` whose `Original` is the bytes read, added through `AddRewrite`, which keeps P1.21's compare-and-swap.
- Codec gains `refs`, `setAliases` and `nounPhrase` ("an issue", "a note"), and loses `planMove`, `plannedMove`, `planIssueMove` and `planNoteMove`. Move calls `nodes.PlanMove(…, k.codec.nounPhrase)`, then the engine. Update doc.go.
Depends on P3.14.
**Files:** internal/nodestore/refs.go; internal/nodestore/refs_test.go; internal/nodestore/codec.go; internal/nodestore/move.go; internal/nodestore/move_refs_test.go; internal/nodestore/doc.go
**Read first:** internal/nodestore/codec.go — Codec, planIssueMove, planNoteMove, plannedMove; internal/nodestore/move.go — Kind.Move, planFailure; internal/issue/move.go — planRewrite, renameDeps; internal/knowledge/move.go — PlanMove, planSuccessorRewrites, renameSuccessors; internal/tree/move.go — Tree.PlanMove, RenameAliases, Rewrite; internal/tree/plan.go — Plan.AddRewrite; internal/nodestore/kind.go — isRefusedRead; internal/knowledge/note.go — validateSupersededBy
**Tests:**
- `move_refs_test.go`: `TestMove_RenameOntoDanglingEntry_KeepsOneEntry`, where issue `a` has `deps: [x, y]` and `related: [x, y]`, `y` dangling, and after the move both are `[y]`.
- `move_refs_test.go`: `TestMove_NoteRenamedToItsOwnSuccessor_DropsTheEntryAndStaysValid`.
- Both must fail on the pre-item tree (the bite check).
- `refs_test.go`: the swapped-after-Load cases (symlink, FIFO, file over 1 MiB) of the issue and knowledge move tests, ported to the engine on a loaded tree.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Mv|KnowMv'`
- `go vet ./internal/nodestore/`
**Closes:** rename-dup-deps; note-self-supersede
- `grep -rnE 'issue\.PlanMove|knowledge\.PlanMove|planMove|plannedMove' internal/nodestore/` prints nothing
**Commit:** `fix(nodestore): rename references of both tree kinds through one engine`

## P3.16 One drop engine and one etag check for Remove — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the engine is the Kind method `planNodeRemoval` (with `planRefDrop`, `dropRefs` and `dependentsError`) in refs.go, returning a `nodeRemoval` whose `dropped` map and `rewritten` paths replace `plannedRemoval`; `refField` gains `blocksRemoval`, set on `deps` only, and the field keys are the exported `RefDeps`, `RefRelated` and `RefSupersededBy`. `Removed.Dropped` only holds keys with drops.

NOTES (2026-10-13): `removalPaths` now takes the rewritten files straight from the plan instead of looking the dropped ids up in the tree, so a holder the load left out for a repeated id is named too.

NOTES (2026-10-13): beyond the planned swapped-after-Load ports, added `TestPlanNodeRemoval_Note_StaleIfMatch_ConflictsBeforeReadingOtherNotes`, which pins the Note side's new etag-before-drops order on the engine (a permission failure cannot reach Remove itself, because the tree load already reads every file). `issue.PlanRemove` and `knowledge.PlanRemove` stay in their packages, now unused by nodestore (P3.18, P3.19).

NOTES (2026-10-13): consequential edit — internal/nodestore/doc.go: made necessary by refs.go now holding the removal planner too.

**What:** Fourth part of A-04. Remove's drops, its etag check and its dependents refusal move into the engine, and drops are reported keyed by field.
**Regression guard.** The Goal is checked by an Acceptance grep; with the old planners still wired the tests pass.
**Goal:**
- `Kind.Remove` plans every drop through the `refs.go` engine and checks `ifMatch` once, in `nodestore`, before any dependents refusal for both kinds.
- Holders of a `blocksRemoval` field (`deps`) are dependents. They are refused with `*DependentsError`, whose message is unchanged, ids in path order, unless shouldDropDeps.
- `Removed.Dropped` maps each field key (exported `RefDeps`, `RefRelated`, `RefSupersededBy`) to the ids of the nodes whose entries were dropped, in tree order.
- `rm`'s text, `--json` and `--kv` output is byte-identical.
**Approach (assumed at the header base):**
- The engine reads the node's own file once, for the etag and, when the node has no Aliases, for `rawfm.Aliases`.
- Delete Codec's `planRemove`, `plannedRemoval`, `planIssueRemove` and `planNoteRemove`, and the translation of `issue.ETagMismatchError` and `issue.DependentsError` in `removalFailure`.
- `removalPaths` takes the rewrites' paths.
- The Note side checks its etag before its drops, as issues already do.
- Value shape: `Removed.DepsDropped` and `RelatedDropped` become `Dropped`. The producer is `Kind.Remove`. The consumers are `cli/rm.go` (`newRmCommand` → `issueRemoveResult`), `remove_test.go` (`TestRemove_DropDeps_…`), `remove_refs_test.go` and `removalPaths`; grep `DepsDropped\|RelatedDropped` at write time.
Depends on P3.15.
**Files:** internal/nodestore/refs.go; internal/nodestore/refs_test.go; internal/nodestore/remove.go; internal/nodestore/codec.go; internal/nodestore/remove_test.go; internal/nodestore/remove_refs_test.go; internal/cli/rm.go
**Read first:** internal/nodestore/remove.go — Kind.Remove, removalFailure, removalApplyFailure, removalPaths, planIssueRemove, planNoteRemove, DependentsError; internal/issue/remove.go — PlanRemove, Removal.planDrops, DependentsError.Error; internal/knowledge/remove.go — PlanRemove, Removal.planDrops; internal/cli/rm.go — removeIssue, issueRemoveResult, issueRemovalFailure; internal/nodestore/remove_test.go — TestRemove_DropDeps_RemovesTheIssueAndItsDependentsDeps, TestRemove_Refusals_RemoveNothing; internal/cli/rm_test.go — TestRm_JSON_ReportsRelatedDropped; internal/cli/know_rm_test.go — TestKnowRm_DependentWriteFails_RollsBackNamingTheDependentFile
**Tests:** port the swapped-after-Load cases of the issue and knowledge remove tests to `refs_test.go`. P3.14's tests move to `Dropped` and stay green, and so do `TestRm_JSON_ReportsRelatedDropped` and the other `TestRm_*` tests.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Rm|KnowRm'`
- `go vet ./internal/nodestore/ ./internal/cli/`
- `grep -rnE 'issue\.(PlanRemove|RemoveOptions|ETagMismatchError|DependentsError)|knowledge\.PlanRemove|planRemove|plannedRemoval' internal/nodestore/` prints nothing
**Commit:** `refactor(nodestore): drop references and check the etag through one engine`

## P3.17 know rm reports the superseded-by entries it dropped — ✅ DONE (2026-10-13)

**What:** Fixes F-24 (`know-rm-report-drops`). `know rm adr-0011` rewrites other Notes' `superseded-by` silently, and `--json` consumers cannot see it.
**Regression guard.** `noteRemoveResult` stores `Data: &noteRemoveData{...}` (a pointer) and `refreshAgentsAfterWrite` asserts `*noteRemoveData` beside its map branch, which know add/mv/update keep. This supersedes the archived plan "2026-10-12 - 02 - glossary-and-decision-notes-plan.md" item 10 note that know rm's output keeps its id/path shape: the change is additive.
**Goal:** `capcom know rm` reports the Notes whose `superseded-by` entries it dropped, mirroring `rm`'s `deps_dropped`. The `--json` data has `id`, `path`, `superseded_by_dropped` (always present, `[]` when none) and `agents` when the block was refreshed. The text adds `superseded_by_dropped: [a, b]` after `path:`, only when the list is non-empty. `know rm` still has no `--kv`.
**Approach (assumed at the header base):** `noteRemoveResult` takes `removed.Dropped[nodestore.RefSupersededBy]`, and its data becomes a struct `noteRemoveData` with `agents` omitempty. `refreshAgentsAfterWrite` (know_add.go after P2.21) sets that field on a `*noteRemoveData` as it sets the map key today. Value shape: know rm's `Data` changes from a map to a struct. Its producer is `noteRemoveResult`, and its consumers are `refreshAgentsAfterWrite` and `know_rm_test.go`'s `data["agents"]` assertion.
**Prose guard:** add the key wherever the docs list what `know rm` or `rm` report (`grep -n 'related_dropped\|know rm' docs/format.md README.md`).
Depends on P3.16; P2.21.
**Files:** internal/cli/know_rm.go; internal/cli/know_add.go; internal/cli/know_rm_test.go; docs/format.md
**Read first:** internal/cli/know_rm.go — removeNote, noteRemoveResult, kindNoteRemove; internal/cli/know.go (know_add.go after P2.21) — refreshAgentsAfterWrite, noteWriteResult; internal/cli/rm.go — issueRemoveResult, issueRemoveData; internal/cli/know_rm_test.go — TestKnowRm_TopLevel_RefreshesTheAgentsBlock, TestKnowRm_SupersededByTarget_DropsTheLinks, TestKnowRm_Leaf_RemovesTheNote; internal/cli/know_test.go — newKnowStore; docs/format.md — `capcom know rm` bullet (Knowledge verbs)
**Tests:**
- `know_rm_test.go`: `TestKnowRm_SupersededByTarget_ReportsTheDroppedHolders` asserts the exact text line `superseded_by_dropped: [git-fixtures, testing-rules]` and the `--json` list.
- `know_rm_test.go`: `TestKnowRm_NoSuccessorLinks_ReportsAnEmptyList` asserts `[]` in `--json` and no text line.
- Both must fail on the pre-item tree (the bite check). The test asserting `data["agents"] == blockRefreshed` stays green.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'KnowRm'`
- `go vet ./internal/cli/`
**Closes:** know-rm-report-drops
**Commit:** `fix(cli): report the superseded-by entries know rm dropped`

## P3.18 Delete the issue move and remove planners — ✅ DONE (2026-10-13)

NOTES (2026-10-13): test mapping, issue -> nodestore/tree: TestPlanMove_RenameTwice_KeepsAliasChain -> nodestore TestMove_RenameIssueTwice_KeepsTheAliasChain; TestPlanMove_RenameBackToOwnAlias_SwapsIt -> TestMove_RenameIssueBackToItsOwnAlias_SwapsIt; TestPlanMove_Rename_RewritesDepsInOtherFiles -> TestMove_RenameIssue_RewritesOtherIssuesDepsNamingTheIdOrAnAlias; TestPlanMove_Rename_RewritesRelatedInOtherFiles -> TestMove_RenameIssue_RewritesOtherIssuesRelatedNamingTheIdOrAnAlias; TestPlanMove_UnparseableIssueFile_DestinationCheckedFirst -> TestMove_UnparseableMovedIssue_DestinationCheckedFirst; TestPlanMove_Rename_NodeFileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt -> TestPlanNodeMove_RenameIssue_FileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt; TestPlanRemove_DropDeps_DropsDepsAndRelatedNamingIdOrAlias -> TestRemove_DropDeps_DropsDepsAndRelatedNamingTheIdOrAnAlias; TestPlanRemove_Dependents_RefusedInPathOrderAndNothingChanges -> TestRemove_Dependents_RefusedInPathOrderAndNothingChanges; TestPlanRemove_StaleIfMatch_ComesBeforeTheDependentsRefusal -> TestRemove_StaleIfMatch_ConflictsBeforeTheDependentsRefusal; TestPlanRemove_UnparseableFiles_RemovesOwnAndListsOthersAsUnread -> TestRemove_UnparseableIssueFiles_RemovesOwnAndListsOthersAsUnread; TestPlanRemove_NodeFileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt -> TestPlanNodeRemoval_Issue_FileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt; TestPlanRemove_ApplyFails_RestoresRewritesChildrenDirAndFile -> tree TestRemovalApply_FileRemovalFails_RestoresChildrenDirAndRewrite + TestRemovalApply_LaterStepFails_RestoresFileBytesAndMode (the regression guard's cited pair; the deleted test's 0600 mode check on the restored dependent rewrite has no exact counterpart there).

NOTES (2026-10-13): no doc comment in internal/issue named the deleted planners and nothing outside the package called them; no other file needed editing.

**What:** Fifth part of A-04. With both engines in `nodestore`, the issue planners and their tests have no caller.
**Regression guard.** The NOTES mapping cites tree/remove_test.go `TestRemovalApply_FileRemovalFails_RestoresChildrenDirAndRewrite` and `TestRemovalApply_LaterStepFails_RestoresFileBytesAndMode` for `TestPlanRemove_ApplyFails_RestoresRewritesChildrenDirAndFile`, which no P3.13–P3.16 item ports.
**Goal:** `internal/issue` holds no move or remove planner: no `PlanMove`, `PlanRemove`, `Move`, `Removal`, `RemoveOptions`, `ETagMismatchError`, `DependentsError`, `renameDeps`, `dropEntries` or `isRefusedRead`. Every case of their deleted tests has a counterpart in P3.13–P3.16's `nodestore` tests; record the mapping as a NOTES line.
**Approach (assumed at the header base):** delete `move.go` and `remove.go` with their tests, and the FIFO helpers only those tests used. Fix any doc comment in `internal/issue` that names the deleted planners (`grep -rn 'PlanMove\|PlanRemove' internal/issue`).
Depends on P3.16.
**Files:** internal/issue/move.go; internal/issue/remove.go; internal/issue/move_test.go; internal/issue/remove_test.go; internal/issue/fifo_unix_test.go; internal/issue/fifo_windows_test.go
**Read first:** internal/issue/move.go — PlanMove, renameDeps; internal/issue/remove.go — PlanRemove, DependentsError, isRefusedRead; internal/issue/remove_test.go — TestPlanRemove_ApplyFails_RestoresRewritesChildrenDirAndFile; internal/issue/move_test.go — newMoveTree, loadMoveTree, unreadSwaps; internal/nodestore/codec.go — planIssueMove; internal/nodestore/remove.go — planIssueRemove, removalFailure; internal/tree/remove_test.go — TestRemovalApply_FileRemovalFails_RestoresChildrenDirAndRewrite
**Tests:** none new; the deleted planners' tests go with them, and the nodestore engine tests of P3.15 and P3.16 cover the behaviour.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/issue/`
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `grep -rnE 'PlanMove|PlanRemove|ETagMismatchError|DependentsError|isRefusedRead' internal/issue/` prints nothing
- `go vet ./...`
**Commit:** `refactor(issue): delete the move and remove planners nodestore replaced`

## P3.19 Delete the Note move and remove planners — ✅ DONE (2026-10-13)

NOTES (2026-10-13): test mapping, knowledge -> nodestore (refusals -> cli/tree, as the regression guard names): TestPlanMove_Rename_RewritesOtherNotesSuccessors (by id, by alias, beside, after the new id) -> nodestore TestMove_RenameNote_RewritesOtherNotesSuccessors (same four cases); TestPlanMove_ReparentOnly_RewritesNoOtherNote -> TestMove_ReparentNote_RewritesNoOtherNote; TestPlanMove_UnparseableOtherNote_JoinsUnread -> TestMove_RenameNote_UnparseableOtherNoteJoinsUnread; TestPlanMove_Rename_RecordsTheOldIDAsAnAlias -> TestMove_RenameNote_RecordsTheOldIDAsAnAlias; TestPlanMove_ReparentOnly_LeavesTheFileBytes -> TestMove_ReparentNote_LeavesTheFileBytes; TestPlanMove_Rename_NodeFileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt -> TestPlanNodeMove_RenameNote_FileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt; TestPlanMove_TakenID_FailsBeforeReadingTheNote -> nodestore TestMove_UnparseableMovedIssue_DestinationCheckedFirst ("taken id", the kind-agnostic engine) + cli TestKnowMv_OntoTakenIDOrAlias_FailsExistsNamingTheNote + tree TestPlanMove_RenameOntoTakenIDOrAlias_RefusedNamingKind; TestPlanMove_UnparseableNote_FailsWithItsError -> nodestore TestMove_UnparseableMovedIssue_DestinationCheckedFirst ("valid rename") + cli TestKnowMv_UnparseableNote_FailsInvalidAtItsRelativeFileAndLine; TestPlanMove_BelowItsOwnChild_NamesANote -> cli TestKnowMv_BelowItsOwnChild_FailsInvalid + tree TestPlanMove_UnderItselfOrDescendant_RefusedNamingKind; TestPlanRemove_DropsSuccessorEntriesNamingTheNote -> TestRemove_Note_DropsOtherNotesSuccessorsNamingTheIdOrAnAlias; TestPlanRemove_UnparseableOtherNote_JoinsUnread -> TestRemove_Note_UnparseableOtherNoteJoinsUnread; TestPlanRemove_UnparseableNote_RemovesItsFile -> TestRemove_UnparseableNote_RemovesItsFile; TestPlanRemove_WithChildren_NamesANote -> cli TestKnowRm_WithChildren_IsRefusedNamingTheFileAndCount + tree TestPlanRemove_WithChildren_RefusedNamingKindAndCount; TestPlanRemove_NodeFileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt -> TestPlanNodeRemoval_Note_FileSwappedAfterLoad_ListsItAsUnreadAndLeavesIt.

NOTES (2026-10-13): fifo_unix_test.go and fifo_windows_test.go kept (TestVerify_FIFORef_IsAFindingNotAHang still uses makeFIFO); newNoteTree, successorsIn and the knowledge_test.go tree import went with the deleted cases; no doc comment in internal/knowledge named the deleted planners and nothing outside the package called them.

NOTES (2026-10-13): re-derived from the assumption that only knowledge and nodestore call rawfm.Aliases outside tests: P1.5 (58262be) and P1.6 (5329e35) added rawfm.Aliases claim reads in internal/index/refresh.go and internal/index/walk.go after the plan base, so the Acceptance grep `grep -rn 'rawfm.Aliases' internal/ --include=*.go | grep -v _test.go` also prints those index lines; they are the index's own name claims for files it leaves out, not a planner fallback, and stay untouched. The knowledge copy is gone; nodestore holds the only planner-side fallback.

**What:** Completes A-04 (`reference-rewrite-engine`).
**Regression guard.** Keeps internal/knowledge/fifo_*_test.go (P0.8's tests still use them); removes `successorsIn` and its knowledge_test.go callers if they have no other caller. The mapping cites cli know_mv_test.go (taken id, below its own child), know_rm_test.go (children) and tree/move_test.go `TestPlanMove_*NamingKind` for the deleted Note-side refusal tests.
**Goal:** `internal/knowledge` holds no move or remove planner: no `PlanMove`, `PlanRemove`, `Move`, `Removal`, `renameSuccessors`, `isOneOf` or `isRefusedRead`. `nodestore` holds the only `isRefusedRead` and the only rawfm alias fallback in the repository. Every case of the deleted tests has a counterpart in `nodestore`; record the mapping as a NOTES line.
**Approach (assumed at the header base):** delete `move.go` and `remove.go` with their tests, the FIFO helpers only those tests used, and from `knowledge_test.go` the `TestPlanMove_*` and `TestPlanRemove_*` cases with the helpers left unused (`newNoteTree`). Fix any doc comment in the package that names the deleted planners.
Depends on P3.18; P3.17.
**Files:** internal/knowledge/move.go; internal/knowledge/remove.go; internal/knowledge/move_test.go; internal/knowledge/remove_test.go; internal/knowledge/knowledge_test.go
**Read first:** internal/knowledge/move.go — PlanMove, renameSuccessors, isOneOf; internal/knowledge/remove.go — PlanRemove, isRefusedRead; internal/knowledge/knowledge_test.go — newNoteTree, successorsIn, TestPlanMove_*, TestPlanRemove_*; internal/knowledge/move_test.go — loadNotes, TestPlanMove_TakenID_FailsBeforeReadingTheNote; internal/knowledge/fifo_unix_test.go — makeFIFO; internal/nodestore/kind.go — isRefusedRead, RecoverAliases; internal/cli/know_mv_test.go — "already taken by note", "cannot move a note below"
**Tests:** none new; the deleted planners' tests go with them, and the nodestore engine tests of P3.15 and P3.16 cover the behaviour.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/knowledge/`
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `grep -rn 'func isRefusedRead' internal/` prints only `internal/nodestore/kind.go`
- `grep -rn 'rawfm.Aliases' internal/ --include=*.go | grep -v _test.go` prints only `internal/nodestore` files
- `go vet ./...`
**Closes:** reference-rewrite-engine
**Commit:** `refactor(knowledge): delete the move and remove planners nodestore replaced`

## P3.20 nodestore raises the store format in Edit and Create — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the raise lives in Kind.raise (kind.go); its failure message passes through relativeText like every other nodestore FileError, so an OS error naming the absolute config path now names it relative to the worktree (codes, paths and the refusal wording unchanged).

NOTES (2026-10-13): know update's noteInputs.path became dead once the pre-validation went, so it was deleted; resolveSuccessors now returns only the successor ids and runs only for --add-superseded-by (--retire alone resolves the Note through Kind.Edit, with the same failure).

NOTES (2026-10-13): related add now raises only after the issue validates (before, its change callback raised ahead of validation); added TestEdit_RaiseFails_FailsAtTheConfigAndWritesNothing beside the two planned Edit tests. create and know add do not render RaisedFormat yet: neither can set a key that needs a newer format today.

**What:** First part of A-03 (`nodestore-post-write`). `related add` and `know update` each raise `config.yaml` inside their `Kind.Edit` change callback (`raiseStoreFormat`), in different orders relative to validation.
**Goal:**
- Each Codec declares the store format a write needs. For issues it is `config.FormatRelated` when the result gains a `related` entry the original lacks. For Notes it is `config.FormatSupersession` when the result becomes retired or gains a `superseded-by` entry.
- `Kind.Edit` and `Kind.Create`, after validation and before the re-check and the write, raise `config.yaml` to that format. They report it in `Written.RaisedFormat` (in a new `Effects` struct embedded in `Written`), 0 when the file was left alone.
- A raise failure is a `*FileError` at `.capcom/config.yaml`, and nothing is written.
- `related add` and `know update` keep their warning text, codes and exit codes, and hold no raise logic.
**Approach (assumed at the header base):**
- Codec gains `requiredFormat func(before, after T) int`. `gainsSupersession` moves from cli/know_update.go into nodestore/codec.go beside a new issue rule.
- Kind gains `raiseFormat` (`s.RaiseFormat`) and the config path, set in `Issues` and `Notes`.
- Edit's order becomes resolve, read, etag, parse, change, validate, raise, re-check, write. Create's becomes build, validate, raise, write, with the zero node as `before`.
- In cli, delete `raiseStoreFormat` and know update's pre-validation. After `editIssue` or `editNode`, render `formatRaisedWarning` from `RaisedFormat`.
**Prose guard:** fix every doc comment listing Edit's or Create's numbered steps (`grep -n '^//  [0-9]' internal/nodestore/edit.go internal/nodestore/create.go`).
Depends on P3.4; P3.19; P2.27.
**Files:** internal/nodestore/codec.go; internal/nodestore/kind.go; internal/nodestore/edit.go; internal/nodestore/create.go; internal/nodestore/edit_test.go; internal/nodestore/create_test.go; internal/cli/related.go; internal/cli/know_update.go
**Read first:** internal/nodestore/edit.go — Edit, recheck, beforeRecheck; internal/nodestore/create.go — Create, write, encode, place; internal/nodestore/kind.go — Issues, Notes, Written; internal/nodestore/codec.go — issueCodec, noteCodec; internal/cli/related.go — newRelatedAddCommand, raiseStoreFormat, formatRaisedWarning; internal/cli/know_update.go — updateNote, gainsSupersession; internal/store/store.go — RaiseFormat, ConfigPath; internal/cli/know_update_test.go — TestKnowUpdate_WritesThatGainNoKey_LeaveTheConfigAlone
**Tests:**
- `edit_test.go`: `TestEdit_GainsRelatedBelowFormatTwo_RaisesBeforeTheWrite` and `TestEdit_InvalidResult_LeavesTheFormatAlone`.
- `create_test.go`: `TestCreate_NoteWithSupersededByBelowFormatThree_RaisesTheFormat`.
- `TestRelatedAdd_*` (including `RaiseFails_FailsInvalidAtTheConfig` and `StaleIfMatchBelowFormatTwo`) and `TestKnowUpdate_FormatTwoStore_*` stay green.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'RelatedAdd|KnowUpdate'`
- `grep -rn 'raiseStoreFormat\|RaiseFormat(' internal/cli/*.go` prints nothing
- `go vet ./internal/nodestore/ ./internal/cli/`
**Commit:** `refactor(nodestore): raise the store format in Edit and Create, not per command`

## P3.21 A Kind runs its sync hook after each write — ✅ DONE (2026-10-13)

NOTES (2026-10-13): WithSync returns a copy of the Kind carrying the hook rather than setting it on the receiver, so a Kind built once (e.g. for capcom fmt) never gains a hook by aliasing; IsSynced is false whenever the hook fails. sync_test.go covers the refused-write case for all four verbs (taken id, stale etag) rather than Edit alone, and pins that WithSync leaves the original Kind hookless.

**What:** Second part of A-03. nodestore gains the hook through which the CLI's AGENTS.md re-sync will run. It is a function field supplied by the caller, so `nodestore` never imports `agents`.
**Regression guard.** Remove deletes the node file, so its test asserts the file is already gone when the hook runs; Edit and Move (new path) assert it is on disk.
**Goal:**
- `nodestore.SyncFunc` (`func() (bool, error)`) is set with `(*Kind[T]).WithSync`.
- A Kind with a hook runs it once after a write has landed: a `Create` at the top level, and every `Edit`, `Move` and `Remove`. It never runs after a failed write or a `Create` below a parent.
- `Effects` gains `IsSynced` and `SyncFailure`, reported on `Written` (and so `Moved`) and on `Removed`, which embeds `Effects`. A hook failure never fails the write.
- A Kind without a hook runs none.
**Approach (assumed at the header base):** add an unexported `sync` field and a helper `k.runSync(effects *Effects)` in kind.go, called at the end of `Create`, `Edit`, `Move` and `Remove`. `nodestore`'s import list gains nothing.
Depends on P3.20.
**Files:** internal/nodestore/kind.go; internal/nodestore/create.go; internal/nodestore/edit.go; internal/nodestore/move.go; internal/nodestore/remove.go; internal/nodestore/sync_test.go
**Read first:** internal/nodestore/kind.go — Kind, Written, Issues, Notes; internal/nodestore/edit.go — Edit; internal/nodestore/create.go — Create; internal/nodestore/move.go — Move, Moved; internal/nodestore/remove.go — Remove, Removed; internal/nodestore/kind_test.go — newStore, writeNodeFile; internal/nodestore/edit_test.go — fakeResolver, newEditedIssue
**Tests:** `sync_test.go` uses a counting hook:
- top-level Create: 1 call; Create below a parent: 0;
- Edit, Move and Remove: 1 each; for Edit and Move (new path) the file is already on disk when the hook runs, for Remove it is already gone;
- a refused Edit: 0;
- a hook error: the write landed and `SyncFailure` is set;
- no hook: zero `Effects`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`
- `go list -deps ./internal/nodestore | grep -x 'github.com/airiclenz/capcom/internal/agents'` prints nothing
- `go vet ./internal/nodestore/`
**Commit:** `refactor(nodestore): run a kind's sync hook after each landed write`

## P3.22 The know verbs take the AGENTS.md re-sync from nodestore — ✅ DONE (2026-10-13)

**What:** Completes A-03 (`nodestore-post-write`). `know add` (top level only), `know update`, `know mv` and `know rm` each call `refreshAgentsAfterWrite` themselves.
**Goal:** these four verbs build the Notes Kind with an AGENTS.md sync hook and render its outcome from the write's `Effects`. The text `refreshed: AGENTS.md`, the `--json` `agents: refreshed` key, including P3.17's `noteRemoveData`, and the warning `…; the capcom block was not refreshed` are unchanged. No know verb calls `agents.Sync` or `refreshAgentsAfterWrite`.
**Approach (assumed at the header base):** add `(a *app) notes(resolver nodestore.Resolver)`, returning `nodestore.Notes(a.store, resolver).WithSync(a.syncAgents)`, in know_add.go. `syncAgents` wraps `agents.Sync(…, agents.ModeRefresh)` and reports `State == agents.StateStale`. `refreshAgentsAfterWrite` becomes `addSyncEffects(result *Result, effects nodestore.Effects)` with the same rendering. `capcom fmt` keeps a Notes Kind without a hook (unchanged).
Depends on P3.21; P3.17; P2.21.
**Files:** internal/cli/know_add.go; internal/cli/know_update.go; internal/cli/know_mv.go; internal/cli/know_rm.go
**Read first:** internal/cli/know.go — refreshAgentsAfterWrite, addNote, noteWriteResult (know_add.go after P2.21); internal/cli/know_update.go — updateNote; internal/cli/know_mv.go — moveNote; internal/cli/know_rm.go — removeNote, noteRemoveResult; internal/cli/agents.go — agentsFailure; internal/agents/block.go — Sync; internal/cli/know_test.go — TestKnowAdd_TopLevel_RefreshesAnExistingAgentsBlock; internal/cli/know_update_test.go — TestKnowUpdate_TangledAgentsMarkers_WarnsAndWritesTheNote
**Tests:** none new. The existing refresh tests pin the behaviour: `know_rm_test.go`, `know_update_test.go`, `know_mv_test.go` and `know_test.go`, each asserting `refreshed: AGENTS.md` or `blockRefreshed`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Know|Agents'`
- `grep -rn 'refreshAgentsAfterWrite' internal/cli/` prints nothing
- `go vet ./internal/cli/`
**Closes:** nodestore-post-write
**Commit:** `refactor(cli): render the AGENTS.md re-sync from nodestore's write effects`

## P3.40 Pin the Issue lifecycle refusals before the rules move — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the reopen refusal pinned is the tree's full text, `status "open" is in the todo category, so the issue is not closed; only a closed issue can be reopened`, of which the plan quoted only the tail; create's closed-status refusal carries no file path (`capcom: status "done" is in the done category; create the issue open, then close it`), so it is pinned without one

**What:** Characterisation tests for A-01: the exact text and Code of every status-category refusal and the already-closed outcome, so P3.41–P3.44 move the rules without changing a byte.
**Tests:** package `cli`, never `t.Parallel()`; capture every expected string by running the pre-item tree, and never weaken an existing assertion.
- `write_test.go` `TestUpdate_StatusIntoOrOutOfClosed_IsRefused`: each case asserts the whole stderr (`... status "done" is in the done category; use close`, `... status "done" is closed; use reopen to move the issue out of it`, after the file path) and exit 1.
- `close_test.go` `TestReopen_OpenIssue_FailsInvalid`: the whole stderr (`... only a closed issue can be reopened`) and, under `--json`, `error.code` `invalid`.
- `write_test.go` `TestCreate_MissingIDOrClosedStatus_IsRefused`: the whole stderr of the closed-status case (`... create the issue open, then close it`).
- new `close_test.go` `TestClose_AlreadyClosed_KeepsClosedTimeAndReason`: `close shipped-fix` (closed in `newWriteStore`) reports `already_closed` in text and as the `--kv` STATUS, and the file stays byte-identical.
**Files:** internal/cli/close_test.go; internal/cli/write_test.go
**Read first:** internal/cli/write_test.go — TestUpdate_StatusIntoOrOutOfClosed_IsRefused, TestCreate_MissingIDOrClosedStatus_IsRefused, newWriteStore, readStoreFile; internal/cli/close_test.go — TestReopen_OpenIssue_FailsInvalid, TestClose_SeveralIDsOneAlreadyClosed_ClosesTheRestAndReportsEach; internal/cli/read_test.go — readIssue; internal/cli/update.go — checkStatusChange; internal/cli/close.go — reopen, closeOne
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Close|Reopen|Update_Status|Create_MissingID'`
- `go vet ./internal/cli/`
**Commit:** `test(cli): pin the issue lifecycle refusals word for word`

## P3.41 One closed-category predicate, owned by config — ✅ DONE (2026-10-13)

NOTES (2026-10-13): judgeDeps in internal/issue/ready.go also reads IsClosed (an open target blocks; a dropped one is counted as satisfied by dropping) instead of comparing to done and dropped separately; same verdicts, so the ready rules state "closed" once.
NOTES (2026-10-13): the index Filter passes ClosedCategories as plain string args (converted in a loop) rather than through toAny, so the SQL driver never sees the named Category type.

**What:** Part 1 of A-01: `config.Category.IsClosed()` and `config.ClosedCategories()` become the one statement of "done or dropped is closed"; `issue` and `index` call them.
**Regression guard.** The Goal's grep is in Acceptance.
**Goal:** `IsClosed` is true for done and dropped only; `issue`'s ready rules, `issue.Validate`'s closing check and the index `Filter`'s closed exclusion read it or `ClosedCategories`; `grep -rn 'func isClosed(' internal/issue` finds nothing; issue and index tests pass unchanged.
**Approach (assumed at the header base):** in `internal/config/config.go` (config stays a leaf): `func (c Category) IsClosed() bool` and `func ClosedCategories() []Category` (a fresh slice per call). Delete `issue.isClosed` in `ready.go`; `validate.go`'s done-or-dropped test calls `category.IsClosed()`. In `index/query.go`, the `NOT IN (?, ?)` clause of the Filter's SQL takes its placeholders and args from `ClosedCategories()`. The cli copies are P3.42's.
Depends on P3.40.
**Files:** internal/config/config.go; internal/config/config_test.go; internal/issue/ready.go; internal/issue/validate.go; internal/index/query.go
**Read first:** internal/config/config.go — Category, categories, CategoryDone, CategoryDropped; internal/issue/ready.go — isClosed, newReadyTree, judgeDeps; internal/issue/validate.go — Validate, requireClosing; internal/index/query.go — Filter.IsClosedIncluded, placeholders, toAny; internal/config/config_test.go — TestConfig_CategoryOf_LooksUpConfiguredStatus
**Tests:** `config_test.go` `TestCategory_IsClosed_OnlyDoneAndDropped` over the five categories and an undeclared one, and `ClosedCategories` returns `[done dropped]`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/config/`
- `GOMEMLIMIT=2GiB go test ./internal/issue/`
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `go vet ./internal/config/ ./internal/issue/ ./internal/index/`
- `grep -rn 'func isClosed(' internal/issue` prints nothing
**Commit:** `refactor(config): own the closed-category predicate`

## P3.42 cli reads the closed predicate from config — ✅ DONE (2026-10-13)

**What:** Part 2 of A-01: cli's `isClosedCategory` is deleted; every caller uses `config.Category.IsClosed`, and the two done-and-dropped lists use `config.ClosedCategories()`.
**Regression guard.** The Goal's grep is in Acceptance.
**Goal:** `grep -rn 'isClosedCategory' internal/` finds nothing; `close`, `reopen`, `update`, `create`, `list --tree`, `show` and `dep graph` output is unchanged (their tests pass as they are).
**Approach (assumed at the header base):** pure substitution at: `checkStatusChange` (`update.go`), `closeOne` (`close.go`), `reopen` (`reopen.go` after P2.23), `newIssue` (`create.go`), `issueTree.shownChildren` and `issueTree.progress` (`list.go`), `depGraph.shownBy` (`dep_graph.go` after P2.25; `dep.go` at the base). `childProgress`'s `Categories` filter (`issue_render.go`) and close's `targetStatus` category arguments become `config.ClosedCategories()`.
Depends on P3.41.
**Files:** internal/cli/update.go; internal/cli/close.go; internal/cli/reopen.go; internal/cli/create.go; internal/cli/list.go; internal/cli/dep_graph.go; internal/cli/issue_render.go
**Read first:** internal/cli/update.go — checkStatusChange, isClosedCategory; internal/cli/close.go — newCloseCommand (targetStatus args), closeOne; internal/cli/reopen.go — reopen, targetStatus; internal/cli/create.go — newIssue; internal/cli/list.go — issueTree.shownChildren, issueTree.progress; internal/cli/dep_graph.go — depGraph.shownBy; internal/cli/issue_render.go — childProgress
**Tests:** none new; P3.40's pins and the existing cli tests guard it.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Close|Reopen|Update|Create|List|Show|Dep'`
- `go vet ./internal/cli/`
- `grep -rn 'isClosedCategory' internal/` prints nothing
**Commit:** `refactor(cli): use config's closed-category predicate`

## P3.43 The Issue lifecycle rules live in package issue — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the cli helper is `statusChangeFailure` in update.go; it maps ErrLeavesClosed, ErrEntersClosed and ErrNotClosed to today's three messages and any other error (the author failure, or ErrAlreadyClosed, which no cli caller can get yet) to invalid with `err.Error()`; cli's `reopen` method is deleted, its body now `issue.Reopen` in the change callback. `issue.Close` has no cli caller until P3.44.

**What:** Part 3 of A-01: `issue.SetStatus`, `issue.Close` and `issue.Reopen` own the status-category state machine (ADR 0003, reinforced); `update` and `reopen` call them and keep only flag handling and the mapping to Codes.
**Regression guard.** `Reopen` returns `*StatusError{Status: i.Status, Category: <its category>, Err: ErrNotClosed}`, and the cli helper maps it inside the change callback (after `editNode` the chain is flattened by `kindFailure`). cli passes `time.Now()` (local, not UTC-truncated) as `at`, and `Reopen` returns the author func's error unwrapped (invalid Code, message `err.Error()`, at the file).
**Goal:** the transition rules are unit-tested in `internal/issue` without cobra or git; `internal/cli` has no `checkStatusChange` and no category check in its reopen path; P3.40's pinned refusals pass unchanged.
**Approach (assumed at the header base):** in `internal/issue/mutate.go`, beside `AppendLog` (no new file): sentinels `ErrAlreadyClosed`, `ErrNotClosed`, `ErrEntersClosed`, `ErrLeavesClosed` and `*StatusError{Status string; Category config.Category; Err error}` with `Unwrap`.
- `SetStatus(i Issue, cfg config.Config, to string) (Issue, error)`: same status is a no-op; leaving a closed category is `ErrLeavesClosed` (Status = from), entering one `ErrEntersClosed` (Status, Category = to's).
- `Close(i Issue, cfg config.Config, status, reason string, at time.Time) (Issue, error)`: `ErrAlreadyClosed` when i's category is closed, `ErrNotClosed` when status is not; else sets Status, Closed, Reason.
- `Reopen(i Issue, cfg config.Config, status, reason string, author func() (string, error), at time.Time) (Issue, error)`: `ErrNotClosed` when i is not closed, checked before `author` runs (today's failure order); `ErrEntersClosed` for a closed status; clears Closed and Reason and appends `reopened: <reason>` through `AppendLog`. `reopenLogPrefix` moves here.
- cli: `applyUpdate` calls `SetStatus`; reopen's change callback calls `Reopen` with `gitx.AuthorName` bound. One cli helper maps `*issue.StatusError` to the invalid Code with today's three messages verbatim; binding: the wording stays in cli because it names cli verbs. `targetStatus`, `requireReason` and `closeOne` are unchanged here.
Depends on P3.42; P2.23; P2.27.
**Files:** internal/issue/mutate.go; internal/issue/mutate_test.go; internal/cli/update.go; internal/cli/reopen.go
**Read first:** internal/issue/mutate.go — AppendLog, appendLogEntry; internal/cli/update.go — applyUpdate, checkStatusChange; internal/cli/reopen.go — newReopenCommand, reopen, reopenLogPrefix, targetStatus; internal/cli/log.go — appendLog; internal/cli/node_edit.go — editNode, editIssue; internal/cli/errors.go — kindFailure; internal/cli/close_test.go — TestReopen_ClosedIssue_ClearsClosingFieldsAndLogsTheReason; internal/issue/mutate_test.go
**Tests:** `mutate_test.go`: table tests for `SetStatus` (same, open to open, into done, out of done, done to dropped), `Close` (fields set; already closed returns `ErrAlreadyClosed` and the Issue unchanged; an open status refused), `Reopen` (fields cleared and the Log line; an open Issue refused without calling `author`; a closed target status refused). `mutate_test.go`: `Reopen` of an open Issue returns a `*StatusError` carrying its Status and Category.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/issue/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Close|Reopen|Update|Create|KV_Supporting'`
- `go vet ./internal/issue/ ./internal/cli/`
- `grep -rn 'checkStatusChange' internal/cli` prints nothing
**Commit:** `refactor(issue): own the close, reopen and status-change rules`

## P3.44 close re-checks the fresh file inside the guarded edit — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the two already_closed paths (index short-cut and the in-edit re-check) share a new helper `alreadyClosed` in close.go instead of duplicating the etag read; the test is a table over text, `--kv` and `--json`, a fresh store each, rather than one `--json` run, so the Goal's three outputs are all pinned.

**What:** Fixes F-11(d), close half (`dep-close-race`), and completes A-01: `closeOne` calls `issue.Close` on the freshly read Issue inside `Kind.Edit`'s change callback, so a close another session made after this command's index snapshot is reported, not overwritten. Binding (ratified F-11 call): the dep-cycle half stays unfixed; concurrent `dep add a b` and `dep add b a` can still store a cycle, which `check` reports as `cycle`; no store-wide write lock.
**Regression guard.** The test adds a todo issue with `deps: [login-flow]`, so the `now_ready` `[]` assertion bites: with the re-check removed that dependent is reported.
**Goal:** closing an Issue that became closed between the index resolve and the edit reports `already_closed` (text line, `--json` outcome, `--kv` STATUS), writes nothing, keeps the file's `closed:` and `reason:` bytes, and announces no `now ready:` for it.
**Approach (assumed at the header base):** in `closeOne`, keep the short-cut for an argument already handled or whose index entry is closed (it reads the file only for the etag, as today). Otherwise the change callback calls `issue.Close(target, cfg, status, reason, closedAt)`; on `issue.ErrAlreadyClosed` it sets a local flag (`editNode` types the failure, so match the flag, not the returned error), and `closeOne` reports `already_closed` with the etag of the file as read now (`readIssueFile`) and no failure. Test seam: an unexported package variable `beforeCloseEdit func(entry index.Entry)`, nil in production, called after the resolve and before `editIssue`.
Depends on P3.43.
**Files:** internal/cli/close.go; internal/cli/close_test.go
**Read first:** internal/cli/close.go — closeOne, closeResult, issueNodes; internal/nodestore/edit.go — Kind.Edit, recheck, beforeRecheck; internal/cli/node_edit.go — editIssue, editNode, issueResolver; internal/cli/update.go — readIssueFile; internal/nodestore/edit_test.go — setBeforeRecheck; internal/cli/write_test.go — newWriteStore, readStoreFile; internal/cli/read_test.go — readIssue
**Closes:** issue-lifecycle-module; dep-close-race
**Tests:** `TestClose_ClosedByAnotherSessionAfterResolve_ReportsAlreadyClosedAndKeepsTheFile`: the seam rewrites `login-flow.md` closed (status `done`, `closed: 2026-01-01T00:00:00Z`, reason `elsewhere`), restored in `t.Cleanup`; asserts exit 0, `--json` outcome `already_closed`, `now_ready` `[]`, and the file equal to what the seam wrote. Bite: with the callback's `issue.Close` re-check removed (seam kept), the outcome is `closed` and `closed:` is overwritten. The fixture adds a todo issue with `deps: [login-flow]`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Close|Reopen'`
- `go vet ./internal/cli/`
**Commit:** `fix(cli): re-check a closed issue on the fresh file inside close`

## P3.45 Pin the help, --kv and warnings of the node verbs — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the six help goldens were written from `capcom <verb> --help` of a binary built from the pre-item tree (c2fa580); the in-process golden test matches them byte for byte. The mv and rm warning checks match the whole rendered stdout line, `warning: ` label included.

**What:** Characterisation tests for A-02: the full `--help` of `mv`, `rm`, `create`, `know mv`, `know rm` and `know add`; `--kv` on the issue verbs and never on the Note verbs (closed issue `kv-flag-scope`: `--kv` only where results carry KV records; there is no Note of that name); the Unread-warning wording of all four mv and rm verbs.
**Tests:** package `cli`, never `t.Parallel()`:
- new `node_verbs_test.go` `TestHelp_NodeVerbs_MatchGolden`: `capcom <verb> --help` stdout equals `testdata/help/<verb>.txt` (`mv`, `rm`, `create`, `know-mv`, `know-rm`, `know-add`), each written once from the pre-item tree; no update flag.
- `read_test.go`: add `know mv`, `know rm`, `know add` to `TestHelp_CommandsWithoutKV_OmitKV`, and `rm` to `TestKV_SupportingCommands_ListKVAndEndWithStatus`.
- `mv_test.go` new `TestMv_UnparseableBystander_WarnsDepsNotChecked`: the whole warning `<path>: deps not checked for <id>: the file does not parse`; `rm_test.go`'s `strings.Contains` check of the same warning tightened to the whole line.
**Files:** internal/cli/node_verbs_test.go; internal/cli/testdata/help/mv.txt; internal/cli/testdata/help/rm.txt; internal/cli/testdata/help/create.txt; internal/cli/testdata/help/know-mv.txt; internal/cli/testdata/help/know-rm.txt; internal/cli/testdata/help/know-add.txt; internal/cli/read_test.go; internal/cli/mv_test.go; internal/cli/rm_test.go
**Read first:** internal/cli/read_test.go — TestHelp_CommandsWithoutKV_OmitKV, TestKV_SupportingCommands_ListKVAndEndWithStatus, newReadStore; internal/cli/write_test.go — mustRun; internal/cli/testharness_test.go — runCLI (empty dir = no chdir, so read testdata/help before any chdir); internal/cli/rm_test.go — TestRm_UnparseableIssue_WarnsDepsNotChecked, newRmStore; internal/cli/know_mv_test.go — TestKnowMv_UnparseableBystander_WarnsAndMoves; internal/cli/root.go — withKV, emit; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Help|KV_Supporting|Mv_|Rm_|KnowMv|KnowRm|KnowAdd'`
- `go vet ./internal/cli/`
**Commit:** `test(cli): pin help, --kv and warnings of mv, rm and create for both kinds`

## P3.46 One mv command for both Tree kinds — ✅ DONE (2026-10-13)

**What:** Part 1 of A-02: a `nodeVerbs[T]` descriptor and one generic mv build `capcom mv` and `capcom know mv`; `know_mv.go` is deleted.
**Regression guard.** The `nodeVerbs.afterWrite` hook takes the `nodestore.Effects` the write returned (P3.22 renders the AGENTS.md sync from Effects), fixed once here; P3.47 and P3.48 use that signature. The Note `kind` is `func(a *app) *nodestore.Kind[knowledge.Note] { return a.notes(nil) }` (P3.22's sync-hooked helper). The Goal is checked by Acceptance.
**Goal:** one function builds both mv commands; P3.45's goldens, every mv and know mv test and `TestDocFileMap_MatchesDirectory` pass unchanged; `know mv` still rejects `--kv`.
**Approach (assumed at the header base):** new `internal/cli/node_verbs.go`: `type nodeVerbs[T any] struct` holding `kind func(*app) *nodestore.Kind[T]` (`nodestore.Issues(a.store, nil)`, or for Notes `a.notes(nil)`, P3.22's sync-hooked helper); the wording the two copies differ in (command path `mv`/`know mv` in the usage error, `an issue`/`a Note` in Short, `issue`/`note` in flag help, `show`/`know show` in the `--if-match` help); `linkKey` (`deps`/`superseded-by`); `writeResult func(id, path, etag string) Result` (`writeResult`/`noteWriteResult`); `afterWrite`, which takes the `nodestore.Effects` the write returned (the Note side's rendering of the AGENTS.md sync status as P3.22 left it; nil for Issues); `hasKV` (only then `withKV`). Package values `issueVerbs` and `noteVerbs`. `func newMvCommand[T any](a *app, verbs nodeVerbs[T]) *cobra.Command` replaces both constructors, and one generic body replaces `move`/`moveNote`; `moveDestination` and `movedWarnings` stay in `mv.go`. A generic `unreadWarnings(paths []string, linkKey, id string)` renders `<path>: <linkKey> not checked for <id>: the file does not parse`. The link key stays on the cli descriptor; `nodestore.Codec` is unchanged. Update `doc.go`'s file map.
Binding: help text, flag sets, warnings, results and exit codes byte-identical.
Depends on P3.44, P3.45; P3.22; P3.19; P2.19; P2.21; P2.27.
**Files:** internal/cli/node_verbs.go; internal/cli/mv.go; internal/cli/know_mv.go; internal/cli/know.go; internal/cli/root.go; internal/cli/doc.go
**Read first:** internal/cli/mv.go — newMvCommand, move, moveDestination, movedWarnings; internal/cli/know_mv.go — newKnowMvCommand, moveNote; internal/cli/know.go — newKnowCommand, noteWriteResult, refreshAgentsAfterWrite (know_add.go after P2.21); internal/cli/update.go — writeResult; internal/cli/root.go — newRootCommand, withKV; internal/nodestore/move.go — Moved, Kind.Move; internal/cli/know_mv_test.go — TestKnowMv_TopLevelChange_RefreshesTheAgentsBlock, TestKnowMv_NothingToDo_FailsUsage; internal/cli/doc.go
**Tests:** none new; P3.45 pins the surface.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Mv|KnowMv|Help|KV_Supporting|DocFileMap'`
- `go vet ./internal/cli/`
- `test ! -e internal/cli/know_mv.go`
- `grep -c 'func newMvCommand\[' internal/cli/node_verbs.go` prints 1
**Commit:** `refactor(cli): build mv and know mv from one generic command`

## P3.47 One rm command for both Tree kinds — ✅ DONE (2026-10-13)

NOTES (2026-10-13): issueRemoveResult and noteRemoveResult now take the nodestore.Removed directly and read their drop lists from it, so they serve as the descriptor's removeResult without adapters; the Note descriptor's removeFailure wraps kindFailure in a closure because kindFailure returns *output.Error, not error.
NOTES (2026-10-13): know rm's knowRmOptions is gone; both commands share rmOptions, where know rm never registers --drop-deps so shouldDropDeps stays false. The unused issueNoun and noteNoun constants were deleted, as the item allows.

**What:** Part 2 of A-02: a generic `newRmCommand[T]` builds `rm` and `know rm`; `--drop-deps` and the dependents hint are an optional extension of the issue descriptor; `know_rm.go` is deleted.
**Regression guard.** `hasDropDeps` also appends ` [--drop-deps]` to Use (rm's Use ends with it, know rm's does not). `kindNoteRemove` and P3.17's `noteRemoveData` move into rm.go with `noteRemoveResult`; `noteNoun` and `issueNoun`, which have no user, may go. `afterWrite` gets `removed.Effects` (P3.46).
**Goal:** one function builds both rm commands; P3.45's goldens and every rm and know rm test pass unchanged; `know rm` has neither `--kv` nor `--drop-deps`.
**Approach (assumed at the header base):** `nodeVerbs[T]` gains `removeResult func(nodestore.Removed) Result` (`issueRemoveResult`/`noteRemoveResult`, fed the drop lists as P3.19 left `Removed`), `removeFailure func(error) error` (`issueRemovalFailure`/`kindFailure`) and `hasDropDeps` (registers `flagDropDeps` and passes it to `Kind.Remove`; otherwise `false`). One generic body replaces `removeIssue`/`removeNote`: tree-problem warnings, `unreadWarnings`, `cleanupWarnings`, `afterWrite`. `cleanupWarnings`, `noteRemoveResult`, `kindNoteRemove` and `noteRemoveData` move into `rm.go`; `supersededByUnreadWarnings` is deleted. Update `doc.go`.
Depends on P3.46.
**Files:** internal/cli/node_verbs.go; internal/cli/rm.go; internal/cli/know_rm.go; internal/cli/know.go; internal/cli/root.go; internal/cli/doc.go
**Read first:** internal/cli/rm.go — newRmCommand, removeIssue, issueRemovalFailure, issueRemoveResult; internal/cli/know_rm.go — newKnowRmCommand, removeNote, cleanupWarnings, supersededByUnreadWarnings, noteRemoveResult, kindNoteRemove; internal/nodestore/remove.go — Removed, Kind.Remove; internal/cli/rm_test.go — TestRm_UnparseableIssue_WarnsDepsNotChecked, TestRm_JSON_ReportsKindAndData; internal/cli/know_rm_test.go — TestKnowRm_TopLevel_RefreshesTheAgentsBlock, TestKnowRm_UnparseableBystander_WarnsAndRemoves; internal/cli/doc.go
**Tests:** none new.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Rm|KnowRm|Help|KV_Supporting|DocFileMap'`
- `go vet ./internal/cli/`
- `test ! -e internal/cli/know_rm.go`
- `grep -c 'func newRmCommand\[' internal/cli/node_verbs.go` prints 1
**Commit:** `refactor(cli): build rm and know rm from one generic command`

## P3.48 create and know add share one write path — ✅ DONE (2026-10-13)

NOTES (2026-10-13): consequential edit — internal/cli/doc.go: made necessary by node_verbs.go now holding the write path of create and know add (file-map line updated)
NOTES (2026-10-13): the nodeVerbs, issueVerbs and noteVerbs doc comments now name create and know add; create's empty-body warning takes options.id, which Kind.Create's ValidateID guarantees equals the written id

**What:** Part 3 of A-02: one generic `createNode[T]` runs the write of `create` and `know add`; their flag sets and node builders (`newIssue`, `newNote`) stay per kind.
**Regression guard.** `createNode` calls `verbs.afterWrite(&result, written.Effects)` (P3.46's signature), so top-level `know add` keeps `refreshed: AGENTS.md`; `create` then appends `emptyBodyWarning` using `options.id`.
**Goal:** `create` and `know add` reach `nodestore.Kind.Create` through one function; P3.45's goldens and every create and know add test pass unchanged.
**Approach (assumed at the header base):** in `node_verbs.go`, `createNode[T any](a *app, verbs nodeVerbs[T], id, parent string, build func() (T, error)) (Result, error)`: `Kind.Create`, `kindFailure`, `verbs.writeResult`, `unwarnedTreeProblems`, `verbs.afterWrite`. `create` keeps `resolveDeps` before it and the empty-body warning after; `addNote` keeps `knowledge.ValidateRef` before it. Whether a write re-syncs AGENTS.md (top level only) is P3.22's rule in nodestore; the descriptor only renders the status.
Depends on P3.47.
**Files:** internal/cli/node_verbs.go; internal/cli/create.go; internal/cli/know_add.go
**Read first:** internal/cli/create.go — create, newIssue, resolveDeps, emptyBodyWarning; internal/cli/know.go — newKnowAddCommand, addNote, newNote (know_add.go after P2.21); internal/nodestore/create.go — Kind.Create; internal/nodestore/kind.go — Written, Notes; internal/cli/errors.go — kindFailure, unwarnedTreeProblems
**Closes:** generic-node-verbs
**Tests:** none new.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Create|KnowAdd|Help|KV_Supporting'`
- `go vet ./internal/cli/`
- `grep -n '\.Create(' internal/cli/create.go internal/cli/know_add.go` prints nothing
**Commit:** `refactor(cli): share the create write path of issues and notes`

## P3.50 One outline walker for list --tree and know tree — ✅ DONE (2026-10-13)

NOTES (2026-10-13): issueTree.visit became a one-line issueTree.walk that calls walkOutline with t.shownChildren and t.add; know tree's visit closure became a walk closure over walkOutline; small issueID and noteID helpers supply idOf.
NOTES (2026-10-13): consequential edit — internal/cli/doc.go: made necessary by outline.go now holding the outline walker (file-map role line updated).

**What:** Part 1 of A-08: a generic `walkOutline[E]` beside `renderOutline` walks both outlines; `issueTree.visit` and the closure inside `outline` go.
**Regression guard.** The Goal is checked by an Acceptance grep.
**Goal:** `list --tree` and `know tree` walk through one function, so `--depth` cuts both alike; their text and `--json` stay byte-identical (`TestListTree_*`, `TestKnowTree_*` unchanged).
**Approach (assumed at the header base):** in `outline.go`, `walkOutline[E any](shownChildren func(parentID string) []E, idOf func(E) string, parentID string, level, indent, maxDepth int, add func(entry E, indent, more, moreDepth int))`: on the last level `--depth` allows (`maxDepth != unlimitedDepth && level >= maxDepth`) each shown child is added with `more = len(shownChildren(child))` and `moreDepth = level+1` and not descended; otherwise added with 0, 0 and walked one level and indent deeper. Issue side: `issueTree.shownChildren`; Note side: the `childrenByParent` map (already filtered for retired and background Notes). The start (whole tree from `""`, or a named start at indent 0 then its subtree from level 1, indent 1) stays with each caller.
Depends on P3.42; P2.19; P2.21.
**Files:** internal/cli/outline.go; internal/cli/outline_test.go; internal/cli/list.go; internal/cli/know_tree.go
**Read first:** internal/cli/list.go — issueTreeResult, issueTree.visit, issueTree.shownChildren, issueTree.add; internal/cli/know.go — outline, childrenByParent, noteOutlineLine, outlineLine (know_tree.go after P2.21); internal/cli/outline.go — outlineRow, renderOutline; internal/cli/know.go — unlimitedDepth (flags.go after P2.19; value 0 = every level); internal/cli/read_test.go — TestListTree_Flags_PrintTheOutline
**Tests:** new `outline_test.go` `TestWalkOutline_DepthCutsAndCountsShownChildren`: a small map tree at maxDepth 0, 1 and 2, from `""` and from below a node; asserts the add calls (entry, indent, more, moreDepth).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'WalkOutline|ListTree|KnowTree|List_NoArgs'`
- `go vet ./internal/cli/`
- `grep -nE 'func \(t \*issueTree\) visit|var visit func' internal/cli/*.go` prints nothing
**Commit:** `refactor(cli): walk list --tree and know tree with one outline walker`

## P3.51 One child-progress count and one issue-node conversion — ✅ DONE (2026-10-13)

NOTES (2026-10-13): issueTree.progress is removed rather than kept as a wrapper; issueTree.add calls progressOf(t.children[entry.ID]) directly, so progressOf is the one cli function counting progress for show and list --tree.

**What:** Part 2 of A-08: `show` and `list --tree` count a parent's closed children with one function, and "every issue, closed included, as `[]issue.Node`" is converted in `index`.
**Regression guard.** `Index.List` is `ListIDs` plus one `Issue` read per child, so the Goal reads "`childProgress` makes one `issueIndex.List` call and counts through `progressOf`".
**Goal:** `childProgress` makes one `issueIndex.List` call and counts through `progressOf`; one cli function counts progress from child entries for `show` and `list --tree`; `index.Nodes` is the one conversion, used by `close`, `dep` and `briefing`; show, list, close, dep and ready output is unchanged.
**Approach (assumed at the header base):** `progressOf(children []index.Entry) *childrenData` in `issue_render.go` (nil for none; closed via `Category.IsClosed`); `childProgress` lists `index.Filter{ParentID: id, IsClosedIncluded: true}` once and calls it, and `issueTree.progress` calls it on `t.children[id]`. `func Nodes(entries []Entry) []issue.Node` beside `Entry.Node` in `internal/index/query.go` replaces the loops in `app.issueNodes` (`close.go`), `loadDepGraph` (`dep_graph.go` after P2.25) and `briefing.ReadyList`.
Depends on P3.50.
**Files:** internal/cli/issue_render.go; internal/cli/list.go; internal/cli/close.go; internal/cli/dep_graph.go; internal/index/query.go; internal/index/query_test.go; internal/briefing/ready.go
**Read first:** internal/cli/issue_render.go — childProgress, childrenData, childrenText; internal/cli/list.go — issueTree.progress, issueTree.add, issueTree.shownChildren; internal/cli/close.go — app.issueNodes (also called by show's loadShowGraph); internal/cli/dep_graph.go — loadDepGraph (dep.go at base); internal/briefing/ready.go — ReadyList; internal/index/query.go — Entry.Node, Index.List, Filter.sql; internal/index/query_test.go — TestEntryNode_IndexedIssue_CarriesWhatTheReadyRulesRead; internal/cli/read_test.go — TestListTree_ClosedChild_CountedButLeftOut, TestShow_JSON_ChildrenCountedOnlyWhenPresent
**Closes:** outline-walker
**Tests:** `query_test.go` `TestNodes_KeepsOrderAndFields`; the existing show `children:` and `TestListTree_ClosedChild_CountedButLeftOut` tests guard the counts.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Show|ListTree|Close|Dep|Ready'`
- `GOMEMLIMIT=2GiB go test ./internal/briefing/`
- `go vet ./internal/index/ ./internal/cli/ ./internal/briefing/`
- `! grep -n 'ListIDs' internal/cli/issue_render.go`
**Commit:** `refactor(cli): count child progress once and convert issue nodes in index`

## P3.55 The agent-settings editor moves to internal/hooks — ✅ DONE (2026-10-13)

NOTES (2026-10-13): internal/hooks also exports PrimeCommand ("capcom prime"), beyond the Goal's export list, so cli's kept primeCommand reads it (`const primeCommand = hooks.PrimeCommand`) instead of duplicating the literal; cli's kept codexHooksPath is now `var codexHooksPath = hooks.Codex.Path` (still used by the --codex-hooks flag help) and claudeSettingsPath is gone (flag help and snippet read hooks.Claude.Path).

NOTES (2026-10-13): Merge returns content unchanged with isAdded false when every event already runs capcom prime; Install's read failure is a *hooks.Error without a line, as before.

NOTES (2026-10-13): consequential edit — .agents/skills/release-notes-capcom/prompts/_capcom.md: made necessary by moving the Claude Code hooks editor out of internal/cli/init.go into internal/hooks/settings.go (the surface map named only init.go)

**What:** Part 1 of A-09 (it replaces P2-12, which is out of scope): the Claude Code and Codex hooks-settings JSON editor moves from `cli/init.go` into a new package `internal/hooks`, its JSON edge cases unit-tested on byte slices.
**Regression guard.** cli keeps `primeCommand`, `codexHooksPath`, `claudeHooksSnippet`, `codexFeatureNote` and the two status names, which init_test.go uses. The status names stay of type string (`claudeHooksAdded = string(hooks.Added)`, and `string(...)` at the `initData` assignment), because init_test.go compares them with `any` values decoded from JSON.
**Goal:** `internal/hooks` exports `Install(worktreeRoot string, target Target) (Status, error)`, the pure `Merge(content []byte, target Target) ([]byte, bool, error)`, `Snippet(target Target) []byte`, the targets `Claude` and `Codex` (each with its `Path`), `Status` values `Added` and `Present` (`"added"`, `"present"`) and `*Error{Path, Line, Message}`; it imports `nodefile` and never `cli` or `agents`; `capcom init` text, `--json` and failures are byte-identical (`init_test.go` unchanged).
**Approach (assumed at the header base):** move `mergeHooks`, `addHookGroup`, `decodeObject`, `encodeObject`, `jsonArray`, `encodeJSON`, `indentJSON`, `settingsFailure`, `jsonMember`, `settingsHook`, `hookGroup`, `hookEvent`, `hookSettings`, `claudeHookEvents`, `codexHookEvents` and their constants. `Merge` is `mergeHooks` without file IO; `Install` reads, merges and writes through `nodefile.WriteFileAtomic`, returning its error as is. Read and shape failures become `*hooks.Error` with today's Path, Line and Message. In cli, `installHooks` maps `*hooks.Error` to the invalid Code at that path and line and anything else through `asError`; `claudeHooksSnippet` keeps its sentence around `hooks.Snippet(hooks.Claude)`; flag help reads `hooks.Claude.Path`. Package comment in `settings.go`.
Depends on P2.9; P3.6.
**Files:** internal/hooks/settings.go; internal/hooks/settings_test.go; internal/cli/init.go
**Read first:** internal/cli/init.go — newInitCommand, installHooks, mergeHooks, addHookGroup, decodeObject, settingsFailure, claudeHooksSnippet, hookSettings; internal/cli/init_test.go — TestInitClaudeHooks_UnusableSettings_FailWithPath, TestInitCodexHooks_UnusableHooksFile_FailsWithPath, TestInitCodexHooks_ExistingHooks_KeepsThemInOrderAndIsIdempotent, TestInit_Plain_PrintsTheHooksSnippetInTextModeOnly, TestInitCodexHooks_WithoutClaudeHooks_StillPrintsTheClaudeSnippet; internal/cli/errors.go — asError (output.AsError after P2.35)
**Tests:** `settings_test.go` table over `Merge`: empty and `null` content, a non-object, a syntax error (its Line), `hooks` not an object, an event not an array, an event already running `capcom prime`, a foreign group shape kept, member order and value bytes kept, a second merge changes nothing.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/hooks/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Init'`
- `go vet ./...`
- `! go list -deps ./internal/hooks/ | grep -qE 'capcom/internal/(cli|agents)$'`
**Commit:** `refactor(hooks): move the agent-settings editor out of init`

## P3.56 The .gitattributes editor moves to internal/hooks — ✅ DONE (2026-10-13)

NOTES (2026-10-13): consequential edit — internal/hooks/settings.go: made necessary by adding the .gitattributes editor to internal/hooks (the package comment named only the agent-settings files, and the Error doc only settings files; both now cover .gitattributes)

NOTES (2026-10-13): cli's *hooks.Error mapping is one helper, hooksFailure, which installHooks and installMergeDriver both call, instead of two copies of the P3.55 switch; init text, --json and failures are unchanged (init_test.go untouched)

NOTES (2026-10-13): AppendMergeAttribute copies content before appending, so a caller's spare capacity is never written; gitattributes_test.go pins that besides the plan's table (which also covers a CRLF file without a final line ending and a present line without one)

**What:** Part 2 of A-09: the merge-driver `.gitattributes` editor moves into `internal/hooks`; `init.go` keeps the command, its texts and `mergeDriverConfig`.
**Regression guard.** The Goal is checked by an Acceptance grep.
**Goal:** `hooks.InstallMergeAttribute(worktreeRoot string) (Status, error)` and the pure `hooks.AppendMergeAttribute(content []byte) ([]byte, bool)` hold the line rules (the line counts as present however its fields are spaced, CRLF kept, a missing final line ending added); `init --merge-driver` output is byte-identical; `init.go` has no `.gitattributes` logic.
**Approach (assumed at the header base):** move `addMergeAttribute`, `hasMergeAttribute`, `appendAttributeLine`, `gitattributesPath` (exported as `GitattributesPath` for init's flag help and texts), `mergeDriverAttribute` and the line-ending constants. Read and replace failures become `*hooks.Error` at `GitattributesPath`; the new-file write's error is returned as is; cli maps both as in P3.55. Writes use `nodefile.ReplaceFileVerbatim` and `nodefile.WriteFileAtomic`.
Depends on P3.55.
**Files:** internal/hooks/gitattributes.go; internal/hooks/gitattributes_test.go; internal/cli/init.go
**Read first:** internal/cli/init.go — installMergeDriver, addMergeAttribute, hasMergeAttribute, appendAttributeLine, mergeDriverConfigText, mergeDriverConfig; internal/cli/init_test.go — TestInitMergeDriver_CRLFGitattributes_StayCRLFAndKeepTheirMode, TestInitMergeDriver_ExistingGitattributes_KeepsItsBytesAndAddsTheLineOnce, TestInitMergeDriver_NoGitattributes_CreatesItOnAnExistingStore
**Closes:** hooks-package
**Tests:** `gitattributes_test.go` table over `AppendMergeAttribute`: empty, LF, CRLF, no final line ending, already present with extra spaces (unchanged, false).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/hooks/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Init'`
- `go vet ./internal/hooks/ ./internal/cli/`
- `! grep -nE 'func (addMergeAttribute|hasMergeAttribute|appendAttributeLine)\(|lineEndingCRLF' internal/cli/*.go`
**Commit:** `refactor(hooks): move the .gitattributes editor out of init`

## P3.60 Pin the per-file problem wording of check and the index — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the pins record where check and the index differ today, and P3.61–P3.63 must keep those differences byte for byte. Check gives alias problems line 7, while the index gives them no line. Check says "repeats the node's own id", the index "repeats the issue's own id". For a repeated id, check reports only the file it leaves out ("also at <kept file>"), while the index warns on both files ("both are left out").

NOTES (2026-10-13): the symlinked issue file points at a regular file outside the store (outside.md at the worktree root); check's makeSymlink helper lives in check_test.go, beside the other shared helpers, not in a unix-only file, because it carries its own Windows skip.

**What:** Characterisation tests for A-06: the code or kind, path, line and message every per-file problem gets from `check` and from the index refresh, pinned on one fixture so P3.61–P3.63 merge the mappings without a byte changing.
**Tests:** both pass on the pre-item tree; the existing worded-as tests (`TestRun_DirectoryProblems_WordedAsTheIndexWarnsThem`, `TestRefresh_DirectoryWarnings_WordedAsTheTreeWalk`, `TestRun_RepeatedAliases_WordedByTheSharedAliasCheck`) stay.
- `check_test.go` `TestRun_FileProblems_PinCodesAndWording`: a store holding a file with an unknown frontmatter key, one with an undeclared status, one with an invalid alias, one whose alias repeats its id, a symlinked issue file (skipped on Windows and as root) and a repeated id; asserts the whole `code + " " + problem.String()` list.
- `index_test.go` `TestRefresh_FileProblems_PinKindsAndWording`: the same fixture through `Open`; asserts every `Warning` (Kind, Path, Line, Message).
**Files:** internal/check/check_test.go; internal/index/index_test.go
**Read first:** internal/check/check_test.go — runCheck, writeFiles, issueFile, TestRun_DirectoryProblems_WordedAsTheIndexWarnsThem (package check_test: needs its own symlink skip helper); internal/check/fifo_unix_test.go — Geteuid skip pattern; internal/check/check.go — Problem, Problem.String; internal/index/index_test.go — newStore, writeIssue, openIndex, makeSymlink, TestRefresh_EachProducer_SetsTheWarningKindAndReplaysIt; internal/index/index.go — Warning, Report
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `go vet ./internal/check/ ./internal/index/`
**Commit:** `test(check): pin per-file problem codes and wording beside the index's`

## P3.61 One problem-kind enum for check, the index and cli — ✅ DONE (2026-10-13)

NOTES (2026-10-13): diagnose imports only tree for now; config, issue and knowledge arrive with P3.62's per-file parse and validate functions, which need them.

NOTES (2026-10-13): the index walk and aliasProblems drop TreeKind's bool instead of keeping their old fallbacks (orphan-directory for the walk, repeated-alias for aliases), because every problem tree.Walk and tree.CheckAliases reports wraps a sentinel TreeKind knows. check's loadTree keeps its old fallback: an unknown tree.Load problem still gets CodeDuplicate, now through the small kindCode Kind-to-Code mapping (RepeatedAlias to duplicate, every other Kind its own string).

**What:** Part 1 of A-06: a new package `internal/diagnose` holds one `Kind` enum and the one mapping from a `tree` sentinel to it.
**Goal:** `diagnose.TreeKind(problem *tree.Error) (Kind, bool)` and `diagnose.ReadRefusal(err error) error` are the only sentinel-to-kind mappings: `grep -rnE 'func (treeCode|walkWarningKind|treeProblemKindOf|readRefusal)\(' internal` finds nothing; check's JSON codes, index warnings and cli warnings are byte-identical (P3.60).
**Approach (assumed at the header base):** `type Kind string` with `InvalidID`, `InvalidAlias`, `Duplicate`, `RepeatedAlias`, `OrphanDirectory`, `NotRegular`, `Frontmatter`, `Field`, valued as today's index kinds. `TreeKind` matches `ErrRepeatedAlias` before `ErrDuplicate` and returns false for no known sentinel. `index.WarningKind` becomes the alias `= diagnose.Kind` and each `WarningX` the matching constant, so `files.problems` JSON and its readers (`rewarn`, cli `isIndexWarnedAbout`) are unchanged and the index schema is not bumped. check maps a Kind to its Code: `RepeatedAlias` to `CodeDuplicate`, every other Kind to its own string. Binding: every `check` JSON code stays byte-identical, so the item carries no CHANGELOG entry; a code change would need a bullet under `## [Unreleased]` in `CHANGELOG.md` (carried in the item's sidecar, landed at closeout), never a release heading, which only `go run ./cmd/release prepare` writes (Note `release-process`). `diagnose` imports `tree`, `config`, `issue` and `knowledge` and never `index` (ADR rule: check never reads the index). Sites at the base: `check.treeCode` and `checker.readFile`, `index.walkWarningKind`, `index.readRefusal` and `indexFile`, the kind choice in `index.aliasProblems`, cli `treeProblemKindOf`.
Depends on P3.60; P1.6; P1.8; P2.13; P2.17; P3.11; P3.6.
**Files:** internal/diagnose/diagnose.go; internal/diagnose/diagnose_test.go; internal/index/index.go; internal/index/walk.go; internal/index/refresh.go; internal/index/problems.go; internal/check/problem.go; internal/cli/errors.go
**Read first:** internal/cli/errors.go — treeProblemKindOf, isIndexWarnedAbout; internal/check/check.go (problem.go after P2.13) — treeCode, checker.loadTree, checker.readFile; internal/index/walk.go — walkWarningKind; internal/index/refresh.go (problems.go after P2.17) — readRefusal, aliasProblems, refresher.indexFile; internal/index/index.go — WarningKind, Warning; internal/tree/tree.go — repeatedAliasError.Unwrap, invalidIDDirectoryError.Unwrap, leftOutFileOf
**Tests:** `diagnose_test.go`: `TreeKind` for every sentinel (repeated alias before duplicate, `ErrTooLarge` as `NotRegular`, a plain error false); `ReadRefusal` for both refusals and a plain error.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/diagnose/`
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Warn|Mv|Rm|Create'`
- `go vet ./...`
- `! go list -deps ./internal/diagnose/ | grep -q 'capcom/internal/index$'`
**Commit:** `refactor(diagnose): one problem-kind enum for check, index and cli`

## P3.62 Per-file findings shared by check and the index — ✅ DONE (2026-10-13)

NOTES (2026-10-13): consequential edit — internal/diagnose/diagnose.go: made necessary by adding Finding and the per-file parse, validate and alias functions to diagnose (the package comment named only Kind and the sentinel mapping)

NOTES (2026-10-13): check.go is untouched, as the plan's regression check ruled: check hands ParseIssue's and ParseNote's one Frontmatter finding to addParseFailure as an issue.Error / knowledge.Error (both frontmatter.Error), so its conflict-marker dedup and line/message reading stay as they were; check's field problems take their code through P3.61's kindCode (Field to "field")

NOTES (2026-10-13): index.aliasProblems stays as a one-line adapter from foundFile to diagnose.AliasFindings, which the Goal names; files_test.go also pins Finding.String with and without a line, and the "without a line" parse case is a YAML error yaml.v3 gives no line for (a tab-indented key)

**What:** Part 2 of A-06: parsing, validating and alias-checking one issue or Note file into findings is one set of `diagnose` functions, used by the index refresh and by `check`.
**Regression guard.** The Goal is checked by an Acceptance grep.
**Goal:** `diagnose.ParseIssue`, `ParseNote`, `ValidateIssue`, `ValidateNote` and `AliasFindings` produce every per-file finding; `index.parseIssue`, `parseNote`, `aliasProblems` and check's per-file parse and field mapping call them; P3.60's pins and every check and index test pass unchanged.
**Approach (assumed at the header base):** `type Finding struct{Kind; Path; Line; Message}` with today's `index.Warning` JSON tags and `String`; `index.Warning` becomes the alias `= diagnose.Finding`. Its producers (walk, refresh, notes, duplicates) and consumers (`rewarn`'s JSON decode, `Report.Warnings`, cli `movedWarnings`, `isIndexWarnedAbout` and the warning rendering) keep the same shape. `ParseIssue(relPath, data) (issue.Issue, []Finding)`: one `Frontmatter` finding at the parser's path and line, else at relPath. `ValidateIssue(relPath, i, cfg) []Finding`: `Field` findings; the Note pair likewise. `AliasFindings(relPath, kind, id string, aliases []string)`: `tree.CheckAliases` rejects as `InvalidAlias` or `RepeatedAlias`. Parse and Validate stay separate: check validates only the nodes `tree.Load` kept, the index every parsed file. check keeps `addParseFailure`'s conflict-marker dedup, its Codes mapped as in P3.61, and its alias problems from `tree.Load`.
Depends on P3.61.
**Files:** internal/diagnose/files.go; internal/diagnose/files_test.go; internal/index/index.go; internal/index/problems.go; internal/index/notes.go; internal/check/issues.go; internal/check/notes.go
**Read first:** internal/index/problems.go — parseIssue, aliasProblems; internal/index/notes.go — parseNote; internal/index/index.go — Warning, WarningKind, Warning.String; internal/check/check.go — addParseFailure, checkConflictMarkers; internal/check/issues.go — issues, checkIssue; internal/check/notes.go — notes, checkNote; internal/index/refresh.go — indexFile, rewarn; internal/cli/mv.go — movedWarnings
**Tests:** `files_test.go`: a parse failure with and without a line, two field problems, both alias rejects, a clean file (no findings).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/diagnose/`
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Warn'`
- `go vet ./internal/diagnose/ ./internal/index/ ./internal/check/`
- `! grep -nE '(issue|knowledge)\.(Parse|Validate)\(' internal/check/*.go internal/index/*.go | grep -v _test.go`
**Commit:** `refactor(diagnose): share per-file findings between check and the index`

## P3.63 Name clashes found by one pure function — ✅ DONE (2026-10-13)

NOTES (2026-10-13): consequential edit — internal/diagnose/diagnose.go: made necessary by adding NameClashes to diagnose (the package comment listed the package's contents and did not name clash detection)

NOTES (2026-10-13): the index's unexported claim type stays in duplicates.go, because walk.go and query.go still use it; only excludeDuplicatesOf now scans its rows into diagnose.Claim. excludeClaims now takes the findings, adds them to the refresh's warnings and excludes each path they name, once per path instead of once per claimant per name; the warnings and the excluded set are unchanged.

**What:** Part 3 of A-06: the index's duplicate-name detection over claims becomes `diagnose.NameClashes`, a pure function unit-tested without SQL.
**Regression guard.** `NameClashes`' input is exactly today's SQL rows (indexed ids plus `aliases` rows, ORDER BY name, path), never the `claims` table, whose kind column is the tree kind (P1.5). The Goal is checked by an Acceptance grep.
**Goal:** `diagnose.NameClashes(claims []Claim) []Finding` returns, for every name two or more files claim, one `Duplicate` finding per claimant per other claimant, worded exactly as the index warns today (`<tree.ErrDuplicate> <kind> "<name>": also the <kind> of <path>; both are left out`); `excludeClaims` warns from it and keeps only the SQL `excluded` update; index warnings are byte-identical.
**Approach (assumed at the header base):** `Claim{Name, Path, Kind string}` (Kind `id` or `alias`), in input order; its input is exactly today's SQL rows (indexed ids plus `aliases` rows, ORDER BY name, path), never the `claims` table. check keeps `tree.Load` as its detector, which since P1.8 drops every claimant of a repeated id as the index does; check's duplicate wording stays as F-10 ratified. No check JSON code changes (P3.61's binding text).
Depends on P3.62.
**Files:** internal/diagnose/clashes.go; internal/diagnose/clashes_test.go; internal/index/duplicates.go
**Read first:** internal/index/duplicates.go — claim, excludeDuplicates, excludeDuplicatesOf, excludeClaims; internal/index/refresh.go — refresher.warn, refresh; internal/index/index_test.go — TestRefresh_DuplicateName_WarnsAndExcludesBoth, TestRefresh_EachProducer_SetsTheWarningKindAndReplaysIt; internal/index/notes_test.go — TestNotes_InvalidAndDuplicateNotes_WarnAndStayOut; internal/index/query_test.go — TestResolveNote_DuplicatedID_RefusesInsteadOfPrefixMatching
**Closes:** shared-diagnostics
**Tests:** `clashes_test.go`: one claimant (none), two files sharing an id, an id against an alias, three claimants (each warned twice), output order follows input.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/diagnose/`
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `go vet ./internal/diagnose/ ./internal/index/`
- `grep -q 'diagnose.NameClashes' internal/index/duplicates.go`
**Commit:** `refactor(diagnose): find name clashes in one pure function`

## P3.70 Release: characterise tap and bucket push failures — ✅ DONE (2026-10-13)

**What:** Covers T-01 (`channel-push-failure-test`): nothing makes the tap or bucket clone, `rev-list`, commit or push fail. A characterisation test before P3.71 restructures this code.
**Files:** cmd/release/channels_test.go
**Read first:** cmd/release/channels_test.go — respondChannel, fakeChannels, releasedFake, runChannelPublish, TestPublishWingetConvergesFromRemoteStateOnceTheTagExists; cmd/release/channels.go — publishPackageFile; cmd/release/publish.go — publishRun.converge; cmd/release/publish_test.go — publishRunResult, runTestPublishWith
**Closes:** channel-push-failure-test
**Tests:** `fakeChannels` gains a failure toggle naming one clone-step command (`gh repo clone`, `git rev-list`, `git commit`, `git push`) and the repository it fails for. New table test `TestPublishChannelStepFailureStopsTheRun` over both repositories and the four commands (bucket cases leave the tap current, so its step is done): `publish` fails with `the homebrew step failed` or `the scoop step failed`, wrapping text that names the repository and the step (`clone`, `read … history`, `commit … to`, `push … to`), the step table is printed, and `result.smokes` is empty.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'TestPublishChannelStepFailure'`
- `go vet ./cmd/release/`
**Commit:** `test(release): cover tap and bucket clone, commit and push failures`

## P3.71 Release: one step table, and a doc.go file map — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the one by-name step helper is the existing `stepStateOf` (publish.go), which `checkGateFor` already used; `resolveWingetPackage` now calls it too, so no new helper was added.

**What:** First part of A-11: `publishSteps` and `publishActions` merge into one step record and one table; the package gets `doc.go` with a file map pinned by a structural test.
**Regression guard.** Also moves the step-name consts (`stepTag`..`stepSmoke`) out of probes.go into steps.go, so probes.go is empty by P3.74. The Goal's "no code finds a step by its index" is checked by Acceptance (status_test.go's `report.Steps[2]` is a test and stays).
**Goal:** `cmd/release` declares each publish step once, as one record `{name, probe, describe, converge, doneDetail}` in one ordered table that `status` and `publish` both read; no code finds a step by its index; `doc.go` holds the package comment and a file map that `TestDocFileMap_MatchesDirectory` keeps equal to the package's non-test files.
**Approach (assumed at the header base):** new `steps.go` takes from `probes.go` the step-name consts (`stepTag`..`stepSmoke`), the step states, `stepReport`, `stepProbe`, `probeEnv`, `unknownStep`, `probeSteps` and the table, whose `publishStep` record gains `describe`, `converge` and `doneDetail`; `publishAction` and `publishActions` (`publish.go`) go. `publishRun.probe` calls `probeSteps`; `converge` and `printPublish` range over `publishSteps`; `runPublish` and `resolveWingetPackage` find a step by name through one helper. No function a table entry names may read `publishSteps` (a Go initialisation cycle). The package comment moves from `main.go` to `doc.go`, which gains a `# Files` map in `internal/cli/doc.go`'s style; `doc_test.go` copies `internal/cli/doc_test.go`'s test.
Depends on P1.40; P1.41; P1.43; P1.44; P3.70.
**Files:** cmd/release/steps.go; cmd/release/probes.go; cmd/release/publish.go; cmd/release/channels.go; cmd/release/main.go; cmd/release/doc.go; cmd/release/doc_test.go
**Read first:** cmd/release/probes.go — publishStep, publishSteps, probeSteps, probeEnv, stepReport, step-name consts; cmd/release/publish.go — publishAction, publishActions, runPublish, publishRun.converge, publishRun.probe, printPublish; cmd/release/channels.go — resolveWingetPackage; cmd/release/status.go — runStatus; cmd/release/main.go — package comment; internal/cli/doc.go — # Files; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory; cmd/release/status_test.go — len(publishSteps)
**Tests:** `doc_test.go` (new); every existing test passes unchanged, `TestPublishConvergesFromRemoteStateOnceTheTagExists` and the `execRepo` argv tests included.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/`
- `go vet ./...`
- `! grep -rnE 'publishActions|steps\[[0-9]+\]' --include='*.go' --exclude='*_test.go' cmd/release`
**Commit:** `refactor(release): one step table for status and publish`

## P3.72 Release: the tag and GitHub release steps in their own files — ✅ DONE (2026-10-13)

NOTES (2026-10-13): `readRebuiltSums`, the helper only the release step's two sums checks (`checkPublishedSums`, `checkUploadedArchives`) call, moved to githubrelease.go with them; the plan did not name it. `removeBuildDir`, `sleepContext` and the `defaultTagTimeout`/`tagPollInterval` constants stay in publish.go, since the run and the publish options own them.
NOTES (2026-10-13): `capcomRepo` left probes.go's const block for its own `const` line in repo.go, which needed a new one-line doc comment; doc.go's map gains tag.go and githubrelease.go and rewords the publish.go, probes.go and version.go lines.

**What:** Second part of A-11, a pure move: the tag step and the GitHub release step leave `publish.go`, `probes.go` and `channels.go`.
**Goal:** `tag.go` holds the tag step and `githubrelease.go` the release step; `publish.go` keeps them out; behaviour unchanged.
**Approach (assumed at the header base):** move `probeTag`, `describeTag`, `convergeTag`, `waitForTag`, `verifyTag` to `tag.go`; `probeRelease`, `describeRelease`, `convergeRelease`, `buildReleaseAssets`, `checkPublishedSums` and the P1.44 archive check, `missingAssets`, `releaseAssetNames`, `assetPaths`, `downloadReleaseSums` to `githubrelease.go`; `capcomRepo` and `isReleaseNotFound` to `repo.go`; `bareRelease` to `version.go`. Update `doc.go`'s map. No renames, no logic edits.
Depends on P3.71.
**Files:** cmd/release/tag.go; cmd/release/githubrelease.go; cmd/release/publish.go; cmd/release/probes.go; cmd/release/channels.go; cmd/release/repo.go; cmd/release/version.go; cmd/release/doc.go
**Read first:** cmd/release/publish.go — describeTag, convergeTag, waitForTag, verifyTag, describeRelease, convergeRelease, buildReleaseAssets, checkPublishedSums, assetPaths; cmd/release/probes.go — probeTag, probeRelease, missingAssets, releaseAssetNames, capcomRepo, isReleaseNotFound, bareRelease; cmd/release/channels.go — downloadReleaseSums; cmd/release/repo.go; cmd/release/version.go
**Tests:** none new; the package's tests pass unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`): no changed lines beyond package and import headers, apart from `doc.go`'s map. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(release): move the tag and release steps into their own files`

## P3.73 Release: Homebrew and Scoop in one file each, smoke looping over them — ✅ DONE (2026-10-13)

NOTES (2026-10-13): the step table's smoke entries are one-line closures (`func(ctx, run) { run.checkHomebrew(ctx) }`) so the field keeps the plan's context-first `func(context.Context, *smokeRun)` while `checkHomebrew`/`checkScoop` stay methods, moved unchanged.
NOTES (2026-10-13): the shared `scoopManifest` also decodes `architecture`, so the probe now reads a manifest whose `architecture` is not an object of `{hash}` objects as pending ("not a readable manifest") where it used to read only `version`; the smoke check's behaviour is unchanged.
NOTES (2026-10-13): the winget consts stay in probes.go in their own const block (they move with P3.74); `formulaTemplate`/`scoopTemplate` stay in render.go with the other template names.

**What:** Third part of A-11: each of Homebrew and Scoop gets one file holding its probe, describe, converge, render and smoke check; the step record gains a smoke check that the smoke verb loops over.
**Regression guard.** `readRepoFile`, `pinnedVersionStep` and `isNotFound` live in probes.go at the base; P3.73 moves them (to channels.go) so probes.go is empty by P3.74.
**Goal:** `homebrew.go` and `scoop.go` hold all of their channel's code; the Scoop manifest is parsed by one function that the probe and the smoke check share; `runSmoke` runs each step's smoke check in table order (Homebrew, then Scoop, the output unchanged).
**Approach (assumed at the header base):** move `tapRepo`, `formulaPath`, `formulaVersionPattern`, `probeHomebrew`, `describeHomebrew`, `convergeHomebrew`, `renderFormula` and `smokeRun.checkHomebrew` to `homebrew.go`; `bucketRepo`, `bucketPath`, `scoopArchitectures`, `probeScoop`, `describeScoop`, `convergeScoop`, `renderScoopManifest`, `checkScoop`, `checkStaleBucket` to `scoop.go`. One `scoopManifest` type and `parseScoopManifest` replace the two anonymous structs; the probe still reads an unparseable manifest as pending, the smoke check as a failure. The step record gains `smoke func(context.Context, *smokeRun)`; `runSmoke` replaces its two channel calls with a loop over `publishSteps`. Shared helpers (`publishPackageFile`, `describePackageFile`, `packageCommitSubject`) stay in `channels.go`; `readRepoFile`, `pinnedVersionStep` and `isNotFound` move from `probes.go` to `channels.go`. Update `doc.go`.
Depends on P3.72.
**Files:** cmd/release/homebrew.go; cmd/release/scoop.go; cmd/release/probes.go; cmd/release/smoke.go; cmd/release/render.go; cmd/release/steps.go; cmd/release/channels.go; cmd/release/doc.go
**Read first:** cmd/release/probes.go — probeHomebrew, probeScoop, formulaVersionPattern, pinnedVersionStep, readRepoFile, isNotFound; cmd/release/smoke.go — runSmoke, smokeRun.checkHomebrew, smokeRun.checkScoop, checkStaleBucket, scoopArchitectures; cmd/release/channels.go — describeHomebrew, convergeHomebrew, describeScoop, convergeScoop, publishPackageFile; cmd/release/render.go — renderFormula, renderScoopManifest; cmd/release/smoke_test.go — TestSmokeChecks; cmd/release/probes_test.go — TestProbesReportEachState
**Tests:** none new; `TestSmokeChecks`, `TestProbesReportEachState` and the channel publish tests pass unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`): changed lines only in the manifest parse, the step record, `runSmoke`'s loop and `doc.go`. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
**Commit:** `refactor(release): one file each for the Homebrew and Scoop channels`

## P3.74 Release: winget in one file, and publish.go for orchestration only — ✅ DONE (2026-10-13)

NOTES (2026-10-13): consequential edit — cmd/release/steps.go: made necessary by moving the winget step out of channels.go (the publishSteps comment named channels.go as a channel step's file; it now names winget.go).
NOTES (2026-10-13): `setupPendingError` stays in channels.go (not on the move list: it is the generic step contract the publish loop reads, though only winget returns it today), and `removeBuildDir`, `sleepContext` and the publishRun's `isWingetPackageListed` field stay in publish.go as part of the run and the verb's defaults; probes_test.go keeps its name (tests unchanged).

**What:** Last part of A-11, a pure move: the winget channel and the smoke step leave `channels.go`, `probes.go`, `render.go` and `publish.go`; `probes.go` goes.
**Regression guard.** With P3.71's step-name consts in steps.go and P3.73's helpers in channels.go, probes.go is empty after this move list and is deleted; Acceptance checks it is gone. The winget template names and `packageData` methods stay in render.go.
**Goal:** `winget.go` holds every winget step, komac and manifest-rendering symbol (the template names and `packageData` methods stay in `render.go`); the smoke step's probe, describe and converge sit in `smoke.go`; `publish.go` holds only the verb, the run, the gate checks and the step-table printing; `probes.go` no longer exists.
**Approach (assumed at the header base):** move to `winget.go`: `wingetRepo`, `wingetPackageID`, `wingetManifestDir`, `probeWinget`, `titleNamesRelease`, `resolveWingetPackage`, `describeWinget`, `convergeWinget`, `submitWingetManifests`, `writeWingetManifests`, `komacCommandLine`, `komacSubmitArgs`, `komacArgs`, `githubToken`, `githubTokenVariable`, `redactedToken`, `komacInstallHint`, `wingetSubmittedDetail`, `wingetManifest`, `wingetManifests`, `renderWingetManifests`. Move `probeSmoke`, `describeSmoke`, `convergeSmoke`, `smokePassedDetail` to `smoke.go`. `probes.go`, then empty, is deleted. Update `doc.go`.
Depends on P3.73.
**Files:** cmd/release/winget.go; cmd/release/channels.go; cmd/release/probes.go; cmd/release/render.go; cmd/release/publish.go; cmd/release/smoke.go; cmd/release/doc.go
**Read first:** cmd/release/probes.go — probeWinget, probeSmoke, titleNamesRelease, pinnedVersionStep, readRepoFile, isNotFound; cmd/release/channels.go — resolveWingetPackage, convergeWinget, submitWingetManifests, githubToken; cmd/release/render.go — wingetManifest, wingetManifests, renderWingetManifests, packageTemplates; cmd/release/publish.go — describeSmoke, convergeSmoke, smokePassedDetail, removeBuildDir, sleepContext; internal/cli/doc_test.go — TestDocFileMap_MatchesDirectory
**Closes:** release-step-table
**Tests:** none new; the package's tests pass unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/`
- `go vet ./...`
- move-only: `git add -A && git diff HEAD --color-moved=zebra --color-moved-ws=allow-indentation-change` (no `--stat`): no changed lines beyond package and import headers, apart from `doc.go`'s map. The check also allows const/var block wrappers, doc comments naming the new file, the doc.go map and test helpers a split needs.
- `test ! -e cmd/release/probes.go`
**Commit:** `refactor(release): one file for the winget channel`

## P3.80 Release: a channel-repository seam for the reads — ✅ DONE (2026-10-13)

NOTES (2026-10-13): re-derived from "probeHomebrew, probeScoop, probeWinget and readRepoFile live in cmd/release/probes.go": the probes now live in homebrew.go, scoop.go and winget.go, and readRepoFile and isNotFound in channels.go (all already in Files except channels.go)
NOTES (2026-10-13): channels.go is edited although Files omits it: readRepoFile stays for checkScoop as the plan says, but as a one-line shim over execChannelRepos.readContents, so the contents argv lives only in execChannelRepos as the Goal requires; P3.82 deletes the shim
NOTES (2026-10-13): consequential edit — cmd/release/publish_test.go: made necessary by probeEnv swapping commands and dir for channels (its probeRelease check builds a probeEnv literal)
NOTES (2026-10-13): the "wraps a sentinel" parse error is unparsableAnswerError, which unwraps to the sentinel errUnparsableAnswer and to the parser's error and reads as the parser's message alone, so probeWinget's "could not parse the <dir> listing" and "could not parse gh pr list output" details stay verbatim; ReadFile's base64 decode failure wraps it too, its message unchanged ("decode <path>: ...")
NOTES (2026-10-13): TestProbesReportEachState gains two winget cases (listing and pull-request output that are not JSON) pinning those two details through the seam

**What:** First part of A-12: the tap, the bucket and winget-pkgs are read through a semantic `channelRepos` interface instead of raw `gh` argv in each probe.
**Regression guard.** Update doc.go's file map (a `channelrepos.go` line) in the same commit, so P3.71's doc_test stays green; this holds for every item from P3.71 on that adds or removes a non-test .go file in cmd/release. probes_test.go's three `probeEnv{repo:…, commands:…, dir:"."}` literals become `channels: execChannelRepos{commands: <same runner>, dir: "."}`. The Goal holds for the three channel probes; `resolveWingetPackage` keeps its listing argv until P3.82. `probeWinget`'s unknown details ("could not parse the <dir> listing", "could not parse gh pr list output") stay verbatim: the adapter's parse error wraps a sentinel the probe words as today.
**Goal:** one interface, `channelRepos`, answers `Exists(ctx, repo) (bool, error)`, `ReadFile(ctx, repo, path) (string, bool, error)`, `ListDirs(ctx, repo, path) ([]string, bool, error)` and `OpenPulls(ctx, repo, query) ([]pullRequest, error)`, a "no" never being an error; `execChannelRepos` is the only code holding their argv, and the three channel probes read through it.
**Approach (assumed at the header base):** new `channelrepos.go` holds the interface, `pullRequest{number, title, url}` and `execChannelRepos{commands runner; dir string}`, taking the argv of `probeHomebrew`, `probeScoop`, `probeWinget` and `readRepoFile` unchanged (a 404 is `false, nil`). `probeEnv` swaps `commands` and `dir` for `channels channelRepos`, built as `execChannelRepos` over the run's runner in `runStatus` and `publishRun.probe`. The argv being unchanged, the runner-level fakes stay green. `readRepoFile` stays for `checkScoop` until P3.82.
Depends on P3.74.
**Files:** cmd/release/channelrepos.go; cmd/release/channelrepos_test.go; cmd/release/steps.go; cmd/release/status.go; cmd/release/publish.go; cmd/release/homebrew.go; cmd/release/scoop.go; cmd/release/winget.go; cmd/release/doc.go; cmd/release/probes_test.go
**Read first:** cmd/release/probes.go — probeEnv, probeHomebrew, probeScoop, probeWinget, readRepoFile, isNotFound; cmd/release/status.go — runStatus; cmd/release/publish.go — publishRun.probe; cmd/release/probes_test.go — fakeRemote.respond, TestProbesReportEachState, TestProbeStepsRunsEveryStepInPublishOrder, TestProbesNeverReadAFailedCallAsDone; cmd/release/repo_test.go — TestExecRepoRunsEachMethodsArgv, TestExecRepoTellsANoFromABrokenCommand; cmd/release/channels.go — resolveWingetPackage
**Tests:** new `channelrepos_test.go` pins each method's argv and its yes, no (404, empty repository) and broken answers, as `TestExecRepoRunsEachMethodsArgv` does for `execRepo`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/`
- `go vet ./cmd/release/`
**Commit:** `refactor(release): read the channel repositories through a seam`

## P3.81 Release: the winget probe does not wait on the search index — ✅ DONE (2026-10-13)

NOTES (2026-10-13): OpenPulls runs the search listing first, then the viewer query (`gh api graphql -f query=...` with nonInteractiveEnv, the latest 100 open pull requests by creation date); the viewer's nodes are kept when their `repository.nameWithOwner` equals the repository case-insensitively, and appended after the search results unless the search already listed that number; a failed or unparsable viewer answer fails OpenPulls like the search does, so probeWinget reports unknown
NOTES (2026-10-13): winget.go is unchanged (not in Files), so probeWinget's "could not parse gh pr list output" detail now also covers an unparsable GraphQL viewer answer
NOTES (2026-10-13): the viewer is gh's signed-in account; a pull request komac opened under a different GITHUB_TOKEN account is still found only through search, as before
NOTES (2026-10-13): the two OpenPulls cases left TestExecChannelReposRunsEachMethodsArgv (which asserts exactly one call) for the new TestExecChannelReposOpenPullsReadsTheViewersPullsBesideTheSearch, which pins both argv and the merge; TestExecChannelReposTellsANoFromABrokenCommand gains a broken and an unparsable viewer answer; TestProbesReportEachState gains an unreadable viewer query case

**What:** Fixes F-14 (`winget-probe-search`): the winget probe finds an open pull request only through GitHub search, which lags, so an immediate rerun submits a duplicate pull request to `microsoft/winget-pkgs`.
**Regression guard.** Every status and publish test reaches the winget probe through `fakeRemote`, whose zero value answers the GraphQL call with an empty nodes list; Acceptance runs the whole package.
**Goal:** `execChannelRepos.OpenPulls` returns the open pull requests that the signed-in account authored in the repository, read without the search index, together with the search listing's, deduplicated by number; `probeWinget` reads `done` when either names the release.
**Approach (assumed at the header base):** `OpenPulls` adds `gh api graphql -f query=<viewer.pullRequests(states: OPEN, first: 100, orderBy CREATED_AT DESC) { nodes { number title url repository { nameWithOwner } } }>` (with `nonInteractiveEnv`), keeps the nodes whose `nameWithOwner` is the repository, and merges them with the `gh pr list --search` result. `probeWinget` still filters with `titleNamesRelease`.
Depends on P3.80.
**Files:** cmd/release/channelrepos.go; cmd/release/channelrepos_test.go; cmd/release/probes_test.go
**Read first:** cmd/release/probes.go — probeWinget, titleNamesRelease; cmd/release/probes_test.go — fakeRemote.respond, wingetPull, TestProbesReportEachState; cmd/release/publish_test.go — publishFake.respond, newPublishFake; cmd/release/status_test.go — statusCheckout, TestStatusReportsTheGateBaselineAndHighestTag; cmd/release/smoke.go — nonInteractiveEnv
**Tests:** `channelrepos_test.go` pins the GraphQL argv and the merge; `probes_test.go`'s `fakeRemote` answers the GraphQL call, and a new `TestProbeWingetFindsAPullRequestSearchHasNotIndexed` (search empty, the viewer's list holding the pull request) reads `done`. The new tests fail on the pre-item tree (bite check). `fakeRemote`'s zero value answers the GraphQL call with an empty nodes list.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'Winget|ChannelRepos|Probe'`
- `go vet ./cmd/release/`
- `GOMEMLIMIT=2GiB go test ./cmd/release/`
**Commit:** `fix(release): find an open winget pull request without the search index`

## P3.82 Release: publish a channel file through the seam — ✅ DONE (2026-10-13)

NOTES (2026-10-13): consequential edit — cmd/release/doc.go: made necessary by removing readRepoFile; the file map's channels.go line no longer says it reads a channel file
NOTES (2026-10-13): execChannelRepos.readContents, kept only for readRepoFile's 404-as-failure read, is folded into ReadFile; ReadFile's argv, decoding and errors are unchanged
NOTES (2026-10-13): publishPackageFile stays as the shared helper convergeHomebrew and convergeScoop call; it now calls run.channels.PublishFile and prints "already holds this <path>: nothing to push" when nothing was pushed
NOTES (2026-10-13): checkScoop on a missing manifest skips with the same "the bucket's manifest <path> in <repo> is unreadable" wording, now ending "there is no such file" instead of gh's 404 text; resolveWingetPackage through ListDirs now refuses a listing that is not JSON instead of reading it as listed (the winget probe already parsed the same listing)

**What:** Second part of A-12: the tap and bucket write, and the remaining raw channel reads, go through `channelRepos`.
**Regression guard.** `TestPublishWritesTheRenderedFileIntoTheClone` builds `&publishRun{…}` directly; that literal sets `channels: execChannelRepos{commands: commands, dir: …}`. `checkScoop` skips on `found == false` with today's "the bucket's manifest %s in %s is unreadable" wording.
**Goal:** `channelRepos.PublishFile(ctx, repo, path, content, subject) (bool, error)` commits and pushes a file only when it changes it; `execChannelRepos` owns the clone, `rev-list`, write, `add`, `diff`, commit and push, each failure naming the repository and the step; `convergeHomebrew`, `convergeScoop`, `resolveWingetPackage` and `checkScoop` use the seam, and `readRepoFile` no longer exists.
**Approach (assumed at the header base):** `publishPackageFile`'s body moves into `execChannelRepos.PublishFile` (an empty repository still pushes `HEAD:main`); the caller prints `already holds this <path>: nothing to push` on `false`. `publishRun` and `smokeRun` gain `channels`, built as `execChannelRepos` over their runner. `resolveWingetPackage` calls `ListDirs`; `checkScoop` calls `ReadFile`.
Depends on P3.81.
**Files:** cmd/release/channelrepos.go; cmd/release/channelrepos_test.go; cmd/release/channels.go; cmd/release/homebrew.go; cmd/release/scoop.go; cmd/release/winget.go; cmd/release/publish.go; cmd/release/smoke.go; cmd/release/channels_test.go; cmd/release/smoke_test.go
**Read first:** cmd/release/channels.go — publishPackageFile, convergeHomebrew, convergeScoop, resolveWingetPackage; cmd/release/smoke.go — smokeRun, runSmoke, smokeRun.checkScoop; cmd/release/channels_test.go — respondChannel, TestPublishWritesTheRenderedFileIntoTheClone, TestPublishCommitsAChannelFileOnlyWhenItChanges, TestPublishPushesAnEmptyBucketToMain; cmd/release/smoke_test.go — smokeFake.respond, smokeReadBucket; cmd/release/publish.go — runPublish, publishRun
**Tests:** `channelrepos_test.go` pins `PublishFile`'s argv for a changed file, an unchanged one and an empty repository, and that a failing clone, `rev-list`, commit or push names the repository and the step. `TestPublishChannelStepFailureStopsTheRun` (P3.70) and the channel publish tests pass unchanged. `TestSmokeChecks`' "the bucket's manifest is unreadable" case passes unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/`
- `go vet ./cmd/release/`
**Commit:** `refactor(release): publish channel files through the channel seam`

## P3.83 Release: inject the channel seam, with an in-memory adapter — ✅ DONE (2026-10-13)

NOTES (2026-10-13): channelReposOrExec (nil options.channels -> execChannelRepos over the run's runner in options.dir), the one helper status, publish and smoke share, sits in cmd/release/channelrepos.go beside execChannelRepos, a file the item's Files did not list; main passes execChannelRepos{commands, dir: "."}, the dir every verb used before
NOTES (2026-10-13): convergeSmoke now forwards publish's options.channels to the smoke run, as it already forwards openRepo
NOTES (2026-10-13): TestProbeWingetFindsAPullRequestSearchHasNotIndexed is deleted from probes_test.go without editing channelrepos_test.go: TestExecChannelReposOpenPullsReadsTheViewersPullsBesideTheSearch already holds the OpenPulls merge cases it moves to ("the viewer's pull request search has not indexed": search empty, viewer list holding it; and the other-repository case); its title cases (first submission, another version) are TestProbesReportEachState cases over memChannelRepos
NOTES (2026-10-13): memChannelRepos answers a missing repository with "no" on every read (OpenPulls included, an empty list) and refuses PublishFile there; OpenPulls ignores the query and returns every open pull request held, since the probe judges titles itself; failures are keyed by (method, repository), and an unparsable answer is stated as a failure wrapping errUnparsableAnswer
NOTES (2026-10-13): the probes table drops the exec-only "winget viewer's pull requests unreadable" case (channelrepos_test.go's "OpenPulls on a broken viewer query" pins it) and gains "winget listing unreadable" and "winget pull request for another version"; TestProbeStepsRunsEveryStepInPublishOrder also moves onto memChannelRepos
NOTES (2026-10-13): statusCheckout's runner now refuses every command, since its repo and channel repositories answer everything status asks; fakeRemote stays in probes_test.go for publishFake and the exec adapter's tests until P3.84

**What:** Third part of A-12: every verb takes its `channelRepos` from outside, and the tests gain `memChannelRepos`, the second adapter; the probe and status tests move onto it.
**Regression guard.** prepare_test.go and smoke_test.go call `executeWith`; they pass nil, keeping the exec adapter over their runner. `memChannelRepos` records every read (method, repository), and `TestStatusProbesTheVersionFileOnlyAboveTheBaseline` asserts none at the baseline. P3.81's `TestProbeWingetFindsAPullRequestSearchHasNotIndexed` moves into channelrepos_test.go as an `OpenPulls` merge test (search empty, viewer list holding the pull request).
**Goal:** `newRootCommand` and the status, publish and smoke options carry a `channelRepos` (nil meaning `execChannelRepos` over the run's runner); `memChannelRepos` holds the tap, bucket and winget-pkgs as plain state (repositories, files, directories, open pull requests, what `PublishFile` wrote and with which subject, a per-method and per-repository failure); the probe and status tests state channel state, not argv.
**Approach (assumed at the header base):** `newRootCommand(commands, openRepo, channels)`; `main` passes `execChannelRepos{commands: execRunner{}}`; `executeWith` (`main_test.go`) takes the adapter. `memChannelRepos` lives in `memchannels_test.go`, recording writes into a runner log the way `memPublish` does. `probes_test.go`'s cases and `statusCheckout` use it; `fakeRemote` stays for `publishFake` until P3.84.
Depends on P3.82.
**Files:** cmd/release/memchannels_test.go; cmd/release/main.go; cmd/release/main_test.go; cmd/release/status.go; cmd/release/publish.go; cmd/release/smoke.go; cmd/release/probes_test.go; cmd/release/status_test.go; cmd/release/prepare_test.go; cmd/release/smoke_test.go; cmd/release/channelrepos_test.go
**Read first:** cmd/release/main.go — main, newRootCommand; cmd/release/main_test.go — execute, executeWith, TestCommandTreeOpensTheInjectedRepo; cmd/release/status_test.go — statusCheckout, TestStatusProbesTheVersionFileOnlyAboveTheBaseline, TestStatusFailsOnAStepItCouldNotProbe; cmd/release/memrepo_test.go — memRepo, memPublish; cmd/release/probes_test.go — TestProbesReportEachState, TestProbesNeverReadAFailedCallAsDone; cmd/release/prepare_test.go — executeWith callers; cmd/release/smoke_test.go — executeWith callers
**Tests:** `TestProbesReportEachState`, `TestProbesNeverReadAFailedCallAsDone` and the status tests run over `memChannelRepos`; `TestCommandTreeOpensTheInjectedRepo` also asserts the injected channel adapter reaches the verbs. `TestStatusProbesTheVersionFileOnlyAboveTheBaseline` asserts no `memChannelRepos` read at the baseline; the winget search case becomes an `OpenPulls` merge test in channelrepos_test.go.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/`
- `go vet ./cmd/release/`
**Commit:** `refactor(release): inject the channel seam and add its in-memory adapter`

## P3.84 Release: publish and smoke tests on the in-memory channel repos — ✅ DONE (2026-10-13)

NOTES (2026-10-13): TestPublishWritesTheRenderedFileIntoTheClone renamed TestPublishWritesTheRenderedFileToTheChannel: the fake has no clone left, and the test now asserts the memChannelPublish records (repo, path, rendered content, subject)
NOTES (2026-10-13): TestPublishCommitsAChannelFileOnlyWhenItChanges drives convergeHomebrew/convergeScoop directly over a run on memChannelRepos, so a repository already holding the rendered file still reaches PublishFile (a full run's probe would report it done first); it asserts which repositories were pushed and the "already holds this ...: nothing to push" line
NOTES (2026-10-13): mutatingCommands also drops "gh release delete", a dead runner prefix nothing in cmd/release issues, alongside "git push", "gh repo clone" and "git commit"
NOTES (2026-10-13): smokeFake's `failing` map went with the bucket answer (its one user was smokeReadBucket); a `bucketFailure` field feeds memChannelRepos.withFailure(ReadFile) instead, and smoke tests calling executeWith pass fake.channels()
NOTES (2026-10-13): probes_test.go's wingetListing helper deleted (no caller left); contentsAPIBase64, wingetPull and wingetViewerPull stay there because channelrepos_test.go uses them; new helper channelReposOf merges memChannelRepos parts
NOTES (2026-10-13): TestPublishChannelStepFailureStopsTheRun (T-01) is now two cases, PublishFile failing for the tap, then for the bucket; the per-step clone/history/commit/push failures stay in channelrepos_test.go

**What:** Last part of A-12: the publish, channel and smoke tests hold the channel repositories in `memChannelRepos`; T-01's test is rewritten onto it; the runner-level channel fakes go.
**Regression guard.** `mutatingCommands` gains `memChannelRepos`' `PublishFile` log prefix (logged as `memPublish.note` does), and the two `unwanted` lists name it in place of "git push"/"gh repo clone"; the dead runner prefixes go. The prose grep adds `winget-pkgs commands\|runner seams\?`; runner.go, the `release-process` Note and adr-0008-amend-bumptest are edited (Notes through `capcom know update`). `TestPublishPushesAnEmptyBucketToMain` is deleted: P3.82's channelrepos_test covers the empty-repository push. The Goal's checks are in Acceptance.
**Goal:** outside `channelrepos_test.go`, no test answers a `gh` or `git` command aimed at the tap, the bucket or winget-pkgs (komac, `gh auth token` and brew stay on the runner); the ADR 0008 Note and `doc.go` name three seams.
**Approach (assumed at the header base):** `publishFake` swaps `remote fakeRemote` and the clone and in-clone `git` cases of `respondChannel` for a `memChannelRepos`; assertions on `git commit`/`git push` lines become assertions on what `PublishFile` recorded. `smokeFake`'s bucket answer and `smokeReadBucket` go. `fakeRemote` is deleted. `TestPublishChannelStepFailureStopsTheRun` fails `PublishFile` for the tap, then the bucket (the per-step failures live in `channelrepos_test.go` since P3.82). Prose guard: every prose site that says the tap, bucket or winget-pkgs commands go through the runner, or that there are two seams, names `channelRepos`; find them with `grep -rn 'two seams\|Two seams\|runner seam\|tap, bucket and winget-pkgs\|winget-pkgs commands\|runner seams\?' cmd/release docs .capcom/knowledge .agents`; edit `adr-0008-decision` through `capcom know update … --body-replace-old/--body-replace-new`.
Depends on P3.83.
**Files:** cmd/release/publish_test.go; cmd/release/channels_test.go; cmd/release/smoke_test.go; cmd/release/probes_test.go; cmd/release/memchannels_test.go; cmd/release/doc.go; .capcom/knowledge/decisions/adr-0008/adr-0008-decision.md; cmd/release/runner.go; .capcom/knowledge/release-process.md; .capcom/knowledge/decisions/adr-0008/adr-0008-amend-bumptest.md
**Read first:** cmd/release/publish_test.go — mutatingCommands, mutatingLines, publishFake, newPublishFake, runTestPublishWith; cmd/release/channels_test.go — releasedFake, respondChannel, callsInClone, TestPublishCommitsAChannelFileOnlyWhenItChanges, TestPublishWritesTheRenderedFileIntoTheClone, TestPublishPushesAnEmptyBucketToMain, TestPublishSkipsChannelsWhoseSetupIsPending; cmd/release/probes_test.go — fakeRemote, contentsAPIBase64, wingetListing, wingetPull; cmd/release/smoke_test.go — smokeReadBucket, smokeFake.respond, TestSmokeChecks; cmd/release/memrepo_test.go — memPublish.note, repoCallName; cmd/release/runner.go — runner; .capcom/knowledge/decisions/adr-0008/adr-0008-decision.md; .capcom/knowledge/release-process.md
**Closes:** channel-repo-seam; winget-probe-search
**Tests:** the rewritten publish, channel and smoke tests; T-01's test covers both repositories and asserts no smoke run.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./cmd/release/`
- `go vet ./cmd/release/`
- `capcom check`
- `grep -nE 'fakeRemote|respondChannel|smokeReadBucket|callsInClone|gh repo clone' cmd/release/*_test.go | grep -v channelrepos_test.go` prints nothing
- `grep -l channelRepos cmd/release/doc.go .capcom/knowledge/decisions/adr-0008/adr-0008-decision.md` lists both
**Commit:** `test(release): hold the channel repositories in memory in the publish and smoke tests`

## P3.90 check.Run takes the opened store — ✅ DONE (2026-10-13)

**What:** Part 1 of A-10: `check.Run(s *store.Store)` validates an opened store, and `briefing.Compose` hands it the store it opened, so a session start opens the store once.
**Regression guard.** The briefing `runCheck` seam changes type to take `*store.Store`, so P0.2's panic test (and any other test assigning it) is updated.
**Goal:** `check.Run`'s parameter is `*store.Store`; `check.ConfigProblems(err error) []Problem` turns a store-open `*config.Error` into the single `config` Problem (nil otherwise); `capcom check` output, a config failure included, and `capcom prime` output are byte-identical.
**Approach (assumed at the header base):** `Run` drops its `store.Open`; `ConfigProblems` holds the mapping `Run` did on that error. The cli check command opens the store; on an error, a non-nil `ConfigProblems(err)` renders as today's problem list, anything else goes through `storeFailure`. `briefing.readHealth` takes the store `Compose` opened (ADR 0009: prime still turns a store failure into a warning; a config failure already stops at `SectionStore`). `check` tests open the store in `runCheck`; its config-failure tests call `store.Open`, then `ConfigProblems`.
Depends on P3.63; P3.4.
**Files:** internal/check/check.go; internal/check/check_test.go; internal/cli/check.go; internal/briefing/briefing.go; internal/briefing/briefing_test.go
**Read first:** internal/check/check.go — Run, checker; internal/check/check_test.go — runCheck, TestRun_OneFixturePerProblemClass_ReportsPathLineAndCode (its "config error" row), TestRun_NamedPipeIssueFile_ReportedNotRegularWithoutBlocking; internal/cli/check.go — newCheckCommand; internal/briefing/briefing.go — Compose, readHealth; internal/cli/check_test.go — TestCheck_WithoutStore_FailsNotInitialized; internal/store/store.go — Open
**Tests:** the check config-failure tests move onto `ConfigProblems`; cli `TestCheck_*` stay unchanged; P0.2's panic test in briefing_test.go assigns `runCheck` with its new type.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Check|Prime'`
- `GOMEMLIMIT=2GiB go test ./internal/briefing/`
- `go vet ./internal/check/ ./internal/cli/ ./internal/briefing/`
**Commit:** `refactor(check): take the opened store instead of cwd`

## P3.91 The store names its issue-tree pathspec — ✅ DONE (2026-10-13)

**What:** Part 2 of A-10: `(*store.Store).IssuesPathspec()` replaces cli's `issuesPathspec` and briefing's own computation for the Elsewhere scan.
**Goal:** `IssuesPathspec() []string` returns the issue tree's path relative to the worktree root with forward slashes, or nil when it lies outside the worktree; `list`, `ready` and `prime` Elsewhere output is byte-identical.
**Approach (assumed at the header base):** in `store.go`, the rule both copies apply (`filepath.Rel` of `IssuesDir` to `WorktreeRoot`, nil on an error or a `..` prefix). `briefing.scanElsewhere` scans nothing on nil; cli's `list` and `ready` pass `a.store.IssuesPathspec()` to `addElsewhere`; `app.issuesPathspec` is deleted (in `output.go` at the base, or the cli file P3.6 left `addElsewhere` in).
Depends on P3.90.
**Files:** internal/store/store.go; internal/store/store_test.go; internal/briefing/briefing.go; internal/cli/list.go; internal/cli/ready.go; internal/cli/output.go
**Read first:** internal/cli/output.go — issuesPathspec, addElsewhere; internal/cli/issue_render.go — relativeSlashPath; internal/briefing/briefing.go — scanElsewhere; internal/cli/list.go — newListCommand; internal/cli/ready.go — newReadyCommand; internal/store/store.go — Store, IssuesDir; internal/store/store_test.go — newRepo
**Tests:** `store_test.go` `TestIssuesPathspec_InsideTheWorktree_IsTheRelativeSlashPath`, and nil for an issues directory outside the worktree.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/store/`
- `GOMEMLIMIT=2GiB go test ./internal/briefing/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'List|Ready|Elsewhere|Prime'`
- `go vet ./internal/store/ ./internal/briefing/ ./internal/cli/`
**Commit:** `refactor(store): name the issue-tree pathspec once`

## P3.92 A worktree lock handle with its key bound — ✅ DONE (2026-10-13)

**What:** Part 3 of A-10: `lock.ForWorktree(s *store.Store) Worktree` pairs the lock directory with this worktree's key, so no caller pairs `lock.Open` and `WorktreeKey` by hand.
**Goal:** `Worktree` has `Acquire(claim, token, isTakeStale)`, `Touch(token)`, `Release(token)`, `Break()`, `Check(token)`, `Read()`, `RecordPath()`, `Key()` and `List()`, each behaving as the `Locks` method for its key; every `capcom lock` output is byte-identical.
**Approach (assumed at the header base):** new `internal/lock/worktree.go`; `lock` gains an import of `store`, which imports neither `lock` nor anything that does. cli's `lockTarget` holds the store and a `lock.Worktree`, and each lock verb drops `target.key`. `briefing.readLocks` keeps `lock.Open(...).List()`, which lists every worktree.
Depends on P3.91.
**Files:** internal/lock/worktree.go; internal/lock/worktree_test.go; internal/cli/lock.go
**Read first:** internal/lock/lock.go — Locks, Open, WorktreeKey, RecordPath, Acquire, Touch, List; internal/cli/lock.go — lockTarget, openLockTarget, emitDiscardedLock; internal/briefing/briefing.go — readLocks; internal/lock/lock_test.go — TestLocks_AcquireTouchRelease_CycleFreesTheLock; internal/store/store.go — Store
**Tests:** `worktree_test.go`, on a `store.Store` literal in a temp dir: the main worktree's `Key()` is `MainWorktreeKey`; a linked worktree's is `wt-<name>`; Acquire then Touch through the handle writes the record at `RecordPath()`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/lock/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Lock'`
- `go vet ./internal/lock/ ./internal/cli/`
**Commit:** `refactor(lock): bind the worktree key in a lock handle`

## P3.93 Commands get the opened store from the root — ✅ DONE (2026-10-13)

NOTES (2026-10-13): openLockTarget becomes an app method returning lockTarget without an error, since the root has already opened the store; its name is kept.
NOTES (2026-10-13): TestStoreCommands_OutsideAStore_FailAsBefore also covers a bad config.yaml (the Goal's third failure), besides outside git and no store; it passed on the pre-item tree before any code change.

**What:** Part 4 of A-10: the root opens the store, through the command annotation, for `agents`, `fmt` and every `lock` subcommand, which drop their own `store.Open`.
**Regression guard.** Keep today's precedence for the storeOnly commands: a missing required flag is still reported before a store-open failure. For storeOnly, `prepareIndex` calls `cmd.ValidateRequiredFlags()` before `store.Open`; no user-visible change. Each wanted stderr is built from the test's dir (`"capcom: " + dir + ": " + store.ErrNotGitRepository.Error() + "\n"`, and from `filepath.Join(root, ".capcom")` for the not-initialised case), not captured once. The Goal's "read `a.store`" is checked by Acceptance.
**Goal:** an `annotationIndex` value `storeOnly` makes `prepareIndex` open the store without the index and set `app.store`; `agents`, `fmt` and `lock` read `a.store`; `prime`, `init` and `check` open it themselves; every failure (outside git, no store, bad config) renders byte-identically.
**Approach (assumed at the header base):** `prepareIndex` returns after `store.Open` when the mode is `storeOnly`. `openLockTarget` builds from `a.store` with `lock.ForWorktree`; `fmt` drops its `store.Open` and its `a.store` assignment; `agents` passes `a.store`. Binding: `check` keeps its own open, because a config failure is its result (a `config` problem), not a failed command; the review counted it among the four.
Depends on P3.92.
**Files:** internal/cli/root.go; internal/cli/agents.go; internal/cli/fmt.go; internal/cli/lock.go; internal/cli/cli_test.go
**Read first:** internal/cli/root.go — prepare, prepareIndex, annotationIndex, closeIndex; internal/cli/lock.go — openLockTarget, lockTarget, newLockAcquireCommand, newLockTouchCommand; internal/cli/fmt.go — newFmtCommand; internal/cli/agents.go — newAgentsCommand; internal/cli/init.go — storeFailure; internal/cli/init_test.go — outsideGitDir, TestInit_OutsideGit_FailsInvalid; internal/cli/lock_test.go — TestLockAcquire_WithoutPurpose_ExitsUsage; internal/cli/testharness_test.go — runCLI
**Closes:** store-entry-points
**Tests:** `cli_test.go` `TestStoreCommands_OutsideAStore_FailAsBefore`: `agents`, `fmt` and `lock status`, outside git and in a repository without a store; asserts the exact stderr and exit code captured from the pre-item tree. `TestStoreCommands_OutsideAStore_FailAsBefore` adds `lock acquire` without `--purpose` outside a store (exit 2, the required-flag error), and builds each wanted stderr from the test's dir.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Agents|Fmt|Lock|Check|Prime|Init|StoreCommands'`
- `go vet ./internal/cli/`
- `grep -n 'store.Open' internal/cli/agents.go internal/cli/fmt.go internal/cli/lock.go` prints nothing
**Commit:** `refactor(cli): open the store in the root for agents, fmt and lock`

## P3.94 A generic frontmatter field table, used by issues — ✅ DONE (2026-10-13)

**What:** Part 1 of A-13: `frontmatter.Schema[T]` with `Text`, `List` and `Raw` field builders replaces the issue package's field-table plumbing.
**Regression guard.** The Goal's "no old helpers" clause is checked by an Acceptance grep.
**Goal:** `internal/frontmatter` exports `Schema[T]` (`NewSchema(noun string, fields ...SchemaField[T])`, `Kind() Kind`, `Decode(path string, data []byte, node *T) (Document, error)`, `Encode(node T, body string) []byte`) and the three builders; `internal/issue/file.go` declares its keys as one `Schema[Issue]` with no `field`, `textField`, `listField`, `fieldKeys` or unreachable return; issue files round-trip byte-identically.
**Approach (assumed at the header base):** `Text(key, at func(*T) *string)`, `List(key, at func(*T) *[]string)` (entry lines dropped), `Raw(key, decode func(Field, *T) error, encode func(T, *Encoder))` for priority, the timestamps and any list keeping its lines. Decode dispatches by key; the field order is the encode order and the Kind's Keys. `issueKind`'s noun stays `an issue`. Built on the decoder as P1.3 left it.
Depends on P3.93; P1.3.
**Files:** internal/frontmatter/schema.go; internal/frontmatter/schema_test.go; internal/issue/file.go
**Read first:** internal/issue/file.go — fields, textField, listField, timeField, fieldKeys, Parse, Serialize, priority, timestamp; internal/frontmatter/decode.go — Decode, Kind, Document, Field, keysHint; internal/frontmatter/encode.go — Encoder.Text, Encoder.List, Encoder.Raw, Encoder.File; internal/frontmatter/decode_test.go — issueKind, noteKind, decode; internal/issue/file_test.go — TestParseSerialize_CanonicalGoldens_AreIdentity, TestParse_Failures_CiteFileLine, TestSerialize_EmptyIssue_WritesPriorityOnly
**Tests:** `schema_test.go`: a two-key struct round-trips; an unknown key fails Decode with the Kind's key hint; a `Raw` decode error surfaces with its line. The existing `issue` file tests stay unchanged.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/frontmatter/`
- `GOMEMLIMIT=2GiB go test ./internal/issue/`
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `go vet ./internal/frontmatter/ ./internal/issue/`
- `grep -nE 'type field |func (textField|listField|fieldKeys)\(|unreachable' internal/issue/file.go` prints nothing
**Commit:** `refactor(frontmatter): add a generic field table and use it for issues`

## P3.95 Notes use the generic frontmatter field table

**What:** Part 2 of A-13: `internal/knowledge/note.go` declares its keys as one `frontmatter.Schema[Note]`.
**Regression guard.** The Goal's "no old helpers" clause is checked by an Acceptance grep.
**Goal:** `note.go` has no `field`, `listField`, `fieldKeys` or unreachable return; Note files round-trip byte-identically (existing knowledge tests); `refs` and `superseded-by` still record their entry lines.
**Approach (assumed at the header base):** title through `Text`; labels and aliases through `List`; refs, retired (written only when true) and superseded-by through `Raw`.
Depends on P3.94.
**Files:** internal/knowledge/note.go
**Read first:** internal/knowledge/note.go — fields, listField, retired, fieldKeys, noteKind, Parse, Serialize, RefLine, SupersededByLine; internal/knowledge/knowledge_test.go — TestSerialize_ThenParse_RoundTrips, TestParse_FullNote_ReadsFieldsAndRefLines, TestParse_RetiredFalse_IsDroppedOnSerialize, TestValidate_SupersededBy_ReportsBadEntriesAtTheirLines
**Closes:** frontmatter-schema
**Tests:** none new; the knowledge round-trip and line tests guard it.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/knowledge/`
- `GOMEMLIMIT=2GiB go test ./internal/check/`
- `go vet ./internal/knowledge/`
- `grep -nE 'type field |func (listField|fieldKeys)\(|unreachable' internal/knowledge/note.go` prints nothing
**Commit:** `refactor(knowledge): declare Note keys with the generic field table`

## P3.97 index exports only what its callers use

**What:** Part 1 of A-15, which runs after every other item: `Index.Refresh`, `Index.Rebuild`, `WithClock` and `Report`'s `Rebuilt`, `Parsed` and `Removed` fields, used only by `Open` and the package's own tests, are deleted or unexported.
**Regression guard.** The Goal's grep is copied into Acceptance as a command with its expected result. The Goal and Acceptance grep must not match `nodestore.Removed` (P3.47 adds it to internal/cli): append `| grep -v 'nodestore\.Removed'` to the Acceptance pipeline and say so in the Goal.
**Goal:** `grep -rnE 'index\.WithClock|\.(Refresh|Rebuild)\(\)|\.(Parsed|Removed|Rebuilt)\b' --include=*.go . | grep -v 'nodestore\.Removed'` finds matches only under `internal/index` (`nodestore.Removed`, which P3.47 adds to internal/cli, is filtered out); `Report` keeps `Indexed` and `Warnings` exported; index tests pass.
**Approach (assumed at the header base):** delete `Refresh` and `Rebuild`, thin wrappers of `refresh(isForced)`, and have the tests call `refresh(false)` and `refresh(true)`. `WithClock` becomes `withClock`; `Open` keeps `options ...Option`. The fields become `rebuilt`, `parsed` and `removed`; `Report()` keeps cloning them.
Depends on P3.95.
**Files:** internal/index/index.go; internal/index/refresh.go; internal/index/index_test.go; internal/index/notes_test.go
**Read first:** internal/index/refresh.go — (*Index).Refresh, (*Index).Rebuild, (*Index).refresh, the sync loop appending report.Parsed/Removed; internal/index/index.go — Option, WithClock, Report, Open, (*Index).Report; internal/index/index_test.go — laterClock, openIndex, refresh helper; internal/index/notes_test.go — report.Rebuilt check; internal/cli/root.go — prepareIndex (indexReport); internal/cli/reindex.go — newReindexCommand
**Tests:** the index tests switch to the unexported names.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/index/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Reindex'`
- `go vet ./...`
- `grep -rnE 'index\.WithClock|\.(Refresh|Rebuild)\(\)|\.(Parsed|Removed|Rebuilt)\b' --include=*.go . | grep -v '^\./internal/index/' | grep -v 'nodestore\.Removed'` prints nothing
**Commit:** `refactor(index): unexport refresh internals used only by tests`

## P3.98 agents and tree export only their production interface

**What:** Part 2 of A-15: `agents.Inspect`, `Write`, `Render`, `TopNotes` and `Path`, used outside `agents` only by a cli test, are unexported, and `tree.(*Tree).Dir`, which has no production caller, is deleted.
**Regression guard.** Drop the `successorsIn` edit (P3.19 has removed it); the rest stands. The Goal's grep is copied into Acceptance as a command with its expected result.
**Goal:** `grep -rnE 'agents\.(Inspect|Write|Render|TopNotes|Path)\b|\.Dir\(\)' --include=*.go internal` finds nothing outside `internal/agents`; agents, tree, knowledge and cli tests pass.
**Approach (assumed at the header base):** run that grep first and unexport exactly what only `agents` and its tests use. `cli/agents_test.go` `TestAgentsCheck_StaleBlock_FailsAtTheMarkerLine` builds its stale block without `agents.Render`: write `# Guide\n`, run `capcom agents` (it appends after a blank line, so the begin marker stays on line 3), then change one Note title inside the block with `strings.Replace`; its assertions stay.
Depends on P3.97.
**Files:** internal/agents/block.go; internal/agents/block_test.go; internal/cli/agents_test.go; internal/tree/tree.go; internal/knowledge/knowledge_test.go
**Read first:** internal/agents/block.go — Path, TopNotes, Render, Inspect, Write, Sync, reconcile, splice; internal/cli/agents_test.go — TestAgentsCheck_StaleBlock_FailsAtTheMarkerLine, writeAgentsGuide; internal/cli/know_test.go — newKnowStore; internal/tree/tree.go — (*Tree).Dir, Nodes; internal/knowledge/knowledge_test.go — successorsIn; internal/agents/block_test.go — writeGuide
**Tests:** the two adjusted tests above; nothing new.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/agents/`
- `GOMEMLIMIT=2GiB go test ./internal/tree/`
- `GOMEMLIMIT=2GiB go test ./internal/knowledge/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Agents|Know'`
- `go vet ./...`
- `grep -rnE 'agents\.(Inspect|Write|Render|TopNotes|Path)\b|\.Dir\(\)' --include=*.go internal | grep -v '^internal/agents/'` prints nothing
**Commit:** `refactor(agents): unexport helpers used only inside the package`

## P3.99 edit.Labels is named for what it does

**What:** Part 3 of A-15: `edit.Labels`, which edits labels, Refs and superseded-by alike, becomes `edit.List`; `internal/edit` keeps only its body and list helpers (P2.9 moved `ETag` to `nodefile`).
**Regression guard.** The Goal's grep is copied into Acceptance as a command with its expected result.
**Goal:** `grep -rn 'edit\.Labels' internal/` finds nothing; `edit.List` keeps `Labels`' behaviour and tests.
**Approach (assumed at the header base):** rename, with a doc comment naming any frontmatter list; callers are what `grep -rn 'edit\.Labels' internal/` finds (at the base `update.go`, `create.go`, `know.go`, `know_update.go`; `know_add.go` after P2.21).
Depends on P3.98.
**Files:** internal/edit/edit.go; internal/edit/edit_test.go; internal/cli/update.go; internal/cli/create.go; internal/cli/know_add.go; internal/cli/know_update.go
**Read first:** internal/edit/edit.go — Labels, package doc ("label merging"); internal/edit/edit_test.go — edit.Labels cases; internal/cli/know_update.go — Labels/Refs/SupersededBy merges; internal/cli/update.go — applyUpdate; internal/cli/create.go — issue builder Labels; internal/cli/know.go — Note builder Labels (know_add.go after P2.21)
**Closes:** trim-shallow-interfaces
**Tests:** `edit_test.go`'s `Labels` tests renamed to `List`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/edit/`
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Update|Create|KnowAdd|KnowUpdate'`
- `go vet ./internal/edit/ ./internal/cli/`
- `grep -rn 'edit\.Labels' internal/` prints nothing
**Commit:** `refactor(edit): rename Labels to List for every frontmatter list`

## P4.1 Briefing failure order beyond index and locks

**What:** Covers T-03 (`briefing-failure-order-test`): the ready, doing, check and Elsewhere failure branches of `Compose` and the documented failure order (store, index, ready, doing, locks, check, then Elsewhere warnings).
**Regression guard.** P0.9 deletes a branch tip's tree object, and a ref naming a missing commit fails `for-each-ref` as a whole, so this test does not reuse P0.9's setup or depend on it: it writes `.git/refs/heads/<b>` holding 40 hex digits of no object through the files map passed to `newBriefingRepo`, and the one warning starts `elsewhere: list branches: `.
**Tests:** package `briefing`, no `t.Parallel()`:
- Seams: beside P0.2's section-reader seam (reused where it already covers a reader), unexported function variables in `briefing.go` for the Ready list (`ReadyList`), the doing query and the check run (`check.Run`), defaulting to the real calls; a test swaps one and restores it in `t.Cleanup`.
- `TestCompose_ReadyOrDoingFails_FailsThatSectionAndKeepsTheRest`: table; a ready error gives failures `[ready]`, a doing error `[doing]`; each has `Work == nil` and `Locks`, `Health` kept.
- `TestCompose_CheckFailsAndElsewhereWarns_FailInOrderAndKeepTheRest`: the check seam fails, and a branch ref `.git/refs/heads/<b>` holding 40 hex digits of no object (written through the files map passed to `newBriefingRepo`) gives one Elsewhere warning starting `elsewhere: list branches: `; failures are `[check elsewhere]` in that order, `Health == nil`, `Work` and `Locks` kept.
- Bite: swapping `readHealth` and `scanElsewhere` in `Compose`, or setting `Health` before the check error returns, fails a test.
Depends on P3.90; P0.2.
**Files:** internal/briefing/briefing.go; internal/briefing/briefing_test.go
**Read first:** internal/briefing/briefing.go — Compose, readWork, readHealth, scanElsewhere, Briefing.fail; internal/briefing/briefing_test.go — newBriefingRepo, briefingStore, failedSections, TestCompose_BrokenIndexAndLocks_FailInOrderAndKeepTheRest; internal/briefing/ready.go — ReadyList; internal/elsewhere/elsewhere.go — Scan; internal/gitx/branches.go — Branches, branchFormat
**Closes:** briefing-failure-order-test
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/briefing/`
- `go vet ./internal/briefing/`
**Commit:** `test(briefing): pin the ready, doing, check and Elsewhere failure order`

## P4.3 Lock Touch by a displaced holder, and a heartbeat that must advance

**What:** Covers T-05 (`lock-touch-tests`): the displaced session's Touch after a stale takeover, and a heartbeat assertion that fails when Touch leaves it unchanged.
**Tests:** `internal/lock/lock_test.go`:
- `TestTouch_TokenOfDisplacedHolder_FailsHeldAndKeepsTheNewRecord`: acquire through `locks`; take over with `Open(commonDir, staleTTL).Acquire(MainWorktreeKey, Claim{Holder: "agent-b"}, "", true)`; read the record file's bytes; `locks.Touch(MainWorktreeKey, oldToken)` fails with a `*HeldError` (`asHeldError`) naming `agent-b`, and the record file's bytes are unchanged.
- `TestTouch_BackdatedHeartbeat_AdvancesIt`: rewrite the stored record with its `heartbeat` one hour earlier (decode the record JSON, change it, `writeRecord`), Touch, then assert `touched.Record.Heartbeat.After(backdated)` and that `Read` returns that heartbeat from disk. Replace the `!Before` heartbeat check in `TestLocks_AcquireTouchRelease_CycleFreesTheLock` with it.
- Bite: deleting `current.Record.Heartbeat = now` in `Touch` fails the second test; dropping the token comparison fails the first.
**Files:** internal/lock/lock_test.go
**Read first:** internal/lock/lock.go — Locks.Touch, Locks.Acquire, Locks.read, Locks.write, heldError, HeldError.Message; internal/lock/lock_test.go — openTest, mustAcquire, asHeldError, writeRecord, staleTTL, TestLocks_AcquireTouchRelease_CycleFreesTheLock, TestAcquire_TakeStaleOnStaleLock_TakesItWithANewToken
**Closes:** lock-touch-tests
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/lock/`
- `GOMEMLIMIT=2GiB go test -race ./internal/lock/ -run 'Touch'`
- `go vet ./internal/lock/`
**Commit:** `test(lock): cover Touch by a displaced holder and a real heartbeat advance`

## P4.5 Merge-driver write failure and a missing replace target

**What:** Covers T-06 (`merge-write-failure-test`): regression guards for a merge driver that cannot write `<ours>` and for `ReplaceFileVerbatim` on a missing path.
**Tests:**
- `internal/cli/merge_driver_test.go` `TestMergeDriver_OursInReadOnlyDirectory_FailsInvalidAndKeepsOurs`: inputs written with `writeDriverInputs`, then their directory set to `0o555` (back to `0o755` in `t.Cleanup`); skipped on Windows and as root. `merge-driver <base> <ours> <theirs> <path>` exits 1 with an invalid failure naming the ours path (stderr in text, `error.code` and `error.path` under `--json`); ours keeps its bytes; no directory entry contains `.tmp-`.
- `internal/nodefile/atomic_test.go` (where P2.9 put the atomic-write tests) `TestReplaceFileVerbatim_MissingPath_FailsNamingItAndCreatesNothing`: `errors.Is(err, fs.ErrNotExist)`, the message names the path, and no file exists afterwards.
- Bite: making `mergeIssueFile` ignore the `ReplaceFileVerbatim` error fails the first; replacing `ReplaceFileVerbatim`'s `Stat` guard with a default mode fails the second.
Depends on P2.9.
**Files:** internal/cli/merge_driver_test.go; internal/nodefile/atomic_test.go
**Read first:** internal/cli/merge_driver.go — mergeIssueFile, gitTextMerge, failedMerge; internal/cli/merge_driver_test.go — writeDriverInputs, readDriverFile, TestMergeDriver_MissingInput_FailsNamingItAndKeepsOurs; internal/cli/init_test.go — outsideGitDir; internal/cli/testharness_test.go — runCLI; internal/tree/atomic.go — ReplaceFileVerbatim, writeTemp, tempPattern (in internal/nodefile from P2.3); internal/cli/know_rm_test.go — TestKnowRm_DependentWriteFails_RollsBackNamingTheDependentFile
**Closes:** merge-write-failure-test
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'MergeDriver'`
- `GOMEMLIMIT=2GiB go test ./internal/nodefile/ -run 'ReplaceFileVerbatim'`
- `go vet ./internal/cli/ ./internal/nodefile/`
**Commit:** `test(cli): cover a merge-driver write failure and a missing replace target`
