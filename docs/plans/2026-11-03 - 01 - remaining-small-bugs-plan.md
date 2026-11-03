# Remaining small bugs — plan

**Goal:** Fix the four small open bugs from the 2026-11-03 audit follow-up: a false write failure on
a failed directory sync, the UI heading renderer eating the author's leading `#`, orphaned children
after an agent timeout in the eval harness, and the doubled path in a `capcom mv` error.
**Date:** 2026-11-03
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 3ec3a0e

**Sources:**
- `.capcom/issues/dir-sync-fails-write.md`, `.capcom/issues/ui-heading-strips-hash.md`,
  `.capcom/issues/agenteval-timeout-orphans-children.md`, `.capcom/issues/mv-error-path-twice.md`
- `docs/reviews/code-audit-2026-11-03.md`
- Notes `coding-conventions` / `error-messages`, `cli-test-harness`, `agent-eval`

**Ratified design calls:**
- **Directory-sync failure after a landed write** (Airic, 2026-11-03): EINVAL, ENOTSUP and
  EOPNOTSUPP mean "unsupported here" and count as success; any other sync error is returned
  wrapping a new sentinel `nodefile.ErrNotSynced`. Callers are not changed.
- **Agent timeout kill** (plan author, 2026-11-03, within the issue's intent): the agent runs in its
  own process group, gets SIGTERM to the group on cancel, and the group is swept with SIGKILL once
  `Run` returns; `agenteval`'s root context is cancelled on SIGINT/SIGTERM so the `agent` loop kills
  its own shell-command groups before it exits. Windows: the kill is unchanged.

**Standing requirements:**
- skills: coding-standards
- Pi 5 host: run one package at a time with `GOMEMLIMIT=2GiB go test -count=1 <pkg>`; never
  `./...` with `-race`; never two test commands at once.
- CLI tests go through `runCLI` and never call `t.Parallel()` (Note `cli-test-harness`).
- Error messages name one file, as `path: message` (Note `error-messages`).

**Out of scope:**
- The `test-*` children of `audit-2026-11-03` and `html-export` (separate plans).

**Regression check (2026-11-03, 3ec3a0e):**
- 1: guard folded (the doc-comment Acceptance check can now fail)
- 3: recast (the SIGKILL sweep runs after every agent run; Goal widened)
- 4: guard folded (symlink and regular-file cases, `move.go` comments, `--json` path and hint)
- 3: guard folded, re-check round (runTask returns ctx.Err() after an interrupt and stop() restores
  the default signal; the sweep skips a nil `command.Process`; cancelled-parent shell-command test)

## 1. Count an unsupported directory sync as success and fix the writeFileAtomic doc comment — ✅ DONE (2026-11-03)

**What:** Fixes `dir-sync-fails-write`: `WriteFileAtomic`, `WriteFileExclusive` and the other atomic
writes return a `syncDir` error after the rename or link landed, so a caller sees a failure for a
write that took effect and a retried `WriteFileExclusive` then fails with `fs.ErrExist`.

**Regression guard.** `grep -n "ReplaceFileVerbatim" internal/nodefile/atomic.go` already matches
at the base (the function's own doc and declaration), so the doc-comment check is
`sed -n '/^\/\/ writeFileAtomic is/,/^func writeFileAtomic/p' internal/nodefile/atomic.go | grep -c ReplaceFileVerbatim`,
which must print 1 or more (it prints 0 at the base).

**Goal:** In `internal/nodefile`, a directory-sync error after a landed write is ignored when it is
EINVAL, ENOTSUP or EOPNOTSUPP (the write returns nil), and any other one is returned naming the path
and wrapping both the exported sentinel `ErrNotSynced` and the sync error; the `writeFileAtomic` doc
comment names every exported caller, `ReplaceFileVerbatim` among them.

**Approach (assumed at the header base):** Add `ErrNotSynced` (exported, with a doc comment saying
the content is in place) to `atomic.go`. Route the post-write sync of both `writeFileAtomic` and
`WriteFileExclusive` through one helper (e.g. `syncWrittenDir(path)`) that calls a package variable
`syncDirectory` (initialised to the platform `syncDir`), drops EINVAL/ENOTSUP/EOPNOTSUPP via
`errors.Is` with the `syscall` errnos (all three exist on Windows too), and otherwise returns
`fmt.Errorf("%s: %w: %w", path, ErrNotSynced, err)`. Add `SetSyncDir(t, fn)` to `export_test.go`
in the style of `SetLink`. Update the doc comments of `WriteFileAtomic`, `WriteFileExclusive` and
`writeFileAtomic` to say a directory sync that is unsupported is skipped and any other sync failure
wraps `ErrNotSynced`; `writeFileAtomic`'s comment lists `ReplaceFileVerbatim` with the other callers
(the omission dates from 995efbc).

**Files:** internal/nodefile/atomic.go; internal/nodefile/export_test.go; internal/nodefile/atomic_test.go
**Read first:** internal/nodefile/atomic.go — writeFileAtomic, WriteFileExclusive, WriteFileAtomic, joinNonNil; internal/nodefile/syncdir_other.go — syncDir;
internal/nodefile/syncdir_windows.go — syncDir; internal/nodefile/export_test.go — SetLink;
internal/nodefile/atomic_test.go — TestWriteFileExclusive_FreePathWithoutHardLinks_RenamesWholeFileAsLFAndLeavesNoTempFile (non-parallel swap pattern)

**Tests:** in `atomic_test.go` (not parallel, they swap a package variable): for each of
`syscall.EINVAL`, `syscall.ENOTSUP`, `syscall.EOPNOTSUPP` wrapped in a `*fs.PathError`,
`WriteFileAtomic` and `WriteFileExclusive` return nil and the file holds the new content; with
`syscall.EIO`, both return an error with `errors.Is(err, nodefile.ErrNotSynced)` and
`errors.Is(err, syscall.EIO)`, whose text starts with the path, and the file holds the new content.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodefile/`
- `go vet ./internal/nodefile/`
- `GOOS=windows go vet ./internal/nodefile/`
- `sed -n '/^\/\/ writeFileAtomic is/,/^func writeFileAtomic/p' internal/nodefile/atomic.go | grep -c ReplaceFileVerbatim` prints 1 or more

**Closes:** dir-sync-fails-write

**Commit:** `fix(nodefile): count an unsupported directory sync as a landed write`

## 2. Keep the author's leading hash in a rendered UI heading — ✅ DONE (2026-11-03)

**What:** Fixes `ui-heading-strips-hash`: `headingText` in `internal/ui/markdown/markdown.go` trims
every leading space and `#`, so `## #42 follow-up` renders as `42 follow-up`.

**Goal:** A heading renders its text with exactly the marker `#`s `isHeading` counted removed, then
surrounding space trimmed: `## #42 follow-up` shows `#42 follow-up`, `# #hashtag` shows `#hashtag`,
and every heading without a leading `#` in its text (trailing `#`s included) renders as before.

**Approach (assumed at the header base):** In `headingText`, trim leading spaces, skip the run of
`#` (the same count `isHeading` uses), then `strings.TrimSpace` the rest. Share the marker count
with `isHeading` through one small helper rather than counting twice in two shapes.

**Files:** internal/ui/markdown/markdown.go; internal/ui/markdown/markdown_test.go
**Read first:** internal/ui/markdown/markdown.go — headingText, isHeading, renderTextLine, renderHeadingLine;
internal/ui/markdown/markdown_test.go — TestHeadingStripsMarkers, TestNotAHeading, testTheme, strip

**Tests:** a table test beside `TestHeadingStripsMarkers`: `## #42 follow-up` → `#42 follow-up`,
`# #hashtag` → `#hashtag`, `  ### Title` → `Title`, `## Title ##` → its current rendering (read it
from the base before the change and pin it).

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/markdown/`

**Closes:** ui-heading-strips-hash

**Commit:** `fix(ui): keep the author's leading hash in a heading`

## 3. Kill a timed-out eval agent's whole process group — ✅ DONE (2026-11-03)

NOTES (2026-11-03): the helper is isolateAgentProcessGroup, returning sweep func() error; a sweep error other than ESRCH is noted at the end of the transcript rather than failing the task.
NOTES (2026-11-03): the tests check a child is gone with os.FindProcess + Signal(0) == os.ErrProcessDone (ESRCH on Unix) instead of syscall.Kill(pid, 0), so the shared waitUntilGone helper in agent_test.go compiles for GOOS=windows; workspace_unix_test.go also covers an agent that finishes normally and leaves a background child.
NOTES (2026-11-03): the cancelled-runTask test (run_test.go) skips on Windows: its agent is a POSIX shell command.
NOTES (2026-11-03): consequential edit — cmd/agenteval/doc.go: made necessary by agent_unix.go/agent_other.go now holding the agent's process-group kill too, not just the shell command's.
NOTES (2026-11-03): consequential edit — docs/agent-eval.md: made necessary by the new group kill after every agent run and the Ctrl-C/SIGTERM handling in main.

**What:** Recast at the regression check (2026-11-03). Fixes `agenteval-timeout-orphans-children`: `workspace.runAgent` in
`cmd/agenteval/workspace.go` cancels with plain `exec.CommandContext` (SIGKILL to one process), so
a timed-out agent's children survive in a workspace being deleted. Read Note `agent-eval` first.

**Regression guard.** The SIGKILL sweep of the agent's process group runs after every agent run,
timed out or not, so a background process a normally finishing agent leaves is killed too before
the workspace is removed; widen item 3's Goal to say no process the agent started survives
runAgent's return (on Unix), and keep the timeout test as the required test.
Interrupt: once `runAgent` returns, `runTask` returns `ctx.Err()` when the root context is done,
before `readCallLog` or judging, so `runRun` neither judges the killed task nor writes a report;
`main` calls the `NotifyContext` stop() once that context is done, so a second Ctrl-C kills at
once. NotifyContext applies on every OS: only the kill is unchanged on Windows.
Start failure: the sweep reads `command.Process` when it is called and does nothing when it is nil
(`--timeout 1ns`, any `os.StartProcess` failure); never capture the pid before `Start`. The 1ns
test `TestRun_AnAgentPastTheTimeoutFailsTheTask` stays green.

**Goal:** On Unix, no process an agent run started survives `runAgent`'s return, whether the run
finished or hit its timeout, and whether the child stayed in the agent's process group or is a shell
command the `agenteval agent` loop put in a group of its own. On Windows the kill is unchanged.

**Approach (assumed at the header base):** In `agent_unix.go` add a helper beside
`isolateProcessGroup` for the agent process: `Setpgid`, `Cancel` sends SIGTERM to `-pid`, and a
returned sweep function reads `command.Process` when called, does nothing when it is nil, and
otherwise sends SIGKILL to `-pid` ignoring ESRCH; `agent_other.go` gets the no-op
twin. `runAgent` applies it and calls the sweep after `command.Run()` returns. In `main.go`, build
the root context with `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`,
so the `agent` verb's loop context is cancelled and `runShellCommand` kills its command's group
before the process exits. Update the doc comments of `runAgent` and both helpers; the `# Files`
map in `doc.go` changes only if a non-test file is added.

**Files:** cmd/agenteval/workspace.go; cmd/agenteval/agent_unix.go; cmd/agenteval/agent_other.go; cmd/agenteval/main.go; cmd/agenteval/run.go; cmd/agenteval/workspace_unix_test.go; cmd/agenteval/agent_test.go; cmd/agenteval/run_test.go
**Read first:** cmd/agenteval/workspace.go — workspace.runAgent; cmd/agenteval/agent_unix.go — isolateProcessGroup; cmd/agenteval/agent_other.go — isolateProcessGroup;
cmd/agenteval/main.go — main; cmd/agenteval/run.go — runTask; cmd/agenteval/agent.go — runShellCommand;
cmd/agenteval/agent_test.go — TestAgentShell_ATimedOutCommandWithABackgroundChildReturnsAtTheTimeout; cmd/agenteval/run_test.go — TestRun_AnAgentPastTheTimeoutFailsTheTask

**Tests:** new `workspace_unix_test.go` (`//go:build unix`): `runAgent` with agent argv
`sh -c 'sleep 300 & echo $! > child.pid; wait'` and a ~300 ms timeout returns `IsTimedOut`, and
within 2 s `syscall.Kill(pid, 0)` on the recorded child reports ESRCH. In `agent_test.go`, modelled
on `TestAgentShell_ATimedOutCommandWithABackgroundChildReturnsAtTheTimeout` (Unix only):
`runShellCommand` under a parent context cancelled (not timed out) kills its background child. In
`run_test.go`: `runTask` whose context is cancelled while the agent runs returns `context.Canceled`;
`TestRun_AnAgentPastTheTimeoutFailsTheTask` (`--timeout 1ns`) stays green. Never reaches a network
or a model.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./cmd/agenteval/`
- `go vet ./cmd/agenteval/`
- `GOOS=windows go vet ./cmd/agenteval/`

**Closes:** agenteval-timeout-orphans-children

**Commit:** `fix(agenteval): kill a timed-out agent with every process it started`

## 4. Name the children directory once when capcom mv refuses it

**What:** Fixes `mv-error-path-twice`, a regression from 134cf37: `Tree.PathFor` returns a
`*tree.Error` naming the parent's children directory, and `nodestore.Kind.planFailure` wraps it in
`fileError(node.Path, err)`, so `capcom mv bar --parent foo` with a symlinked `foo` prints
`.capcom/issues/bar.md: .capcom/issues/foo: children directory of foo is not a real directory`.

**Regression guard.** Test a symlinked and a regular-file `foo` children path, as `create_test.go`'s
`TestCreate_ChildOfParentWhoseChildrenPathIsNotARealDirectory...` does (symlink case skipped on Windows and as root; inline the skips in `mv_test.go`, `internal/cli` has no `makeSymlink`).
Every comment `grep -n "FileError" internal/nodestore/move.go` finds saying a plan failure is a `*FileError` (`Move`'s doc step 3, `planFailure`'s doc) gets the `*tree.Error` exception.
Via `kindFailure` → `output.AsError`, `--json` then carries no `error.path` (today `.capcom/issues/bar.md`) and the hint becomes `capcom mv --help`, not `capcom check`: the same rendering `create --parent` gets via `resolveFailure`.

**Goal:** A move refused because the destination parent's children path is not a real directory
fails with the `*tree.Error` naming that directory relative to the working tree root, so the CLI
prints `.capcom/issues/foo: children directory of foo is not a real directory` with no moved-node
prefix; `errors.Is(err, tree.ErrNotDirectory)` holds, nothing moves, and every other `mv` failure
prints as before.

**Approach (assumed at the header base):** In `planFailure` (`internal/nodestore/move.go`), before
the `fileError` fallback, return a `*tree.Error` through `k.relativeTreeError`, as `Create` does via
`resolveFailure`. `know mv` shares the generic `Kind`, so it is fixed too.

**Files:** internal/nodestore/move.go; internal/nodestore/move_test.go; internal/cli/mv_test.go
**Read first:** internal/nodestore/move.go — planFailure, Move (doc step 3); internal/nodestore/load.go — resolveFailure, relativeTreeError;
internal/cli/errors.go — kindFailure; internal/cli/output/errors.go — AsError; internal/cli/mv_test.go — newMoveStore;
internal/nodestore/create_test.go — TestCreate_ChildOfParentWhoseChildrenPathIsNotARealDirectory_FailsNamingItAndWritesNothing

**Tests:** `move_test.go`: `Kind.Move` below a parent whose children path is a symlink, and one
where it is a regular file (the symlink case skipped on Windows and as root), returns a
`*tree.Error` whose `Path` is `.capcom/issues/foo` and wraps `ErrNotDirectory`. `mv_test.go`
(through `runCLI`), in both cases: `capcom mv bar --parent foo` with `.capcom/issues/foo` a symlink or a regular file exits 1, its
stderr contains `.capcom/issues/foo: children directory of foo is not a real directory` and not
`bar.md:`, and `.capcom/issues/bar.md` is still in place.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/`
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Mv|Move' ./internal/cli/`

**Closes:** mv-error-path-twice

**Commit:** `fix(nodestore): name the children directory once when a move refuses it`
