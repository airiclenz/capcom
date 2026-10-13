# Code and Architecture Review: capcom, 2026-10-13

**Date:** 2026-10-13
**Base:** `main` at `d192883` (v0.5.0 just cut)
**Scope:** the whole repository: 114 source files and 93 test files, about 47k lines. That covers `cmd/capcom`, `cmd/release`, all 18 `internal/` packages, `.github/` workflows and scripts, `.claude/` and the root files.
**Mission:** capcom is a single-binary, git-native issue tracker and Knowledge store for coding agents. Issues and Notes are markdown files under `.capcom/`, versioned with the code. A disposable SQLite index under the git dir answers queries, and capcom never runs a writing git command.

## How this review was made

- **Code audit.** The repository was cut into 18 parts. Each part was reviewed through four lenses: intent and drift, correctness, security, and critical-path tests. A concurrency lens also ran on the 7 parts that share mutable state across processes. That was 79 lens reviews, rolled up into 17 groups.
- **Adversarial verification.** Every finding, not only the High ones, was put to an independent verifier as one of 63 claims. 56 were confirmed and 7 refuted, with none left unclear. Many Mediums were downgraded to Low, and several line citations were corrected. The nine defects the architecture exploration noticed (`U-01` to `U-09`) went through a second adversarial pass against the source: 2 were confirmed and 7 refuted. This document uses the **post-verification severity** and the **corrected locations** throughout.
- **Machine checks.** `go vet` ran on linux, windows and darwin. `go test -cover` ran on all packages, and `go test -race` ran one package at a time. See Appendix B.
- **Architecture exploration.** The `internal/cli` package, the domain and storage packages (tree, nodestore, issue, knowledge, store, frontmatter, rawfm, edit) and the infrastructure and release packages (index, check, lock, config, gitx, agents, briefing, elsewhere, `cmd/release`) were each explored for deepening opportunities.
- **Continuity.** The previous audit (`docs/reviews/archived/code-audit-2026-10-12.md`) was compared with this one. Its fixed items are not repeated. Where a finding here is a residue or a regression of an earlier fix, the item says so.

### Vocabulary

The architecture sections use these words in a fixed sense:

| Word | Meaning |
|---|---|
| **module** | Anything with an interface and an implementation (a function, a type, a Go package). |
| **interface** | Everything a caller must know to use the module. |
| **implementation** | The code behind the interface. |
| **depth** | How much behaviour sits behind how small an interface. A **deep** module has a lot of behaviour behind a small interface. A **shallow** module's interface is nearly as complex as its implementation. |
| **seam** | A place where behaviour can change without editing in place. |
| **adapter** | Something that satisfies an interface at a seam. |
| **leverage** | What callers gain from depth. |
| **locality** | What maintainers gain from depth: change, bugs and knowledge sit in one place. |

Domain terms follow the glossary: Tree, Node, Issue, Note, Knowledge, Alias, Tree lock, Briefing, store format, Ready.

### Headline summary

| Severity | Security | Correctness | Concurrency | Intent / drift | Test gap | **Total** |
|---|---|---|---|---|---|---|
| High | 2 | - | - | - | - | **2** |
| Medium | 1 | 4 | 2 | 2 | 4 | **13** |
| Low | - | 6 | 3 | 7 | 5 | **21** |
| **Total** | **3** | **10** | **5** | **9** | **9** | **36** |

The 36 items are the 34 `F-` and `T-` items, which hold all 56 confirmed audit claims (claims that share a root cause grouped into one item), and the 2 `U-` items confirmed in the second pass (U-02, U-09, both Low). Each item's severity is that of its worst sub-point. The 7 refuted audit claims and the 7 refuted `U-` items are in Appendix A.

**The most important findings:**

1. **A hostile repository can make capcom write outside the working tree and hang `prime` (F-01, F-02).** `AGENTS.md` is followed through any symlink, both when it is read and when it is rewritten. A committed `AGENTS.md -> ~/.bashrc` makes `capcom agents` append attacker-controlled Note titles to the user's shell startup file. A link to `/dev/zero` or a FIFO makes `prime`, `check` and every Note write run out of memory or hang. The 2026-10-12 fix that taught the block writer to keep symlinks added this regression. The same unguarded-read pattern also reaches `config.yaml`, `path#symbol` Refs and other-branch blobs.
2. **The read guard is not a single module, so it has been applied file by file (F-02).** The 2026-10-12 audit fixed non-regular and oversized reads for node files only. Every other repository-controlled read still uses a plain `os.ReadFile`.
3. **Writes by concurrent sessions are guarded only for the edited file itself (F-11, F-05).** Move, remove and format rewrite other Nodes' files from a snapshot taken when the plan was built, with no compare-and-swap. `show` and `know show` pair index content with a fresh etag, so a later `--if-match` write can overwrite a change the caller never saw. The deployment the project designs for, several agent sessions in one clone, is exactly where these races occur.
4. **Duplicate ids are still handled two ways (F-10, F-09).** `tree.Load` keeps the first file of a repeated id and joins the dropped file's subtree to it, while the index drops both files. Names of files the index leaves out can still fall through to unique-prefix matching. Both are residues of the 2026-10-12 findings.
5. **Architecture.** Workflow rules and post-write effects live in cobra handlers rather than in domain modules (A-01, A-03). Reference rewriting is written four times (A-04), and generic file IO is lodged in `tree` (A-05). Each of these is the structural home for a cluster of the defects above. `internal/cli` (6032 lines in 30 files) and `cmd/release` (3747 lines) are the largest packages, and six source files are over 500 lines (section 5).

**Overall health.** The codebase is in good shape. Vet is clean on three operating systems, every test passes, there are no data races, and statement coverage is 81-100% per package. The documented invariants are mostly honoured, and the critical paths are densely tested. The defects cluster at boundaries: repository-controlled bytes against the read guard, the index snapshot against the file on disk, the tree's view of duplicates against the index's, and one process's plan against another process's write. None of them is a deep design flaw.

### How to use this document for a plan

- Every item has a stable ID: `F-` for defects, `T-` for test gaps, `U-` for defects noticed during architecture exploration and verified in a second pass and `A-` for architecture candidates. Section 6 gives a phased order with explicit dependencies, and section 7 maps every item to a suggested capcom issue slug.
- Every fix item gives a **Fix** (a goal plus an approach) and an **Acceptance** line (the test that must fail before the fix and pass after it). That maps onto a plan item's Goal, Approach, Tests and Acceptance.
- Line numbers are those at `d192883`. An implementer should re-derive them by symbol.
- Some items carry a **Design call**. That is a choice the plan writer must put to the user before the plan is saved. Each one comes with a recommendation.

---

## 2. Findings: defects

Items are ordered by severity, then by package. **Claims** gives the numbers from `verify-list.txt` in the run directory (Appendix C) for traceability. Every claim cited here was confirmed by an independent verifier.

### 2.1 High

#### F-01: An `AGENTS.md` symlink is followed outside the working tree when the Managed block is written

- **Severity:** High · **Category:** Security (intent drift, and a regression) · **Claims:** #10 · **Lenses:** Intent + Security
- **Files:** `internal/agents/block.go:242-255` (`resolveGuide`), `:228-231` (`reconcile` writes the target with `tree.WriteFileAtomicVerbatim`), `:262-263` and `:358-367` (ModeWrite appends a block when there are no markers), `:176` (`Render` writes Note ids and titles). Callers are `internal/cli/agents.go:65` (`capcom agents`, ModeWrite) and `internal/cli/know.go:387` (`refreshAgentsAfterWrite`, ModeRefresh, after every Note write).
- **Problem:** `resolveGuide` calls `filepath.EvalSymlinks` and accepts any target, with no check that it stays under the worktree root. `reconcile` then atomically replaces that target. Everywhere else capcom refuses to leave the tree: Refs that escape through a symlink are an error, and non-regular node entries are never read. This was added by item 7 ("keeps symlinks") of `docs/plans/archived/2026-10-12 - 00 - code-audit-fixes-plan.md`, which followed links but never contained them.
- **Trigger:** A cloned repository commits `AGENTS.md -> /home/<user>/.bashrc` (or `../../.profile`). The user or an agent runs `capcom agents`, which the block's own intro text tells agents to do. ModeWrite finds no markers in the target and appends the block there. The block carries attacker-controlled Note ids and titles, so a title such as `$(curl …|sh)` inside backticks runs at the next login. ModeRefresh, run from any Note write, does the same once the target holds a stale block. The only existing test (`block_test.go:249`) covers a symlink that stays inside the tree.
- **Fix:**
  - **Goal:** the guide target is written only when its resolved path lies inside the resolved worktree root.
  - **Approach:** after `EvalSymlinks(path)`, compare the result with `filepath.EvalSymlinks(worktreeRoot)` using a path-prefix check on cleaned paths, not a string prefix. When the target is outside, return an `agents.Error` that names the link and its target. Callers then report it as a problem, not as a command failure. The in-repo `AGENTS.md -> CLAUDE.md` case keeps working.
- **Acceptance:** A new test in `internal/agents/block_test.go` points `AGENTS.md` at a file outside `t.TempDir()`'s worktree, runs `Sync` in ModeWrite and in ModeRefresh, and asserts that an `agents.Error` names the link and the outside file's bytes are unchanged. Skip it on Windows if symlinks need privileges. The existing in-tree symlink test still passes. `GOMEMLIMIT=2GiB go test ./internal/agents/ ./internal/cli/ -run 'Agents|Know'`.
- **Related:** F-02(a), the read half of the same file, ships in the same or the next commit. A-05 later gives the containment and read guard a single home.

#### F-02: Repository-controlled files are read with no regular-file check and no size cap

One root cause, five read sites. The 2026-10-12 audit fixed this rule ("non-regular entries are never read; node files over 1 MiB are left out with a warning") for **node files only**, in `tree.ReadNodeFile`. Every other repository-controlled read still uses a plain `os.ReadFile` or an unbounded git read. The attacker in the threat model is a hostile repository or branch: crafted symlinks, huge files, malicious remote-tracking branch content.

- **Severity:** High overall. Sub-points carry their own severity. · **Category:** Security (intent drift)
- **Shared fix:** add **one** guarded reader beside `tree.ReadNodeFile`, for example `tree.ReadRegularFile(path string, limit int64) ([]byte, error)`. It returns the existing sentinels `tree.ErrNotRegular` and `tree.ErrTooLarge`. Its steps:
  1. `Lstat` the path and refuse anything that is not regular.
  2. Open it. Off Windows, open with `O_NOFOLLOW|O_NONBLOCK` and then clear `O_NONBLOCK`.
  3. `Stat` the open handle and refuse it unless `os.SameFile` holds and the mode is regular.
  4. Read through `io.LimitReader(limit+1)` and refuse anything over the limit.

  Route every site below through that reader. A-05 later moves it, together with `atomic.go` and `read.go`, into a `nodefile` package as a pure move. Do not wait for A-05 to fix the security items.

| Sub | Severity | Site (corrected) | Trigger | Specific fix |
|---|---|---|---|---|
| **(a)** `AGENTS.md` read | **High** (claim #11) | `internal/agents/block.go:285-293` (`readGuide`). Reached from `check.Run` → `agentsBlock` (`internal/check/check.go:142,551`), from `briefing.readHealth` (`internal/briefing/briefing.go:162`), from `refreshAgentsAfterWrite` (`internal/cli/know.go:387`) and from `capcom agents`. | A committed `AGENTS.md -> /dev/zero`, `/dev/urandom`, a FIFO or `/dev/tty`. `capcom prime` at session start then grows memory until it is OOM-killed, or blocks forever. The `recover` at `briefing.go:82` cannot catch either. This breaks ADR 0009 ("prime turns every source failure into a warning and exits 0") and Compose's contract ("a Briefing never costs a session its start"). A link to a directory makes `check.Run` fail as a whole, which drops the Health section. | Read through the guarded reader with a 1 MiB cap. Turn a refusal into an `agents.Error` problem, never a command failure. |
| **(b)** `config.yaml` read | Medium (claim #53) | `internal/store/store.go:165` (`loadConfig`) and `:126` (`RaiseFormat`). The cited `:415` and `:376` were stale; the file has 221 lines. | `.capcom/config.yaml -> /dev/zero` makes every command hang or exhaust memory, because `Open` reaches `loadConfig` first. A link to a sensitive file (`~/.ssh/...`) gets parsed, and the strict-parse errors that cite a line can echo fragments of it into output an agent relays. | Use the guarded reader. Return a path-naming error for a non-regular or oversized file, because a bad config fails the command by design. `RaiseFormat`'s atomic rename replaces the link itself, so there is no write-through. |
| **(c)** `path#symbol` Ref check | Medium (claim #43) | `internal/knowledge/verify.go:151-163` (`checkSymbol`: `os.Stat` + `os.ReadFile` + `string(data)`). Reached from `check.checkNote` (`check.go:536`), `know verify` (`cli/know.go:478`) and `prime` (via `check.Run`). | A Ref `blob.bin#x` naming a huge committed file is held in memory twice on every `prime`. A Ref naming a local FIFO (git cannot commit one, so it must be an untracked local entry) blocks `prime` and `check` forever. Symlink escapes are already refused (`checkRef`). | Use the guarded reader, with the cap as a named constant. Map `ErrNotRegular` and `ErrTooLarge` to Ref Findings ("is not a regular file", "is larger than …"). Search with `bytes.Contains` instead of `string(data)`. |
| **(d)** Other-branch blobs in the Elsewhere scan | Medium (claims #27, #32) | `internal/gitx/branches.go:190-196` (`ShowFile` → `git cat-file blob`), `internal/gitx/gitx.go:100-115` (`run` buffers all of stdout, then copies it to a string and to `[]byte`), `internal/elsewhere/elsewhere.go:239-251` (`branchStatus` → `issue.Parse`), and `ChangedFiles` with `changeKind`, which folds the `T` type change into Modified. | A fetched remote-tracking branch (a hostile fork or PR branch) commits a highly compressible multi-GB `.capcom/issues/x.md`. Every reading command and `prime` within the day window then pulls it into memory about three times and is OOM-killed. The type-change half is weak: a blob cannot be a FIFO, and a symlink blob yields only target text, which the parser rejects with a warning. | `ShowFile` takes a size limit. Check `git cat-file -s <rev>:<path>` first, or use `--batch-check`, and return a sentinel above 1 MiB. The scan turns that into a "larger than 1 MiB" warning reusing the `tree.ErrTooLarge` wording. Optionally, make `ChangedFiles` use `diff --raw -z --no-renames` and carry the destination mode, so the scan skips anything that is not `100644`/`100755`. |
| **(e)** `ReadNodeFile` check-then-open race | Low (claim #61) | `internal/tree/read.go:27-46`. The doc comment at `:20-25` claims "a symlink is not followed" and "a file swapped or grown after the first check is refused too". | A same-user local process swaps the entry between `Lstat` and `os.Open`. A swapped-in symlink is followed, because the post-open `Stat` describes the target. A swapped-in FIFO blocks inside `Open`. A hostile repository's static files cannot do this. | It is the same reader: `O_NOFOLLOW` + `O_NONBLOCK` plus `os.SameFile`. Make `ReadNodeFile` a thin wrapper over the guarded reader so the doc comment becomes true. |

- **Acceptance:**
  - (a) An agents test with `AGENTS.md` as a symlink to a FIFO made with the existing `makeFIFO` helper (unix only) and to an oversized file asserts an `agents.Error` and no hang. A CLI `prime` test with the same fixture asserts exit 0 with a warning.
  - (b) A store test with `config.yaml` as a symlink to a FIFO and to a file over 1 MiB asserts that `Open` fails promptly with an error naming `.capcom/config.yaml`.
  - (c) A knowledge test with a FIFO Ref and an oversized Ref asserts a Finding, not a hang. `fifo_unix_test.go` and `fifo_windows_test.go` pin this per OS.
  - (d) An elsewhere or gitx test commits a blob over 1 MiB on a side branch and asserts one warning naming the path and the branch, and no entry.
  - (e) A unix tree test swaps a regular file for a symlink through a test seam between the `Lstat` and the open, and asserts `ErrNotRegular`.
  - Run packages one at a time: `GOMEMLIMIT=2GiB go test ./internal/agents/`, then the same for `./internal/store/`, `./internal/knowledge/`, `./internal/gitx/`, `./internal/elsewhere/` and `./internal/tree/`.
- **Related:** F-01. A-05 (`nodefile` becomes the home of the guarded reader). This is a residue of 2026-10-12 finding "Symlinked or other non-regular node files are stat'd as links but read through".

### 2.2 Medium

#### F-03: Rerunning `publish` for an older version downgrades the Homebrew tap and the Scoop bucket

- **Severity:** Medium · **Category:** Correctness · **Claims:** #3 · **Lens:** Correctness
- **Files:** `cmd/release/probes.go:304-310` (`pinnedVersionStep`, the comparison at `:306-309`) and `cmd/release/channels.go:38-67, 115-124` (`convergeHomebrew`, `convergeScoop` and `publishPackageFile` commit and push).
- **Problem:** `pinnedVersionStep` reports the tap or bucket as pending whenever the pinned version differs from the requested one, including when the pinned version is **newer**. Converge then overwrites it with the older formula or manifest. A tag that already exists skips `checkPushable` (`publish.go:170-175`), so no gate stops this either. `probes_test.go:196,203` cover only an older pin.
- **Trigger:** The `publish v0.5.0` run stops after its release step. Then v0.5.1 is published fully. The maintainer reruns `publish v0.5.0` to finish winget, and that silently downgrades `airiclenz/homebrew-tap` and `airiclenz/scoop-bucket` for every user. `smoke`'s `checkStaleBucket` already treats a bucket at another version as normal for a release that is not the latest.
- **Fix:**
  - **Goal:** a channel pinned to a newer version than the one being published reports done ("pins newer X") and is never rewritten.
  - **Approach:** parse both versions with the existing version helper in `cmd/release/version.go` and compare them in `pinnedVersionStep`. Report pending only when the pinned version is lower or absent.
- **Acceptance:** In `probes_test.go`, a newer pin reads done with the "pins newer" detail. In `publish_test.go`, a converge with a newer pin issues no clone, commit or push on the `memRepo`/`fakeRunner` seams. `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'Pinned|Publish'`.
- **Related:** A-11 (per-channel files; this logic lands in `homebrew.go` and `scoop.go`).

#### F-04: A draft release left by an interrupted `gh release create` is counted as published

- **Severity:** Medium · **Category:** Correctness · **Claims:** #6 · **Lens:** Correctness
- **Files:** `cmd/release/repo.go:294-318` (`ReleaseAssets` asks for `--json assets` and reads only name and state), `cmd/release/probes.go:142-148` (`probeRelease`) and `cmd/release/publish.go:438-444` (`convergeRelease` uploads missing assets only). "draft" appears nowhere in `cmd/release`.
- **Problem:** Given assets, `gh release create` makes a draft, uploads, then publishes. A Ctrl-C or kill during the upload leaves a draft. On a rerun, convergeRelease uploads only the missing assets and never un-drafts the release, `probeRelease` reports done, and the Homebrew, Scoop and winget steps then push URLs that return 404 for everyone else. `smoke` downloads through authenticated `gh`, which can read draft assets, so only the optional `brew upgrade` smoke step would notice.
- **Fix:**
  - **Goal:** a draft release is never reported done, and a rerun publishes it once its assets are complete.
  - **Approach:** add `isDraft` to the `gh release view --json` fields in `execRepo.ReleaseAssets` and to the repo seam's return type, and carry it into `memRepo`. `probeRelease` reports a draft as pending ("draft"). `convergeRelease` runs `gh release edit <tag> --draft=false --latest --repo airiclenz/capcom` after the asset upload. Every `gh release` call must name `--repo airiclenz/capcom` (ADR 0008).
- **Acceptance:** `probes_test.go` shows a draft reading pending. `publish_test.go` converges a draft with all assets present and asserts exactly one un-draft call, after which the probe reads done. `repo_test.go` pins the argv of the new `execRepo` call. `GOMEMLIMIT=2GiB go test ./cmd/release/`.
- **Related:** F-13 (the same interrupted-create scenario) and A-11.

#### F-05: `show` and `know show` pair index content with an etag read later from disk

- **Severity:** Medium · **Category:** Concurrency (intent drift) · **Claims:** #20, #23 · **Lenses:** Intent + Correctness + Concurrency. Five lens reports flagged this independently.
- **Files:**
  - `internal/cli/show.go`: `:53` (entry from `issueIndex.Resolve`), `:70` (body from `issueIndex.Body`, which selects from the fts table, `internal/index/query.go:280-290`), `:74` (`readIssueFile`, a separate disk read) and `:89-92` (`etag := edit.ETag(file)` rendered beside the index-derived fields).
  - `internal/cli/know.go:257-275` (`noteShowResult`: body at `:263`, file and etag at `:267-271`).
- **Problem:** The etag is documented as "the bytes a later `--if-match` is checked against" (ADR 0010: "etag derives from file bytes"). Here it comes from a later read than the content shown. No check ties the shown snapshot to the file bytes.
- **Trigger:**
  - (a) Agent A runs `capcom show X`. Agent B runs `capcom update X --status doing` after A's index refresh and before A's file read. A sees `status: todo` with the etag of B's `doing` version. A's `update --if-match <etag>` then passes and overwrites a change A never saw, so both sessions believe they own the Issue.
  - (b) A write that keeps size and mtime (same length, within one mtime tick) is skipped by the refresh by design. The stale pairing then repeats deterministically, with no race needed.
- **Fix:**
  - **Goal:** every field, body and etag that `show` and `know show` print derive from one read of the file's bytes.
  - **Approach:** parse the bytes already read for the etag with `issue.Parse` or `knowledge.Parse`, as `verifyNotes` does at `know.go:470`, and render fields and body from that parse. Use the index only to resolve the name and for index-only data such as readiness and children. If the file no longer parses, fail with the same error a write would give for that file.
- **Acceptance:** A CLI test rewrites the file between index open and the body read. Use a seam, or write a same-size, same-mtime edit after a refresh, using `os.Chtimes`. The test asserts that the printed body and status match the etag's bytes. Add one test each for `show` and `know show`. `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Show'`.
- **Related:** F-11 (the same class of lost update) and F-20.

#### F-06: `dep rm` and `related rm` resolve the argument by prefix before matching a dangling entry exactly

- **Severity:** Medium (`dep rm`, claim #18). `related rm` is Low (claim #22). · **Category:** Correctness (intent drift) · **Lenses:** Intent + Correctness
- **Files:**
  - `internal/cli/dep.go:154-206` (`depRemoval`, the fallback at `:196-206`, the not-found message at `:178`).
  - `internal/cli/related.go:143-160` (`relatedRemoval`, the fallback at `:143-147`, the ambiguous case at `:148-151`).
  - The holder test is `graph.names` (`dep.go:298-303`), which goes through `issue.readyTree.resolve` (`internal/issue/ready.go:112-120`) and matches only an exact id or Alias. Name resolution is `index.resolveID` (`internal/index/query.go:145-178`), which takes a unique prefix.
- **Problem:** The doc comment promises "a dep naming no issue at all is removed by its exact text". The exact-text fallback runs only when `Resolve` returns `ErrNotFound`. A dangling entry whose text is a prefix of a live id resolves to that live Issue instead.
- **Trigger:** Issue `x` has `deps: [auth]`. `auth` was removed, renamed on another branch, or its file is unusable, while `auth-refactor` exists. `capcom dep rm x auth` resolves to `auth-refactor` and fails "x does not depend on auth". If x also depends on `auth-refactor`, the command removes that **valid** dep and keeps the dangling one. If `auth` prefixes two Issues, it fails as ambiguous. The dangling dep blocks x for good (`BlockMissing`, `ready.go:127-128`), and only a hand edit clears it. `related rm` behaves the same way. Its test `TestRelatedRm_DanglingEntry_RemovesItByItsText` (`related_test.go:222`) covers only a dangling text that prefixes nothing.
- **Fix:**
  - **Goal:** an entry whose stored text equals the argument (after normalisation) is removed, whatever the argument also resolves to.
  - **Approach:** in `depRemoval` and `relatedRemoval`, first test `entry == normalised(arg)` against the holder's list. When it matches and `arg` is not an exact id or Alias of a live Issue, remove by exact text. Otherwise resolve as today, and test `entry == arg || graph.names(id)(entry)`. On `tree.ErrAmbiguous`, fall back to the exact-text test before failing.
- **Acceptance:** CLI tests for `dep rm` and `related rm` cover four cases: a dangling entry that prefixes one live Issue, a dangling entry that prefixes two, a dangling entry plus a valid dep on the prefixed Issue (only the dangling one is removed), and the existing exact-id path unchanged. `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'DepRm|RelatedRm'`.
- **Related:** Name resolution is written three times: `tree.Resolve`, `index.resolveID` and `issue.readyTree.resolve` (domain exploration, under A-05). F-09 is the same prefix-fall-through family.

#### F-07: Text output passes terminal control sequences from repository content

- **Severity:** Medium · **Category:** Security · **Claims:** #19 · **Lens:** Security
- **Files:** `internal/cli/issue_render.go:259-262` (`flatten`, the only filter, at `:260`), `internal/cli/output.go:220` (`kvLineBreaks`: CR and LF only). Titles and field values flow through `issue_render.go:79,183` and `know.go:150,308,530`. Bodies print verbatim at `show.go:70` and `know.go:263`. `internal/issue/validate.go:20` checks only that a title is not empty.
- **Problem:** A YAML double-quoted `"\e[..."` or `"\x1b..."` parses to a raw ESC. Text output of `show`, `list`, `know tree`, `know search`, `know show` and `dep list` writes titles, values and bodies to the terminal unfiltered.
- **Trigger:** A hostile repository or branch commits an Issue titled `"\e[2J\e[31mx"`, or a body carrying OSC sequences. A human runs `capcom show` and the screen is cleared or rewritten to hide text, the window title is set, or, on some terminals, the clipboard is written.
- **Design call:** what to show in place of a control character.
  - (1) A visible escape such as `\x1b`. **Recommended:** it keeps the information for a human debugging a file.
  - (2) U+FFFD.
- **Fix:**
  - **Goal:** in text mode, no C0 control other than TAB and LF, no DEL and no C1 control from repository content reaches stdout. `--json` and `--kv` are byte-for-byte unchanged.
  - **Approach:** add one `sanitizeTerminal(string) string` in `output.go`, used by `flatten` and by every body write in the text renderer. The existing `--kv` line-break rule stays as it is.
- **Acceptance:** A CLI test creates an Issue and a Note whose title and body contain `\x1b[2J`, `\x9b` and `\x07`, then asserts the text output carries the chosen replacement while `--json` round-trips the raw bytes. `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Show|List|Know'`.
- **Related:** A-14 (once `cli/output` is extracted, sanitising becomes part of that module's interface).

#### F-08: Multi-document YAML is silently truncated in frontmatter and in `config.yaml`

- **Severity:** Medium (the config sub-point). The frontmatter sub-point is Low. · **Category:** Intent drift (strict parsing, ADR 0009) · **Claims:** #24, #29 · **Lens:** Intent
- **Files:**
  - (a) `internal/config/config.go:250-252` (`RaiseFormat` prepends `format: N` when there is no format line) and `:315-317` (`parser.document` decodes once). Reached from `store.RaiseFormat` (`store.go:124-144`, which writes the result without re-parsing it), from `related add` (`related.go:76`) and from the `know update` supersession raise (`know_update.go:184`).
  - (b) `internal/frontmatter/decode.go:94-117` (`yaml.Unmarshal` parses only the first document). `Split` (`frontmatter.go:76-90`) closes only on an exact `---` line, so a `...` line stays inside the front matter.
- **Problem:** yaml.v3 stops at a document end. Everything after it escapes the strict unknown-key, duplicate-key and syntax checks, which contradicts "an unknown key is an error, never ignored".
- **Trigger:**
  - (a) A hand-written `config.yaml` that has **no** format line (the shipped template has one) and starts with `---` and custom `statuses:`. A write that needs a newer format turns it into `format: 2\n---\nstatuses: ...`. Every later command then sees a one-key document. Statuses, types, plans, `lock.ttl` and `elsewhere.days` fall back to their defaults, and Issues with custom statuses drop out of the index. With a `%YAML` directive, the result is invalid YAML and every command fails.
  - (b) Hand-edited front matter `title: a` / `...` / `junk: [` decodes cleanly. The next `fmt` or write drops the trailing lines silently.
- **Fix:**
  - **Goal:** a second YAML document in front matter or in `config.yaml` is an error that cites its line, and `RaiseFormat` never produces one.
  - **Approach:** in both `document` decoders, use `yaml.NewDecoder` and require a second `Decode` to return `io.EOF`. Otherwise return the package's `*Error` at the second document's line. In `config.RaiseFormat`, insert the new format line after any leading `%` directives and the `---` header line rather than before them.
- **Acceptance:**
  - `decode_test.go`: `"---\ntitle: a\n...\nbogus: 1\n---\n"` fails with an error citing the `...` line.
  - `config_test.go`: a config whose second document is `statuses:` fails parsing.
  - `RaiseFormat` on `"---\nstatuses: {...}\n"` and on `"%YAML 1.2\n---\n..."` yields a file that re-parses to the same statuses with the new format.
  - `GOMEMLIMIT=2GiB go test ./internal/frontmatter/`, then `./internal/config/`.
- **Related:** A-07 (store format ownership) and A-13 (frontmatter field table).

#### F-09: Names claimed by files the index leaves out fall through to unique-prefix matching

- **Severity:** Medium for Aliases of invalid files (claim #35). Low for walk-refused files (claim #36). · **Category:** Correctness (residue) · **Lenses:** Correctness, Intent
- **Files:** `internal/index/refresh.go:263-281` (a file with problems gets only a `files` row, and `insert()` is skipped, so its Aliases are never stored), `internal/index/query.go:154-160` (Alias lookup), `:196-205` (`unusableClaim` compares only `path.Base(file) == name+".md"`), `:164-176` (prefix query) and `:136-137` (the contract: "the id of an invalid file [fails] with tree.ErrNotFound, never as a prefix"). Walk-refused files are handled at `internal/tree/tree.go:298-306, 350-365` and `internal/index/walk.go:30-32`, which only warns, so there is no files row.
- **Problem:** This is a residue of the 2026-10-12 finding "Id resolution falls back to a unique prefix when the exact id is excluded". That fix covered the **id** of a parsed-but-invalid file. It did not cover the file's **Aliases**, or files refused at walk time (symlink, FIFO, over 1 MiB, and the subtree such a refusal orphans).
- **Trigger:**
  - (a) `capcom mv auth auth-rework` leaves `auth-rework.md` with `aliases: [auth]`, and a sibling `auth-tests` exists. Then `auth-rework.md` becomes invalid, for example because a status was removed from `config.yaml` or a hand edit added an empty label. `capcom close auth` now finds nothing exact, no Alias and no unusable claim, and prefix-matches `auth-tests`. **The write lands on the wrong Issue.** `ResolveNote` shares `resolveID`, and `close.go:168`, `update.go:240` and `know_update.go:210` all write through it.
  - (b) `foo.md` is a symlink or too large, and `foo-bar.md` exists. `capcom close foo` then acts on `foo-bar`. The same condition detected at read time (`refresh.go:249-252`) *is* recorded, so the outcome depends on which phase spots it.
- **Fix:**
  - **Goal:** every name that a left-out file claims (its id and its Aliases), whether the file was refused at walk time or at read and validate time, resolves to `ErrNotFound` "… is invalid" and never to a prefix match.
  - **Approach:** for (a), keep the claimed names of a file with problems. Either insert its Aliases rows marked unusable, or add a `claims(path, name)` table filled from the parse (or from `rawfm` Alias recovery when it does not parse). For (b), record walk-refused node files, and every node under the subtree they orphan, as `files` rows with a problem. `unusableClaim` then consults both. Bump the index schema, which triggers a rebuild and is safe because the index is disposable.
- **Acceptance:** `query_test.go` tests cover `Resolve("auth")`, where `auth` is an Alias of an invalid file beside `auth-tests`, and `Resolve("foo")`, where `foo.md` is a symlink beside `foo-bar.md` (unix). Both return `ErrNotFound` naming the file. A CLI `close` test shows `auth-tests` untouched. `GOMEMLIMIT=2GiB go test ./internal/index/`, then `./internal/cli/ -run Close`.
- **Related:** A-06 (one diagnostics module would record left-out files' claims in one place), F-06 and F-10.

#### F-10: A repeated id leaks the dropped file's subtree and Aliases

All three sub-points come from one root cause. When two files answer to one id, `tree.Load` keeps the first one by path, while the index and the documented rule drop **both**. This is a residue of the 2026-10-12 finding "`tree.Load` keeps the first of a duplicate id while the index drops both". The id itself is now refused for writes, but the dropped file's subtree and Aliases still leak. It also leaves a residue of the closed issue `resolve-dropped-alias`.

- **Severity:** Medium · **Category:** Intent drift + Correctness · **Claims:** #63, #47, #12 · **Lenses:** Intent + Correctness

| Sub | Severity | Files | Trigger | Fix |
|---|---|---|---|---|
| **(a)** The subtree joins the kept node | Medium (#63) | `internal/tree/tree.go:320-321` (walk recurses into the dropped duplicate's directory with `parentID=id`), `:373-393` (`indexIDs` drops only the repeated node) and `:573-585` (`PathFor` builds from `byID[id]`, the kept file). `internal/nodestore/move.go:66` keeps `Destination.ParentID = node.ParentID`. `triage` (`create.go:312-326`) does not block. | Two branches each file id `x`, under different parents, and are merged: `a/x.md`, `b/x.md`, `b/x/c2.md`. `ChildrenOf("x")` lists `c2` under `a/x.md`. Running `capcom know mv c2 --id c2-new` with no reparent moves `b/x/c2.md` to `a/x/c2-new.md`, silently re-parenting the Node and its subtree. | Drop the dropped duplicate's subtree from `nodes` and `children` and report it, as orphan directories are reported. Alternatively, key `children` by the parent's path. Also have `PlanMove` and `PlanRemove` refuse a Node whose `ParentID` is in `t.duplicates`. |
| **(b)** Aliases of a dropped file are invisible to claim checks | Medium (#47) | `internal/nodestore/create.go:120-128` (`claimFailure`), `nodestore/move.go:85,95` and `tree/move.go:133`. `checkDroppedAliases` (`tree.go:484-521`) leaves an Alias "only one dropped node lists" mapped nowhere. | After the merge above, the dropped `b/x.md` lists `aliases: [old-bar]`. `capcom create --id old-bar` or `capcom mv y --id old-bar` succeeds and writes a second holder of `old-bar`, which stays hidden until the `x` collision is fixed. This breaks ADR 0002 ("a new id may not reuse an id or alias"). | `claimFailure` and Move's rename path also read the Aliases of every file named by an `ErrDuplicate` problem, with `ReadAliases`/`RecoverAliases` as `orphanClaimFailure` does. Better still, expose `Tree.Claimant(name)` covering live Nodes, dropped duplicates, orphan-directory files and unparseable files (A-05). |
| **(c)** The `AGENTS.md` block lists a duplicate | Low (#12) | `internal/agents/block.go:137` (`TopNotes` passes `noAliases`) and `:145-160`. `internal/index/query.go:183-193` (the index refuses with `ErrDuplicate`). | A top-level `zeta.md` and a nested `zoo/zeta.md` are merged. `capcom agents` writes `` - `zeta`: … `` into the block, and every agent is told to run `capcom know show zeta`, which then errors. This happens only in a store that `check` already reports as a duplicate. | Skip any top-level Node with a duplicate problem, using the `tree.Errors` entries that wrap `tree.ErrDuplicate`, and pass the real Alias function. With (a) done, this falls out if `Load` drops both files. |

- **Design call:** should `tree.Load` drop **both** files of a repeated id, matching the index and format.md, or keep the first and refuse writes around it? **Recommended: drop both,** with their subtrees reported as orphans. Then (a) and (c) disappear, and only (b) needs the extra claim scan. This changes what `tree.Nodes()` returns for `check`, so check's duplicate wording must stay as it is (`TestRun_DirectoryProblems_WordedAsTheIndexWarnsThem`).
- **Acceptance:**
  - Tree tests: with `a/x.md`, `b/x.md` and `b/x/c2.md`, `ChildrenOf("x")` does not list `c2`, and a move of `c2` is refused or keeps it under `b/x`.
  - Nodestore tests: `create --id old-bar`, where `old-bar` is an Alias of a dropped duplicate, is refused with the claim error naming the dropped file.
  - Agents test: a duplicated top-level id is not rendered.
  - `GOMEMLIMIT=2GiB go test ./internal/tree/`, then `./internal/nodestore/`, then `./internal/agents/`.
- **Related:** A-06 (one duplicate detector shared by index and check, so the two can no longer disagree), A-05 (`Tree.Claimant`) and F-09.

#### F-11: Writes from a stale snapshot have no compare-and-swap (move, remove, format, create, dep add, close)

One root cause runs through every sub-point: every writing command plans from a snapshot read at its start, and only `nodestore.Edit` re-checks the file it writes (`edit.go:67,79-89`). Nothing serialises writers across processes; `edit.go:34-35` says so: "the store takes no lock". The deployment named in the bundle is several agent sessions in one clone or worktree, with the advisory Tree lock as the only coordination.

- **Severity:** Medium (sub-point (a)). The rest are Low. · **Category:** Concurrency · **Claims:** #59, #48, #41, #56, #60, #17 · **Lenses:** Concurrency, Correctness

| Sub | Severity | Files (corrected) | Trigger | Local fix |
|---|---|---|---|---|
| **(a)** Rewrites of other Nodes' files overwrite blindly, and rollback restores blindly | **Medium** (#59, #48). Issue-side duplicate #41 is Low. | `internal/tree/plan.go:40-54` (`AddRewrite`: `Stat`, then `WriteFileAtomic(Content)`; undo writes `Original`), `internal/issue/move.go:57-115` (reads every Issue file), `internal/issue/remove.go:126,153` (the cited `:296` was wrong), `internal/knowledge/move.go:59-65`, `internal/knowledge/remove.go:59,73`, `internal/nodestore/move.go:79-102`, `nodestore/remove.go:108-118` and `nodestore/format.go:40-59`. | Session A runs `capcom mv auth-fix login-fix` (reads every Issue, runs the claim checks, then applies). Meanwhile session B runs `capcom close api-work`, an Issue that depends on `auth-fix`. A then writes its pre-close copy of `api-work` with the renamed dep, and **B's close is lost silently**. A rollback after a later step fails writes `Original` over B's change as well. `--if-match` on A guards only the moved file. The window grows with the size of the tree. | Make each rewrite a compare-and-swap. In apply, re-read the file through the guarded reader and fail with a stale error (wrapping the sentinel `--if-match` already uses, so the CLI maps it to the same code) when the bytes differ from `Original`, which rolls back the steps already done. In undo, restore only when the file still equals `Content`, and otherwise report a failure to roll back that names the file. `Format` does the same per file. |
| **(b)** A replace-mode write recreates a Node that was moved or removed | Low (#56) | `internal/tree/atomic.go:84-90` (`writeFileAtomic` → `os.Rename`) and `:110` (`writeTemp` runs `os.MkdirAll`). `nodestore.place` (`create.go:373`). `tree/remove.go:225-235` (`removeDirStep`) and `tree/move.go:176-178` (`os.Mkdir(newDir)`). | A edits `x` while B moves or removes `x` in the gap between A's re-read and its rename. A's write brings `x` back at its old path, so two files claim `x` (B kept `x` as an Alias), or a removed Issue reappears. A create under a parent being removed recreates `<parent>/` with no node file. The result is visible and recoverable. | Give replace mode a variant that refuses a missing target: `Lstat` just before the rename, with no `MkdirAll`. For create, and for Move's `Mkdir`, check that the parent node file (`<dir>.md`) still exists right before placing. |
| **(c)** `renameStep` can replace a file created a moment earlier | Low (#60) | `internal/tree/plan.go:88-91` (`Lstat`, then `os.Rename`) and `:100`. | A `capcom create --id y` (via `WriteFileExclusive`) lands in the microseconds between the move's `Lstat` and its `Rename`. The create reports success, and its file is then replaced by the moved file. | For node-file renames, use the no-replace primitive `placeExclusive` already provides: `os.Link(old,new)` (atomic `EEXIST`), then `os.Remove(old)`, with the undo reversed to match. On Linux, `renameat2(RENAME_NOREPLACE)` is an alternative. Directory renames are already safe, because the target is non-empty. |
| **(d)** `dep add` and `close` decide on the index snapshot, outside the guarded edit | Low (#17) | `internal/cli/dep.go:123-128` (`FindCycle` on `loadDepGraph`, which reads index rows at `:283`) and `internal/cli/close.go:178` (category from the index entry), `:187-192` (the change callback overwrites `Status`, `Closed` and `Reason`). | `dep add a b` and `dep add b a` run at once. Both pass the cycle check and both commit, so the store holds a cycle (`check` reports `CodeCycle`). Two concurrent `close X` runs: the second one overwrites the first's closed time and reason, and both announce the same "now ready". | Close: inside the change callback, re-check the fresh file's category and return `already_closed` without writing (A-01 makes this `issue.Close`'s own rule). Dep cycle: a re-check inside the callback only narrows the window, because the reverse edge lives in another file. Only a store-wide write lock closes it fully (see the design call below). |

- **Design call:** how far to go.
  - (1) **Local compare-and-swap only:** sub-points (a) to (c) as above, plus (d)'s close re-check. The dep-cycle race stays, reported by `check`. **Recommended,** because it is cheap, local and needs no new ADR.
  - (2) Also add a **store-wide OS write lock**, the `lock` package's `.lck` mechanism under `<git-dir>/capcom/`, held across resolve, check and write for every writing verb. That closes every sub-point fully, including the dep cycle. It is a new cross-process mechanism beside the advisory Tree lock, so it needs an ADR, which touches ADR 0005, and a decision on behaviour under contention: block with a timeout, or fail.
- **Acceptance:**
  - (a) A tree test whose `Plan` holds a rewrite of file F. The test changes F after the plan is built and before `Apply`, then asserts that `Apply` fails with the stale error, F holds the concurrent bytes, and earlier steps were rolled back. A second test changes F after the rewrite and before a failing later step, then asserts that undo does not overwrite it and the failure to roll back names F.
  - (b) A tree test removes the target between plan and write and asserts the replace write fails and creates no directory.
  - (c) A tree test places a file at the destination after `Lstat`, through a test seam, and asserts the rename fails with `EEXIST` and both files survive.
  - (d) A CLI test closes an already-closed file inside the change callback, through a seam, and asserts `already_closed` and an unchanged `closed:` time.
  - `GOMEMLIMIT=2GiB go test ./internal/tree/`, then `./internal/nodestore/`, then `./internal/cli/ -run 'Close|Dep'`.
- **Related:** the apply step for (a) is already a single shared place (`tree.Plan.AddRewrite`, used by all four planners), so (a) does **not** wait for A-04. A-04 later removes the four planners that build `Original`. A-05 (`nodefile`) becomes the home of the primitives from (b) and (c) when P2-03 moves them. A-01 (close rule) and F-05.

### 2.3 Low

#### F-12: The release gate does not bind to the commit it judged

- **Severity:** Low · **Category:** Intent drift (ADR 0008) and Concurrency · **Claims:** #4, #7 · **Lenses:** Intent, Concurrency
- **Files:** `cmd/release/publish.go:170-175` (`checkPushable`, the only `checkGate` call, runs only when `steps[0]`, the tag step, is not done), `cmd/release/gate.go:56` (resolves HEAD to a SHA once), `publish.go:328` (`convergeTag` re-reads the symbolic `HEAD`), `cmd/release/repo.go:323` (`PushMain` pushes `HEAD:main`), `publish.go:384-391` (`verifyTag`) and `.github/workflows/tag-on-version-bump.yml:14-16` (tags every VERSION bump on main with no CI dependency).
- **Problem:**
  - (a) Once the version's tag exists on origin, `publish` skips the gate entirely ("converges from remote state"). Tags are made by the workflow without waiting for CI. So a bump that reached main outside `publish`, such as a manual push after `prepare` or a feature commit that carries a VERSION change, is built and released from a commit whose CI may be red or still running. `TestPublishConvergesFromRemoteStateOnceTheTagExists` (`publish_test.go:445-470`) pins the skip, but nothing tells a resume apart from a tag made outside `publish`.
  - (b) The gate judges a resolved SHA, but the push uses the symbolic `HEAD`. A commit made by another session in the seconds between them goes to `origin/main` without a green run. The release itself still builds from the tag, so the released binaries are unaffected.
- **Why Low:** (a) needs a bump pushed outside `publish`, which breaks the release process. (b) needs a commit inside a seconds-long window and does not change what is released.
- **Design call:** this warrants revisiting `adr-0008-decision`, which says both "publish converges" and "the gate has no bypass".
  - (1) While the tag is done but the release step is still pending, keep the **CI half** of the gate: run `peelBumps` from the tag's commit and require `checkGreenRun` for it. Drop only the clean-tree and HEAD-contains-`origin/main` conditions, which matter only for the push. Record the resolved SHA in `publishRun` and use it everywhere after: `ShowFile(sha, VERSION)`, `IsAncestor(sha, origin/main)`, `git push origin <sha>:refs/heads/main` and `LastCommitTouching(sha, VERSION)`. Re-check that HEAD still equals the SHA before pushing. **Recommended.**
  - (2) Amend ADR 0008 to state the skip explicitly.
- **Acceptance:** A `publish_test.go` case where the tag exists, the release is pending and CI is red on the tag commit, through the `memRepo` seam, asserts that `publish` refuses with the gate error. A case where HEAD moves between the gate and the push asserts a refusal, and asserts that `PushMain` receives the SHA. `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'Publish|Gate'`.
- **Related:** A-11.

#### F-13: When a release exists without SHA256SUMS, the rebuilt sums are uploaded unchecked

- **Severity:** Low · **Category:** Correctness (intent) · **Claims:** #5 · **Lenses:** Intent + Correctness
- **Files:** `cmd/release/publish.go:433-441` (`checkPublishedSums` runs only when SHA256SUMS is already uploaded; otherwise the rebuilt file is uploaded at `:438-441`), `cmd/release/probes.go:164-171` (`releaseAssetNames` puts SHA256SUMS last), `cmd/release/dist.go:112-113, 229` (builds with whatever `go` is on PATH) and `smoke.go:279-300` (`checkAssets` catches the mismatch, but only at the end). `go.mod` has `go 1.24.0` and no `toolchain` line.
- **Problem and trigger:** `gh release create` uploads the archives and dies before SHA256SUMS lands. The rerun comes after a Go upgrade, or from another host. The rebuilt SHA256SUMS then describes different archive bytes, and Homebrew, Scoop and winget render their hashes from it and push. Smoke fails the run at the end, but by then every channel carries bad hashes and user installs fail their checksum. When SHA256SUMS *is* published, archives about to be uploaded beside it are never checked against it.
- **Fix:**
  - **Goal:** no SHA256SUMS is uploaded that does not match the archives already on the release.
  - **Approach:** when SHA256SUMS is absent but archives exist, download and hash the uploaded archives and require them to match the rebuild. Otherwise refuse with the `gh release delete-asset` way out that `checkPublishedSums` already names. When SHA256SUMS exists, check every rebuilt archive against it before uploading. Optionally, add a `toolchain` line to `go.mod` and state in `dist`'s Long text that byte identity depends on it.
- **Acceptance:** In `publish_test.go` with the `memRepo` seam, the release holds archives A and B and no sums, and the rebuild yields a different A. Assert that `publish` refuses and uploads nothing. `GOMEMLIMIT=2GiB go test ./cmd/release/ -run Publish`.
- **Related:** F-04 and A-11.

#### F-14: The winget "done" probe depends on GitHub's lagging search index

- **Severity:** Low · **Category:** Concurrency · **Claims:** #2 · **Lens:** Concurrency
- **Files:** `cmd/release/probes.go:256-274` (`gh pr list --search "... in:title"`) and `cmd/release/channels.go:211` (`komac update --submit`). A second listing call sits at `channels.go:162` and `probes.go:233`.
- **Problem and trigger:** The smoke step fails right after winget submitted its pull request, and the maintainer reruns `publish` at once. Search has not yet indexed the new pull request, so the probe reads pending and komac submits a **duplicate** pull request to `microsoft/winget-pkgs`, a third-party repository. komac's own existing-PR check is also search-based.
- **Fix:** probe through a read that does not use the search index, such as `gh api graphql` on the viewer's open pull requests filtered to winget-pkgs, or list the branches komac creates in the maintainer's fork.
- **Acceptance:** A probe test asserts that the new argv is used (through `fakeRunner`), or a fake reports an open pull request through the A-12 seam. `GOMEMLIMIT=2GiB go test ./cmd/release/ -run Winget`.
- **Related:** A-12 (an `OpenPulls` method on the channel-repository seam is the natural home).

#### F-15: The tagger script passes green without tagging

- **Severity:** Low · **Category:** Correctness (intent) · **Claims:** #8, #9 · **Lenses:** Correctness, Intent
- **Files:** `.github/scripts/version-bump.sh:17-18` (fallback to `after^`), `:26-29` (the comment says a push is "routinely a burst of commits"), `:35-39` (bare `X.Y.Z` only; a refusal goes to stderr, exits 0). `cmd/release/publish.go:267-269` (`checkPushable` accepts a leading `v`), `cmd/release/dist.go:197` (accepts a leading `v`) and `cmd/release/versionbump_test.go:175` (tests only the zero-SHA case with the bump at the head).
- **Problem:**
  - (a) When `github.event.before` is unusable (a force-push, or branch creation), the scan narrows to the head commit. A bump buried under later commits is missed, the job passes green, and a re-run gets the same `before`.
  - (b) A `v`-prefixed VERSION is accepted by `publish` and `dist` but refused, silently, by the tagger. `publish` then waits out its full `--tag-timeout` and fails pointing at a workflow run that is green. This breaks "never swallow errors silently". `prepare` always writes a bare version, so (b) needs a forbidden hand edit.
- **Design call:** which side owns the VERSION shape. **Recommended:** `checkPushable` and `dist` require a bare `X.Y.Z`, matching `prepare` and the tagger, and the script **exits non-zero** after any refusal.
- **Fix:**
  - In the fallback, start the range at the newest reachable release tag: `before="$(git describe --tags --abbrev=0 --match 'v[0-9]*' "$after" 2>/dev/null || git rev-parse --verify --quiet "${after}^" || true)"`.
  - Exit 1 on a refused VERSION.
  - Make `checkPushable` and `dist` reject a leading `v`, naming the VERSION file.
- **Acceptance:** `versionbump_test.go` gains two cases. In one, `before` is absent from the clone and the bump sits under a later commit, and the tag is created. In the other, VERSION is `v0.6.0` and the script exits non-zero. Both are real-git tests, isolated and skipped on Windows per ADR 0008's bump-test amendment. `dist_test.go` and `publish_test.go` assert that `v`-prefixed VERSION is rejected. `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'VersionBump|Dist|Pushable'`.

#### F-16: A missing `config.yaml` is treated as format 3, and re-running `init` raises the store format

- **Severity:** Low · **Category:** Intent drift (ADR 0009) · **Claims:** #50, #51 · **Lens:** Intent
- **Files:** `internal/store/store.go:166-168` (`loadConfig` uses `config.Default()` on ENOENT, which gives `Format = FormatVersion = 3`), `internal/config/config.go:167` (`Default`) and `:211` (`keylessFormat = 1`), `internal/store/store.go:121-122` (the `RaiseFormat` doc says a missing file means 1), `internal/check/check.go:445-456` (`declaredFormat` re-stats the file and returns `missingConfigFormat`, a workaround), `internal/store/store.go:90, 200-216` (`Init` → `createConfig` writes `DefaultFile`, which has an active `format: 3`) and `internal/cli/init.go:228` (prints only "created .capcom/config.yaml").
- **Problem:** format.md and ADR 0009 say a missing `config.yaml` or `format` key means format 1. "Missing means 1" is written three times (`keylessFormat`, `missingConfigFormat`, a comment), while `Store.Config.Format` says 3. Today only `check` reads `Format`, and it compensates. `Init` on an existing store with no `config.yaml` writes `format: 3`, which silently locks out older binaries and breaks Init's own promise that "a re-run is a no-op".
- **Trigger:** A store with Issues and no `config.yaml`. Someone runs `capcom init --merge-driver` or `--claude-hooks` to add a feature, and the format jumps from 1 to 3 with no warning. Teammates on v0.1.x or format-2 binaries then fail every command.
- **Design call:** what `init` should do on an existing store that lacks `config.yaml`.
  - (1) Create nothing.
  - (2) Write the template with the `format:` line commented out. **Recommended:** the user still gets the documented template, and the format stays 1.

  Either way, `init` on a **new** store keeps writing `format: 3`.
- **Fix:** `loadConfig` sets `Format` to an exported `config.MissingFormat` (= 1) when the file is missing. Delete `check`'s second stat. `createConfig` decides by whether this call created the store root.
- **Acceptance:** `store_test.go`: opening a store with no `config.yaml` gives `Config.Format == 1`. `cli/init_test.go`: a re-run on an existing store without `config.yaml` leaves the store at format 1, and `check` still reports format 1. `GOMEMLIMIT=2GiB go test ./internal/store/`, then `./internal/check/`, then `./internal/cli/ -run Init`.
- **Related:** absorbed by **A-07**. Fix it there unless A-07 is deferred.

#### F-17: `check` has no store-format gate for the Issue `related` key, and the thresholds are duplicated

- **Severity:** Low · **Category:** Intent drift (ADR 0009) · **Claims:** #16 · **Lens:** Intent
- **Files:** `internal/check/check.go:480-502` (`checkSupersession` emits `CodeFormatTooOld` for Note keys only), `:325-365` (`checkIssue`/`checkRelated` never compare against the declared format), `:81` and `internal/cli/related.go:21` (`supersessionFormatVersion = 3`, defined twice), `related.go:18` (`relatedFormatVersion = 2`, defined in `cli` only) and `docs/format.md:469`.
- **Problem and trigger:** A hand edit or a merge leaves `related: [x]` in a store that declares format 1. `check` passes, but every format-1 binary rejects the unknown key and leaves the Issue out.
- **Fix:** export the per-key minimum formats from `config` (`config.FormatRelated`, `config.FormatSupersession`) and have `cli` and `check` read them from there. Add a `format-too-old` problem at the Issue's `related` line (`RelatedLine()`) when the declared format is below the minimum. Update `docs/format.md:469`.
- **Acceptance:** A `check_test.go` case with format 1 and an Issue carrying `related` produces `format-too-old` at that line. `grep -rn 'FormatVersion = [23]' internal/` finds a single definition per key. `GOMEMLIMIT=2GiB go test ./internal/check/`.
- **Related:** absorbed by **A-07**, and A-03 (a Codec declaring `RequiredFormat` would read the same constants).

#### F-18: `elsewhere.days: 0` still scans branches dated in the future

- **Severity:** Low · **Category:** Intent drift · **Claims:** #26
- **Files:** `internal/elsewhere/elsewhere.go:133` (`cutoff := Now.Add(-Days*day)`) and `:137` (`Tip.After(cutoff)`). The `Options.Days` doc is at `:72-73`. `internal/config/config.go:143,483` allows 0.
- **Problem and trigger:** The doc says 0 "ignores every branch". A tip whose committer date is in the future, from clock skew or a hostile remote-tracking branch, still passes. The result is extra Elsewhere entries and scan work, and `--no-elsewhere` still disables the scan fully.
- **Fix:** return early from `Scan` when `Days == 0`, before any git call. Optionally, bound the window from above as well (`Tip` not after `Now` plus a skew).
- **Acceptance:** A `elsewhere_test.go` case with `Days: 0` and a future-dated side branch (fixed committer date in env) gets an empty report and runs no git. `GOMEMLIMIT=2GiB go test ./internal/elsewhere/`.

#### F-19: `gitx.HasCommit` reports a fatal git failure as "no such commit"

- **Severity:** Low · **Category:** Correctness · **Claims:** #33. The test residue of refuted #30 joins it.
- **Files:** `internal/gitx/branches.go:134-144` (the cited `:283` was wrong). Compare `IsAncestor` at `:153-157`, which checks the exit code. The caller is `internal/elsewhere/elsewhere.go:106-114`.
- **Problem:** Every non-zero exit of `rev-parse --verify --quiet` maps to `(false, nil)`, including 128 on a corrupt object store. The scan then warns "HEAD has no commit yet" instead of giving the git error.
- **Fix:** return `false, nil` only when `ExitCode() == 1`, and return the error otherwise.
- **Acceptance:** `branches_test.go`: `HasCommit` on a non-repository directory, or on a repository with a corrupted HEAD object, returns a non-nil error. This also covers the "non-git-exit error path" that the verifier of refuted #30 found unpinned. `GOMEMLIMIT=2GiB go test ./internal/gitx/`.
- **Related:** T-08.

#### F-20: Index reads after `Open` do not come from one snapshot

- **Severity:** Low · **Category:** Concurrency · **Claims:** #34
- **Files:** `internal/index/query.go:56-62, 296-306` (`List` → `issues` → `Issue` per id), `internal/index/index.go:208-238` (`Issue` runs five statements: the row, then labels, deps, related and Aliases), `query.go:270-276` (`resolved` fails "id … not found"), `notes.go:186-199`, `index.go:39` (WAL) and `refresh.go:49` (only the refresh is a transaction).
- **Problem and trigger:** Agent A runs `capcom ready`, which lists every Issue in about 5×N statements. Agent B's command commits a refresh that removes or renames an Issue partway through. A's command then fails as a whole with `id "x" not found`. This also hits `close.go:148` after its write has already succeeded. Without a failure, an `Entry` can mix two versions, such as the category of one and the deps of the other. It is transient and read-only, and a retry is consistent.
- **Fix:** read each command's queries from one snapshot. Open a deferred read transaction on a single `db.Conn` (`BEGIN DEFERRED`, because `_txlock=immediate` would make `BeginTx` take the write lock), or fetch every Entry's rows in a few set-based statements. Also skip an id that vanished instead of failing the command.
- **Acceptance:** An index test lists Issues, deletes one Issue's rows through a second connection mid-iteration (via a seam), and asserts that `List` returns a consistent set without error. `GOMEMLIMIT=2GiB go test ./internal/index/`.

#### F-21: A directory that vanishes during the walk fails the whole refresh

- **Severity:** Low · **Category:** Concurrency (correctness) · **Claims:** #39
- **Files:** `internal/tree/tree.go:279-282` (`treeWalker.walk` returns the raw `os.ReadDir` error) and `:320-323` (recursion). `internal/index/walk.go:22-28` wraps it, `refresh.go:69-72` returns it, and `index.go:183-185` fails `Open`. The cited `index/walk.go:230-236` was wrong.
- **Problem and trigger:** Another session's `capcom mv` or `rm` of an Issue with children, or a `git checkout`, renames `<id>/` between the parent's `ReadDir` and the child's. Any reading command then exits 1. A file that vanishes the same way is skipped (`tree.go:357-360`, `walk.go:37-39`, `refresh.go:242-245`), but a directory is not.
- **Fix:** in `walk`, treat `errors.Is(err, fs.ErrNotExist)` on a non-root directory as empty.
- **Acceptance:** A tree test removes a child directory through a walk seam (or a `ReadDir` hook) between the listing and the descent, and asserts that `Walk` succeeds without that subtree. `GOMEMLIMIT=2GiB go test ./internal/tree/`.

#### F-22: The Log section is found without regard to fenced code blocks

- **Severity:** Low · **Category:** Correctness · **Claims:** #40
- **Files:** `internal/issue/mutate.go:41-43` (`appendLogEntry` takes the first `## Log` line), `:70` (`isSectionBoundary`) and `internal/issue/merge.go:96-99` (`parseLog`), with the fallback at `merge.go:62-66`. Compare `internal/rawfm/conflict.go:107-124`, which does skip fences.
- **Problem and trigger:** An Issue body quotes the Log format inside a code fence above the real `## Log` section. `capcom log` and `reopen` then insert the entry after the example, in the description. On a merge, `parseLog` takes the same fake section and falls back to a plain text merge, so concurrent Log appends on two branches conflict.
- **Fix:** one fence-aware line scanner, shared by `mutate.go` and `merge.go`, that tracks ```` ``` ```` and `~~~` open and close. Reuse `rawfm`'s approach.
- **Acceptance:** `mutate_test.go` and `merge_test.go` cases with a fenced `## Log` example before the real section: the append lands in the real Log, and the merge of two appends succeeds entry by entry. `GOMEMLIMIT=2GiB go test ./internal/issue/`.

#### F-23: Renaming a Note to its own successor makes it superseded by itself

- **Severity:** Low · **Category:** Correctness · **Claims:** #42
- **Files:** `internal/knowledge/move.go:54-59` (`PlanMove` changes only the Aliases) and `:78` (skips the Note itself when rewriting successors), `internal/knowledge/note.go:233-235` (`Validate` accepts a successor that does not exist) and `:278-279` (refuses "names the note itself"), `internal/tree/move.go:133` and `internal/index/notes.go:61` (the index leaves an invalid Note out). `nodestore.Move` never validates the planned content.
- **Trigger:** Note `auth-v1` has `superseded-by: [auth]`, and no Note `auth` exists. `capcom know mv auth-v1 --id auth` writes `auth.md` with `superseded-by: [auth]`. The Note then drops out of `know tree`, search and the AGENTS.md block.
- **Fix:** in `PlanMove`, drop `planned.NewID` from the moved Note's own `SupersededBy`, as `renameSuccessors` drops duplicates, or refuse with an `*Error` citing `SupersededByKeyLine()`. Better still, have `nodestore.Move` run `Validate` on the planned content.
- **Acceptance:** A `move_test.go` case "rename to own successor" either removes the entry or refuses, and leaves the Note valid. `GOMEMLIMIT=2GiB go test ./internal/knowledge/`.
- **Related:** A-04 (with a single rename operation over every reference field, including the moved Node's own, this cannot happen).

#### F-24: `know rm` does not report the superseded-by entries it removed from other Notes

- **Severity:** Low · **Category:** Intent drift · **Claims:** #49 · **Lenses:** Intent + Correctness
- **Files:** `internal/nodestore/remove.go:19-34` (`Removed` has no field for them), `:117-122` (copies only the deps and related drops) and `:164-167` (`planNoteRemove` fills `supersededByDropped`). `internal/cli/know_rm.go:54-63, 87-93` (the result is only id and path), compared with `internal/cli/rm.go:30-31, 76`.
- **Problem:** The doc on `Removed` promises "the entries of other nodes the removal dropped". `know rm adr-0011` rewrites `adr-0003.md` silently, and `--json` and `--kv` consumers cannot see it. The rewrite itself is documented (`format.md:427`) and visible in `git diff`.
- **Fix:** add `SupersededByDropped []string` to `Removed`, fill it in `Remove`, and emit it in `noteRemoveResult` as `superseded_by_dropped`, mirroring `issueRemoveResult`. Update `docs/format.md` and the README's result-key list where they list `rm` keys.
- **Acceptance:** A CLI test for `know rm` on a Note that another Note lists as `superseded-by` asserts the key in `--json` output and in text. `GOMEMLIMIT=2GiB go test ./internal/nodestore/`, then `./internal/cli/ -run KnowRm`.
- **Related:** absorbed by **A-04**, where drops are reported keyed by field and this falls out generically.

#### F-25: Move removes the emptied old parent directory as a hard step

- **Severity:** Low · **Category:** Intent drift · **Claims:** #58
- **Files:** `internal/tree/move.go:191-192` (appends `removeEmptyDirStep` to `m.steps`) and `:197-215`. Compare `internal/tree/remove.go:120-123, 206-219` (`AddCleanup` plus a `*CleanupError` warning) and `internal/tree/plan.go:57-58, 70-72`.
- **Problem and trigger:** A concurrent write into the old parent between `ReadDir` and `Remove` (`ENOTEMPTY`), or a Windows handle held on the directory, rolls back an otherwise complete move, every rename and rewrite included, over a harmless empty directory.
- **Fix:** `m.AddCleanup(func() error { return removeEmptyDir(oldDir, os.Remove) })`, delete `removeEmptyDirStep`, and have `Move.Apply` surface cleanup warnings as Remove does. Update the "PlanMove adds no cleanups" doc comment.
- **Acceptance:** A tree test makes the old directory's removal fail through the remove seam and asserts that the move succeeds, with a cleanup warning naming the directory. `GOMEMLIMIT=2GiB go test ./internal/tree/`.

### 2.4 Defects noticed during architecture exploration (verified in a second pass)

These nine were noticed while exploring the architecture and were then adversarially verified against the source at `d192883`. Two are confirmed and listed here with their corrected locations. The other seven (U-01, U-03 to U-08) were refuted and are in Appendix A with the reason; they keep their ids so the references in section 4 stay traceable.

| ID | Where | What | Likely home |
|---|---|---|---|
| **U-02** | `internal/issue/move.go:125-140` (`renameDeps`, applied to `deps` and `related` at `:104-105`). Compare `internal/knowledge/move.go:107-119` (`renameSuccessors`). | **Verified: CONFIRMED (Low).** `renameDeps` skips a renamed entry only when `newID` is already in its output, so an entry already equal to `newID` that comes *after* the renamed one is kept as well. Trigger: Issue `a` has `deps: [x, y]` with `y` dangling, and `capcom mv x y` rewrites `a` to `deps: [y, y]`; `related: [x, y]` becomes `[y, y]` the same way (`[y, x]` comes out right). `issue.Validate` does not reject repeated entries, so the file is written. The dependency graph tolerates the repeat (`issue/deps.go:36`), but the file and `show` carry it. No test covers this order (`issue/move_test.go`). | A-04 (one rename operation for every reference field) |
| **U-09** | `internal/cli/ready.go:9` (used at `:63` and `:90`). `internal/cli/prime.go:11` imports the same package unaliased. | **Verified: CONFIRMED (Low, cosmetic).** Package `cli` imports `internal/briefing` under two names: `briefing` in `prime.go` and the misleading alias `readylist` in `ready.go`, so a search for `briefing.` misses the `ready` path. No behaviour is affected. | (none; fix it with the Phase 2 moves) |

- **Fix (U-02):** dedupe on the output as `renameSuccessors` does: replace a renamed entry with `newID`, then skip any entry equal to `newID` once one is kept. A-04 replaces both functions with one rename op, so fix it there unless A-04 is declined.
- **Acceptance (U-02):** an `issue` test renames `x` to the dangling `y` with `deps: [x, y]` and `related: [x, y]` on another Issue and asserts both lists are `[y]`. `GOMEMLIMIT=2GiB go test ./internal/issue/`.
- **Fix (U-09):** drop the alias and write `briefing.ReadyList` and `briefing.ReadyOptions`.
- **Acceptance (U-09):** `grep -rn readylist internal/` finds nothing, and `go vet ./internal/cli/` is clean.

---

## 3. Findings: test gaps

Every item below is a missing or weak test on a path the project names as critical, or on a documented contract. All were independently verified. Each acceptance line is the test to add, and it must fail against a deliberately broken implementation, which is the bite check.

| ID | Severity | Claims | Files (corrected) | Gap | Test to add (acceptance) | Related |
|---|---|---|---|---|---|---|
| **T-01** | Medium | #1 | `cmd/release/channels.go:80-125` (`publishPackageFile`, with error wrapping at `:88-89, 115-117, 122-124`) and `publish.go:221-225`. The fake `respondChannel` (`channels_test.go:65-71, 104-105`) has no failure toggle. | Nothing makes `gh repo clone`, `rev-list`, `git commit` or `git push` fail for the tap or bucket. The release-convergence path is marked critical, and `channels.go` sits at 77% coverage. | A table test fails each of clone, rev-list, commit and push in turn. It asserts that `publish` returns an error naming the repository and the step, and that `smoke` does not run. Add a failure toggle to `fakeChannels`. | Easier after **A-12** (the `memChannelRepos` adapter). Do it first as a characterisation test if A-11 or A-12 will touch this code. |
| **T-02** | Medium | #13 | `internal/briefing/briefing.go:82-89` (deferred `recover` → `SectionInternal`). `cli/prime.go:64-68` calls `Compose` outside `renderBriefing`'s own recover. | Nothing anywhere references `SectionInternal` or forces a panic inside `Compose`. This is the "never costs a session its start" guarantee. | Add a source seam, for example an unexported package variable for one section reader, set in `export_test.go`, that panics. Assert that the result has exactly one `SectionInternal` Failure and every section is nil. | Do it before F-02(a) lands, because it pins the same guarantee. |
| **T-03** | Low | #14, #15 | `briefing.go:120-121` (index Close failure), `:126-127` (`SectionReady`), `:131-133` (`SectionDoing`), `:162-165` (`SectionCheck`) and `:190-192` (one `SectionElsewhere` per warning). The citations were corrected by the verifier. | The partial-Work rule and the documented failure order ("store, index, ready, doing, locks, check, then Elsewhere warnings", `briefing.go:53-55`) are pinned only for `[index locks]` (`briefing_test.go:171`). | Extend the ordered-failure test with a `check.Run` error and an Elsewhere warning, such as a store in a repository with an unreadable branch ref. Assert the section order, `Health == nil` on a check failure, and that the other sections survive. Reuse the T-02 seam for the ready and doing failures. | T-02 |
| **T-04** | Medium | #37 | `internal/index/refresh.go:242-245` (`ReadNodeFile` not-exist after `forget`), `internal/index/walk.go:37-39` (`Lstat` not-exist) and `refresh.go:226-228` (`rewarn` on corrupt stored `problems` JSON). `walk.go` has the lowest coverage in scope, at 74.1%. | Index refresh is a named critical path. Only deletion *before* the walk is tested (`TestRefresh_FileDeleted_DropsItsRows`). | (1) Delete a listed file between the walk and the read, through the `commitTx`-style seam or a new read hook, and assert that `Refresh` succeeds, its rows are gone, and it is not in `Parsed`. (2) Write bad JSON into `files.problems` and assert that `Refresh` returns an error naming the path. | Do it before F-09 and F-20, which change `refresh.go`. |
| **T-05** | Medium | #45, #46 | `internal/lock/lock.go:236-238` (`Touch` with a token that does not match → `*HeldError`) and `internal/lock/lock_test.go:89` (the assertion is `!touched.Heartbeat.Before(acquired.Heartbeat)`; the cited `:71` was off). | (a) The path a displaced session hits after a stale takeover has no test. (b) The heartbeat-advance assertion passes when the heartbeat is unchanged. Both are on the bundle's critical-path list (lock touch). | (a) Acquire, take over with `Acquire(..., isTakeStale=true)`, then `Touch(key, oldToken)` returns `*HeldError`, and the new holder's record and heartbeat are unchanged on disk. (b) Backdate the stored heartbeat (as the stale-resume test does), Touch, and assert that the heartbeat is strictly `After` the backdated value, in memory and on disk. | (none) |
| **T-06** | Low | #21 | `internal/cli/merge_driver.go:99-101` (the `tree.ReplaceFileVerbatim(<ours>)` failure → `failedMerge`) and `internal/tree/atomic.go:46-48` (the `ReplaceFileVerbatim` missing-path branch, left unpinned per the verifier of refuted #54). | The write is a temp file plus a rename, so ours cannot be half-written. These tests guard only against a future regression. | A merge-driver test with `<ours>` replaced by a directory, or in a read-only directory (skipped on Windows and as root), asserts a `CodeInvalid` failure naming ours, exit 1, the original bytes intact and no `.tmp-` litter. A tree test calls `ReplaceFileVerbatim` on a missing path and asserts the error names it. | (none) |
| **T-07** | Low | #25, #28 | `internal/elsewhere/elsewhere.go:95-99` (`Branches`), `:188-199, 209-213` (`IsAncestor`), `:218-222` (`ChangedFiles`) and `:260-266` (`issueID` filter, called from `scanBranch` at `:224`). | The "never fails" contract is pinned only for an unborn HEAD and an unparseable copy. The hidden, temp and non-`.md` filter is never exercised. | (1) `Scan` at a directory that is not a repository gives one "list branches" warning and no entries. (2) A corrupt or dangling branch ref gives a per-branch warning while a healthy second branch is still reported. (3) A side branch adding `.hidden.md`, `notes.txt` and one real Issue reports only the real Issue. | F-02(d) and F-18 change the same file. Add these tests first. |
| **T-08** | Low | #31 (plus the residue of refuted #30) | `internal/gitx/branches.go:153-157` (`IsAncestor`: any exit other than 1 is an error) and `:134-144` (`HasCommit`). | Only the true and false cases of `IsAncestor` are tested (`branches_test.go:108`). | `IsAncestor` with a ref that does not exist returns a non-nil error and `false`. The `HasCommit` error case is covered by F-19's acceptance. | F-19 |
| **T-09** | Low | #52 | `internal/store/store.go:189` (`createDir` "exists and is not a directory"), `:204-213` (`createConfig` open, write and close errors) and `RaiseFormat`'s read error that is not ENOENT. The write-failure branch of `RaiseFormat` is already covered by `TestRelatedAdd_RaiseFails_FailsInvalidAtTheConfig`. | `capcom init` runs this code on a user's repository, and a stray `.capcom` file is the realistic way it fails. | (1) `.capcom` as a regular file: `Init` errors naming `.capcom` and creates nothing. (2) `config.yaml` as a directory: `RaiseFormat` errors and the file is unchanged. | F-02(b) and F-16 change `store.go`. Do this first. |

The machine-check *leads* that no lens confirmed are listed in Appendix B.

---

## 4. Architecture review: deepening candidates

These candidates were found by exploring `internal/cli`, the domain and storage packages, and the infrastructure and release packages, then merged and de-duplicated across the three reports. Where two reports rated the same candidate differently, both ratings are given and the stronger one is used. Dependency categories:

- **in-process:** pure computation. Test it directly.
- **local-substitutable:** a local stand-in exists, such as a temp-dir filesystem or real SQLite.
- **ports & adapters:** a seam with a production adapter and a test adapter.

The ADRs are taken as settled: ADR 0001 (the directory is the hierarchy), ADR 0002 (slug ids plus Aliases), ADR 0005 (advisory Tree lock), ADR 0007 (real-file tests), ADR 0008 (release tool) and ADR 0009 (store format). A candidate that touches one says so.

### Summary

| ID | Candidate | Strength | Category | Absorbs defects |
|---|---|---|---|---|
| A-01 | Issue lifecycle rules into `issue` | **Strong** | in-process | F-11(d) close, and the "closed category" check written 4× |
| A-02 | Generic Tree-node verbs in `cli` | **Strong** (mv/rm), Worth exploring (update) | in-process | (none; U-05 and U-07 were refuted) |
| A-03 | Post-write effects into `nodestore` | **Strong** | local-substitutable | (none; U-01 was refuted), and supports F-17 |
| A-04 | One reference-rewrite engine in `nodestore` | **Strong** | in-process (filesystem: local-substitutable) | F-24, F-23, U-02 |
| A-05 | `tree` owns node-file layout and claims, file IO moves to `nodefile` | **Strong** (layout and IO), Worth exploring (claims) | in-process / local-substitutable | the home for F-02's guarded reader, F-10(b), F-11(b) and (c) |
| A-06 | One per-file diagnostics module for `index` and `check` | Worth exploring | in-process | structurally F-10 and F-09 (U-04 was refuted) |
| A-07 | Store format owned by `config` | **Strong** | in-process | F-16, F-17 |
| A-08 | One outline and progress walker | Worth exploring | in-process | (none; locality) |
| A-09 | Agent-settings and `.gitattributes` editors out of `init` | Worth exploring | local-substitutable | (none; testability) |
| A-10 | Entry points take the store, not `cwd` | Speculative to Worth exploring | in-process | (none; one store open per session start) |
| A-11 | `cmd/release`: one step table, one file per channel | **Strong** | in-process | the home for F-03, F-04, F-13 (U-06 was refuted) |
| A-12 | `cmd/release`: channel-repository seam | Worth exploring | ports & adapters | T-01 (testable), F-14 |
| A-13 | Generic frontmatter field table | Speculative | in-process | (none; touches F-08(b)) |
| A-14 | Extract `internal/cli/output` | **Strong** (as the one package split that earns its keep) | in-process | the home for F-07's sanitiser (U-08 was refuted) |
| A-15 | Trim shallow exported interfaces | Speculative | in-process | (none; dead interface) |

### A-01: Move the Issue lifecycle rules from the CLI into `issue`

- **Strength:** Strong. The cli exploration rated it Strong and the domain exploration Worth exploring; both found it. · **Category:** in-process (pure)
- **Files:** `internal/cli/update.go:103-165` (`checkStatusChange` and `isClosedCategory` at `:147-165`), `internal/cli/close.go:161-201` (`closeOne` sets Status, Closed and Reason), `:242-258` (reopen) and `:274` (`targetStatus`), `internal/cli/create.go:129-170` (create must not start Closed; `defaultType`), `internal/cli/log.go:39`, `internal/cli/know_update.go:231-273` (`applyNoteUpdate`, which duplicates `applyUpdate`), `internal/issue/ready.go:158`, `internal/issue/validate.go:51` and `internal/index/query.go:117` (SQL).
- **Problem:** The status-category state machine lives only in cobra handlers: close needs a reason and a time, reopen clears them and writes a Log entry, and an update may not enter or leave the closed category. "Category is done or dropped" is written four times (`cli` `isClosedCategory`, `issue` `isClosed`, `validate.go:51`, index SQL). The interface a caller must learn is the whole of `close.go`, and any future non-CLI caller could bypass the rules.
- **Solution:** Add `config.Category.IsClosed()`. Add `issue.Close(i, status, reason, at)`, `issue.Reopen(i, status, reason, author, at)` and `issue.SetStatus(i, to, cfg)`, which return a new Issue or a typed error (`ErrAlreadyClosed`, `ErrClosedTransition`). The CLI keeps flag handling and the mapping to error Codes. Optionally, add a shared "update spec" applied to both kinds.
- **Wins:** **Locality:** the workflow rules live in one module, testable without cobra or git, which matches ADR 0007's "pure parts" unit tests. **Leverage:** `close`, `reopen`, `update` and the index all call one predicate.
- **Absorbs:** the F-11(d) close race. Calling `issue.Close` on the *fresh* Issue inside `Kind.Edit`'s change callback makes "already closed" a re-check on current bytes.
- **ADR:** ADR 0003 (rules read the category, never the status name) is reinforced, not changed.
- **Risks:** Error wording and Codes must stay byte-identical, and the CLI tests pin them. Do this after F-05, so `show` is not mid-change.

### A-02: One generic module for the Tree-node verbs in `cli`

- **Strength:** Strong for mv and rm, Worth exploring for update and create. · **Category:** in-process
- **Files:** `internal/cli/mv.go:27-80` against `know_mv.go:20-67` (nearly line for line), `rm.go:38-90` against `know_rm.go:25-66`, `update.go:200-237` (`editNode`, already generic), the resolvers at `update.go:233` and `know_update.go:203`, and `create.go:83` against `know.go:350` (`know add`).
- **Problem:** Each pair differs only in the `nodestore.Kind`, the result builder, the Unread-warning wording ("deps" or "superseded-by", `mv.go:72` against `know_rm.go:79`), the post-write AGENTS.md refresh, and whether `--kv` is on. The pairs have already drifted in form: the Unread warning is inlined on one side and a helper on the other. (The `--kv` asymmetry, U-05, is deliberate, and `dep add`'s missing `--if-match`, U-07, loses no write; both are refuted in Appendix A.) With 4 verbs × 2 copies, the copies fail the deletion test, because deleting one copy loses nothing.
- **Solution:** A generic descriptor `nodeVerbs[T]` holding the Kind constructor, the noun, the write-result builder, an `afterWrite func(*Result)` and a `kv` switch, plus `newMvCommand[T]` and `newRmCommand[T]`. `rm`'s `--drop-deps` and `DependentsError` become an optional extension. Push the link-key name ("deps" or "superseded-by") into `nodestore.Codec` beside `Noun`.
- **Wins:** **Leverage:** one interface per verb, so a new flag lands on both kinds at once. **Locality:** `--kv` parity becomes one decision.
- **Absorbs:** no verified defect. U-05 and U-07 were refuted.
- **Risks:** Help text and flag sets are user-visible, and `--help` output must not change unintentionally. `--kv` is registered only on commands whose results carry KV records (closed issue `kv-flag-scope`), so the descriptor's `kv` switch must leave the Note verbs without it.

### A-03: Post-write effects belong in `nodestore`, not in each handler

- **Strength:** Strong · **Category:** local-substitutable (temp-dir store)
- **Files:** `internal/cli/related.go:71-83` and `internal/cli/know_update.go:176-191` (`raiseStoreFormat` inside the `Kind.Edit` change callback), `internal/cli/know.go:386` (`refreshAgentsAfterWrite`, called separately by `add` (top level only), `update`, `mv` and `rm`) and `internal/nodestore/edit.go:59-63`.
- **Problem:** Two store rules are re-implemented per command: "a write that adds a format-N key raises `config.yaml` first", and "a write to Knowledge's top level re-syncs AGENTS.md". The two copies already differ: `know update` validates before it raises the format and `related add` does not. That is harmless today only because `related add`'s change cannot invalidate an Issue the index accepted (U-01, refuted in Appendix A).
- **Solution:** Add a hook in `Kind.Edit` and `Kind.Create` after validation. Alternatively, `Codec` declares `RequiredFormat(before, after T) int` and nodestore raises the format itself, in the order validate → raise format → re-check → write. The Notes Kind returns the AGENTS.md sync status alongside `Written`.
- **Wins:** **Locality:** the order is enforced once. **Leverage:** every caller, present and future, gets it.
- **Absorbs:** no verified defect; it keeps the order from drifting. It works with A-07, because `RequiredFormat` reads `config.FormatRelated` and `config.FormatSupersession`.
- **ADR:** ADR 0009. The format rules are unchanged, only enforced in one place.
- **Risks:** nodestore gains a dependency on store and agents for the effects. Keep it behind a function field or interface supplied by the CLI so nodestore does not import `agents`. The base rule "no interface without ≥2 adapters" is met by a production adapter plus a test adapter.

### A-04: One reference-rewrite engine in `nodestore`

- **Strength:** Strong · **Category:** in-process (filesystem local-substitutable via `t.TempDir()`)
- **Files:** `internal/issue/move.go:57-140`, `internal/issue/remove.go:83-180`, `internal/knowledge/move.go:38-124`, `internal/knowledge/remove.go:39-104`, `internal/nodestore/codec.go:29-97` and `internal/nodestore/remove.go:53-64, 132-169, 235-246`.
- **Problem:** Four near-identical loops each read every other Node file, parse it or add it to Unread, rewrite a reference list and emit a `tree.Rewrite`. Only the field (deps and related, or superseded-by) and the operation (rename or drop) differ. Further duplication:
  - `isRefusedRead` is copied three times (`issue/remove.go:178`, `knowledge/remove.go:100`, `nodestore/kind.go:128`).
  - The `rawfm` Alias fallback is copied twice (`issue/remove.go:97`, `knowledge/remove.go:50`).
  - Kind-specific fields leak into the generic module (`depsDropped`, `relatedDropped`, `supersededByDropped`, `nodestore/remove.go:57-61`).
  - The etag check happens in `issue.PlanRemove` for Issues but in nodestore for Notes, which gives two error vocabularies (`issue.ETagMismatchError` and `issue.DependentsError`, translated to `nodestore.ConflictError` and `DependentsError`).
  - The rename functions have already drifted: `renameSuccessors` dedupes its output and `renameDeps` does not, so a list already holding the new id after the renamed entry ends up with it twice (U-02).
- **Solution:** `Codec` gets a list of reference fields, `{Key, Get(T) []string, Set(*T, []string), BlocksRemoval bool}`. nodestore owns a single `planReferenceRewrites(nodes, names, op)` and a single etag check. `Removed` and `Moved` report drops keyed by field. `issue` and `knowledge` keep only Parse, Serialize, Validate and the domain rules.
- **Wins:** **Locality:** rename and drop are written and tested once, and the reading of `Original` that F-11(a)'s compare-and-swap checks against happens in one planner instead of four. **Leverage:** a new reference field, such as Note Refs to Issues, is one table entry. Four functions, the `plannedMove`/`plannedRemoval` adapters and two error types are deleted.
- **Absorbs:** F-24 (drops reported per field), F-23 (the rename op also applies to the moved Node's own fields), U-02. Typed step errors in `removalApplyFailure` are a reasonable part of the engine, but not a defect fix (U-03 was refuted). F-11(a) is fixed earlier, in `tree.Plan`, and A-04 must keep its compare-and-swap test green.
- **ADR:** ADR 0002 (Aliases) is unchanged.
- **Risks:** This is the largest refactor in the list, and wording of Unread warnings and `rm` result keys is pinned by CLI tests. Land T-04-style characterisation tests for move and remove results first, and keep the `--json` keys identical, adding only `superseded_by_dropped`.

### A-05: `tree` owns the node-file layout and the id-claim concept, and file IO moves to `nodefile`

- **Strength:** Strong for layout and IO, Worth exploring for claims · **Category:** in-process / local-substitutable
- **Files:** `internal/tree/atomic.go` and `internal/tree/read.go` (plus `syncdir_*.go`), `internal/nodestore/create.go:18-21, 118-266, 320`, `internal/nodestore/move.go:83-98`, `internal/cli/mv.go:113-114`, `internal/cli/errors.go:30`, `internal/index/query.go:17`, `internal/knowledge/note.go:41, 272` and `internal/edit` (`ETag`).
- **Problem:**
  - Six packages re-derive the `.md` extension, "id = base name" and "children directory = path minus `.md`".
  - nodestore walks orphan directories again with its own copy of `tree.Walk`'s skip rules (hidden prefix, extension, `ModeIrregular`; `create.go:222-261` against `tree.go:279-371`).
  - The ordering constraints between claim checks are scattered across the verbs: "each later refusal relies on the earlier ones" (`create.go:45-49`), the orphan check before triage, and the unparsed check after (`move.go:84-97`).
  - `tree` hosts generic atomic writes and guarded reads used by `lock`, `store`, `agents`, `check`, `cli/init`, `cli/update` and `index`. `lock` and `store` import `tree` only for that.
  - The Alias resolution rule is implemented three times: `tree.Resolve` (`tree/resolve.go:54`), `index.resolveID` in SQL (`index/query.go:146`) and `issue.readyTree.resolve` (`issue/ready.go:112`). The three treat a doubly-claimed Alias slightly differently.
- **Solution:**
  - `tree` exports `IDOf(path)` and `ChildrenDir(path)`. `Load` takes a second, recovering `AliasFunc`.
  - `Tree.Claimant(name)` returns any holder: a live Node, a dropped duplicate, a file in an orphan directory, or an unparseable file.
  - Move `atomic.go`, `read.go`, `syncdir_*.go` and `edit.ETag` into a small `internal/nodefile` package (or `fsx`). It holds the F-02 guarded reader, the F-11(b) refuse-missing replace and the F-11(c) no-replace rename.
- **Wins:** **Locality:** the layout rules live in one module, and so does every file-safety rule. Create and Move shrink to load, resolve, claim, triage, write. **Leverage:** `lock` and `store` stop importing `tree`, and every reader gets the hostile-repo guard for free.
- **Absorbs:** F-02 (its permanent home), F-10(b) (`Claimant` covers dropped duplicates), F-11(b) and F-11(c).
- **ADR:** ADR 0001 (the directory is the hierarchy) is unchanged and only centralised.
- **Import cycles:** `nodefile` must import nothing from `internal/` except possibly a sentinel-error package, and in particular must not import `tree`. `tree` imports `nodefile`. `lock`, `store`, `agents`, `check`, `index` and `cli` switch their imports.
- **Risks:** The moves touch about 10 importers, so do the pure move (section 5, P2-03) first, then the claim concept. The sentinels `tree.ErrNotRegular` and `tree.ErrTooLarge` are matched by callers with `errors.Is`. Keep them, as aliases or by moving them and updating every `errors.Is` site.

### A-06: One per-file diagnostics module shared by `index` and `check`

- **Strength:** Worth exploring · **Category:** in-process
- **Files:** `internal/check/check.go:32-72, 206-237, 295-306`, `internal/index/index.go:73-94`, `internal/index/walk.go:61-69` and `internal/index/refresh.go:287-328, 412-476`.
- **Problem:** The same file problems have two names: `CodeX` in `check` and `WarningX` in `index`. Three mapping functions translate between them (`treeCode`, `walkWarningKind`, `readRefusal`). Two duplicate detectors (`tree.Load` for `check`, SQL `excludeClaims` for the index) disagree, as F-10 shows. (The `duplicate` against `repeated-alias` naming, U-04, never reaches a user, because index warning kinds are not rendered; see Appendix A.) Tests such as `TestRun_DirectoryProblems_WordedAsTheIndexWarnsThem` are all that keeps them in sync.
- **Solution:** One module, in `tree` or in a new package `diagnose`, with a single Kind enum, `FileProblems(kind, relPath, data, cfg) []Finding` and `NameClashes(claims) []Finding`. The index caches the per-file result in `files.problems`, and `check` adds the cross-file rules. ADR rule "check never reads the index" is kept, because only the *function* is shared.
- **Wins:** **Locality:** one source for problem kinds and wording, and consistent JSON codes. The "worded as" sync tests can be deleted.
- **Absorbs:** no verified defect directly. Structurally it settles F-10, because there is one duplicate detector, and F-09, because claims of left-out files are recorded uniformly.
- **Risks:** `check`'s JSON codes are a public interface. Renaming `check`'s `duplicate` code for an in-file repeated Alias would be a user-visible change and needs a CHANGELOG note. Do F-10 and F-09 first as local fixes, then this.

### A-07: The store format is owned by `config`

- **Strength:** Strong · **Category:** in-process
- **Files:** `internal/config/config.go:37-48, 165-167, 208-211`, `internal/store/store.go:164-168`, `internal/check/check.go:77-86, 445-457` and `internal/cli/related.go:17-21`.
- **Problem:** Two rules have no single owner:
  - "A missing `config.yaml` means format 1" is written three times (`keylessFormat`, `missingConfigFormat`, a comment), and `Store.Config.Format` says 3 anyway.
  - The per-key thresholds (`related` needs 2, the supersession keys need 3) are copied into `cli` and `check`, and `check` lost one of them.
- **Solution:** Add `config.MissingFormat`, and have `loadConfig` return format 1 for a missing file. Export `config.FormatRelated` and `config.FormatSupersession`. `check` reads `Config.Format` directly. Split `config.go` into `config.go`, `parse.go` and `format.go` (section 5).
- **Wins:** **Locality:** one owner per threshold, and `check` gets simpler.
- **Absorbs:** F-16 and F-17.
- **ADR:** ADR 0009 is implemented more faithfully. It is not changed.
- **Risks:** Low. `config` stays a leaf package, importing nothing from `internal/`.

### A-08: One outline and progress walker

- **Strength:** Worth exploring · **Category:** in-process
- **Files:** `internal/cli/list.go:233-327` (`issueTree.visit`/`add`/`progress`, `issuesByParent`), `internal/cli/know.go:159-221` (`outline`, `childrenByParent`), `internal/cli/issue_render.go:105` (`childProgress`, two SQL queries per Issue) and `list.go:316` (the same progress computed in memory). "List every Issue including Closed, then convert to `[]issue.Node`" is repeated at `close.go:147`, `dep.go:283` and `list.go:234`.
- **Solution:** One generic `walkOutline[E](children, start, maxDepth, isShown)` beside `renderOutline` in `outline.go`, and an `index.AllIssueNodes()` (or `issue.Progress(children)`) used by `show`, `list` and `close`.
- **Wins:** **Locality:** one walker. `--depth` behaves the same in `list --tree` and `know tree`.
- **Risks:** The outline text output is pinned by tests, so keep it byte-identical.

### A-09: Move the agent-settings and `.gitattributes` editors out of `init`

- **Strength:** Worth exploring · **Category:** local-substitutable
- **Files:** `internal/cli/init.go:122-227, 340-522`: hook groups, `decodeObject`/`encodeObject` keeping member order, and adding a line while keeping CRLF.
- **Problem:** About 300 lines of pure file-editing logic sit in the CLI and are tested only end to end (`init_test.go:313-692`).
- **Solution:** Move them into `internal/agents` or a new `internal/hooks` package, as `InstallHooks(root, target) (Status, error)` and `InstallMergeAttribute(root)`.
- **Wins:** `init.go` shrinks to about 120 lines. **Locality:** the JSON-merge edge cases are unit-tested on byte slices.
- **Import cycles:** `hooks` imports `nodefile` (or `tree`) for atomic writes and never imports `cli`.
- **Risks:** Low, because the end-to-end tests stay as a safety net.

### A-10: Entry points take the store, not `cwd`

- **Strength:** Speculative to Worth exploring · **Category:** in-process
- **Files:** `internal/briefing/briefing.go:91, 162, 179-188`, `internal/check/check.go:120`, `internal/cli/output.go:234-250` and `internal/cli/lock.go:241-242`. Six commands open the store themselves: `check.go:21`, `agents.go:45`, `fmt.go:44`, `lock.go:231`, `prime.go:62` and `init.go:71`.
- **Problem:**
  - `briefing.Compose` opens the store, then `check.Run(cwd)` opens it again, repeating git discovery and the config parse.
  - The Issue-tree pathspec for Elsewhere is computed in both `briefing` and `cli`.
  - Lock callers pair `lock.Open` with `WorktreeKey` by hand.
- **Solution:** `check.Run(*store.Store)`, `elsewhere.Scan(store, now)` or `store.IssuesPathspec()`, and a `lock.ForWorktree(store)` handle with the key bound. Add a `store` value to the CLI's command annotation, which removes four of the six manual opens. `prime` and `init` stay special.
- **Wins:** smaller interfaces and one store open per session start.
- **Risks:** `prime` must keep turning a store failure into a warning (ADR 0009).

### A-11: `cmd/release` gets one step table and one file per package channel

- **Strength:** Strong · **Category:** in-process
- **Files:** `cmd/release/probes.go:75-89` (`publishSteps`), `publish.go:74-92, 171, 206, 237-244` (`publishActions` and the loops), `channels.go:158`, `render.go:159-194` and `smoke.go:392-486`.
- **Problem:** `publishSteps` and `publishActions` are parallel tables joined by name (`publish.go:241`), with one positional assumption left, `steps[0]` as the tag step (`publish.go:171`). They cannot silently mis-pair (U-06, refuted in Appendix A), but two tables per step is one too many. Channel knowledge is spread by verb across four files: the Scoop manifest is rendered at `render.go:167`, parsed at `probes.go:219` and parsed again at `smoke.go:432`. A new channel touches four or five files.
- **Solution:** One `step` record, `{name, probe, describe, converge, smoke, doneDetail}`. Put each channel in its own file (`homebrew.go`, `scoop.go`, `winget.go`), keep `publish.go` for orchestration only, and have `smoke.go` loop over the channels' smoke checks.
- **Wins:** **Locality** per channel. There is no positional coupling, and `smoke.go` (622 lines) and `publish.go` (547) shrink along channel lines.
- **Absorbs:** no verified defect. It is the natural home for F-03 (pinned-version compare in `homebrew.go`/`scoop.go`), F-04 and F-13 (the release step) and F-14 (`winget.go`).
- **ADR:** ADR 0008. Preserve its rules: the gate has no bypass, every `gh release` names `--repo airiclenz/capcom`, verb tests never run git, gh, go or komac, and a channel with pending setup is skipped.
- **Risks:** Release tooling is hard to exercise end to end. Keep `TestPublishConvergesFromRemoteStateOnceTheTagExists` and the argv-pinning `execRepo` tests green throughout.

### A-12: `cmd/release` gets a seam for channel repositories

- **Strength:** Worth exploring · **Category:** ports & adapters
- **Files:** `cmd/release/channels.go:80-126, 157-172, 291-304`, `probes.go:176-285, 313-327`, `smoke.go:419-430` and the test doubles at `probes_test.go:44` (`fakeRemote`), `smoke_test.go:35` (`smokeFake`) and `channels_test.go`.
- **Problem:** The capcom repository has a semantic seam (`repo`: `execRepo` plus `memRepo`). The tap, the bucket and winget-pkgs are reached through raw `gh`/`git`/`komac` argv, so the tests parse command strings, and the winget listing call is duplicated (`probes.go:233`, `channels.go:162`).
- **Solution:** A `channelRepos` interface with `Exists(repo)`, `ReadFile(repo, path)`, `PublishFile(repo, path, content, subject) (changed bool, err)` and `OpenPulls(repo, query)`, with `execChannelRepos` and `memChannelRepos` as adapters. komac and brew stay on the `runner` seam. That gives two adapters, which meets the house rule.
- **Wins:** **Leverage** across probe, converge and smoke. Tests describe channel state, not argv.
- **Absorbs:** T-01, where failure injection becomes trivial, and F-14, where `OpenPulls` uses a non-search read.
- **ADR:** ADR 0008's amendment on test seams. This adds a seam beside `repo` and `runner` and does not change the policy.
- **Risks:** Only `execChannelRepos`'s own tests may pin argv, mirroring the `execRepo` rule.

### A-13: A generic frontmatter field table

- **Strength:** Speculative · **Category:** in-process
- **Files:** `internal/issue/file.go:12-104` and `internal/knowledge/note.go:45-131`. Both repeat a `field` type, `listField`, `fieldKeys` and a Parse loop ending in an "unreachable" return.
- **Solution:** `frontmatter.Schema[T]` with `Text`, `List` and `Raw` builders. It saves about 80 lines.
- **Relation:** touches F-08(b). Do F-08 first, independently.
- **Risks:** The canonical key order and the round trip must stay byte-identical.

### A-14: Extract `internal/cli/output`

- **Strength:** Strong. It is the one package split the cli exploration found worth making. · **Category:** in-process
- **Files:** `internal/cli/output.go` (Result, KVRecord, Envelope, Renderer, text/json/kv) and the top of `internal/cli/errors.go` (Error, Code, the exit codes).
- **Problem:** The output module is already deep, with a small interface (Result in, bytes out), but it shares a package with 28 command files, so nothing stops a command from reaching past its interface.
- **Solution:** Move it to `internal/cli/output`. The white-box test calls (`newRenderer`, `storeFailure`) move with it. It imports only `elsewhere`, and later `nodestore` and `tree` if the error mapping moves, and **must never import `cli`**.
- **Wins:** **Locality** for every output rule: the `--kv` WARNING/STATUS order, the exit codes, and the F-07 sanitiser.
- **Absorbs:** no verified defect (U-08 was refuted). It is the home for F-07.
- **Risks:** The command files gain an import. Keep `runCLI` unchanged.

### A-15: Trim shallow exported interfaces

- **Strength:** Speculative · **Category:** in-process
- **Items:**
  - `index.Refresh`, `index.Rebuild`, `index.WithClock` and the `Report` fields are used only by `Open` and by tests.
  - `agents.Inspect`, `Write`, `Render`, `TopNotes` and `Path` are used outside the package only by a CLI test; only `Sync`, `FileName`, `Error` and `State` are used in production.
  - `tree.(*Tree).Dir` (`tree.go:550`) has no caller outside tests. This is a machine-check lead.
  - `internal/edit` is a mostly shallow grab bag: `Labels` is reused as a generic list add and remove (`cli/know_update.go:248,256`), and `ETag`, a node-file concept, moves to `nodefile` with A-05.
- **Wins:** smaller interfaces. This follows the base rule "delete dead code [must]".
- **Risks:** None, beyond moving test helpers to `export_test.go`.

---

## 5. Modularization plan: splitting the large files and packages

The house standard is files under about 400 lines, functions of about 20-30 lines, and a `doc.go` mapping every non-test file in any package with more than about 10 files, enforced by a structural test. Today only `internal/cli` has a `doc.go` (`internal/cli/doc.go`, pinned by `internal/cli/doc_test.go`).

### 5.1 Sizes

Non-test Go lines at `d192883`:

**Largest files**

| File | Lines | Over ~400? | Proposal |
|---|---|---|---|
| `cmd/release/smoke.go` | 622 | yes | A-11 per-channel smoke checks, or P2-13 |
| `internal/tree/tree.go` | 586 | yes | P2-01 |
| `internal/check/check.go` | 565 | yes | P2-04 |
| `internal/cli/know.go` | 553 | yes | P2-08 |
| `cmd/release/publish.go` | 547 | yes | A-11, or P2-13 |
| `internal/cli/init.go` | 522 | yes | A-09 (or P2-12) |
| `internal/config/config.go` | 498 | yes | P2-05 |
| `internal/index/refresh.go` | 476 | yes | P2-06 |
| `internal/cli/dep.go` | 452 | yes | P2-10 |
| `internal/lock/lock.go` | 450 | borderline | leave: cohesive, one concept |
| `cmd/release/repo.go` | 408 | borderline | leave: one seam plus its adapter |
| `internal/nodestore/create.go` | 395 | no, but it hosts helpers shared by every verb | P2-02 |
| `cmd/release/dist.go` | 390 | no | leave |
| `internal/cli/lock.go` | 384 | no | leave. The cli exploration calls it "thin, well-factored, model to follow". |
| `internal/agents/block.go` | 376 | no | leave. Do not grow it with A-09, which goes to `internal/hooks`. |

**Packages**

| Package | Source lines | Files | Test lines | Note |
|---|---|---|---|---|
| `internal/cli` | 6032 | 30 | 7732 | Has `doc.go` and a structural test. See the decision in 5.4. |
| `cmd/release` | 3747 | 13 | 5970 | Has more than 10 files but **no `doc.go`**. Add one with A-11 or P2-13. |
| `internal/index` | 1616 | 6 | 1500 | |
| `internal/tree` | 1584 | 9 | 2105 | Reaches 13 files after P2-01, so it needs a `doc.go` and a structural test. A-05 / P2-03 then take files out again. |
| `internal/issue` | 1467 | 10 | 2118 | |
| `internal/nodestore` | 1298 | 8 | 1525 | Reaches 11 files after P2-02, so it needs a `doc.go`. |
| `internal/check` | 565 | 1 | 667 | |
| `internal/config` | 498 | 1 | 353 | |

### 5.2 Pure moves (zero behaviour change, safe first, one commit each)

Rules for every P2 item:

- Only moves: no renames of exported symbols and no logic edits. The diff should be file creation plus deletion of the same text.
- `go vet` and the package's tests pass unchanged.
- Update `doc.go`, which the structural test enforces, where it exists or is created.
- One `refactor(<pkg>): …` commit per item.

Line ranges are from the architecture exploration at `d192883`. Re-derive them by symbol at implementation time.

| ID | Split | Target files and what moves | Notes |
|---|---|---|---|
| **P2-01** | `internal/tree/tree.go` (586) | `errors.go`: sentinels, `Error`, `Errors` (about `:43-118, 244-269, 434-448`). `ids.go`: `ValidateID`, `ValidateAlias`, `CheckAliases` (about `:147-170, 396-432`), plus `RenameAliases` from `move.go:156`. `walk.go`: `Walk`, `treeWalker`, `leftOutFile` (about `:220-371`). `claims.go`: `indexIDs`, `indexAliases`, the duplicates bookkeeping (about `:373-547`). `tree.go` keeps `Tree`, `Load`, the accessors and `PathFor`. | Add `internal/tree/doc.go` and a structural test modelled on `internal/cli/doc_test.go`. It must come before P2-03 and A-05. |
| **P2-02** | `internal/nodestore/create.go` (395) | `create.go`: `Create` only. `load.go`: `load`, `triage`, `isBlockingProblem`, `resolveFailure`, `relativeTreeError`, which Create, Move, Remove and Format share. `claim.go`: `claimFailure`, the orphan and unparsed checks, `findNodeFile`. `write.go`: `write`, `encode`, `place`, `validationError`, shared with Edit and Format. | Add a `doc.go`. A reader of Move or Edit no longer opens `create.go` for its helpers. It must come before A-04 and A-05. |
| **P2-03** | `internal/tree/{atomic.go, read.go, syncdir_*.go}` → **new package `internal/nodefile`** | The atomic writes (`WriteFileAtomic`, `WriteFileAtomicVerbatim`, `ReplaceFileVerbatim`, `WriteFileExclusive`, `lfWriter`), `ReadNodeFile`, the F-02 guarded reader and `syncDir`. `edit.ETag` moves here too. | This is the first, mechanical half of A-05. Keep `tree.ErrNotRegular` and `tree.ErrTooLarge` matchable: define them in `nodefile` and re-export them from `tree` as `var ErrNotRegular = nodefile.ErrNotRegular`, so every `errors.Is` site still works. Update the imports in `lock`, `store`, `agents`, `check`, `index`, `cli` and `nodestore`. `nodefile` imports nothing from `internal/`. Platform files keep their build tags. The export_test seams (`SetLink`) move with them. |
| **P2-04** | `internal/check/check.go` (565) | `problem.go`: the `Problem` type and Code constants (`:32-72`), and tree-load errors mapped to Problems (`:183-237`). `issues.go`: the Issue rules (parse, fields, plan lookup, deps, related, cycles; `:248-400`). `notes.go`: the Note rules (format gate, supersession, retired subtrees, Ref checks; `:404-547`). `agents.go`: the AGENTS.md check (`:550-`). `check.go` keeps `Run`. | It must come before A-06 and A-07. |
| **P2-05** | `internal/config/config.go` (498) | `config.go`: the types, `Default`, the lookups (`:1-203`). `format.go`: `RaiseFormat`, `findFormatLine` (`:229-300`). `parse.go`: the strict YAML parser (`:301-498`). | It must come before A-07. |
| **P2-06** | `internal/index/refresh.go` (476) | `refresh.go`: refresh orchestration, `indexFile`, `forget`, `rewarn`. `problems.go`: file problems mapped to Warnings (`readRefusal` and its friends, about `:287-328`). `duplicates.go`: duplicate and claim exclusion (about `:412-476`). | It must come before A-06. |
| **P2-07** | Flag-name constants scattered across `internal/cli` (`list.go:22`, `update.go:23-30`, `know.go:21`, `create.go`, `know_update.go`) | A new `internal/cli/flags.go` collects every shared flag constant (`flagParent`, `flagDepth`, `unlimitedDepth`, `flagIfMatch`, `flagTitle`, …). | This removes hidden coupling, because commands no longer import their flags from a sibling command's file. |
| **P2-08** | `internal/cli/know.go` (553) | `know.go`: the `know` root command. `know_tree.go`: `know tree` and the outline (`:159-221`). `know_show.go`: `know show` (`noteShowResult`, about `:257-275`). `know_search.go`: `know search`. `know_add.go`: `know add` (about `:350-`) and `refreshAgentsAfterWrite` (`:386`). `know_verify.go`: `know verify` (about `:470-`). | It must come before A-02, A-03 and A-08. Matches the existing `know_update.go`, `know_mv.go` and `know_rm.go` naming. |
| **P2-09** | `internal/cli/close.go` (299) | `close.go`: close. `reopen.go`: reopen (about `:242-258`) and `targetStatus` (`:274`). | Small. It must come before A-01. |
| **P2-10** | `internal/cli/dep.go` (452) | `dep.go`: the `dep` root plus `dep add` and `dep rm` (`depRemoval`). `dep_list.go`: `dep list`. `dep_graph.go`: `loadDepGraph` (`:283`) and `names` (`:298-303`), which `related.go` also uses. | Independent of the fixes. |
| **P2-11** | `internal/cli/update.go` (288) shared helpers | `node_edit.go`: `editNode`, `readTextInput`, `writeResult`, which `log`, `close`, `related`, `dep` and `know update` use. `update.go` keeps the `update` command. | This is preparation for A-02. |
| **P2-12** | `internal/cli/init.go` (522), **only if A-09 is declined** | `init.go`: the command. `init_settings.go`: the agent-settings JSON editor (`:122-227` and part of `:340-522`). `init_gitattributes.go`: the `.gitattributes` editor. | If A-09 is accepted, skip this and move the code straight to `internal/hooks`. |
| **P2-13** | `cmd/release/smoke.go` (622) and `publish.go` (547), **only if A-11 is deferred** | `publish.go`: orchestration. `publish_tag.go`: `convergeTag`, `verifyTag`, `waitForTag`. `publish_release.go`: `convergeRelease`, `checkPublishedSums`. `smoke.go`: orchestration and `checkAssets`. `smoke_channels.go`: the per-channel smoke checks (`:392-486`). Add `cmd/release/doc.go`. | A-11 supersedes this with per-channel files. Prefer A-11. |
| **P2-14** | `internal/cli/output.go` and the top of `errors.go` → **new package `internal/cli/output`** (A-14) | Result, KVRecord, Envelope, Renderer, Error, Code, the exit codes, `newRenderer`, `storeFailure`. | This is a package move, so it touches every command file's references, and exported names may need capitalising. It is still behaviour-preserving. Do it after P2-07 and P2-08. |

### 5.3 Package-level changes, and the Go constraints they must respect

| New or changed package | From | Import rule |
|---|---|---|
| `internal/nodefile` (P2-03, A-05) | `tree` `atomic.go`, `read.go`, `syncdir`, `edit.ETag` | A leaf: it imports nothing from `internal/`. `tree` imports it, and `lock` and `store` import it **instead of** `tree`. |
| `internal/cli/output` (P2-14, A-14) | `cli/output.go` and part of `errors.go` | It may import `elsewhere`, and later `nodestore`/`tree` for the error mapping. It must **never** import `internal/cli`. |
| `internal/hooks` (A-09) | `cli/init.go` editors | Imports `nodefile` for atomic writes. It must not import `cli` or `agents`, so `agents` stays small. |
| `internal/diagnose` (A-06, optional) | the problem kinds and mappings in `check` and `index` | Imports `tree`, `config`, `issue` and `knowledge`. It must **not** import `index`, because the ADR rule says `check` never reads the index. `index` and `check` both import `diagnose`. |
| `config` thresholds (A-07) | `cli`, `check` | `config` stays a leaf, imported by 18 files in 7 packages. It must not import `issue` or `knowledge`. |
| Per-channel files in `cmd/release` (A-11) | `probes.go`, `publish.go`, `render.go`, `smoke.go`, `channels.go` | It stays in `package main`. ADR 0008: `cmd/release` imports nothing from `internal/`. |

The **runCLI harness must keep working.** Every command stays in package `cli`, so `internal/cli/testharness_test.go` (`runCLI`, `runCLIWith`) and the roughly 7,700 test lines run unchanged. CLI tests never call `t.Parallel()` (the `cli-test-harness` Note). A white-box test that moves with `cli/output` (`newRenderer`, `storeFailure`) becomes a test in the new package.

### 5.4 Decision for the user: split `internal/cli` into `issuecmd` and `knowcmd` subpackages?

**Recommendation: no.** Split out `cli/output` only (A-14), move domain logic down (A-01, A-03, A-04, A-09), and keep the commands in one package.

The argument from the cli exploration:

- Most of the 6032 lines are **domain logic in the wrong layer**, not a layout problem. A-01 (lifecycle), A-03 (post-write effects), A-09 (init editors) and A-08 (walkers) move several hundred lines out of `cli` into the modules that own the rules.
- A subpackage split would have to export `app` (store, index, renderer, `indexWarnings`, `emit`), `Result`, `Error`, `kindFailure`, `resolveFailure`, `editNode` and every flag constant. The new **interface would be nearly as wide as the implementation**: shallow modules with no depth gained.
- A-02 argues the opposite way: Issue and Note commands should **share** code (generic `nodeVerbs[T]`). Separating them into two packages would push them further apart.
- `runCLI` and its test suite work best with one command package.

**Alternative:** split into `issuecmd` and `knowcmd` anyway, accepting the wide exported surface, purely to shrink the per-package file count. This is not recommended, and it should be revisited only after A-01 to A-04 and A-08 to A-09 have landed and `cli` has actually shrunk.

### 5.5 Pure moves against deepening refactors

| Kind | Items | Behaviour | Review effort |
|---|---|---|---|
| **Pure moves** | P2-01 to P2-14 | Identical. The tests are unchanged. | Low: check that the diff is move-only (`git diff --color-moved=zebra`). |
| **Deepening refactors** | A-01 to A-15 | Identical by intent, with interfaces changed. The tests move to the new interface, and several defects are fixed on the way (section 6, Phase 3). | Medium to high. Characterisation tests come first. |

---

## 6. Recommended implementation order

### 6.1 Constraints for whoever implements

- **Host: a Raspberry Pi 5 with little memory.**
  - Run one package or one test at a time, as `GOMEMLIMIT=2GiB go test ./internal/<pkg>/ [-run <Test>]`.
  - **Never** run `go test -race ./...`. Race-test single packages only.
  - **Never** run two test commands at once.
  - `go vet ./...` is fine. CI runs `go vet ./...` and `go test ./...` on Linux, macOS and Windows, so Windows-only code (`oslock_windows.go`, `syncdir_windows.go`, the FIFO tests) is covered there.
- **Git:**
  - Commit straight to `main`, with no feature branches or pull requests (implement-plan's local run branch fast-forwarded to `main` is fine).
  - One commit per item. Pure moves use `refactor(...)`, fixes `fix(...)`, tests `test(...)`.
  - No AI attribution anywhere: no co-author trailers, no "generated with" footers, not in commits, code or docs.
  - Never tag, bump `VERSION` or edit a CHANGELOG release heading by hand (Note `release-process`).
- **Tracking:**
  - Every item or cluster becomes a capcom issue (`capcom create --id <slug> --title <title> [--dep <id>]`, section 7). Use `capcom dep add` for the orderings below, and `capcom close <id> --reason …` when an item is verified.
  - Read the Notes `issue-workflow`, `coding-conventions`, `cli-test-harness` and `store-layout` (`capcom know show <id>`) before starting.
- **Bite check:** every `fix(...)` item's new test must fail against the tree as it was before the item.
- **Standing skill:** `coding-standards`.

### 6.2 Phases

Test items that pin code an earlier phase rewrites are **pulled forward** as characterisation tests. The rest stay in Phase 4.

#### Phase 0: High security, then the other hostile-repository reads (quick, local, no refactor needed)

| Step | Item | Depends on | Notes |
|---|---|---|---|
| 0.1 | T-02: `Compose` panic-recovery test | (none) | Pins the "prime never fails" guarantee before the reads change. |
| 0.2 | F-02 shared guarded reader, plus F-02(e) (`tree/read.go`) | (none) | Adds the guarded reader beside `ReadNodeFile`. `ReadNodeFile` wraps it. |
| 0.3 | **F-01** and **F-02(a)** (`AGENTS.md` containment and guarded read) | 0.2 | Two commits, or one `fix(agents)`. These are the High items. |
| 0.4 | T-09, then **F-02(b)** (`config.yaml` read) | 0.2 | T-09 first pins `store.go`'s error branches. |
| 0.5 | **F-02(c)** (Ref `path#symbol` read) | 0.2 | |
| 0.6 | T-07, then **F-02(d)** (branch-blob size cap) | (none) | T-07 first pins the Elsewhere scan's warnings. |
| 0.7 | **F-07** (terminal control sequences) | (none) | Resolve the design call first. Recommended: a visible escape. |

#### Phase 1: correctness and concurrency fixes that do not depend on a refactor

| Step | Item | Depends on | Notes |
|---|---|---|---|
| 1.1 | **F-05** (`show` and `know show` from one read) | (none) | Medium |
| 1.2 | **F-06** (dangling `dep rm` and `related rm`) | (none) | Medium |
| 1.3 | **F-08** (multi-document YAML) | (none) | Medium. Covers config and frontmatter. |
| 1.4 | T-04, then **F-09** (claims of left-out files) | T-04 | Medium. Index schema bump. |
| 1.5 | **F-10** (duplicate-id subtree, Aliases, block) | (none) | Medium. Resolve the "drop both" design call first. |
| 1.6 | **F-11(a)** (compare-and-swap in `tree.Plan.AddRewrite` and undo, plus `Format`) | (none) | Medium. Resolve the F-11 scope design call first. |
| 1.7 | **F-11(b)**, **F-11(c)** (refuse-missing replace, no-replace rename) | (none) | Low. They live in `tree/atomic.go` and `tree/plan.go`. |
| 1.8 | **F-03**, **F-04** (release: no downgrade; draft probe) | (none) | Medium |
| 1.9 | **F-12** (gate binds the SHA; CI half kept when the tag exists) | (none) | Low. Resolve the ADR 0008 design call first. |
| 1.10 | **F-13** (SHA256SUMS check), **F-15** (tagger and VERSION shape) | F-15 needs its design call | Low |
| 1.11 | **F-18**, **F-19** + T-08, **F-20**, **F-21**, **F-22**, **F-25** | F-20 after T-04 | Low. Each is independent and small. |
| 1.12 | **U-09** (`readylist` alias in `cli/ready.go`) | (none) | Low, cosmetic. Do it with the Phase 2 moves. |

**Deferred to Phase 3, where a refactor absorbs them** (do not fix twice):

| Fix | Absorbed by |
|---|---|
| F-11(d) close re-check | A-01 |
| F-16, F-17 | A-07 |
| F-23, F-24, U-02 | A-04 |
| F-14 | A-12 |

If a user declines one of those refactors, its absorbed fixes move back into Phase 1 as standalone items. Each item's Fix line above already works without the refactor.

#### Phase 2: pure file splits (mechanical, behaviour-preserving, one commit each)

Recommended order (`→` means "before"):

1. `P2-01 (tree.go) → P2-03 (nodefile package)`
2. `P2-02 (nodestore/create.go)`
3. `P2-04 (check.go)`, `P2-05 (config.go)`, `P2-06 (index/refresh.go)`
4. `P2-07 (cli flags.go) → P2-08 (know.go) → P2-09 (close.go) → P2-10 (dep.go) → P2-11 (node_edit.go) → P2-14 (cli/output package)`
5. `P2-12` only if A-09 is declined. `P2-13` only if A-11 is deferred.

Each P2 item's acceptance:

- `go vet ./...`
- the package's own tests, one package at a time with `GOMEMLIMIT=2GiB`
- for P2-03 and P2-14, every importing package's tests, one at a time
- a move-only diff (`git diff --color-moved=zebra` shows no changed lines beyond package and import headers)

#### Phase 3: deepening refactors, with the fixes they absorb

| Step | Candidate | Depends on | Absorbs |
|---|---|---|---|
| 3.1 | **A-07** Store format owned by `config` | P2-04, P2-05 | F-16 (resolve the `init` design call first), F-17 |
| 3.2 | **A-14** `cli/output` (sanitiser and exit codes live there) | P2-14 | (none) |
| 3.3 | **A-05** Node-file layout and `Tree.Claimant` | P2-01, P2-02, P2-03, F-10, F-11(b), F-11(c) | Generalises F-10(b) into `Claimant` |
| 3.4 | **A-04** Reference-rewrite engine | P2-02, F-11(a) (keep its test green); A-05 recommended | F-23, F-24, U-02 |
| 3.5 | **A-03** Post-write effects in `nodestore` | A-07; A-04 recommended | (none) |
| 3.6 | **A-01** Issue lifecycle into `issue` | P2-09, P2-11 | F-11(d) close |
| 3.7 | **A-02** Generic Tree-node verbs | A-01, A-03, P2-07, P2-11 | (none). Keep `--kv` off the Note verbs (`kv-flag-scope`). |
| 3.8 | **A-08** Outline and progress walker | P2-07, P2-08 | (none) |
| 3.9 | **A-09** `internal/hooks` | P2-03 | (none). It removes the need for P2-12. |
| 3.10 | **A-06** Shared diagnostics | F-09, F-10, P2-04, P2-06 | (none). Any `check` JSON code change needs a CHANGELOG note through the release flow, never a hand-edited heading. |
| 3.11 | T-01 (characterisation), then **A-11** release step table and per-channel files | F-03, F-04, F-12, F-13 | (none). It removes the need for P2-13. Add `cmd/release/doc.go`. |
| 3.12 | **A-12** Channel-repository seam | A-11 | F-14. T-01 is rewritten onto `memChannelRepos`. |
| 3.13 | A-10, A-13, A-15 | after the rest | (none). Optional, speculative. |

#### Phase 4: the remaining test gaps

These test gaps are not pulled forward: **T-03**, **T-05** and **T-06**. T-08 is already done with F-19. Any of them can be done at any time, and they are independent.

### 6.3 Dependency summary (for `capcom dep add`)

```
F-02(shared) -> F-01, F-02(a), F-02(b), F-02(c)
T-09 -> F-02(b) ; T-07 -> F-02(d), F-18 ; T-04 -> F-09, F-20 ; T-02 -> F-02(a)
P2-01 -> P2-03 -> A-05 ; P2-02 -> A-04, A-05 ; F-10, F-11(b), F-11(c) -> A-05
F-11(a) -> A-04 ; A-07 -> A-03 ; A-04 -> A-03 (recommended)
P2-04, P2-05 -> A-07 ; P2-04, P2-06, F-09, F-10 -> A-06
P2-07 -> P2-08 -> P2-14 -> A-14 ; P2-09, P2-11 -> A-01 -> A-02 ; A-03 -> A-02
F-03, F-04, F-12, F-13, T-01 -> A-11 -> A-12
```

---

## 7. Suggested capcom issues

These are not yet filed. Every id is kebab-case, 1-4 words of `a-z0-9`, at most 40 characters, and none collides with an existing id under `.capcom/issues/`. An optional parent, `code-review-1013` ("Code and architecture review 2026-10-13"), can hold them all, filed under it with `--parent`.

### Defects

| Item | Suggested id | Title | `--dep` |
|---|---|---|---|
| F-02 (shared, plus e) | `guarded-file-reader` | One guarded reader for repo-controlled files (regular file, size cap, no follow) | (none) |
| F-01 | `agents-symlink-escape` | AGENTS.md symlink is followed outside the worktree on write | `guarded-file-reader` |
| F-02(a) | `agents-guide-read-guard` | AGENTS.md read has no regular-file check or size cap; prime can hang or OOM | `guarded-file-reader` |
| F-02(b) | `config-read-guard` | config.yaml read through symlinks with no size cap | `guarded-file-reader`, `store-init-error-tests` |
| F-02(c) | `ref-symbol-read-guard` | Ref path#symbol check reads FIFOs and huge files | `guarded-file-reader` |
| F-02(d) | `branch-blob-size-cap` | Elsewhere scan reads other-branch blobs with no size cap | `elsewhere-failure-tests` |
| F-03 | `release-no-downgrade` | publish rerun for an older version downgrades the tap and the bucket | (none) |
| F-04 | `release-draft-probe` | A draft release left by an interrupted create is probed as done | (none) |
| F-05 | `show-etag-snapshot` | show and know show pair index content with a later disk etag | (none) |
| F-06 | `dangling-ref-removal` | dep rm and related rm prefix-resolve a dangling entry before exact text | (none) |
| F-07 | `terminal-control-escape` | Text output passes terminal control sequences from repository content | (none) |
| F-08 | `yaml-multi-document` | A second YAML document in front matter or config.yaml is silently ignored | (none) |
| F-09 | `left-out-name-claims` | Names claimed by files left out of the index fall through to prefix matching | `refresh-race-tests` |
| F-10 | `duplicate-id-subtree` | A repeated id leaks the dropped file's subtree and Aliases | (none) |
| F-11(a) | `stale-rewrite-cas` | Move, remove and fmt overwrite other Nodes' files from a stale snapshot | (none) |
| F-11(b) | `replace-missing-target` | A replace-mode write recreates a Node another session moved or removed | (none) |
| F-11(c) | `rename-no-replace` | Node rename can replace a file created a moment earlier | (none) |
| F-11(d) | `dep-close-race` | dep add and close decide on the index snapshot outside the guarded edit | `issue-lifecycle-module` |
| F-12 | `release-gate-binding` | Release gate is skipped once the tag exists, and the push uses symbolic HEAD | (none) |
| F-13 | `release-sums-check` | Rebuilt SHA256SUMS uploaded unchecked beside existing archives | (none) |
| F-14 | `winget-probe-search` | Winget done-probe relies on the lagging GitHub search index | `channel-repo-seam` (or none) |
| F-15 | `tagger-silent-pass` | version-bump.sh passes green without tagging (fallback range, v prefix) | (none) |
| F-16 | `missing-config-format` | Missing config.yaml treated as format 3; init re-run raises the format | `config-owns-format` |
| F-17 | `check-related-format` | check has no store-format gate for the related key | `config-owns-format` |
| F-18 | `elsewhere-days-zero` | elsewhere.days 0 still scans future-dated branches | `elsewhere-failure-tests` |
| F-19 | `hascommit-fatal-exit` | HasCommit reports a fatal git failure as no commit | (none) |
| F-20 | `index-read-snapshot` | Index reads after Open do not come from one snapshot | `refresh-race-tests` |
| F-21 | `walk-vanished-dir` | A directory vanishing during the walk fails the whole refresh | (none) |
| F-22 | `log-fence-aware` | Log section lookup ignores fenced code blocks | (none) |
| F-23 | `note-self-supersede` | Renaming a Note to its own successor makes it invalid | `reference-rewrite-engine` (or none) |
| F-24 | `know-rm-report-drops` | know rm does not report the superseded-by entries it removed | `reference-rewrite-engine` (or none) |
| F-25 | `move-dir-cleanup` | Move removes the emptied old directory as a hard, rolling-back step | (none) |

### Defects from the architecture exploration (verified)

| Item | Suggested id | Title | `--dep` |
|---|---|---|---|
| U-02 | `rename-dup-deps` | Issue rename leaves the new id twice in deps or related | `reference-rewrite-engine` (or none) |
| U-09 | `ready-import-alias` | cli/ready.go imports internal/briefing under the alias readylist | `split-large-files` (or none) |

U-01 and U-03 to U-08 were refuted (Appendix A) and get no issue.

### Test gaps

| Item | Suggested id | Title | `--dep` |
|---|---|---|---|
| T-01 | `channel-push-failure-test` | No test for tap or bucket clone, commit and push failures in publish | (none) |
| T-02 | `briefing-panic-test` | Compose panic recovery (SectionInternal) has no test | (none) |
| T-03 | `briefing-failure-order-test` | Briefing ready, doing, check and Elsewhere failure branches untested | `briefing-panic-test` |
| T-04 | `refresh-race-tests` | Index refresh "changed under the walk" branches untested | (none) |
| T-05 | `lock-touch-tests` | Lock Touch with a foreign token untested; heartbeat assertion too weak | (none) |
| T-06 | `merge-write-failure-test` | Merge driver write failure and ReplaceFileVerbatim missing path untested | (none) |
| T-07 | `elsewhere-failure-tests` | Elsewhere scan git-failure warnings and file-name filter untested | (none) |
| T-08 | `isancestor-error-test` | IsAncestor error path untested | (none) |
| T-09 | `store-init-error-tests` | Store Init and RaiseFormat error branches untested | (none) |

### Pure moves (Phase 2)

| Item | Suggested id | Title | `--dep` |
|---|---|---|---|
| P2 parent | `split-large-files` | Split the large source files (pure moves) | (none) |
| P2-01 | `split-tree-go` | Split tree/tree.go into errors, ids, walk, claims | (none) |
| P2-02 | `split-nodestore-create` | Split nodestore/create.go into create, load, claim, write | (none) |
| P2-03 | `nodefile-package` | Move atomic writes and guarded reads from tree to internal/nodefile | `split-tree-go`, `guarded-file-reader` |
| P2-04 | `split-check-go` | Split check/check.go into problem, issues, notes, agents | (none) |
| P2-05 | `split-config-go` | Split config/config.go into config, parse, format | (none) |
| P2-06 | `split-index-refresh` | Split index/refresh.go into refresh, problems, duplicates | (none) |
| P2-07 | `cli-flags-file` | Collect cli flag constants in flags.go | (none) |
| P2-08 | `split-know-go` | Split cli/know.go per subcommand | `cli-flags-file` |
| P2-09 | `split-close-go` | Split reopen out of cli/close.go | (none) |
| P2-10 | `split-dep-go` | Split cli/dep.go into dep, dep_list, dep_graph | (none) |
| P2-11 | `node-edit-helpers` | Move shared edit helpers out of cli/update.go | (none) |
| P2-12 | `split-init-go` | Split cli/init.go editors (only if hooks-package is declined) | (none) |
| P2-13 | `split-release-files` | Split cmd/release smoke.go and publish.go (only if A-11 is deferred) | (none) |
| P2-14 / A-14 | `cli-output-package` | Extract internal/cli/output | `split-know-go`, `cli-flags-file` |

### Deepening refactors (Phase 3)

| Item | Suggested id | Title | `--dep` |
|---|---|---|---|
| A-01 | `issue-lifecycle-module` | Move close, reopen and status rules into the issue package | `split-close-go`, `node-edit-helpers` |
| A-02 | `generic-node-verbs` | One generic mv, rm and create module for both Tree kinds | `issue-lifecycle-module`, `nodestore-post-write` |
| A-03 | `nodestore-post-write` | Format raise and AGENTS.md re-sync enforced by nodestore | `config-owns-format` |
| A-04 | `reference-rewrite-engine` | One reference-rewrite engine in nodestore | `split-nodestore-create`, `stale-rewrite-cas` |
| A-05 | `node-layout-claims` | tree owns the node-file layout and Tree.Claimant | `nodefile-package`, `duplicate-id-subtree` |
| A-06 | `shared-diagnostics` | One per-file diagnostics module for index and check | `left-out-name-claims`, `split-check-go`, `split-index-refresh` |
| A-07 | `config-owns-format` | Store format and per-key thresholds owned by config | `split-config-go`, `split-check-go` |
| A-08 | `outline-walker` | One outline and progress walker for list and know tree | `split-know-go` |
| A-09 | `hooks-package` | Move init's settings and .gitattributes editors to internal/hooks | `nodefile-package` |
| A-10 | `store-entry-points` | check, elsewhere and lock take the store, not cwd | (none) |
| A-11 | `release-step-table` | cmd/release: one step table and one file per channel | `release-no-downgrade`, `release-draft-probe`, `release-gate-binding`, `channel-push-failure-test` |
| A-12 | `channel-repo-seam` | cmd/release: seam for tap, bucket and winget repositories | `release-step-table` |
| A-13 | `frontmatter-schema` | Generic frontmatter field table (speculative) | `yaml-multi-document` |
| A-15 | `trim-shallow-interfaces` | Unexport index, agents and tree symbols used only by tests | (none) |

---

## Appendix A: Refuted and dropped claims

Seven audit claims were refuted by independent verification and are excluded from this document. One line each, with the reason:

| Claim | Location | Claim in brief | Why it was refuted |
|---|---|---|---|
| #30 | `internal/gitx/branches.go:141` | `HasCommit`'s true and missing-revision paths are untested. | About ten Elsewhere Scan tests require entries and no warnings, so an always-false regression fails them, and the unborn-HEAD false case is pinned. Only the non-git-exit error path is unpinned, which is folded into F-19 and T-08. |
| #38 | `internal/index/refresh.go:253-254` | A permission-denied read aborts the whole refresh, breaking "a bad file never fails a command". | This is documented, deliberate design: Refresh "fails only when the index or the tree cannot be read", and format.md defines a bad file by its parse or validation problems, not by I/O failure. git records only 644 and 755 modes, so a hostile repository cannot plant an unreadable file. |
| #44 | `internal/lock/lock.go:182-185` | Acquire, Touch, Release, Check and Read error on a corrupt record, against format.md:596. | format.md:596-597 names exactly what happens to a corrupt record, and the 2026-10-12 fix plan, item 13, kept `Release`/`Acquire` unchanged on purpose. `TestRead_CorruptRecord_FailsNamingTheFile` pins it. At most the wording is loose. |
| #54 | `internal/tree/atomic.go:45` | `ReplaceFileVerbatim` and `WriteFileAtomicVerbatim` have no tests. | CRLF and mode are pinned in the tree package (`remove_test.go:141`, `move_test.go:238`) and through callers (`init_test.go:674`, `store_test.go:204`, `merge_driver_test.go:132`). The failing-reader cleanup is shared and tested (`tree_test.go:855`). The remaining missing-path branch is folded into T-06. |
| #55 | `internal/tree/atomic.go:78` | `WriteFileExclusive` failure paths are untested. | The link fallback is tested (`atomic_test.go:55, 78`, via the `SetLink` seam) and the failing reader is shared code tested at `tree_test.go:855`. Only `copyExclusive`'s mid-copy cleanup is untested, which needs a filesystem without hard links plus an I/O error. That is Low at most, and the closed issue `exclusive-copy-test` already covers the fallback. |
| #57 | `internal/tree/atomic.go:221` | `lfWriter`'s held-back CR branch is never exercised. | `TestWriteFileAtomic_CRLF_WrittenAsLF` (`tree_test.go:837-852`) feeds `io.MultiReader("a\r\nb\r", "\nlone\rcr\r")`, which exercises both the straddling CRLF and the trailing lone CR. |
| #62 | `internal/tree/resolve.go:64-77` | A unique prefix resolves silently to the kept duplicate, so writes act on it. | Every write caller passes the resolved path to `k.triage`, and `isBlockingProblem` blocks the duplicate problem, which names the kept file as `Other`. Index-based writes are pinned by `TestUpdate_DuplicatedID_…` (`cli/write_test.go:781`) and `TestClose_DuplicatedID_…` (`cli/close_test.go:163`). |

Seven of the nine `U-` items from the architecture exploration were refuted in the second pass:

| Item | Location | Claim in brief | Why it was refuted |
|---|---|---|---|
| U-01 | `internal/cli/related.go:71-83` | `related add` raises `config.yaml` to format 2 before the Issue is validated, so a parse-valid but invalid Issue leaves the store raised with no link. | The holder is resolved from an index refreshed at command start (`annotationIndex: indexRefresh`), and the index leaves out every Issue that fails `issue.Validate` (`index/refresh.go:279-281`, `:309`), so an invalid Issue cannot be resolved. The change only appends to `related`, which `issue.Validate` does not check, so the edited Issue validates whenever the original did. Only a concurrent edit between refresh and read could reach the raise, and the etag re-check after `change` gives the same raise-without-write in `know update` too; the format raise is monotonic and harmless (ADR 0009). |
| U-03 | `internal/nodestore/remove.go:218-226` | `removalApplyFailure` finds the failed step's path by searching the error text for `path+":"`, which is fragile. | No misattribution trigger: every step error starts with its absolute path and a colon (`tree/plan.go:89`, `tree/atomic.go:65-77`, `*fs.PathError`), the earliest match wins, and no removal path plus `:` is a substring of another's. `TestRemove_ApplyFails_NamesTheFailedFileRelativeToTheWorktree` pins it. A typed error is a robustness preference, kept as an option in A-04. |
| U-04 | `internal/check/check.go:217`, `internal/index/refresh.go:317-327` | A repeated Alias gets two JSON codes: `duplicate` from `check`, `repeated-alias` from the index. | The index's `WarningKind` never reaches a user: index warnings are rendered only through `Warning.String()` (`cli/root.go:216-218`), and the kind is used only internally to match tree problems (`cli/errors.go:168-185`, which maps `ErrRepeatedAlias` correctly). The only public code is `check`'s, pinned on purpose by `TestRun_RepeatedAliases_WordedByTheSharedAliasCheck` (`check_test.go:424-438`). The cited `check_test.go:382-388` is a cross-file duplicate, which both sides call `duplicate`. |
| U-05 | `internal/cli/mv.go:49`, `rm.go:60`, `create.go:76`; `know_mv.go`, `know_rm.go`, `know.go` | `know mv`, `rm` and `add` reject `--kv`, which the Issue verbs accept. | Deliberate: `kv-flag-scope` was fixed by registering `--kv` only on commands whose results carry KV records (`withKV`, `cli/root.go:171-177`; plan 2026-10-11-05 item 7). Note results carry none (`noteWriteResult`, `cli/know.go:373-379`), and plan 2026-10-11-03 dropped `--kv` for Notes on purpose. The rule is documented on `withKV`. |
| U-06 | `cmd/release/publish.go:171`, `:206`, `:237-244` | `publish` joins `publishSteps` to `publishActions` by position, so reordering one table silently mis-pairs a probe with its converge. | `probe` builds `steps` by walking `publishActions` and looking each probe up **by name** in `publishSteps` (`publish.go:240-241`), so `steps[i]` always belongs to `publishActions[i]` whatever order `publishSteps` has, and a missing name panics rather than mis-pairing. `resolveWingetPackage` also looks up by name (`channels.go:158`). The one positional assumption, `steps[0]` as the tag, is commented and only breaks if the tag stops being the first action, which would break the release order itself. |
| U-07 | `internal/cli/dep.go:105-151` | `dep add` has no `--if-match`, while `related add` has one. | No lost write: `dep add`'s edit appends to the list read from the file inside `Kind.Edit`, which re-checks the etag before writing (`nodestore/edit.go:59-77`), so a concurrent change is never overwritten. `dep rm` has no `--if-match` either, so the dep verbs are consistent with each other. A flag-parity wish, not a defect. |
| U-08 | `internal/cli/close.go:115`, `show.go:62` | `close` joins several failures with `failure.Error()` (with the path) and `show` with `failure.Message`. | `show`'s failures come only from `resolveFailure`, which builds them with `Errorf` and no `Path` (`show.go:167-176`), so `Message` equals `Error()` there. `close`'s failures can carry a path, and `Error()` keeps it in the joined message. Each is right for its inputs; the output does not differ. |

Nothing else was dropped. The usual ~20-finding cap and Medium floor were deliberately **not** applied, so every confirmed claim is reported. No placeholder findings were found.

## Appendix B: Machine-check results

**Toolchain:** Go 1.26.3 (linux/arm64), module `github.com/airiclenz/capcom`, 20 packages.

| Check | Result |
|---|---|
| `go vet ./...` on linux, plus `GOOS=windows` and `GOOS=darwin` | clean |
| `go test -count=1 -cover ./...` | all pass |
| `go test -race -count=1`, package by package, all 18 packages with tests | no data races, all pass |
| Tool-confirmed findings | none |

**Coverage** (each package's own tests):

| Package | Coverage | Files below 80% |
|---|---|---|
| `cmd/release` | 88.3% | `channels.go` 77.0%, `dist.go` 79.3% |
| `cmd/capcom` | 0.0% | a one-line `main.go`, exercised via `cli.Run` |
| root `capcom` | 0.0% | `version.go`: `Version()` has no direct test |
| `internal/agents` | 92.4% | |
| `internal/briefing` | 88.0% | |
| `internal/check` | 87.9% | |
| `internal/cli` | 93.6% | |
| `internal/config` | 95.6% | |
| `internal/edit` | 100% | |
| `internal/elsewhere` | 83.7% | |
| `internal/frontmatter` | 96.8% | |
| `internal/gitx` | 85.5% | |
| `internal/index` | 83.5% | `walk.go` 74.1%, `schema.go` 79.2% |
| `internal/issue` | 95.9% | |
| `internal/knowledge` | 92.8% | |
| `internal/lock` | 87.6% | `oslock_windows.go` not run on this host |
| `internal/nodestore` | 89.1% | |
| `internal/rawfm` | 100% | |
| `internal/store` | 81.7% | |
| `internal/tree` | 88.9% | `atomic.go` 77.6%, `syncdir_other.go` 75.0% |

**Leads from coverage** (no lens confirmed these, so they are not findings):

- `internal/index/refresh.go:118` `databaseError` never executes in tests, so the path for DB failures is untested.
- `internal/tree/tree.go:550` `(*Tree).Dir` has no caller outside tests (see A-15).
- `cmd/release/channels.go` (77%) and `dist.go` (79.3%) are partly untested (see T-01).
- Windows-only code (`oslock_windows.go`, `syncdir_windows.go`) is vetted under `GOOS=windows`. CI runs it on Windows.

**Not run, because the tools are not installed:** golangci-lint (the repo has no `.golangci*` config either), staticcheck, govulncheck (so **no vulnerability scan** was performed), gosec, shellcheck (so `.github/scripts/version-bump.sh` was not linted) and actionlint (so `.github/workflows/*.yml` were not linted). Whole-module `go test -race ./...` was deliberately not run, because of the host constraint.

**Recommendation:**

- Install `govulncheck` and `staticcheck`, and preferably `shellcheck` and `actionlint`, and run them once before the next release.
- Consider adding `govulncheck` and `staticcheck` steps to `.github/workflows/ci.yml`, where memory is not constrained. A `shellcheck` step there would also cover `version-bump.sh`, which F-15 changes.

## Appendix C: Run metadata

- **Audit run directory:** `docs/skill-runs/code-audit/2026-10-13/`. It holds `bundle.md`, `tools.md`, `ledger.md`, the `findings-*-*.md` files (79 lens reports), the `merged-*.md` files (17 group rollups, with `.claude` and the root empty), `verify-list.txt` (63 claims) and `verify-1.md` to `verify-21.md` (verdicts: 56 confirmed, 7 refuted, 0 unclear).
- **Architecture exploration:** three explorer reports, covering `internal/cli`, the domain and storage packages, and the infrastructure and release packages. They are summarised in section 4 and were not kept in the repository. Their nine `U-` defects were verified in a second pass against `d192883` (2 confirmed, 7 refuted), by reading the code without running tests.
- **Previous audit:** `docs/reviews/archived/code-audit-2026-10-12.md`. Residues of it are flagged in F-02, F-09 and F-10, and a regression from its fix plan in F-01.
- **Base commit:** `d192883` (`chore(version): bump version to v0.5.0`).
