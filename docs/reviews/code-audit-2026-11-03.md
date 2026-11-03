# Code Review — whole repository (cmd/, internal/, workflows, plugin and build metadata) — 2026-11-03

**Scope:** every Go package under `cmd/` and `internal/`, plus the CI workflows, plugin manifests, hook config and demo assets (494 files). The project's own test run with coverage was consulted; linters and the vulnerability scanner were not installed and `-race` was skipped per the host memory rule.
**Mission:** capcom is a git-native issue tracker and knowledge store for coding agents, keeping issues and Notes as markdown files under `.capcom/` with a disposable SQLite index, shipped as one CLI plus a terminal UI.
**Files reviewed:** 494

## Executive Summary

The most important finding is that a hostile issue or Note body can crash the terminal UI: the markdown renderer trusts forged link markers embedded in body text and indexes a slice with a negative number. Everything else is Medium or below. The codebase is otherwise in good health: the store, lock, index, config, hooks and check packages match their documented contracts and are well tested. Two themes cut across the findings. First, untrusted repository text is not treated uniformly (the UI renderer and sidebar, symlinked children directories, config `plans` paths) even though the CLI text renderer already does it right. Second, several hostile-repository and error-path guards (recap, stage, clipboard, ui write path) are untested, so regressions would pass CI. No Critical findings were found.

## Intent & Architecture Findings

### Medium — Agent timeout kills only the agent process, not its children `[Intent & Structure]`

- **Where:** `cmd/agenteval/workspace.go:134` (with `cmd/agenteval/agent_unix.go:15`, `cmd/agenteval/agent.go:296`)
- **What:** The harness kills a timed-out agent with plain `exec.CommandContext` (SIGKILL to the one process, no process group), while the package already has `isolateProcessGroup` for exactly this purpose and applies it only to the `agent` verb's shell commands.
- **Why it matters:** `agenteval run -- agenteval agent ...` hitting the task timeout while a shell command runs leaves that shell child (in its own `Setpgid` group) alive for up to the 2-minute shell timeout, still with the shim first on PATH, inside a workspace that is being deleted. The same holds for any real agent (`claude -p` and so on) that spawns children; `WaitDelay` only stops the harness waiting.
- **Fix:** In `runAgent`, start the agent in its own process group and kill the whole group on cancel, reusing or generalising `isolateProcessGroup` (Windows stays as it is).

### Medium — Conflict scanner and fence-aware code mask disagree on what a code line is `[Intent & Structure + Correctness]`

- **Where:** `internal/rawfm/conflict.go:387`
- **What:** `ConflictMarkers` tracks fences with the streaming `CodeFence.IsCode`, so a fence opener that no later line closes (a stray "```" or a "~~~" typo) makes every following line count as code and skips all conflict markers after it. `ClosedCodeLines` (`fence.go:473/478`, used by `issue/mutate.go:102`) follows the opposite rule: a block left open hides nothing.
- **Why it matters:** A body with one unbalanced fence line and a later merge that leaves `<<<<<<<`/`=======`/`>>>>>>>` below it passes `capcom check` (`check/check.go:82`) and the frontmatter decoder (`frontmatter/decode.go:165`), so a file with conflict markers can be committed.
- **Fix:** Compute the code mask with `ClosedCodeLines(lines, bodyStart)` and skip only the lines it marks, so both consumers share one definition of a code line. Add a test with a body of "```\ntext\n<<<<<<< HEAD\nx\n=======\ny\n>>>>>>> b" run through `check`.

## Medium Findings

### Medium — Forged link marker in a body panics the terminal UI `[Intent & Structure + Correctness + Security + Critical-Path Tests]`

- **Where:** `internal/ui/markdown/links.go:184,254` (panic at `links.go:259`)
- **What:** The link-marker scheme trusts any `ESC _capcom-link:<n> ESC \` found in a rendered line, but issue and Note bodies reach `markdown.Render` raw (`internal/ui/detail.go:345`). `nextLinkMarker` accepts a negative index via `strconv.Atoi`, and `stripLinkMarkers` guards only `open == noLink` and `open >= len(targets)`, so `targets[-5]` panics. No `recover()` exists in `internal/ui`. A forged non-negative index also spoofs a followable link to an unrelated id.
- **Why it matters:** A hostile repository or PR commits a body containing `\x1b_capcom-link:-5\x1b\\x\x1b_capcom-link/\x1b\\` (also reproducible from a fenced block or heading); opening that issue or Note in `capcom ui`, or restoring it from `ui-state.yaml`, kills the TUI with an index-out-of-range panic, repeatedly. Crash only, no data loss. *(independently verified)*
- **Fix:** Reject `index < 0 || index >= len(targets)` in `nextLinkMarker`, `stripLinkMarkers` and `openLinkAtEnd`, treating it as a foreign marker, and strip ESC and other C0/APC control bytes (keeping tab and newline) from source lines at the top of `Render` so only the renderer can create markers. Add a `Render` test with a forged open marker (negative and out-of-range index) around visible text, asserting no panic and no reported Link.

### Medium — Winget probe trusts any pull request with a matching title `[Security]`

- **Where:** `cmd/release/winget.go:60` (feed at `cmd/release/channelrepos.go:166`)
- **What:** `probeWinget` counts any open winget-pkgs pull request whose title names `AiricLenz.Capcom` and the version as "submitted", and any title naming the package as a first submission in flight. `OpenPulls` is fed `gh pr list --search "<id> in:title"`, which is not filtered by author, and winget-pkgs accepts pull requests from anyone.
- **Why it matters:** Any GitHub user can open a PR titled "New version: AiricLenz.Capcom version 0.15.0" before the maintainer publishes; publish then skips the winget step as done (or parks it as setup-pending) and the real submission never happens, while the report shows a PR the maintainer never opened. Release integrity, not code execution.
- **Fix:** Accept only pull requests authored by the signed-in account: add `--author @me` to the search, or compare the author login from `--json author`.

### Medium — Symlinked children directory redirects `mv` and child creation outside the store `[Security]`

- **Where:** `internal/tree/move.go:151` (also `:163`, and `PathFor` at `internal/tree/tree.go:290`)
- **What:** `planRenames`/`PlanMove` use `os.Stat` (follows symlinks) on the new and children directories, and `PathFor` joins a children dir without `Lstat`. `Walk` reports a symlink `issues/foo` as a problem but keeps `issues/foo.md`.
- **Why it matters:** A committed symlink `issues/foo -> /elsewhere` beside `foo.md` makes `capcom mv bar --parent foo` (or creating a child of `foo`) skip the mkdir step and write `bar.md` through the link to an arbitrary directory; the node vanishes from the repo. Writes only the node's own file. *(independently verified)*
- **Fix:** `Lstat` the destination and children directories in `planRenames`, `PlanMove` and the create path, and refuse anything that is not a real directory.

### Medium — Config `plans` directories skip local-path validation `[Correctness]`

- **Where:** `internal/config/parse.go:116` (and `internal/config/config.go:271`)
- **What:** `plans` entries go through `nameList` only, so unlike `agents.file`/`agents.guides` they are never checked with `localPath`: `plans: [../../../etc, /srv/x, 'a\b']` is accepted although the documented contract is "relative to the repository root".
- **Why it matters:** `PlanPath` does `filepath.Join(root, dir, plan)` and `os.Stat`s outside the worktree, so a hostile repo's `config.yaml` makes `plan: <name>` resolve for files outside the repo, and `check`, `context` and `ready` report found or not-found and print a `../..` path.
- **Fix:** Validate each `plans` entry with `p.localPath` (slash-separated, `filepath.IsLocal`) in the `keyPlans` case, as `guides` does.

### Medium — Directory-sync failure reported as a failed write `[Correctness]`

- **Where:** `internal/nodefile/atomic.go:153` (and `:119` in `WriteFileExclusive`)
- **What:** A `syncDir` error after a successful rename or link is returned as a write failure although the new content is already in place.
- **Why it matters:** On a filesystem whose directory fsync fails (EINVAL/ENOTSUP on some network or FUSE mounts, or a transient EIO), `WriteFileAtomic` reports an error for a write that took effect, and a retry of `WriteFileExclusive` then fails with `fs.ErrExist`. *(independently verified)*
- **Fix:** Treat a directory-sync error as best effort (ignore EINVAL/ENOTSUP), or return a distinct "written, but not synced" error callers can tell apart.

### Medium — `ReplaceFileVerbatim` can resurrect a file removed between check and write `[Correctness]`

- **Where:** `internal/nodefile/atomic.go:81`
- **What:** The "fails when path is missing" guard is a bare `os.Stat` before `WriteFileAtomicVerbatim`, which calls `writeFileAtomic(..., isReplacing=false)`, runs `MkdirAll` and renames over a missing path.
- **Why it matters:** If another session moves or deletes the node file between the Stat and the rename, the old path is silently recreated, the exact case `ReplaceFileAtomic` prevents with `checkReplaceTarget`.
- **Fix:** Route the verbatim mode through `writeFileAtomic` with `isReplacing=true` so the missing-target check and no-mkdir rule apply.

### Medium — Control bytes in titles and lock records reach UI rows unsanitised `[Security]` (uncertain)

- **Where:** `internal/ui/closed.go:185` (also `knowledge.go:186`, `ready.go:213`, `detail.go:176`, `detail.go:294`, `statusbar.go:97-106`, `write.go:212`, `:215`)
- **What:** `oneLine` only joins `strings.Fields`, keeping ESC/C0/C1 bytes; titles, Note titles and lock Holder/Purpose go through it and `fit`/`ansi.Truncate` (`layout.go:635`), which preserves escape sequences.
- **Why it matters:** A hostile repo or a prompt-injected agent in a parallel worktree can plant an OSC/CSI sequence (title spoofing, OSC 52 clipboard write, forged status text or y/n prompt). Whether the terminal library neutralises non-SGR sequences is unsettled. Open question from the verifier: does bubbletea v2 forward or drop the raw sequences?
- **Fix:** Make `oneLine` (or a shared display sanitiser) drop control characters, e.g. `ansi.Strip` plus dropping runes where `unicode.IsControl` holds, and apply it to every repo-sourced or lock-record string drawn. A throwaway test rendering a model with a title containing `"\x1b]0;x\x07"` and inspecting `m.View().Content` settles whether it is exploitable.

### Medium — Heading renderer strips author's leading `#` from heading text `[Correctness]`

- **Where:** `internal/ui/markdown/markdown.go:327`
- **What:** `headingText` uses `TrimLeft(line, " #")`, which removes every leading space and `#` after the marker, not just the marker.
- **Why it matters:** A heading such as `## #42 follow-up` or `# #hashtag` renders as `42 follow-up` / `hashtag`, dropping the issue-number hash the author wrote.
- **Fix:** Count the marker (n `#` from `isHeading`), slice it off, then `TrimSpace` the rest.

### Medium — Recap's hostile-repository guards are untested `[Critical-Path Tests]`

- **Where:** `internal/recap/recap.go:361` (duplicate-id warning at `:191`)
- **What:** The oversized (over 1 MiB) blob guard (`gitx.ErrBlobTooLarge` becoming a warning) and the second-file-claims-a-seen-id warning have no test; `recap_test.go` has no "TooLarge" or "already used" case.
- **Why it matters:** These are the only defences when `capcom recap` reads a crafted history; a regression would pass the suite.
- **Fix:** Add `TestCompute` cases committing a file over 1 MiB and two files with one id (`a/x.md`, `b/x.md`), asserting a `recap:` warning, the file left out of the Report, and the rest of the recap intact.

### Medium — Staged-overlay size cap and directory refusal are untested `[Critical-Path Tests]`

- **Where:** `internal/stage/overlay.go:150`
- **What:** The 1 MiB cap on a staged file (`ReadRegularFile` returning `nodefile.ErrTooLarge`) and the staged-directory `ErrNotRegular` refusal have no test.
- **Why it matters:** A batch that grows a file past the cap would be accepted on disk but refused when read back, so the stage must mirror disk behaviour; nothing pins that.
- **Fix:** Stage a ReplaceFile over 1 MiB and a CreateFile'd directory, and assert `ReadRegularFile` fails with `nodefile.ErrTooLarge` and `nodefile.ErrNotRegular`.

### Medium — Staged catalog filter parity with the index is untested `[Critical-Path Tests]`

- **Where:** `internal/stage/catalog.go:241`
- **What:** `isSelected` re-implements the index query filter in memory, but `catalog_test.go` exercises only a few fields (Limit shown).
- **Why it matters:** Divergence from `index.List` (Statuses, Categories, Types, Priorities, ParentID, Plan, Labels/NoLabels, IsClosedIncluded) would silently change what a staged batch selects.
- **Fix:** Add a table test applying each filter field to the same issues through `Catalog.List` and `index.List`, asserting equal ids.

### Medium — UI write-path failure branches are untested `[Critical-Path Tests]`

- **Where:** `internal/ui/write.go:343`
- **What:** The post-write failure branches of `applyWritten`/`applyConflict` have no test: `withRefreshFailure`, the `reloadAfterFormat` path when the conflicted issue cannot be re-read (about `:375`), and the generic non-conflict write error notice (`writeFailurePrefix`). No test references `refreshErr`, `readErr`, `reloadAfterFormat` or `refreshAfterFormat`.
- **Why it matters:** These are the ops write path's error paths, a stated critical path.
- **Fix:** Add tests that make `Refresh` fail (or delete the issue file between conflict and re-read) and assert the notice reads `<notice> (refresh: ...)` / `write: ... (reload: ...)`, the prompt is not opened when the re-read fails, and the on-disk file is unchanged.

### Medium — `batch.Run` update operation logic is untested `[Critical-Path Tests]`

- **Where:** `internal/batch/run.go:171`
- **What:** The empty-change branch (`op.Update.IsEmpty()` rewrites the issue via `EditIssue`) and the `MissingPlan` result set when an update sets a plan appear in neither `run_test.go` nor `cli/apply_test.go`.
- **Why it matters:** `apply` is the batch write path; a regression would silently drop the missing-plan warning or fail a no-op update.
- **Fix:** Add `TestRun_UpdateSettingPlan_ReportsMissingPlanOnTheResult` (plan with no file yields `Result.MissingPlan`, an existing plan yields "") and `TestRun_EmptyUpdate_RewritesTheIssueUnchanged`.

### Medium — Real clipboard runner and its timeout are never exercised `[Critical-Path Tests]`

- **Where:** `internal/clipboard/clipboard.go:216` (timeout at `:23`, `:178`)
- **What:** `runProgram`, the only production runner, and the 2 s `runTimeout` kill are not tested; every test injects `Writer.Run`.
- **Why it matters:** A regression in stdin delivery or in the kill of a wedged helper (a hung xclip/tmux would freeze every copy) would pass the suite.
- **Fix:** Add a test that runs the real default runner against a helper (re-exec of the test binary via `os.Args[0]` with an env switch), asserting stdin delivery, a non-zero exit being returned, and a helper that sleeps past a shortened timeout being killed with a context error; make the timeout a field or var so the test need not wait 2 s.

### Medium — Busy-index branch of the briefing is untested `[Critical-Path Tests]`

- **Where:** `internal/briefing/briefing.go:112`
- **What:** The `errors.Is(err, index.ErrBusy)` branch of `readWork` has no test; the only index-failure test asserts the "; run capcom reindex" suffix, never the busy path that must not carry it.
- **Why it matters:** Several agents share a worktree index at session start, so busy is the realistic failure; a regression would tell agents to rebuild a healthy index.
- **Fix:** Add a test that holds the index busy (or stubs index open to return `ErrBusy`) and asserts one SectionIndex failure whose message lacks the reindex hint while the rest of the Briefing (locks, check) is still composed.

### Medium — `ready.hide` and duplicate-key config handling are untested `[Critical-Path Tests]`

- **Where:** `internal/config/parse.go:226`
- **What:** No test sets a non-empty `hide` list, an unknown `ready.*` key, a non-list or empty-string `hide` entry, or a duplicate top-level key; `config_test.go` has no `hide` or `ready` case.
- **Why it matters:** A regression that drops the user's hide labels or accepts a malformed entry would silently change which issues `capcom ready` shows to agents.
- **Fix:** Add a load case (`ready:\n  hide: [wip, blocked]` yields `Ready.Hide == [wip blocked]`) and failure rows in the invalid-config table (`ready:\n  hide: wip`, `ready:\n  other: x`, `ready:\n  hide: [""]`, `types: [bug]\ntypes: [task]`), each failing at the offending line.

### Medium — Dependency idempotence and alias matching are untested in ops `[Critical-Path Tests]`

- **Where:** `internal/ops/deps.go:46-77`
- **What:** AddDep's no-op for a dep already present and RemoveDep's matching of a dep written as an Alias (via `graph.Resolve`) have no test; the AddDep/RemoveDep tests cover only new dep, stale ETag, fake catalog, cycle, not-a-dep and last-dep. The AddDep closure also appends without re-checking presence, so a stale Catalog can duplicate a dep.
- **Why it matters:** This is the ID/alias resolution and dependency write path; a regression would silently duplicate a dep or leave an aliased dep in place.
- **Fix:** Add tests: AddDep twice (and with an alias of an existing dep) leaves Deps and file bytes unchanged; RemoveDep with the id of an issue stored as an alias removes that entry. Also skip the append in the closure when the file already names the dep.

### Medium — `cmd/gendocs` deletes files with no test `[Critical-Path Tests]`

- **Where:** `cmd/gendocs/main.go:44`
- **What:** `run`/`removeStalePages` delete every `capcom*.md` in the output directory before regenerating, and `cmd/gendocs` has no test file (0.0% coverage).
- **Why it matters:** A regression in the glob, or in the order (remove before a successful generate), would silently delete or leave stale CLI reference pages; a bad `-out` removes unrelated files.
- **Fix:** Add a test calling `run(t.TempDir())` with a pre-seeded `capcomold.md` and an unrelated `notes.md`: assert the stale page is gone, the unrelated file survives, and the reference pages are written.

## Recommended Action Order

1. Fix the UI link-marker panic (bounds check plus control-byte stripping in `Render`); it is a small change and a crash a hostile PR can trigger repeatedly. Do the control-byte sanitiser for `oneLine` in the same pass, after the quick probe that settles whether it is exploitable.
2. Close the hostile-repository path gaps: `Lstat` checks in `internal/tree`, and `localPath` validation for config `plans`.
3. Restrict the winget probe to the signed-in author before the next release.
4. Make the directory-sync error non-fatal and give `ReplaceFileVerbatim` the same missing-target rule as `ReplaceFileAtomic`.
5. Unify the fence rule between conflict scanning and `ClosedCodeLines` (design choice: one definition of a code line), then fix the heading `#` trimming.
6. Kill the whole process group on agent timeout in the eval harness.
7. Add the missing tests, starting with the hostile-input guards (recap, stage overlay, UI write errors, clipboard runner), then the filter parity, config and dependency tests.

## What Looked Good

The core store is solid: config parsing, hooks, the SQLite index, issue and Note parsing, the lock protocol, nodestore etag handling and `gitx` (argument vectors, no shell, read-only) all matched their documented contracts and carry coverage between 87 and 100 percent. Error typing, all-or-none `apply`, and `cmd/release` testing were also well done, and the CI workflows are SHA-pinned and scoped. These areas need no attention.
