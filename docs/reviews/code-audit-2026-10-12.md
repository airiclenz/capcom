# Code Review — whole repository — 2026-10-12

**Scope:** the whole capcom repository: `cmd/`, `internal/`, `.github/`, `.claude/` and the root files (Go sources, tests, CI and tag workflows, configuration).
**Mission:** capcom is a single-binary Go CLI that is a git-native issue tracker and knowledge store for AI coding agents, with Issues and Notes as markdown files under `.capcom/`, a disposable SQLite index, an advisory Tree lock and a separate release dev tool.
**Files reviewed:** 192

## Executive Summary

The most important finding is that a symlink or other non-regular file under `.capcom/` is accepted as a node, stat'd as a link but read through, so a hostile cloned repo can make every reading command and the `prime` session hook hang, exhaust memory or fail. Overall the code is healthy: `go vet` is clean, all tests pass, the race detector found nothing on the packages it ran, and no Critical or High defects were found. The recurring theme is that the write path and the read path disagree about edge cases (duplicate ids, prefix resolution, unreadable files, atomic-write durability), so mutating commands can act on things the reading commands treat as absent. The remaining findings are a handful of narrow correctness defects and a few untested critical paths.

## Intent & Architecture Findings

### Medium — `tree.Load` keeps the first of a duplicate id while the index drops both `[Intent + Correctness]`

- **Where:** `internal/tree/tree.go:295-313` (`indexIDs`), `internal/tree/tree.go:394-414` (`indexAlias`), compare `internal/index/refresh.go:385`
- **What:** `docs/format.md` ("Errors") and the index say two files answering to the same id or alias are both left out. `tree.Load` keeps the lexicographically first file and keeps the dropped duplicate's subtree attached to the kept node. Nothing in nodestore, issue or knowledge checks for a duplicate.
- **Why it matters:** After two branches each add `foo` and are merged, `capcom show foo` and `ready` treat `foo` as absent, while `rm`, `mv` and `know rm/mv` act on one of the two files. `PlanRemove` counts children that are not in the kept node's directory, `PathFor` places new children under the wrong file, and `agents.TopNotes` renders a Managed-block entry for a Note the index calls a duplicate.
- **Fix:** Make `Load` drop every file of a repeated id or alias, and their subtrees, reporting them like orphan subtrees, or have the mutating commands refuse an id with a duplicate warning. Candidate for `/improve-codebase-architecture`: `Load` and `index/refresh.go` each implement duplicate detection with different rules.

### Medium — Managed-block rewrite of `AGENTS.md` is a diverged copy of the atomic write and severs symlinks `[Intent + Correctness]`

- **Where:** `internal/agents/block.go:293-350` (`replaceFile`)
- **What:** `replaceFile` duplicates `tree.WriteFileAtomicVerbatim` and lacks its `temp.Sync()` before the rename and the `syncDir` after (`internal/tree/atomic.go:73-80`). It also renames the temp file over the path, so when `AGENTS.md` is a symlink (for example `AGENTS.md -> CLAUDE.md`) the link is replaced by a regular file. Its permission-preserving and failure branches have no test.
- **Why it matters:** `capcom agents` or a top-level Note write silently severs the link and leaves the target stale. After a crash or power loss shortly after the rename, the file can come back empty or truncated, destroying hand-written bytes outside the markers that the package promises to keep. *(independently verified)*
- **Fix:** Delete `replaceFile` and call `tree.WriteFileAtomicVerbatim` on `filepath.EvalSymlinks(path)` (the resolved target), using the stat'd mode or `0o644` when the file is missing. Add tests that a `0o600` guide keeps its mode, that a symlinked `AGENTS.md` stays a link, and that a read-only directory returns an error with the original bytes intact and no `.AGENTS.md.tmp-*` litter.

### Medium — Symlinked or other non-regular node files are stat'd as links but read through `[Intent + Correctness + Security]`

- **Where:** `internal/tree/tree.go:258`, `internal/index/walk.go:36` and `:489`, `internal/index/refresh.go:232-239`, `internal/check/check.go:218`, `internal/cli/update.go:185` and `:249`
- **What:** `tree.Walk` skips only real directories, `walkIssues` only `Lstat`s each file, and every reader calls `os.ReadFile`, which follows the link with no size cap or regular-file check. The recorded size and mtime are the link's own, so editing the target never marks the file dirty. A directory-valued entry makes `indexFile` return a hard error and abort the whole refresh instead of warning.
- **Why it matters:** A hostile repo or branch can commit `.capcom/issues/x.md -> /dev/zero` (unbounded read), a FIFO or tty (blocks forever) or a directory (hard failure). Every reading command, including the `prime` session hook, then hangs, runs out of memory or fails until the file is removed, which breaks the "a bad file never fails a command" invariant. A legitimate symlink also serves stale rows forever.
- **Fix:** In `tree.walk` and `walkIssues`, skip anything where `!Type().IsRegular()` and warn at that path ("not a regular file"). Read store files through one helper that rejects non-regular files and caps the size with `io.LimitReader`. Add a test, skipped on Windows and as root, with an unreadable file and a symlinked node.

## Medium Findings

### Medium — Id resolution falls back to a unique prefix when the exact id is excluded `[Correctness]`

- **Where:** `internal/index/query.go:141-168` (`resolveID`)
- **What:** When the exact id is absent from the queryable set (a duplicate, or an invalid file with no `issues` row), resolution falls through to unique-prefix matching.
- **Why it matters:** With a duplicate or invalid `foo` and a sibling `foo-bar`, `capcom close foo`, `rm foo` or `update foo` mutates `foo-bar`, with only a warning printed. *(independently verified)*
- **Fix:** When no exact or alias match exists, check the file and duplicate state for the normalised name and return a not-found or duplicate error. Never prefix-match for write commands.

### Medium — `release prepare` judges "above the highest tag" from stale local tags `[Correctness]`

- **Where:** `cmd/release/prepare.go:107`
- **What:** `checkPrepare` uses `checkout.ReleaseTags` (local `git tag --list`) and never fetches. Only `checkGate` runs `FetchMain`, and the tag-on-version-bump workflow creates tags on origin only.
- **Why it matters:** Running `prepare v0.24.1` when origin already has that tag but the clone does not passes the check and makes two local commits for an already-released version, which publish then cannot ship.
- **Fix:** Call `checkout.FetchMain(ctx)` (or a tags-only fetch) at the start of `checkPrepare` and record a failure as a refusal like the other checks.

### Medium — `Create` and `Edit` check, then overwrite, with no exclusive write `[Intent & Structure]`

- **Where:** `internal/nodestore/create.go:62`, `internal/nodestore/edit.go:44`, `internal/tree/atomic.go:69`
- **What:** `Create` checks that the id is free and `Edit` checks `--if-match` against earlier bytes, then both write through a rename that replaces any existing file, with no exclusive create, re-check or OS lock.
- **Why it matters:** Two sessions creating the same id at once, or an edit landing between the etag check and the rename, silently overwrite the other write. That breaks the id-uniqueness and `--if-match` invariants. The window is small and the Tree lock is advisory. *(independently verified)*
- **Fix:** Create with an exclusive create (`O_EXCL` or link-style) and re-verify the etag immediately before the rename in `Edit`, or hold a short `.lck` OS lock around the read-modify-write.

### Medium — `mv --parent ""` silently moves a subtree to the top level `[Correctness]`

- **Where:** `internal/cli/mv.go:75` (`moveDestination`, shared with `know mv`)
- **What:** An explicitly empty `--parent ""` is folded into the same value as `--parent .` (`Parent = ""`, `IsReparented = true`), which `nodestore.resolveParent` treats as the root.
- **Why it matters:** A script or agent running `capcom mv child --parent "$PARENT"` with `PARENT` unset moves the issue and its whole subtree to the top level, rewriting ids and aliases.
- **Fix:** When `isReparented` and `strings.TrimSpace(parent) == ""`, return `UsageErrorf("--parent needs an id or %q", rootParentArg)`, and map only `rootParentArg` to the root.

### Medium — `elsewhere: days` overflows the cutoff arithmetic `[Correctness]`

- **Where:** `internal/elsewhere/elsewhere.go:224` (accepted at `internal/config/config.go:434-438`)
- **What:** `time.Duration(options.Days) * day` overflows int64 above about 106751 days, and config rejects only negatives.
- **Why it matters:** `elsewhere: days: 999999` ("scan every branch") wraps to a cutoff centuries in the future, so the scan silently reports nothing and `ready` and `prime` stop warning about unmerged issue changes.
- **Fix:** Cap `days` in `config.elsewhere` with an error above a sane bound such as 36500, and/or compute the cutoff saturating.

### Medium — One bad `ref` aborts `check` and `know verify` for every Note `[Intent + Correctness]`

- **Where:** `internal/knowledge/verify.go:119-125` (`checkRef`)
- **What:** Only ENOENT and ENOTDIR count as "missing". Any other `EvalSymlinks` failure (a symlink loop, ENAMETOOLONG, EACCES) returns a hard error that `internal/check/check.go:522` and `internal/cli/know.go:441` turn into a command failure.
- **Why it matters:** A single Note with `refs: [loop/x]` where `loop` links to itself stops `capcom check` and `know verify` for the whole store instead of yielding one Finding.
- **Fix:** Classify loop, name-too-long and permission errors as unresolvable and return a Finding ("cannot be resolved: <err>"). Reserve the error return for unexpected I/O failures or a failure of the worktree root.

### Medium — A corrupt lock record cannot be cleared with `break` `[Correctness]`

- **Where:** `internal/lock/lock.go:260-269` (also `242-253`, `173-177`, `302-336`; parse failures at `378-383`)
- **What:** `Break`, `Release` and `Acquire --take-stale` call `l.read` first, which fails on invalid JSON or an empty token, and `List` aborts on that one key.
- **Why it matters:** A hand-edited or zero-length record (after a disk-full or non-durable rename) leaves the explicit human escape hatch returning a parse error and hides every other worktree's lock, until the file is deleted by hand under the git dir.
- **Fix:** In `Break`, treat a read error as a corrupt record, remove it and return a Status noting it was discarded. Make `List` skip and report corrupt keys instead of failing.

### Medium — A failed index commit is reported with a misleading second error `[Correctness]`

- **Where:** `internal/index/refresh.go:75-77` (deferred rollback at `:51`)
- **What:** When `tx.Commit()` fails (for example SQLITE_BUSY past the 5s timeout under concurrent agents), the deferred `tx.Rollback()` still runs and joins `sql: transaction has already been committed or rolled back` into the returned error.
- **Why it matters:** The real cause is buried behind a spurious error that every reading command prints.
- **Fix:** Track whether Commit was attempted, or ignore `sql.ErrTxDone` from Rollback, so only the commit error is returned.

### Medium — The tag-on-version-bump script has no test `[Critical-Path Tests]`

- **Where:** `.github/scripts/version-bump.sh:17-40`
- **What:** Nothing exercises the script that decides which commits get a release tag. `memRepo.tagAfterPush` (`cmd/release/memrepo_test.go:101`) re-implements the decision in Go, so the shell logic is never run and the two can drift. Untested branches include the zero-sha and unreachable-`before` fallback, a burst with two bumps, a non-X.Y.Z VERSION and an unchanged VERSION.
- **Why it matters:** A wrong decision mis-tags or skips a release, and tagging is on the release critical path.
- **Fix:** Add a hermetic test (Go test shelling to bash in a `t.TempDir()` git repo with fixed author env, or a bats file). Assert `bumps=<sha>=v1.2.3` for one bump, two pairs oldest first for a double bump, empty output for a non-bump push, a stderr refusal for `v1.2`, and the fallback when `before` is the zero sha.

### Medium — The production process runner has 0% coverage `[Critical-Path Tests]`

- **Where:** `cmd/release/runner.go:28-47` (`execRunner.run`)
- **What:** Every release step depends on its contract: `env` overrides the inherited environment (the winget token reaches komac this way), `Dir` is honoured, stdout is returned, and a failure carries the stderr text. All other tests use `fakeRunner`.
- **Why it matters:** Dropping `os.Environ()`, ignoring `dir` or losing stderr would pass the suite and surface only during a real publish.
- **Fix:** Add a hermetic test that re-executes the test binary as the child (the `TestMain` helper pattern). Assert that env overrides an inherited variable, the child sees `dir` as its cwd, stdout is verbatim, a non-zero exit returns an error containing stderr, and a cancelled ctx kills the child.

### Medium — The `mv` guard against overwriting an untracked target is untested `[Critical-Path Tests]`

- **Where:** `internal/tree/plan.go:98` (`renameStep`, `ErrMoveTarget`)
- **What:** No test mentions `ErrMoveTarget`. The `os.Lstat` guard is the only thing stopping `mv` from overwriting a dotfile, a non-node file or a stray `<id>/` directory at the destination, because `os.Rename` replaces files on Unix.
- **Why it matters:** A regression would destroy data silently.
- **Fix:** Add `PlanMove` plus `Apply` tests that pre-create the target node file, and separately the new children directory. Assert `errors.Is(err, tree.ErrMoveTarget)`, that the target bytes are unchanged and that the source node is still in place.

### Medium — Concurrent index refresh has no test `[Critical-Path Tests]`

- **Where:** `internal/index/refresh.go:44`
- **What:** The design rests on `busy_timeout(5000)`, `_txlock=immediate` and WAL (`index.go:39`), but no test runs two `Open`/`Refresh` calls at once against one `index.db`.
- **Why it matters:** Cross-process races between agent sessions are the named threat model, and a regression in the SQLite setup would surface only in the field.
- **Fix:** Add a test that opens two Indexes on one store and refreshes both from goroutines while a file is edited. Assert neither returns SQLITE_BUSY, both Reports agree and no rows are doubled or half-written. Run it with `go test -race ./internal/index/`.

### Medium — The diverged local/remote-tracking branch pair is untested `[Critical-Path Tests]`

- **Where:** `internal/elsewhere/elsewhere.go:144-205` (`redundantRef`)
- **What:** Tests cover only the same-commit and behind cases (`elsewhere_test.go:288`, `:301`). When `x` and `origin/x` have each diverged, `redundantRef` must keep both.
- **Why it matters:** A regression that treats divergence as redundant would silently hide unmerged issue changes, which is the point of the Elsewhere scan.
- **Fix:** Add a test where local and remote each carry a distinct commit touching an issue file, and assert both `x` and `origin/x` entries appear.

## Recommended Action Order

1. Reject non-regular node files in `tree.walk`, `walkIssues` and the readers; it is the only finding a third party can trigger and it blocks session start.
2. Make duplicate handling consistent between `tree.Load` and the index (candidate for `/improve-codebase-architecture`), then remove the prefix fallback in `resolveID` for writes; these share the root cause that reads and writes disagree.
3. Replace `agents.replaceFile` with `tree.WriteFileAtomicVerbatim` on the resolved symlink target, adding its tests.
4. Quick wins: the `mv --parent ""` refusal, the `elsewhere.days` cap, the `checkRef` error classification, `FetchMain` in `checkPrepare`, the commit-error double report and the corrupt-lock `Break`.
5. Add exclusive create and etag re-check in `nodestore` (needs a small design decision on locking).
6. Close the critical-path test gaps: `version-bump.sh`, `execRunner`, `ErrMoveTarget`, concurrent refresh and the diverged branch pair.

## What Looked Good

`go vet` is clean and every package's tests pass, with coverage between 80% and 100% in nearly every package. The lock package, the frontmatter parser, `internal/gitx`, `internal/issue`, config parsing and the release tool's version handling and winget templates held up under review with no defects, and the tag workflow is least-privilege. The `internal/cli` surface is well covered through `runCLI`. These areas should be left alone.
