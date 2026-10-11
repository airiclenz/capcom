# Plan — capcom v1 (B): knowledge, tree lock, briefing

**Goal:** Complete capcom v1 on top of the issue core: Knowledge Notes and `capcom know`, the per-worktree Tree lock, Elsewhere branch warnings, `capcom check`, the `prime` Briefing with the AGENTS.md Managed block and Claude hooks, then the format and migration docs and dogfooding. Items keep the numbers of the original combined plan, so cross-item references stay valid.
**Date:** 2026-10-11 · **Status:** unexecuted
**sized for:** ~200k-context host
**base:** a607d08
**Sources:** `CONTEXT.md`; `docs/adr/0001`–`0007`; `docs/design/research-notes.md`; `README.md`; `docs/plans/2026-10-11 - 00 - capcom-v1-issues-plan.md` (prerequisite; items 1–10)
**Ratified design calls** — owner, 2026-10-11 (grilling session):
- **Platforms:** Linux, macOS, Windows from day one; nothing apogee-specific.
- **Layout:** `.capcom/config.yaml`, `.capcom/issues/`, `.capcom/knowledge/`; index and locks under the git dir, never committed. capcom requires a git repository.
- **Tree:** directory tree is the hierarchy (ADR 0001); ids are chosen kebab slugs, 2–4 words, ≤40 chars, no prefix, never generated; Aliases (ADR 0002).
- **Statuses:** configurable, each mapped to `todo|doing|waiting|done|dropped`; logic reads only the category (ADR 0003). Defaults `open`=todo, `parked`=waiting, `done`=done, `dropped`=dropped.
- **Issue fields:** `title, status, type, priority (0–4, default 2), labels, deps, plan, aliases, created, closed, reason`; id = filename, never stored; no `updated`; body free markdown with `## Acceptance` and `## Log` by convention. Default types `bug, feature, task, chore`.
- **Plan link:** `plan:` holds a plan basename resolved against config `plans` dirs (default `docs/plans`, `docs/plans/archived`); Planned = plan set.
- **Branches:** state = checked-out tree; other local branches and worktrees reported as Elsewhere, never merged (ADR 0004). Store root resolved, never hard-coded; capcom never commits and never runs a writing git command.
- **Lock:** per worktree, records under `<git-common-dir>/capcom/locks/`, token + heartbeat + TTL (2h default), advisory (ADR 0005).
- **Knowledge:** Notes with `title, labels, refs`; `know tree/show/search/add/verify`; refs are paths or `path#symbol` (text search).
- **Agents:** hand-written AGENTS.md core stays; capcom owns only the `<!-- BEGIN CAPCOM -->…<!-- END CAPCOM -->` block; `prime` for SessionStart and PreCompact; no git hooks ever.
- **Output:** text by default (no wrap, no tips, stderr only on failure); `--json` envelope `{schema:1, kind, data, elsewhere, warnings}` / `error{code,message}`; `--kv` on lock, show, close; exit 0 ok, 1 error, 2 usage, 3 not_found or locked.
- **Migration:** no importer; a guide plus a strict `check` (ADR 0006). **Tests:** real temp dirs, SQLite, git (ADR 0007).
- **Stack:** module `github.com/airiclenz/capcom`; deps `modernc.org/sqlite`, `gopkg.in/yaml.v3`, `github.com/spf13/cobra`, plus `golang.org/x/sys` for the Windows file lock, nothing else.
**Standing requirements:** skills: coding-standards · Pi 5 host: single packages or tests with `GOMEMLIMIT=2GiB`, never `./...` with `-race`, never two test runs at once · No AI attribution anywhere · Every error message names the file (and line when known).
**Out of scope:** `/implement-plan` and other skill edits (lock calls, `.capcom/` in tickets.md; separate plan in `~/repos/skills`); apogee cutover (migration, bd removal, hooks, AGENTS.md; separate plan in apogee); release pipeline, Homebrew formula, MCP server, central data-branch mode, raw SQL query, importers. Items 1–10 (plan `2026-10-11 - 00 - capcom-v1-issues-plan`, a prerequisite).
**Regression check (2026-10-11, a607d08):**
- 11: guard folded
- 12: guard folded
- 13: guard folded; ADR 0004 worktree gap stated
- 14: guard folded
- 15: recast — top-level `know add` refreshes the Managed block (decision); guard folded
- 16: guard folded
- 15: guard folded (ordered settings merge; prime bypasses shared Refresh)
- 15: init snippet kept — owner Q18, not a tip; "no tips" does not apply

## 0. Verify the issue-core plan is archived

**What:**
**Goal:** plan `2026-10-11 - 00 - capcom-v1-issues-plan.md` exists under `docs/plans/archived/` and every one of its items 1–10 carries ✅ in its heading; the packages it built (`internal/cli`, `internal/config`, `internal/store`, `internal/gitx`, `internal/tree`, `internal/issue`, `internal/index`) exist and build.
**Approach (assumed at the header base):** read-only checks; if the plan is not archived, stop the run — this plan depends on it.
**Files:** none
**Tests:** none.
**Acceptance:** `test -f "docs/plans/archived/2026-10-11 - 00 - capcom-v1-issues-plan.md"` ; `GOMEMLIMIT=2GiB go build ./...`
**Commit:** none (verification only)

## 11. Knowledge: Notes and `capcom know`

Depends on item 0 (provides items 3 and 5).
**What:**
**Goal:** Note files under `.capcom/knowledge/` (frontmatter `title, labels, refs, aliases`; body free) load through the same `tree` package and index (FTS over title and body, label filter); `capcom know tree [id] [--depth N]` prints the indented outline of ids and titles only; `know show <id> [--with-children]`; `know search <words> [--label]`; `know add --id <slug> --title <t> [--parent] [--label] [--ref]... [--body-file]`; `know verify` lists every Note whose ref is a missing path, or a `path#symbol` whose file lacks the symbol text, with `path:line` of the ref in the Note.
**Approach (assumed at the header base):** `internal/knowledge` (note.go for the format, verify.go for refs) reusing `internal/tree` and `internal/index` (add a `notes` table and a `kind` column to FTS); CLI in `internal/cli/know.go`. Refs resolve from the worktree root; escaping it is invalid.
**Regression guard.** Every shared index table is scoped by `kind` and issue queries filter `kind='issue'`. The schema version bumps so item 5's mismatch rebuild fires; refresh walks `.capcom/knowledge/`.
**Files:** `internal/knowledge/note.go`, `internal/knowledge/verify.go`, `internal/knowledge/knowledge_test.go`, `internal/index/notes.go`, `internal/index/notes_test.go`, `internal/index/query.go`, `internal/index/schema.go`, `internal/index/refresh.go`, `internal/cli/know.go`, `internal/cli/know_test.go`
**Read first:** internal/index/schema.go — files(kind), FTS5 table, schema version; internal/index/refresh.go — Refresh(store); internal/index/query.go — search, label filter; internal/tree/tree.go — Tree.Load, per-kind alias callback; CONTEXT.md — Note, Ref, Label
**Tests:** tree depth limiting; show with children; add nested; verify flags missing file, missing symbol, passes on present ones, rejects `../` escape; Note with no refs never flagged; issue and note may share an id; `capcom search <word only in a Note>` → `not_found`; an old-schema index.db rebuilds.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/knowledge/` ; `GOMEMLIMIT=2GiB go test -count=1 -run Notes ./internal/index/` ; `GOMEMLIMIT=2GiB go test -count=1 -run Know ./internal/cli/`
**Commit:** `feat(knowledge): notes tree, search and ref verification`

## 12. Tree lock

Depends on item 0 (provides item 2).
**What:**
**Goal:** `capcom lock acquire --purpose <text> [--holder <text>] [--token <t>] [--take-stale]`, `lock touch --token`, `lock release --token`, `lock status [--all]`, `lock break`, `lock check [--token <t>]` work per worktree with records at `<git-common-dir>/capcom/locks/<key>.json` (`_main` for the primary worktree, else `wt-<linked worktree name>`); acquire returns a new token, or `resumed` with the caller's own token even when stale; a lock held by another fails `locked` with holder, purpose, age and stale flag; `--take-stale` only takes a stale lock; `check` exits 0 when free or held by its token, 3 when another holder has it; every lock command supports `--kv` (`LOCK=acquired|resumed|held|free|released|stale HOLDER= PURPOSE= SINCE= HEARTBEAT= TOKEN= STATUS=`).
**Approach (assumed at the header base):** `internal/lock` with record `{token, purpose, holder, host, user, branch, since, heartbeat}`; the read-modify-write is guarded by an OS lock on a sibling `.lck` file — `flock` in `oslock_unix.go`, `LockFileEx` via `golang.org/x/sys/windows` in `oslock_windows.go` — and records written atomically. Staleness = `now - heartbeat > cfg.Lock.TTL`. Token = 128-bit random hex.
**Regression guard.** `oslock_unix.go` carries `//go:build unix` (`_unix` is no GOOS suffix); `oslock_windows.go` relies on its suffix.
**Files:** `internal/lock/lock.go`, `internal/lock/oslock_unix.go`, `internal/lock/oslock_windows.go`, `internal/lock/lock_test.go`, `internal/cli/lock.go`, `internal/cli/lock_test.go`, `go.mod`, `go.sum`
**Read first:** docs/adr/0005-the-tree-lock-is-a-token-with-a-heartbeat-not-a-process.md — record path, resume, take-stale, lock check; internal/tree/atomic.go — WriteFileAtomic; internal/store/store.go — Open, GitDir, GitCommonDir; internal/config/config.go — Config.Lock.TTL; internal/cli/output.go — Renderer (kv, STATUS= last); internal/cli/testharness_test.go — runCLI
**Tests:** acquire/touch/release cycle; second acquirer refused; own-token resume of a stale lock; take-stale on fresh lock refused, on a stale lock wins; break; `lock check` free, own token, other holder; two worktrees hold independent locks and `status --all --kv` emits one `WORKTREE=` group per record, `STATUS=` last; linked worktree `_main` gets its own record; 20 goroutines racing acquire → exactly one wins; kv shape.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/lock/` ; `GOMEMLIMIT=2GiB go test -count=1 -run Lock ./internal/cli/` ; `GOOS=windows go vet ./internal/lock/`
**Commit:** `feat(lock): per-worktree tree lock with token and heartbeat`

## 13. Elsewhere: changes on other branches

Depends on item 0 (provides item 7).
**What:**
**Goal:** `ready`, `list` and `show` report, separately from their result, issues changed on other local branches (including those checked out in other worktrees) that are not in HEAD: for each branch whose tip is newer than `elsewhere.days` and has commits not in HEAD, each `.capcom/issues/**` file added, modified or deleted relative to the merge base is reported as `<id>: <status> on <branch> (unmerged)` or `added`/`removed`; `--no-elsewhere` skips the scan; JSON puts it in `elsewhere`. No fetch, no remotes.
**Approach (assumed at the header base):** `internal/gitx` gains `Branches()` (`for-each-ref refs/heads --format`), `ChangedFiles(base, branch, pathspec)` (`diff --name-status HEAD...<branch>`) and `ShowFile(branch, path)`; `internal/elsewhere` parses the branch copy with `issue` parsing and builds the entries. All git calls read-only.
**Regression guard.** Diffs run with `--no-renames -z`; a rename reports as `removed` + `added`. The `--kv` renderer omits Elsewhere. Every Elsewhere failure (git error, unborn HEAD, branch copy that fails to parse) becomes one `warnings` entry and the result still prints; an unparsable copy still reports `modified`. The pathspec is built from `store.Root` relative to the worktree root; `show` limits it to the shown ids' files. Gap vs ADR 0004 ("worktrees whose issue files differ"), stated not fixed: uncommitted changes in other worktrees are not scanned.
**Files:** `internal/gitx/branches.go`, `internal/gitx/branches_test.go`, `internal/elsewhere/elsewhere.go`, `internal/elsewhere/elsewhere_test.go`, `internal/cli/ready.go`, `internal/cli/list.go`, `internal/cli/show.go`, `internal/cli/output.go`, `internal/cli/read_test.go`
**Read first:** internal/gitx/gitx.go — run wrapper (GIT_OPTIONAL_LOCKS=0); internal/issue/file.go — Parse (KnownFields); internal/config/config.go — Config.Elsewhere.Days; internal/cli/output.go — Envelope.Elsewhere, Renderer; internal/cli/ready.go, list.go, show.go — command RunE; docs/adr/0004-issues-are-versioned-with-the-code.md
**Tests:** temp repo: branch closes an issue → ready on main shows the entry; branch renames an issue → removed + added; merged branch → no entry; old branch beyond the window ignored; worktree on a branch reported; `--no-elsewhere`; current branch never reported against itself; `show --kv` with a diverged branch keeps `STATUS=` last; unborn HEAD → warning, result printed.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/elsewhere/ ./internal/gitx/` ; `GOMEMLIMIT=2GiB go test -count=1 -run 'Ready|List|Show' ./internal/cli/`
**Commit:** `feat(elsewhere): report issue changes on unmerged local branches`

## 14. `capcom check`

Depends on items 0 and 11.
**What:**
**Goal:** `capcom check` validates the whole store and prints one `path:line: message` per problem (JSON: list of `{path, line, code, message}`), exit 1 when any: config errors; frontmatter schema and unknown keys; undeclared status or type; malformed id; orphan directory; duplicate id or alias in a tree; deps that resolve to nothing; dependency cycles; closed without `closed`/`reason`; `plan` that resolves in no `plans` dir; Note refs broken (`know verify`); Managed block stale (wired in item 15). `--fix` is out of scope.
**Approach (assumed at the header base):** `internal/check` collects `Problem` values from the existing validators (`issue.Validate`, `tree` load errors, `issue.FindCycle`, `knowledge.Verify`) — no rule reimplemented; reads files directly, not the index.
**Regression guard.** Add `issue.Cycles(edges) [][]string` (each cycle once, canonical rotation; `FindCycle` built on it) and report cycles from it. `tree.Load` returns every load problem with its path, not the first. Note parse/validate (`internal/knowledge/note.go`) and the `.capcom/knowledge/` tree load are check sources. With problems, JSON is `kind: check`, `data: [{path,line,code,message}]`, exit 1 — not the error envelope.
**Files:** `internal/check/check.go`, `internal/check/check_test.go`, `internal/cli/check.go`, `internal/cli/check_test.go`, `internal/cli/output.go`, `internal/issue/deps.go`, `internal/issue/deps_test.go`, `internal/tree/tree.go`, `internal/tree/tree_test.go`
**Read first:** internal/issue/validate.go — Validate returning all problems; internal/tree/tree.go — Tree.Load, orphan/duplicate errors; internal/issue/deps.go — FindCycle; internal/knowledge/verify.go, note.go — Verify, note parse; internal/config/config.go — Config.Plans; internal/cli/output.go — Envelope, Renderer; docs/adr/0006-migration-is-agent-written-files-checked-by-capcom.md
**Tests:** one fixture per problem class asserting path and line, incl. a bad Note (unknown key, no title); clean store exits 0; plan resolved from the archived dir passes; many problems all reported in one run, incl. two tree problems; one 3-cycle → exactly one problem; JSON shape on exit 1.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/check/ ./internal/tree/` ; `GOMEMLIMIT=2GiB go test -count=1 -run Cycle ./internal/issue/` ; `GOMEMLIMIT=2GiB go test -count=1 -run Check ./internal/cli/`
**Commit:** `feat(check): whole-store validation with path:line problems`

## 15. Briefing and the Managed block: prime, agents, init --claude-hooks

Depends on items 12, 13 and 14.
**What:** Recast at the regression check (2026-10-11).
**Goal:** `capcom prime` prints the Briefing — Ready work split planned/unplanned (capped at 15 each, with counts), issues in a `doing` status, Tree locks of all worktrees with age and stale flag, Elsewhere entries, broken Note refs and check problem count — and never fails the session (any internal error becomes one warning line, exit 0); `capcom agents` writes or refreshes the `<!-- BEGIN CAPCOM -->…<!-- END CAPCOM -->` block in `AGENTS.md` (appending it when absent, touching nothing outside the markers) with: capcom is the issue register of record, the core commands (`ready`, `show`, `create`, `close`, `know tree`), and the top level of the Knowledge tree (ids and titles); `agents --check` and `check` report a stale block; `init --claude-hooks` merges SessionStart (`startup|resume|clear|compact`) and PreCompact hooks running `capcom prime` into `.claude/settings.json`, preserving every other key, and plain `init` prints that snippet instead.
**Approach (assumed at the header base):** `internal/agents` renders the block deterministically (golden test) and splices by markers; `internal/cli/prime.go` composes from existing packages only; settings merge via `encoding/json` into a generic map.
**Regression guard.** When AGENTS.md already contains the Managed block, any capcom write that changes the top level of the Knowledge tree (a top-level `know add`) refreshes the block in the same command, so `check` stays green; document this in docs/format.md (item 16 owns the docs wording). Only a present-but-outdated block is a problem; absent AGENTS.md or markers is not. The block is rendered and spliced in the file's line ending, outside bytes kept verbatim, staleness compared with CR stripped. settings.json: decode the top level as ordered `(key, json.RawMessage)` pairs via `Decoder.Token` (not a generic map), rewrite only `hooks`, re-indent untouched values with `json.Indent`; encode `SetEscapeHTML(false)`, 2-space indent. prime opts out of `root.go`'s shared store-open and `Refresh` and does both inside its own warning wrapper. Plain `init` prints the hooks snippet: it is that command's own requested result (owner, 2026-10-11, Q18), not a tip, so the header's "no tips" rule does not apply; text mode only (absent from `--json`); item 2's init tests updated.
**Files:** `internal/agents/block.go`, `internal/agents/block_test.go`, `internal/cli/prime.go`, `internal/cli/prime_test.go`, `internal/cli/agents.go`, `internal/cli/agents_test.go`, `internal/cli/init.go`, `internal/cli/init_test.go`, `internal/cli/know.go`, `internal/cli/know_test.go`, `internal/cli/check_test.go`, `internal/check/check.go`, `internal/cli/root.go`
**Read first:** internal/cli/root.go — shared pre-run, store open, index Refresh; internal/cli/init.go — init command; internal/cli/know.go — know add; internal/check/check.go — Problem collector;
internal/knowledge/note.go — Note load (top-level ids/titles); internal/elsewhere/elsewhere.go — branch scan; internal/lock — records of all worktrees, staleness;
internal/cli/testharness_test.go — runCLI
**Tests:** prime golden output on a fixture store; prime with a broken config still exits 0 with a warning; prime with an unreadable or garbage index.db exits 0 with one warning line; plain `init` prints the hooks snippet in text mode only; agents appends, then refreshes in place, leaving surrounding text byte-identical; CRLF AGENTS.md stays CRLF and is not stale; stale block detected by check; check without AGENTS.md exits 0; top-level `know add` refreshes an existing block; claude-hooks merge keeps existing hooks, `&&`, `<`, `>` and integers byte-identical, and is idempotent; a non-alphabetical settings.json differs only inside `hooks`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/agents/` ; `GOMEMLIMIT=2GiB go test -count=1 -run 'Prime|Agents|Init|Check|Know' ./internal/cli/`
**Commit:** `feat(cli): prime briefing, AGENTS.md managed block and Claude hooks`

## 16. Docs and dogfooding

Depends on item 15.
**What:**
**Goal:** `docs/format.md` covers issues, Notes, config, tree, aliases and the lock record; `docs/migrating-from-beads.md` gives the bd→capcom field mapping (from `docs/design/research-notes.md` and the grilling: aliases for bd ids, `parked` label → status, `planned` label dropped, `spec_id` → plan basename, `blocks` → deps, parent-child → nesting, related/discovered-from/notes/comments → Log, acceptance/design → sections, dropped fields) and the steps (final `bd sync` + `bd export`, agent writes files, `capcom check` until clean, `capcom reindex`, re-point `core.hooksPath` if bd owned it); `README.md` usage reflects the shipped commands; capcom's own repo runs `capcom init`, holds its follow-up work as issues and carries the Managed block in `AGENTS.md`; `capcom check` passes on the repo.
**Approach (assumed at the header base):** docs written from the code and the ADRs; the dogfood step uses the freshly built binary (`go run ./cmd/capcom ...`).
**Regression guard.** Every README statement the shipped behaviour contradicts is rewritten (`grep -n 'design phase\|will\|planned\|git mv\|lock acquire' README.md`). `docs/format.md` states that a top-level `know add` refreshes an existing Managed block (item 15's decision).
**Files:** `docs/format.md`, `docs/migrating-from-beads.md`, `README.md`, `AGENTS.md`, `.capcom/config.yaml`, `.capcom/issues/`, `.capcom/knowledge/`
**Read first:** README.md — status banner, What it will do, Usage (planned); docs/format.md — item 4's issue section (extended, not replaced); docs/adr/0006-migration-is-agent-written-files-checked-by-capcom.md — guide path; docs/design/research-notes.md — MIGRATION line; AGENTS.md — hand-written core + Managed block
**Tests:** none new; the acceptance runs the binary against the repo.
**Acceptance:** `go run ./cmd/capcom check` ; `go run ./cmd/capcom agents --check` ; `go run ./cmd/capcom prime`
**Commit:** `docs: format reference, beads migration guide and capcom dogfooding`
