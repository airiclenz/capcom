# Plan: capcom start, capcom finish, the Lock token ADR, and the plugin hook fix

**Goal:** One command claims an Issue and its Tree lock, one closes it and frees the lock, and a
proposed ADR settles how agents keep the Lock token. The plugin hook stands down only for a real
`capcom prime` hook, and six doc statements match the code.
**Date:** 2026-10-22
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** d37439c

**Sources:**
- `.capcom/issues/agent-ergonomics/start-command.md`, `finish-command.md`, `lock-token-recall.md`
- Notes `adr-0005`, `adr-0005-decision`, `adr-0010`, `adr-0012`, `issue-workflow`, `cli-test-harness`, `docs-layout`
- `internal/hooks/settings.go` — `PrimeCommand`, `addHookGroup` (the hook-form rule)

**Ratified design calls (user, 2026-10-22):**
- **Token ADR:** recommends a purpose-keyed token file under the git dir; `capcom start <id>` resumes from it without `--token`.
- **lock-token-recall:** stays open; the owner closes it on accepting the ADR, which makes the follow-ups Ready.
- **finish without --token:** closes the Issue and reports the lock left alone, as a wrong token does.
- **Plugin hook:** `prime.sh` matches only the hook form `"command": "capcom prime"`, in sh.

**Standing requirements:**
- skills: coding-standards
- Tests per `cli-test-harness`: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.
- After changing a command, flag or help text: `go run ./cmd/gendocs` and commit `docs/site/reference/cli/`.
- Glossary terms (Issue, Ready, Tree lock, Lock token, Managed block) in code, help and docs.

**Out of scope:**
- Implementing the token file (filed as follow-ups by item 1, gated on ADR acceptance).
- `--holder`, `--take-stale` or `--status` on start/finish; batch start/finish of several Issues.
- Codex and apogee hook installers; any change to `capcom init --claude-hooks`.

**Regression check (2026-10-22, d37439c):**
- 1: guard folded
- 2: guard folded (writer's decision, plus the readiness, test-seam and doing-refusal guards)
- 3: guard folded (plus the writer's decision on the lock route)
- 4: guard folded; supersedes CHANGELOG.md:115
- 5: guard folded (writer's decision, plus the llms.txt and Acceptance guards)
- 6: guard folded (writer's decision)
- 7: guard folded

## 1. ADR 0016: how an agent session keeps its Lock token

**What:**
**Goal:** Knowledge holds a proposed `adr-0016` Note tree recommending a purpose-keyed token file,
and follow-up Issues under `agent-ergonomics` depend on `lock-token-recall`. No code changes.
**Approach (assumed at the header base):** ADRs live in Knowledge, not `docs/adr/` (`docs/adr/README.md`,
ADR 0010 amendment); this supersedes the issue's "under docs/adr/" wording. Shape it as `adr-0015` is:
`.capcom/knowledge/decisions/adr-0016.md` (Status: Proposed, reading order) with children
`adr-0016-decision` and `adr-0016-rejected`, added with `capcom know add`. The decision compares the
options named in `lock-token-recall` against its four constraints and recommends: `lock acquire` and
`start` write the token to a file under `<git-common-dir>/capcom/locks/` keyed by worktree and
purpose; an acquire or start naming the same purpose without `--token` resumes from it; release and
finish delete it. It states the cost (any session naming the same purpose resumes) and why it is
acceptable (the purpose is the Issue id `prime` lists as doing). Rejected: `--token-file`, harness
session id, resume line alone. File follow-ups with `capcom create --parent agent-ergonomics --dep
lock-token-recall` (one for the token file in `internal/lock` and `lock acquire`, one for `start`
and `finish` using it), each with a self-contained body and `## Acceptance`. Log the ADR on
`lock-token-recall` with `capcom log`; never close it.
**Regression guard.** The token file must not be read as a lock record: `recordKeys` (`internal/lock/lock.go`)
takes every non-hidden `*.json` in `capcom/locks/` as a worktree key. `adr-0016-decision` and the
`internal/lock` follow-up's body name the rule: a hidden or non-`.json` name, or a sibling directory
such as `capcom/tokens/`, with the purpose encoded into a safe file name.
**Files:** .capcom/knowledge/decisions/adr-0016.md; .capcom/knowledge/decisions/adr-0016/; .capcom/knowledge/decisions.md; .capcom/issues/agent-ergonomics/
**Read first:** .capcom/knowledge/decisions/adr-0015.md — Status line, reading order; .capcom/knowledge/decisions.md — the records list;
.capcom/issues/agent-ergonomics/lock-token-recall.md — constraints, Acceptance; internal/lock/lock.go — recordKeys, Locks.Acquire, Record;
internal/lock/overview.go — Locks.Overview; docs/adr/README.md
**Tests:** none (no code).
**Acceptance:**
- `capcom know show adr-0016` prints `Status: Proposed`; `capcom know tree adr-0016` lists both children.
- `capcom dep list lock-token-recall` names at least two dependents under `agent-ergonomics`.
- `capcom show lock-token-recall` still reads `status: open`; `capcom check` reports no problems.
- `git diff --stat HEAD -- internal cmd` is empty.

**Commit:** `docs(adr): propose ADR 0016 on keeping the Lock token`

## 2. capcom start

**What:**
**Goal:** `capcom start <id> [--token <t>]` claims a Ready Issue (first doing status), holds this
worktree's Tree lock with the Issue id as purpose, and prints the Issue in full then the lock line
and `token <t>` line; every refusal writes nothing and names the command that resolves it.
**Approach (assumed at the header base):** New `internal/cli/start.go`, registered in `root.go`
beside `newNextCommand`; no new package. It composes `lock.ForWorktree` / `Locks.Check` /
`Locks.Acquire` / `Locks.Release` with `ops.Claim` (ADR 0012) and `issue.Ready` for the verdict.
Order: resolve the Issue, read its Readiness and `Locks.Check(key, token)`, refuse, then acquire the
lock, then claim; when the claim fails after a fresh acquire, release that lock before returning.
Refusals, all before any write: Closed → `capcom reopen <id> --reason <text>`; standing → the code
`close` uses for a standing Issue, naming `capcom list --tree <id>`; blocked → conflict, naming
`capcom dep list <id>`; in doing without the lock held by `--token` → conflict, naming
`capcom start <id> --token <token>`; lock held by another → locked (exit 3), naming
`capcom lock status`; no doing status → `ops.ErrNoDoingStatus`. In doing with the lock held by
`--token` is a success that leaves the Issue file byte-identical and resumes the lock
(`LOCK=resumed`, same token). A ready.hide label does not refuse. Text: `show`'s rendering, then
`lockText`. `--json` kind `start`: show's issue data plus a `lock` object (`lockData`, token set).
`--kv`: show's record plus `LOCK=` and `TOKEN=`. Long help leads with one sentence and an Example.
Run `go run ./cmd/gendocs`.
**Regression guard.** After the claim writes through ops, refresh the issue index before rendering the Issue (ops never refreshes it); reach the lock through openLockTarget and lock.Worktree's Check/Acquire/Release methods rather than Locks with a key.
Refuse when the Issue has no Readiness verdict or it is not Ready, naming the resolver per reason:
Blocking → `capcom dep list <id>`, OpenChildren → `capcom list --tree <id>`, HeldBy →
`capcom show <ancestor>`, waiting category → `capcom update <id> --status <todo status>`. Add a test
seam like close.go's `beforeCloseEdit`, `beforeStartClaim`, run after the acquire. When the lock is
free, the doing refusal names `capcom lock acquire --purpose <id>` (or `capcom update <id> --status
<todo>`); `capcom start <id> --token <token>` only when the lock is held.
**Files:** internal/cli/start.go; internal/cli/start_test.go; internal/cli/root.go; docs/site/reference/cli/
**Read first:** internal/cli/lock.go — openLockTarget, emitLock, newLockData, lockText; internal/lock/worktree.go — Worktree.Check,
Worktree.Acquire, Worktree.Release; internal/ops/ops.go — Ops.Claim; internal/cli/show.go — emitShown, showResult, loadShowGraph;
internal/issue/ready.go — Ready, Readiness; internal/index/refresh.go — Index.Refresh; internal/cli/lock_test.go — newLockStore, acquireToken; internal/cli/reindex_test.go — newIssueStore
**Tests:** `internal/cli/start_test.go` via `runCLI`: fresh start (status doing, no `ready:` line, lock
held, token printed in text, `--kv` `TOKEN=`, `--json` `.data.lock.token`); each refusal leaves the
issue file and lock record unchanged (closed, standing, blocked, open children, held by an ancestor,
waiting category, doing-without-token with the lock free and with it held, lock held by another);
second run with `--token` succeeds and the issue file is unchanged; claim failure (the Issue closed
by a `beforeStartClaim` hook after the acquire) releases the lock and exits non-zero.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Start' ./internal/cli/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
- `go vet ./internal/cli/`

**Commit:** `feat(cli): add capcom start to claim an Issue and its Tree lock`

## 3. capcom finish

**What:**
**Goal:** `capcom finish <id> --reason <text> [--token <t>] [--keep-lock]` closes the Issue,
releases the Tree lock when `--token` holds it, prints `close`'s outcome and `now ready:` lines, a
lock line, and ends with the top Ready Issue as `capcom next` would choose it.
**Approach (assumed at the header base):** New `internal/cli/finish.go`, registered in `root.go`;
reuses `closeOne` / `ops.Closer` and `requireReason` from `close.go` and `briefing.ReadyList` as
`next.go` calls it, after refreshing the index the close changed. A close failure (no such Issue,
standing) fails finish with close's error and touches no lock. Lock: `--keep-lock` → untouched, no
token needed; token holds it → `Locks.Release`, `released`; free → `free`; held by another or no
`--token` while held → left alone, the line says so and names `capcom lock status`, exit stays 0.
An already-Closed Issue reports `already_closed` and still releases a matching lock. Last line:
`next: <id>  <title>`, or `nothingReadyMessage`; exit 0 either way. `--json` kind `finish`: `close`
(`closeData`), `lock` (`lockData`, no token), `next` (`{id,title}` or null). `--kv`: one record
`ID`, `LOCK`, `NEXT`, `STATUS=<close outcome>`. Help leads with one sentence and an Example. Run
`go run ./cmd/gendocs`. Depends on item 2.
**Regression guard.** Reach the lock through openLockTarget and lock.Worktree's Check/Release methods, as item 2 does, not Locks with a key.
A lock that cannot be read after a successful close (a corrupt or unreadable
record, so `Release`/`Check` fail with an error that is not a `HeldError`) is reported as a lock line
or warning naming `capcom lock status` and `capcom lock break`, and finish exits 0.
**Files:** internal/cli/finish.go; internal/cli/finish_test.go; internal/cli/root.go; docs/site/reference/cli/
**Read first:** internal/cli/close.go — closeOne, closeResult, alreadyClosed, requireReason, closeData; internal/cli/next.go — nothingReadyMessage;
internal/briefing/ready.go — ReadyList; internal/index/refresh.go — Index.Refresh; internal/lock/lock.go — Locks.Release, Locks.Check, HeldError;
internal/cli/lock.go — endedState, newLockData, lockText; internal/cli/close_test.go — setBeforeCloseEdit, newShipStore; internal/cli/lock_test.go — writeLockRecord
**Tests:** `internal/cli/finish_test.go` via `runCLI`: start then finish frees the lock, prints
`now ready:` and `next:`; `--keep-lock` leaves the lock; wrong token and no token leave it and say
so with exit 0; already-Closed repeats `already_closed` and releases; nothing Ready prints the
message, exit 0; standing Issue fails and the lock is unchanged; a corrupt lock record
(`writeLockRecord`) after the close is reported and exits 0; `--kv` and `--json` shapes.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Finish' ./internal/cli/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/clidoc/`
- `go vet ./internal/cli/`

**Commit:** `feat(cli): add capcom finish to close an Issue and free its Tree lock`

## 4. The Managed block teaches start and finish

**What:**
**Goal:** The Managed block's claim line is `capcom start <id>` (claim it, take the Tree lock, keep
the token it prints) and its work loop has a `capcom finish` line; this repo's `AGENTS.md` block,
`docs/site/agents/managed-block.md` and `docs/site/reference/file-format.md` match it, and
file-format's work-loop description starts with `next`.
**Approach (assumed at the header base):** In `internal/agents/block.go`, `claimCommand` /
`claimPurpose` become the start line; keep the rule that leaves it out when `config.yaml` declares
no doing status. Add `finish` to `loopClosing` beside `close`. Update `block_test.go` and
`internal/cli/agents_test.go` expectations. Regenerate this repo's block with
`go run ./cmd/capcom agents`. In `file-format.md`'s Managed block paragraph, list the loop as the
code emits it (`next`, `ready`, `show`, `start`, `log`, `close`, `finish`, `reopen`) and say the
claim line names `capcom start` (left out with no doing status). Depends on items 2 and 3.
**Regression guard.** Reword every sentence saying the claim line names the doing status or that renaming it stales the block
(`grep -rn -i 'doing status' internal/agents docs/site/agents/managed-block.md docs/site/reference/file-format.md`); only an empty
doing status still matters. Recast or delete `TestRun_RenamedDoingStatus_ReportsTheBlockOutOfDate` (`internal/check/agents_test.go`). The CHANGELOG entry says an
existing block reads as out of date after upgrading (run `capcom agents` once) and that renaming the doing status no longer stales it, superseding CHANGELOG.md:115.
**Files:** internal/agents/block.go; internal/agents/block_test.go; internal/cli/agents_test.go; internal/check/agents_test.go; AGENTS.md; docs/site/agents/managed-block.md; docs/site/reference/file-format.md
**Read first:** internal/agents/block.go — render, claimCommand, claimPurpose, loopClosing, Sync; internal/agents/block_test.go — goldenBlock;
internal/check/agents_test.go — TestRun_RenamedDoingStatus_ReportsTheBlockOutOfDate, writeCurrentBlock; internal/cli/agents_test.go — renamedDoingConfig;
internal/agents/guides_test.go — blockGuide; internal/cli/agents.go — Sync call; docs/site/agents/managed-block.md — "Keeping it current"; docs/site/reference/file-format.md — "The Managed block in AGENTS.md"
**Tests:** `block_test.go` pins the new claim and finish lines; no-doing-status case still omits the
claim line; the renamed-doing-status test in `internal/check/agents_test.go` is recast or deleted.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/check/`
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Agents' ./internal/cli/`
- `grep -n 'capcom start <id>' AGENTS.md docs/site/agents/managed-block.md`
- `go run ./cmd/capcom agents --check`

**Commit:** `feat(agents): name capcom start and finish in the Managed block`

## 5. The guides teach start and finish

**What:**
**Goal:** The `issue-workflow` Note, the plugin skill and every manual page that tells an agent to
claim with `update --status` plus `lock acquire`, or to finish with `close` plus `lock release`,
name `capcom start` and `capcom finish` instead.
**Approach (assumed at the header base):** Edit `.capcom/knowledge/issue-workflow.md` ("Starting
work", "Claiming work") with `capcom know update` or by hand plus `capcom check`. In
`plugins/capcom/skills/capcom/SKILL.md` sections 3 and 4, replace the claim and close lines with
`start` and `finish`, keeping `lock` commands for multi-file work outside an Issue. Rule for other
sites: every page under `docs/site/` and `README.md` describing the claim or finish sequence;
find them with `grep -rn -e '--status active' -e 'lock acquire' -e 'lock release' docs/site README.md`
excluding `docs/site/reference/cli/`. Depends on item 4.
**Regression guard.** The plugin ships from main ahead of releases, so SKILL.md names capcom start and capcom finish with a one-line fallback for a capcom binary that lacks them (`capcom start --help` fails): the update --status plus lock acquire, and close plus lock release, sequences; no version number in the text.
The site rule and its grep cover `docs/site README.md llms.txt` (llms.txt:5 claims with
`update --status active`).
**Files:** .capcom/knowledge/issue-workflow.md; plugins/capcom/skills/capcom/SKILL.md; docs/site/concepts/issues.md; docs/site/concepts/tree-locks.md; README.md; llms.txt
**Read first:** plugins/capcom/skills/capcom/SKILL.md — "3. Work loop", "4. Tree lock"; .capcom/knowledge/issue-workflow.md — "Claiming work", "Starting work";
.claude-plugin/marketplace.json — plugins source; llms.txt — agent work-loop line; docs/site/concepts/issues.md — claim example;
docs/site/concepts/tree-locks.md — acquire, release, scripted token example; README.md — command tour
**Tests:** none (docs).
**Acceptance:**
- `capcom know show issue-workflow | grep -n 'capcom start'` and `grep -n 'capcom finish' plugins/capcom/skills/capcom/SKILL.md`
- `grep -n 'capcom start' docs/site/concepts/issues.md docs/site/concepts/tree-locks.md README.md llms.txt` matches in every file
- `capcom check` reports no problems.

**Commit:** `docs: teach capcom start and finish in the workflow Note, skill and manual`

**Closes:** start-command; finish-command

## 6. Plugin hook stands down only for a real capcom prime hook

**What:**
**Goal:** `plugins/capcom/hooks/prime.sh` stays silent only when a settings file holds a hook whose
`"command"` is exactly `"capcom prime"`; any other mention (a permission rule, a comment, a
different command) no longer silences it. The skill says the plugin runs `capcom prime` at session
start and before compaction.
**Approach (assumed at the header base):** Defect: the loop's `grep -qs 'capcom prime'` matches any
mention. Replace it with `grep -Eqs '"command"[[:space:]]*:[[:space:]]*"capcom prime"'`, the rule
`hooks.addHookGroup` applies (`PrimeCommand`). Fix the header comment to match. Check
`docs/site/agents/agent-setup.md`'s plugin paragraph says "already run `capcom prime` as a hook";
reword only if it does not. In SKILL.md section 3, say "at session start and before compaction".
New Go test `internal/hooks/plugin_script_test.go` runs the script with `sh` (skip when `sh` is not
on PATH): temp git repo with `.capcom/`, a fake `capcom` on PATH that echoes `primed`,
`CLAUDE_PROJECT_DIR` and `HOME` set to temp dirs.
**Regression guard.** plugin_script_test.go skips when runtime.GOOS is windows as well as when sh is not on PATH, since Git's sh on windows-latest CI may not find the extensionless fake capcom; item 5 owns SKILL.md section 3's work-loop lines and item 6 changes only the sentence about when the plugin runs capcom prime
**Files:** plugins/capcom/hooks/prime.sh; internal/hooks/plugin_script_test.go; plugins/capcom/skills/capcom/SKILL.md; docs/site/agents/agent-setup.md
**Read first:** plugins/capcom/hooks/prime.sh — settings loop; internal/hooks/settings.go — PrimeCommand, addHookGroup, Snippet, Claude, indentJSON;
plugins/capcom/hooks/hooks.json — SessionStart, PreCompact; internal/hooks/helpers_test.go — test helpers; docs/site/agents/agent-setup.md — "Claude Code plugin";
plugins/capcom/skills/capcom/SKILL.md — "3. Work loop"
**Tests:** settings with `"permissions": {"allow": ["Bash(capcom prime)"]}` → prints `primed`
(fails on the base tree); settings with the `capcom init --claude-hooks` snippet, compact and
indented → silent; no `.capcom/` → silent.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'PluginScript' ./internal/hooks/`
- `sh -n plugins/capcom/hooks/prime.sh`

**Commit:** `fix(plugin): stand down only for a real capcom prime hook`

## 7. Doc statements that no longer match the code

**What:**
**Goal:** `AGENTS.md` names `cmd/gendocs` as a dev tool beside `cmd/release`; `README.md` and
`docs/site/reference/json-output.md` say every command except `capcom ui` takes `--json`;
json-output's `--kv` paragraph points to the CLI reference, not the README.
**Approach (assumed at the header base):** `AGENTS.md` layout sentence: `cmd/release` and
`cmd/gendocs` (generates the CLI reference) are dev tools, never shipped. README "Machine-readable
output" bullet and json-output's opening "`--json`, which every command takes": except `capcom ui`,
which refuses it (`internal/cli/ui.go`). json-output's `--kv` paragraph: which commands take
`--kv` is on each command's page in [the CLI reference](cli/); state where `WARNING=` lines go
relative to `STATUS=` inline, as `internal/cli/output` emits them. Depends on item 4 (AGENTS.md).
**Regression guard.** Rule: every statement that every command takes or prints `--json`, found by
`grep -rn -i 'every command' README.md llms.txt docs/site plugins internal/cli/root.go`. If the root
help (`internal/cli/root.go:28`, mirrored in `docs/site/reference/cli/capcom.md`) is in scope, run
`go run ./cmd/gendocs` and commit `docs/site/reference/cli/`; otherwise exclude it explicitly.
**Files:** AGENTS.md; README.md; docs/site/reference/json-output.md; llms.txt; plugins/capcom/skills/capcom/SKILL.md; internal/cli/root.go and docs/site/reference/cli/ (only if the root help is in scope)
**Read first:** README.md — "Machine-readable output" bullet; docs/site/reference/json-output.md — frontmatter description, opening paragraph, "The --kv format";
AGENTS.md — layout paragraph above the block; internal/cli/ui.go — --json refusal in Args; internal/cli/output/output.go — kvRenderer (warnings first);
internal/cli/root.go — root Long help; plugins/capcom/skills/capcom/SKILL.md — "3. Work loop"; llms.txt — work-loop line
**Tests:** none (docs).
**Acceptance:**
- `grep -n 'cmd/gendocs' AGENTS.md`; `grep -n 'except .capcom ui' README.md docs/site/reference/json-output.md`
- `! grep -n 'README.md#what-it-does' docs/site/reference/json-output.md`

**Commit:** `docs: correct the dev-tool, --json and --kv statements`
