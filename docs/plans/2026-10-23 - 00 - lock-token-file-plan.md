# Plan: capcom keeps the Lock token in a token file (ADR 0016)

**Goal:** capcom keeps each session's Lock token in a token file keyed by worktree and purpose, so
`capcom lock acquire`, `capcom start` and `capcom finish` resume or release the Tree lock without
`--token`. The Managed block, the Briefing and the guides stop telling agents to keep the token.
**Date:** 2026-10-23
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** b095f8d

**Sources:**
- Notes `adr-0016`, `adr-0016-decision` (accepted), `adr-0005-decision`, `cli-test-harness`, `store-layout`, `issue-workflow`
- `.capcom/issues/agent-ergonomics/lock-token-file.md`, `start-finish-token-file.md`

**Ratified design calls (user, 2026-10-23):**
- **ADR 0016:** accepted; the Tree lock is a public signal between agents, not a secret.
- **touch and release:** keep `--token` required, as the ADR says; no `--purpose` on either.

**Standing requirements:**
- skills: coding-standards
- Tests per `cli-test-harness`: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.
- After changing a command's help text: `go run ./cmd/gendocs` and commit `docs/site/reference/cli/`.
- Glossary terms (Tree lock, Lock token, Managed block, Briefing, Issue) in code, help and docs.

**Regression check (2026-10-23, b095f8d):**
- 1: guard folded (doc comments in internal/lock stop saying a free-lock release changes nothing or calling the token a secret).
- 2: guard folded (writer decision: rival-session purposes; lockReleaseLong drops "changes nothing").
- 3: guard folded (writer decision: take-over mismatch test, CHANGELOG entry superseding CHANGELOG.md [Unreleased] start/finish lines; plus no-token test and --keep-lock skips SessionToken).
- 4: recast (writer decision: owns every verbatim Briefing and Managed block quote, quickstart.md and tree-locks.md added; plus start-sentence fragment and preStartBlock).
- 4 (round 4): guard folded (CHANGELOG entry superseding CHANGELOG.md [Unreleased] line 12; start clause only with a doing status; "--token <token>" fallback in held/stale and resume sentence; preStartBlock from the base-rendered constant; prime.go keep-the-token grep).
- 4: rejected — Acceptance grep of docs/site and README for 'this worktree: \(free\|held\|stale\)', 'claim it and take the Tree lock', 'free the Tree lock; it names' prints nothing; those patterns also match the new free/held/stale, claim and finish lines item 4 writes into briefing.md and managed-block.md, and managed-block.md:10 prose.
- 5: guard folded (writer decision: guard covers free-lock "changes nothing"; leaves item 4's quotes; plus grep exclusions, widened regex, multi-line pass, lock-token glossary Note and "secret").

**Out of scope:**
- `--purpose` on `lock touch` or `lock release`; harness session ids.
- Sweeping token files whose lock is gone, beyond release, break and the next acquire replacing them.

## 1. The token file in internal/lock

**What:**
**Goal:** `internal/lock` writes a token file whenever Acquire takes or resumes a lock, returns the
kept token for a purpose, deletes the token files holding a released token, and deletes all of a
worktree's token files on Break. The Lock overview never lists a token file as a worktree.
**Approach (assumed at the header base):** New `internal/lock/token.go`; extends package `lock`, no
new package. Binding:
- Path `<lock dir>/<key>.<hex sha256(purpose)>.token`, owner-only (`0o600`), written with
  `nodefile.WriteFileAtomicVerbatim`. Content `token + "\n" + purpose + "\n"`; a reader takes the first
  line as the token and the rest, one trailing `\n` removed, as the purpose. An empty purpose writes
  no file and keeps no token.
- `Locks.Acquire` writes the token file for `claim.Purpose` under the guard BEFORE writing the
  record, so a failed file write takes no lock; its `(Status, bool, error)` signature is unchanged.
- `Locks.SessionToken(key, token, purpose string) (string, error)`: `token` when non-empty, else the
  kept token whose file exists and whose purpose line equals `purpose`, else `""`. Malformed or
  other-purpose files give `""`; only a read error other than not-exist fails, naming the path.
  `Worktree.SessionToken(token, purpose)` wraps it like the other Worktree methods.
- `Locks.Release` deletes, under the guard, every token file of key whose token equals the token
  given, after a release or on a free lock; a `*HeldError` deletes nothing.
- `Locks.Break` deletes every token file of key, the corrupt-record case included. A file belongs to
  key only when its name is exactly key + `.` + 64 lowercase hex + `.token`, so `wt-a` never
  matches `wt-a.b`'s files.
- `recordKeys` is unchanged; update the package doc comment to name the token files.
**Regression guard.** Every doc comment in `internal/lock` that says a free-lock release changes
nothing (`Locks.Release`, lock.go:246-247) or calls the Lock token a secret (`Record.Token`,
lock.go:57, against ADR 0016) is reworded; find them with
`grep -n 'changes nothing\|secret' internal/lock/*.go`, which prints nothing afterwards.
**Files:** internal/lock/token.go; internal/lock/token_test.go; internal/lock/lock.go; internal/lock/worktree.go; internal/lock/overview_test.go
**Read first:** internal/lock/lock.go — Locks.Acquire, Locks.Release, Locks.Break, recordKeys, withGuard, Record;
internal/nodefile/atomic.go — WriteFileAtomicVerbatim; internal/lock/worktree.go — Worktree, ForWorktree;
internal/lock/lock_test.go — openTest, mustAcquire, writeRecord; internal/lock/overview_test.go — mustOverview
**Tests:** `token_test.go`: acquire writes the file (content, `0o600` on Unix); SessionToken returns
it, `""` for another purpose, an empty purpose and a malformed file, and the flag token when given;
a resume rewrites it; release deletes it (another key's file stays); break deletes all of key's
files only; a token file beside a record leaves `Overview` listing one worktree with no problem;
the guard grep above prints nothing.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/` passes; `go vet ./internal/lock/` is clean.
- `go build ./...` succeeds.

**Commit:** `feat(lock): keep the Lock token in a purpose-keyed token file`

## 2. capcom lock acquire resumes from the kept token

**What:**
**Goal:** `capcom lock acquire --purpose <text>` without `--token` resumes the lock its token file
holds, a stale one included, and prints the token again; `--token` wins over the file. Release and
break remove token files; the lock help says capcom keeps the token for the session.
**Approach (assumed at the header base):** In `newLockAcquireCommand`, resolve
`target.lock.SessionToken(token, purpose)` and pass the result to `Acquire`; nothing else in the
flow changes, since a non-matching kept token behaves as no token. Rewrite `lockLong`,
`lockAcquireLong`, `lockReleaseLong`, `lockBreakLong` and `lockExample` / `lockAcquireExample`:
acquire "keeps the Lock token for the session in a token file under the git directory; acquiring
again with the same --purpose and no --token resumes your lock, even a stale one, and prints the
token again"; release also deletes the kept token files holding that token; break deletes this
worktree's kept token files. The resume example becomes the same `--purpose` without `--token`.
Touch, check and status are unchanged. Run `go run ./cmd/gendocs`.
**Regression guard.** Every new CLI test that simulates a rival session acquires under a purpose
different from the holder's (acquireToken uses "item 12"); lockReleaseLong no longer says releasing
a free lock "changes nothing" — it succeeds and only deletes the kept token files holding that token.
**Files:** internal/cli/lock.go; internal/cli/lock_test.go; docs/site/reference/cli/
**Read first:** internal/cli/lock.go — newLockAcquireCommand, lockLong, lockAcquireLong, lockReleaseLong, lockBreakLong;
internal/cli/lock_test.go — acquireToken, makeLocksStale, writeLockRecord, TestLockAcquire_HeldByAnother_FailsLockedWithHolder;
internal/clidoc/reference_test.go — committedReferenceDir; cmd/gendocs — run
**Tests:** CLI tests: acquire twice with one purpose → second `LOCK=resumed`, same `TOKEN=`; a stale
lock (backdated record) resumes the same way; another purpose fails exit 3 `locked`; an explicit
wrong `--token` fails `locked` though the file holds the right one; after release, and after break,
no `*.token` is left in `.git/capcom/locks/`; with a token file beside the record,
`lock status --all --kv` lists one worktree and `lock check` warns nothing. Every rival acquire
uses a purpose other than "item 12"; `lock release --help` no longer says "changes nothing".
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Lock' ./internal/cli/` and `go test ./internal/clidoc/` pass.
- `go run ./cmd/capcom lock acquire --help` says the token is kept for the session.

**Commit:** `feat(lock): acquire resumes the Tree lock from the kept token`

## 3. capcom start and finish use the kept token

**What:**
**Goal:** `capcom start <id>` resumes from the token kept for the Issue id, and `capcom finish <id>`
releases through it and deletes it, neither needing `--token`; a kept token of another purpose or
one that no longer matches is never used. Depends on item 1.
**Approach (assumed at the header base):** In `app.start`, set `token, err =
target.lock.SessionToken(token, entry.ID)` before `target.lock.Check`; Acquire then writes the file
and `claimForStart`'s release on a failed claim deletes it. In `app.finish`, resolve
`SessionToken(token, outcome.ID)` after the close and before `finishLock`; a read error leaves the
lock alone with note "left alone: the kept token cannot be read; run capcom lock status" and the
error as the warning. `finishLock`'s note becomes "no --token given and no kept token for <id>"
when nothing resolved, "the kept token for <id> does not hold it" for a kept one, and keeps
"--token does not hold it" for a flag. Update `startLong`, `startExample`, `finishLong`,
`finishExample`: the token is kept for the session and `--token` is optional. Run gendocs.
**Regression guard.** The kept-token-mismatch finish test takes the lock over instead of breaking
it (backdate the record so it is stale, then `lock acquire --purpose other --take-stale`), since
Break deletes the worktree's token files; the item's CHANGELOG entry says start and finish resume
and release without --token. This supersedes CHANGELOG.md [Unreleased] lines 10-11 (`--token`
resumes start; finish without `--token` leaves the lock alone). `app.finish` resolves
`SessionToken` only when `!isKeepLock`, so `--keep-lock` keeps today's Read-only path and its
"kept (--keep-lock)" note (finish.go:32: it needs no token). `TestFinish_TokenNotHoldingTheLock_LeavesItAndSaysSo`
"no token" holds the lock with `acquireToken` (purpose "item 12"), so no token is kept for
login-flow, and expects "no --token given and no kept token for login-flow".
**Files:** internal/cli/start.go; internal/cli/start_test.go; internal/cli/finish.go; internal/cli/finish_test.go; docs/site/reference/cli/
**Read first:** internal/cli/finish.go — app.finish, finishLock, unreadableFinishLock, finishLong;
internal/cli/start.go — app.start, claimForStart, startLong; internal/lock/lock.go — Locks.Release, Locks.Acquire;
internal/cli/finish_test.go — TestFinish_TokenNotHoldingTheLock_LeavesItAndSaysSo, startToken; internal/cli/lock_test.go — acquireToken, makeLocksStale
**Tests:** start then finish, neither with `--token` → Issue closed, `LOCK=released`, no `*.token`
left; start twice → second `LOCK=resumed`, same token, Issue file byte-identical; a lock held
under another purpose → start fails exit 3 and writes nothing; start, backdate the record so it
is stale, then `lock acquire --purpose other --take-stale` → finish closes the Issue and its lock
line names the kept token not holding the lock; `--keep-lock` leaves lock and file, and still
prints "kept (--keep-lock)" when the token file cannot be read; the "no token" case of
`TestFinish_TokenNotHoldingTheLock_LeavesItAndSaysSo` holds the lock with `acquireToken` and
expects "no --token given and no kept token for login-flow".
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'Start|Finish' ./internal/cli/` and `go test ./internal/clidoc/` pass.
- `go run ./cmd/capcom finish --help` no longer requires `--token` to release.

**Commit:** `feat(cli): start and finish resume and release through the kept token`

## 4. The Managed block and the Briefing stop asking agents to keep the token

**What:**
Recast at the regression check (2026-10-23).
**Goal:** The Managed block and the `capcom prime` Briefing tell agents to re-run their acquire or
start to resume and never to keep the token; `AGENTS.md` carries the new block. Depends on items 2, 3.
**Approach (assumed at the header base):** In `internal/agents/block.go`: `claimLine` becomes
"- `capcom start <id>`: claim it and take the Tree lock before you start"; the finish line becomes
"- `capcom finish <id> --reason <text>`: close it and free the Tree lock; it names the issue to work
on next"; the Tree lock paragraph says `capcom start <id>` takes the lock for an issue, capcom keeps
the Lock token for the session, and running the same acquire or start again resumes it after a
restart or compaction and prints the token again; the touch, release and check sentences stay. In
`internal/cli/prime.go`, `worktreeLockFree` / `Held` / `Stale` become:
"this worktree: free; before a multi-file change run capcom lock acquire --purpose <text>, or capcom start <id> for an issue";
"this worktree: held; if it is yours, run the same capcom lock acquire --purpose <text> or capcom start <id> again to resume it, otherwise do not edit it";
"this worktree: stale; if it is yours, run the same acquire or start again to resume it, otherwise ask the user before taking it over with --take-stale".
Regenerate `AGENTS.md` with `go run ./cmd/capcom agents`; update the block and Briefing text in
`docs/site/agents/managed-block.md`, `docs/site/agents/briefing.md` and the block description in
`docs/site/reference/file-format.md`.
**Regression guard.** Item 4 owns every verbatim quote of the Briefing lock lines and of the
Managed block lines anywhere in the docs, including docs/site/getting-started/quickstart.md and
docs/site/concepts/tree-locks.md — add both to its Files, so no doc quotes stale text between items
4 and 5. The `capcom start <id>` sentence of the Tree lock paragraph is its own fragment, rendered
only when doingStatus != "" (as claimLine is), so `TestRender_NoDoingStatus_LeavesOutTheClaimLine`
stays green. `preStartBlock` keeps its replacer keys and is built from the base-rendered block
constant the item adds for `TestInspect_States`, not from `goldenBlock`. The item's CHANGELOG entry
supersedes CHANGELOG.md [Unreleased] line 12 (claim line "keeping the token it prints", finish line
with `--token <token>`): it states the new claim and finish lines, the lock paragraph and the new
free/held/stale `this worktree:` lines of `capcom prime`. `worktreeLockAdvice` / `addLocks` take
whether config.yaml declares a doing status and drop the `capcom start <id>` clause when it does not;
the lock paragraph names `capcom start` only inside the doingStatus != "" fragment, its unconditional
resume sentence naming only acquire. The held and stale lines and the lock paragraph's resume
sentence keep "(or pass --token <token>)", for a lock taken without a token file (v0.9.2, --token only).
**Files:** internal/agents/block.go; internal/agents/block_test.go; internal/cli/agents_test.go; internal/check/agents_test.go; internal/cli/prime.go; internal/cli/prime_test.go; AGENTS.md; docs/site/agents/managed-block.md; docs/site/agents/briefing.md; docs/site/reference/file-format.md; docs/site/getting-started/quickstart.md; docs/site/concepts/tree-locks.md
**Read first:** internal/agents/block.go — render, claimLine, loopClosing, blockGuidance; internal/agents/block_test.go — goldenBlock, preStartBlock, TestRender_NoDoingStatus_LeavesOutTheClaimLine, TestRender_NoNotes_RecommendsTheTreeLock, TestInspect_States;
internal/cli/prime.go — worktreeLockAdvice, addLocks, worktreeLockFree/Held/Stale; internal/cli/prime_test.go — TestPrime_FixtureStore_PrintsTheBriefing, TestPrime_FreshLock_AdvisesItIsHeld; internal/cli/start.go — doing-status refusal;
internal/cli/agents_test.go — TestAgents_RenamedDoingStatus_ClaimsWithStart; internal/check/agents_test.go — writeCurrentBlock, TestRun_NoDoingStatus_ReportsTheBlockOutOfDate; docs/site/reference/file-format.md — The Managed block in AGENTS.md
**Tests:** update the block and Briefing expectations; `prime_test` asserts each new lock line verbatim;
`render(goldenNotes, "")` contains no "capcom start"; a Briefing for a store declaring no doing status
names no "capcom start" in its free and held lines; the held and stale lines carry "--token <token>";
`preStartBlock` applies its replacer to the base-rendered block constant, and `TestInspect_States`
gains a stale case for that constant as it renders at base (keep-the-token wording).
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/`, `./internal/check/`, and `-run 'Agents|Prime' ./internal/cli/` pass.
- `go run ./cmd/capcom agents --check` reports the block current; `grep -c 'keep the token' AGENTS.md` prints 0.
- `grep -c 'keep the token' internal/cli/prime.go` prints 0.

**Commit:** `feat(agents): the Managed block and Briefing rely on the kept Lock token`

## 5. The guides and Notes teach the kept token

**What:**
**Goal:** No guide, Note or manual page tells an agent to keep the Lock token or that resuming needs
`--token`; the store layout names the token files. Depends on item 4.
**Approach (assumed at the header base):** Edit the `issue-workflow` Note (start/finish without
`--token`); the `store-layout` Note and the lock directory description in
`docs/site/reference/file-format.md` (`<key>.<hash>.token` beside `<key>.json`, never committed);
one sentence in `adr-0005-decision` pointing to ADR 0016 for where the token is kept;
`plugins/capcom/skills/capcom/SKILL.md`; `docs/site/concepts/tree-locks.md` (a "The kept token"
section and the scripted example); `docs/site/concepts/issues.md`;
`docs/site/getting-started/quickstart.md`; `README.md`; `llms.txt`. Run `capcom check` after Note edits.
**Regression guard.** Rule: every sentence telling an agent to keep, store or carry the Lock token,
that resuming needs `--token`, that calls the Lock token a secret, or that releasing a free lock
changes nothing (docs/site/concepts/tree-locks.md among them), found by
`grep -rn -i -E 'keep (the|its|your) (lock )?token|token it prints|token to keep|resume.*--token|--token[^.]*resum|secret[^.]*token|token[^.]*secret|free[^.]*changes nothing' --include=*.md --include=*.txt . | grep -v -E 'docs/plans|skill-runs|CHANGELOG|docs/reviews|docs/site/reference/cli/|\.capcom/issues/|adr-0016'`
plus the multi-line pass `rg -U -i -g '!docs/plans/**' -g '!docs/skill-runs/**' 'resume with\s+`?capcom lock acquire[^.]*--token' .`; the rule, not
the Files list, sets the scope. Item 5 leaves the Briefing/block quotes item 4 already updated.
Rewrite the lock-token glossary Note (resuming by naming the same purpose again; a public signal,
not a secret) and file-format.md:819-821 (the token field).
**Files:** .capcom/knowledge/issue-workflow.md; .capcom/knowledge/store-layout.md; .capcom/knowledge/decisions/adr-0005/adr-0005-decision.md; .capcom/knowledge/glossary/glossary-coordination/lock-token.md; docs/site/reference/file-format.md; plugins/capcom/skills/capcom/SKILL.md; docs/site/concepts/tree-locks.md; docs/site/concepts/issues.md; docs/site/getting-started/quickstart.md; README.md; llms.txt
**Read first:** docs/site/concepts/tree-locks.md — acquire resume paragraph, start/finish section, scripted example;
.capcom/knowledge/glossary/glossary-coordination/lock-token.md; docs/site/reference/file-format.md — The lock record (token field, locks directory listing);
.capcom/knowledge/issue-workflow.md — claiming and finishing paragraphs; plugins/capcom/skills/capcom/SKILL.md — work loop, Tree lock paragraph; llms.txt — work loop line
**Closes:** lock-token-file; start-finish-token-file
**Tests:** none (docs only).
**Acceptance:**
- The guard grep and the multi-line pass print nothing; `go run ./cmd/capcom check` reports no problems.
- `go run ./cmd/capcom know show store-layout | grep -c '\.token'` prints at least 1.

**Commit:** `docs: teach the kept Lock token in the guides and Notes`
