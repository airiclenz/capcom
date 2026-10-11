# Plan — capcom v1 (A): issue core

**Goal:** Build capcom's core issue tracker: module, CLI and output contract, config and `init`, the directory tree with aliases, the issue file format, the SQLite index, and the list/show/search/ready/create/update/log/close/reopen/dep/mv commands. Knowledge, the Tree lock, Elsewhere, `check`, `prime` and the Managed block follow in plan `2026-10-11 - 01 - capcom-v1-knowledge-lock-briefing-plan`.
**Date:** 2026-10-11 · **Status:** unexecuted
**sized for:** ~200k-context host
**base:** a607d08
**Sources:** `CONTEXT.md`; `docs/adr/0001`–`0007`; `docs/design/research-notes.md`; `README.md`
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
**Out of scope:** `/implement-plan` and other skill edits (lock calls, `.capcom/` in tickets.md; separate plan in `~/repos/skills`); apogee cutover (migration, bd removal, hooks, AGENTS.md; separate plan in apogee); release pipeline, Homebrew formula, MCP server, central data-branch mode, raw SQL query, importers. Items 11–16 (plan `2026-10-11 - 01 - capcom-v1-knowledge-lock-briefing-plan`).
**Regression check (2026-10-11, a607d08):**
- 1: guard folded (decision: `.gitattributes`)
- 2: guard folded (decisions: ordered statuses, absolute git dirs)
- 3: guard folded
- 4: guard folded (decision: YAML style; CRLF goldens via item 1's `.gitattributes`)
- 5: guard folded; yields to ADR 0003
- 6: guard folded; yields to research-notes skills contract
- 7: guard folded; yields to ADR 0002
- 8: recast — `--dep` removed (decision); guard folded
- 9: guard folded; its `--dep` cycle guard moot after item 8's decision
- 10: guard folded; supersedes ADR 0001 and README `git mv` lines
- 8: guard folded (decision: one exported Log append in `internal/issue/mutate.go`); yields to research-notes skills contract (`--add-label/--remove-label`)
- 9: guard folded (decision: reopen uses item 8's Log function)
- 8: guards folded (decisions: author `$GIT_AUTHOR_NAME` → user.name → `$USER`; show exposes etag; literal `--body-append` + `--body-append-file`); recast

## 1. Scaffold the module, CLI harness and output contract — ✅ DONE (2026-10-11)

NOTES (2026-10-11): `CHANGELOG.md` with `[Unreleased]` (named in the Approach) was not created — it is not in the item's Files and the implementer never writes the CHANGELOG; the closeout must create it (with `## [Unreleased]`) when applying this entry.
NOTES (2026-10-11): `Run` does not call `os.Getwd()` yet — no command in this item needs the working directory; item 2's store discovery adds it (the harness already enters `dir` via `t.Chdir`).
NOTES (2026-10-11): output-contract choices the plan left open — a failure in `--kv` mode prints `MESSAGE=<path:line: msg>` then `STATUS=<code>` on stdout; `--json --kv` together and `--kv` on a command that offers no KV records are usage errors (exit 2); an untyped internal failure is reported with code `invalid`, exit 1 (the code set stays closed); text-mode warnings print as `warning: ...` lines on stdout after the result; cobra suggestions are disabled ("no tips").

**What:**
**Goal:** `go build ./cmd/capcom` yields a binary whose root command prints help; an in-process `cli.Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int` drives every command in tests; one output layer renders text, `--json` (versioned envelope) and `--kv` (`KEY=VALUE` lines ending `STATUS=`); typed errors map to the codes `not_found, exists, invalid, conflict, locked, not_initialized` and exit codes 0/1/2/3. CI runs `go vet` and `go test ./...` on ubuntu, macos and windows.
**Approach (assumed at the header base):** cobra root in `internal/cli/root.go`; `cmd/capcom/main.go` calls `os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))`. `internal/cli/output.go` holds `Envelope` and a `Renderer` chosen by the global `--json`/`--kv` flags; `internal/cli/errors.go` holds the `Error{Code, Message, Path, Line}` type. `version` prints the embedded `VERSION`. `AGENTS.md` (what capcom is, where knowledge lives, commit to main, no attribution, Pi test rule), `CLAUDE.md` = `@AGENTS.md`, `CHANGELOG.md` `[Unreleased]`, `VERSION` `0.0.0`.
**Regression guard.** Add `.gitattributes` with `* text=auto eol=lf` (and add it to Files) so committed goldens and fixtures stay LF on the windows CI job. `VERSION` is embedded by a root file `version.go` (`package capcom`, `//go:embed VERSION`) imported by `internal/cli` — `//go:embed` rejects `..`. `runCLI` enters `dir` with `t.Chdir` (go.mod `go 1.24`+, CLI tests not parallel); `Run` reads `os.Getwd()`.
**Files:** `go.mod`, `go.sum`, `version.go`, `.gitattributes`, `cmd/capcom/main.go`, `internal/cli/root.go`, `internal/cli/output.go`, `internal/cli/errors.go`, `internal/cli/cli_test.go`, `internal/cli/testharness_test.go`, `.github/workflows/ci.yml`, `AGENTS.md`, `CLAUDE.md`, `VERSION`
**Read first:** docs/adr/0007-tests-use-real-temp-files-sqlite-and-git.md — run(args, stdin) harness, GIT_CONFIG_GLOBAL, fixed author env; .gitignore — /capcom, *.out, /CLAUDE.local.md; docs/design/research-notes.md — Skills integration contract (KEY=VALUE + STATUS=), Prior art (versioned JSON envelope); README.md — Usage (planned)
**Tests:** Run with `--json` on an unknown command → envelope with `error.code=invalid`, exit 2; `version` text and JSON; `--kv` renderer emits `STATUS=` last; a shared test helper `runCLI(t, dir, args...)` returning stdout, stderr, exit.
**Acceptance:** `GOMEMLIMIT=2GiB go build ./cmd/capcom && GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/` ; `go vet ./...` ; `grep -q windows .github/workflows/ci.yml && grep -q macos .github/workflows/ci.yml`
**Commit:** `feat(cli): scaffold capcom module, CLI harness and output contract`

## 2. Config, store root and `capcom init`

Depends on item 1.
**What:**
**Goal:** `capcom init` in a git repo creates `.capcom/config.yaml` (defaults from the header), `.capcom/issues/`, `.capcom/knowledge/` and is idempotent; outside a git repo every command fails `not_initialized`/`invalid` with a clear message; any command run in a subdirectory finds the store by walking up to the worktree root; bad config fails with `path:line`.
**Approach (assumed at the header base):** `internal/config` parses YAML into `Config{Statuses map[string]Category, Types []string, Plans []string, Ready{Hide []string}, Lock{TTL duration}, Elsewhere{Days int}}` with validation (every status names a known category; at least one `todo` and one `done`). `internal/store` exposes `Open(cwd) (*Store, error)` resolving `Root` (the `.capcom` dir), `WorktreeRoot`, `GitDir`, `GitCommonDir` via `internal/gitx` read-only wrappers (`git rev-parse --show-toplevel --git-dir --git-common-dir`). Root is a field, never a constant elsewhere (ADR 0004). `gitx` runs git with `GIT_OPTIONAL_LOCKS=0`, read-only calls only.
**Regression guard.** Config `statuses` stays a YAML mapping decoded via yaml.Node so file order is preserved, and the first status listed in each category is that category's default (used by create, close, reopen). Store.GitDir and Store.GitCommonDir are absolute paths (`git rev-parse --path-format=absolute ...`), tested from a subdirectory. Only commands that need the store open it (`version`, help, completion run anywhere); `init` outside git → `invalid`, a store command without `.capcom/` → `not_initialized`. `WorktreeRoot` is kept via `filepath.EvalSymlinks`; tests compare EvalSymlinks on both sides. Config `Error.Line` comes from yaml.Node positions.
**Files:** `internal/config/config.go`, `internal/config/config_test.go`, `internal/store/store.go`, `internal/store/store_test.go`, `internal/gitx/gitx.go`, `internal/gitx/gitx_test.go`, `internal/cli/init.go`, `internal/cli/init_test.go`
**Read first:** docs/adr/0004-issues-are-versioned-with-the-code.md — store root resolved at startup; docs/adr/0003-statuses-are-configurable-and-behaviour-reads-their-category.md — category set; docs/adr/0005-the-tree-lock-is-a-token-with-a-heartbeat-not-a-process.md — locks under <git-common-dir>/capcom/locks; CONTEXT.md — Status, Status category
**Tests:** init creates the tree and re-run is a no-op; init outside git → `invalid`; store command without `.capcom/` → `not_initialized`; `version` outside git works; subdirectory discovery with absolute GitDir/GitCommonDir; invalid category and unknown YAML key rejected with `Error.Line` asserted; first-listed status is its category's default; Elsewhere days default 30, TTL default 2h.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/config/ ./internal/store/ ./internal/gitx/` ; `GOMEMLIMIT=2GiB go test -count=1 -run Init ./internal/cli/`
**Commit:** `feat(store): config, store root discovery and capcom init`

## 3. The tree: ids, children, aliases, resolution, atomic writes

Depends on item 2.
**What:**
**Goal:** one generic `tree` package loads any `.capcom/<kind>/` directory into nodes (id from filename, parent from the sibling directory rule of ADR 0001), validates ids (`^[a-z0-9]+(-[a-z0-9]+){1,3}$`, ≤40 chars; legacy aliases may be any `[a-z0-9.-]+`), detects duplicate ids/aliases across the whole tree, resolves an argument by exact id, then alias, then unique id prefix (ambiguous → `invalid` listing candidates), and writes files atomically (temp file in the same dir + rename, fsync) with LF endings.
**Approach (assumed at the header base):** `internal/tree` with `Node{ID, Path, ParentID, Aliases}` and `Tree{Load(dir), Resolve(arg), ChildrenOf(id), PathFor(id, parentID)}`; aliases come from a per-kind callback. `WriteFileAtomic` lives here; A node `x.md` plus directory `x/` = parent with children; a directory without its `.md` is an error.
**Regression guard.** `WriteFileAtomic` writes from an `io.Reader` (or an unexported hook) so tests fail it after the temp file opens — no permission tricks. Always fsync the temp file; fsync the directory only off Windows (`syncdir_other.go` `//go:build !windows`, `syncdir_windows.go` no-op). `Load` of a missing dir returns an empty Tree and skips dotfiles and non-`.md` files (temp files are `.<id>.md.tmp-*`).
**Files:** `internal/tree/tree.go`, `internal/tree/resolve.go`, `internal/tree/atomic.go`, `internal/tree/syncdir_other.go`, `internal/tree/syncdir_windows.go`, `internal/tree/tree_test.go`, `internal/tree/resolve_test.go`
**Read first:** docs/adr/0001-the-directory-tree-is-the-hierarchy.md — sibling-directory rule, unique ids across tree; docs/adr/0002-ids-are-chosen-slugs-and-old-names-live-on-as-aliases.md — 2–4 word slugs, aliases, no reuse; CONTEXT.md — Tree, Id, Alias, Parent; docs/design/research-notes.md — MIGRATION (dotted legacy ids)
**Tests:** table tests for id rules; nested load builds parents; orphan directory error; duplicate id in two subtrees error naming both paths; alias and prefix resolution incl. ambiguity; atomic write leaves no temp file on success and the old file intact when an erroring reader fails it; missing dir loads empty; dotfiles and temp files ignored.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/`
**Commit:** `feat(tree): directory-tree hierarchy, id rules, aliases and atomic writes`

## 4. Issue file format

Depends on item 3.
**What:**
**Goal:** issue files parse into a typed `Issue` and serialise back byte-stable (parse → serialise of a canonical file is identity); unknown frontmatter keys are an error with `path:line`; `docs/format.md` documents the issue file, fields, config and tree rules.
**Approach (assumed at the header base):** `internal/issue/file.go`: frontmatter between `---` lines decoded with `yaml.v3` `KnownFields(true)`; fixed key order on write (`title, status, type, priority, labels, deps, plan, aliases, created, closed, reason`), empty fields omitted, timestamps RFC3339 UTC, body kept verbatim. `ETag(bytes)` = FNV-1a 64 hex of the file bytes. Validation against `config.Config` (declared status/type, priority 0–4, closed category requires `closed` and `reason`) is a separate `Validate` func returning all problems.
**Regression guard.** Canonical serialisation pins YAML style — sequences in flow style (`labels: [a, b]`), 2-space indent wherever block style is unavoidable, implemented by hand-written emission or an encoder with SetIndent(2), never yaml.v3 defaults. `priority` is always written (0 is not empty). yaml line numbers get the opening `---` offset so errors cite the file line. Goldens stay LF via item 1's `.gitattributes`.
**Files:** `internal/issue/issue.go`, `internal/issue/file.go`, `internal/issue/file_test.go`, `internal/issue/validate.go`, `internal/issue/validate_test.go`, `docs/format.md`
**Read first:** docs/adr/0003-statuses-are-configurable-and-behaviour-reads-their-category.md — closing reason by category; CONTEXT.md — Closed, Log, Plan, Label, Dependency; docs/adr/0006-migration-is-agent-written-files-checked-by-capcom.md — path:line errors for agent fix loop
**Tests:** round-trip goldens (minimal, full, priority 0, flow-style labels, `---` inside a code block); unknown key cites its file line; bad priority; closed without reason; UTF-8 kept; CRLF → LF.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/issue/`
**Commit:** `feat(issue): markdown issue file format with stable round-trip`

## 5. The index

Depends on item 4.
**What:**
**Goal:** a SQLite index at `<git-dir>/capcom/index.db` mirrors the issue (and later note) trees; every command refreshes it incrementally by comparing each file's size and mtime and re-parsing only changed, added or removed files; deleting the db or `capcom reindex` rebuilds it; a file that fails to parse is reported as a warning with `path:line` and excluded, while the rest still works.
**Approach (assumed at the header base):** `internal/index` over `modernc.org/sqlite` with WAL; tables `files(path, size, mtime, kind)`, `issues(id, parent, status, category, type, priority, plan, created, closed, title, path)`, `labels`, `deps(from_id, to_id)`, `aliases`, plus an FTS5 table over title and body. Schema version mismatch rebuilds. `Refresh(store)` is called once per command by the CLI layer; the index never writes to `.capcom/`.
**Regression guard.** Open with `_pragma=busy_timeout(5000)`; each refresh/rebuild is one `BEGIN IMMEDIATE` transaction. Db meta stores a hash of the parsed status map and rebuilds on mismatch, as for the schema version (ADR 0003: rules read the current category). Duplicate ids/aliases (item 3's detection) warn naming both paths and exclude both. A parse error is kept on its `files` row and re-warned every refresh until the file changes; text-mode warnings go to stdout after the result. A row is dirty when its mtime ≥ the start of the refresh that recorded it (racy-clean).
**Files:** `internal/index/index.go`, `internal/index/schema.go`, `internal/index/refresh.go`, `internal/index/index_test.go`, `internal/cli/reindex.go`, `internal/cli/reindex_test.go`, `internal/cli/root.go`
**Read first:** internal/index — Refresh, schema (files, issues, deps, aliases, fts); internal/issue/file.go — Parse, ETag; internal/issue/validate.go — Validate; internal/tree — Tree.Load, duplicate detection; internal/store/store.go — Store.GitDir; internal/config — Config.Statuses; internal/cli/root.go — per-command hook
**Tests:** first refresh indexes a temp tree; edit one file → only that row changes; same-size edit in one mtime tick seen; delete a file → rows gone; parse failure → warning on every refresh, others indexed; duplicate id across subtrees → warning, both excluded; status remapped in config → category follows; schema version bump → rebuild; FTS query finds a body word; reindex after deleting index.db rebuilds it.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/` ; `GOMEMLIMIT=2GiB go test -count=1 -run Reindex ./internal/cli/`
**Commit:** `feat(index): disposable SQLite index with incremental refresh and FTS`

## 6. Read commands: list, show, search

Depends on item 5.
**What:**
**Goal:** `capcom list` (no args: all non-closed issues) supports `--status`, `--category`, `--label`, `--no-label`, `--type`, `--priority`, `--parent`, `--plan <substring>`, `--planned/--unplanned`, `--all`, `--limit`; `capcom show <id|alias|prefix>...` prints each issue in full, `--kv` prints `ID= ISSUE_STATUS= CATEGORY= TITLE=` lines and `STATUS=ok|not_found` last per id; `capcom search <words>` is full-text. Not found is `not_found`, never empty output.
**Approach (assumed at the header base):** queries in `internal/index/query.go`; text renderers in `internal/cli/issue_render.go`: list = one line per issue `id  status  P<n>  title` (ids indented two spaces per tree depth), show = frontmatter as `key: value` lines then the body verbatim, no wrapping. JSON `kind` values `issue-list`, `issue`, `search`.
**Regression guard.** Each search word is quoted as an FTS5 string (`"` + word with `"` doubled + `"`) and the words ANDed. `STATUS=` is only the per-id outcome; the issue's status is `ISSUE_STATUS=` (keeps research-notes' skills contract: unambiguous status, distinct not-found).
**Files:** `internal/index/query.go`, `internal/index/query_test.go`, `internal/cli/list.go`, `internal/cli/show.go`, `internal/cli/search.go`, `internal/cli/issue_render.go`, `internal/cli/read_test.go`
**Read first:** internal/index/query.go — list filters, FTS search; internal/tree/resolve.go — Resolve (id, alias, prefix); internal/cli/output.go — Envelope, kv renderer; internal/cli/errors.go — Error codes; internal/cli/testharness_test.go — runCLI
**Tests:** each filter; show by alias and prefix; multi-id show with one missing → exit 3 and the found ones printed; `--kv` shape (`ISSUE_STATUS=`, `STATUS=` last); search ranking returns the matching issue; search a hyphenated slug, an apostrophe and a dotted filename; no wrapping.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/index/` ; `GOMEMLIMIT=2GiB go test -count=1 -run 'List|Show|Search' ./internal/cli/`
**Commit:** `feat(cli): list, show and search issues`

## 7. Ready rules and `capcom ready`

Depends on item 6.
**What:**
**Goal:** `capcom ready` lists exactly the Ready issues per `CONTEXT.md` (category `todo`; every dep `done` or `dropped`; no non-closed children; no `waiting` or `dropped` ancestor), hides issues carrying a `ready.hide` label unless `--show-hidden`, supports `--planned/--unplanned`, `--label`, `--no-label`, `--limit`, and warns when a satisfying dep is `dropped`. JSON entries carry `blocking` (unsatisfied deps) when `--all` lists non-ready todo issues with reasons.
**Approach (assumed at the header base):** pure function `issue.Ready(nodes, cfg) []Readiness` in `internal/issue/ready.go`, fed from index rows; `internal/cli/ready.go` renders it.
**Regression guard.** Each dep string resolves by exact id, then alias (never prefix), before it is judged (ADR 0002: every lookup resolves an Alias).
**Files:** `internal/issue/ready.go`, `internal/issue/ready_test.go`, `internal/cli/ready.go`, `internal/cli/ready_test.go`
**Read first:** CONTEXT.md — Ready, Closed; internal/issue/ready.go — Ready, Readiness; internal/index — deps, aliases tables; internal/config — Ready.Hide; internal/cli/ready.go — render, --all blocking
**Tests:** table cases for each rule incl. dep on dropped (ready + warning), parent with open child (not ready, child ready), parked grandparent hides subtree, custom status `triage: todo` is ready, hidden label, planned filter, a dep that resolves to nothing blocks with reason `missing`, a dep named by alias is satisfied.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run Ready ./internal/issue/ ./internal/cli/`
**Commit:** `feat(issue): ready rules by status category`

## 8. Write commands: create, update, log

Depends on item 6.
**What:** Recast at the regression check (2026-10-11).
**Goal:** `capcom create --id <slug> --title <t> [--parent <id>] [--type] [--priority] [--label]... [--plan] [--status] [--body-file <path|->]` writes a valid issue file (refusing an existing id or alias with `exists`, a bad slug with `invalid`, never generating an id); `capcom update <id>` changes `--title --status --type --priority --plan`, `--add-label/--remove-label`, `--body-append <text>`, `--body-append-file <path|->`, `--body-replace-old <s> --body-replace-new <s>` (old must match exactly once), guarded by optional `--if-match <etag>` (`conflict` on mismatch); `capcom log <id> <text>` appends `- YYYY-MM-DD <git user.name>: <text>` under `## Log`, creating the section. A status change into a closed category is refused here (use `close`).
**Approach (assumed at the header base):** mutations in `internal/issue/mutate.go` operate on a parsed `Issue` and return new bytes; CLI commands in `internal/cli/create.go`, `update.go`, `log.go`. Every write re-validates; output prints id and new etag.
**Regression guard.** Remove `--dep` from create and update — dependencies have exactly one writer, `capcom dep add|rm` in item 9, which owns id resolution and cycle checks. A status change out of a closed category is refused too (`invalid`, "use reopen"). The Log append (dated line, author = first non-empty of `$GIT_AUTHOR_NAME`, `git config user.name` via gitx, `$USER`) is one exported function in `internal/issue/mutate.go` used by `capcom log`, and item 9's reopen calls the same function — no second implementation. Yields to research-notes.md:18 (skills contract): label flags are `update --add-label/--remove-label`, not `--label +x/-x`. `show` exposes the etag: `ETAG=` in `--kv`, `etag` in the JSON `issue` data, an `etag:` line in text. `--body-append` is always literal text (no `@` syntax); `--body-append-file <path|->` (matching create's `--body-file`) appends a file or stdin.
**Files:** `internal/issue/mutate.go`, `internal/issue/mutate_test.go`, `internal/gitx/gitx.go`, `internal/gitx/gitx_test.go`, `internal/cli/create.go`, `internal/cli/update.go`, `internal/cli/log.go`, `internal/cli/issue_render.go`, `internal/cli/write_test.go`
**Read first:** internal/issue/file.go — Parse, serialise, ETag; internal/issue/validate.go — Validate; internal/tree/resolve.go — Tree.Resolve; internal/tree/tree.go — Tree.Load, PathFor;
internal/tree/atomic.go — WriteFileAtomic; internal/gitx/gitx.go — run wrapper; internal/cli/issue_render.go — show renderer, kv;
internal/cli/testharness_test.go — runCLI
**Tests:** create nested under parent creates `parent/` dir; duplicate id and alias collision; bad slug; body from stdin; label add/remove; replace-old matching 0 or 2 times fails; if-match conflict; log creates section then appends; author from each source (`$GIT_AUTHOR_NAME`, `git config user.name`, `$USER`) under ADR 0007's hermetic env; show's etag (`--kv`, JSON, text) round-trips into `update --if-match`; `--body-append "@alice"` appends literally; `--body-append-file` from a file and from stdin; update to `done` refused; update out of `done` refused.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/issue/ ./internal/gitx/` ; `GOMEMLIMIT=2GiB go test -count=1 -run 'Create|Update|Log' ./internal/cli/`
**Commit:** `feat(cli): create, update and log issues`

## 9. Close, reopen and dependencies

Depends on item 8.
**What:**
**Goal:** `capcom close <id>... --reason <text> [--status <closed-category status, default: the done category's default>]` sets status, `closed` (now, UTC) and `reason`, refusing without a reason; `capcom reopen <id> --reason <text> [--status]` returns to the default todo status, clears `closed`/`reason` and appends the reason to the Log; `capcom dep add|rm <id> <dep-id>` and `capcom dep list <id>` (deps and dependents) manage edges, refusing self-deps and any edge that creates a cycle (`invalid`, naming the cycle). `close --kv` prints per id `ID= STATUS=closed|already_closed|not_found`.
**Approach (assumed at the header base):** cycle detection as a pure func `issue.FindCycle(edges, from, to)` in `internal/issue/deps.go`; dependents derived from the index `deps` table (only one direction is stored).
**Regression guard.** close and reopen take their default status from item 2's first-listed-per-category rule (`done` and `todo` categories). `dep add|rm` is the only dep writer (item 8 has no `--dep`). Every dep string resolves through `tree.Resolve` (exact, then alias) to its canonical id before `FindCycle`, before it is stored, and when `dep rm` and `dep list` dependents match. reopen's Log append uses item 8's exported `internal/issue` Log function, and its Read first points there, not at `internal/cli/log.go`.
**Files:** `internal/issue/deps.go`, `internal/issue/deps_test.go`, `internal/cli/close.go`, `internal/cli/dep.go`, `internal/cli/close_test.go`, `internal/cli/dep_test.go`
**Read first:** internal/index/schema.go — deps(from_id, to_id), aliases; internal/tree/resolve.go — Tree.Resolve; internal/issue/validate.go — Validate closed/reason rule; internal/config/config.go — Config.Statuses; internal/issue/mutate.go — exported Log append under `## Log`; CONTEXT.md — Closed, Dependency
**Tests:** close multi-id with one already closed; close without reason fails; close as `dropped`; close under a config with no status named `done`; reopen clears fields and logs; dep cycle of length 3 refused; cycle and dependent cited through an alias; dep list shows both directions.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run 'Cycle|Deps' ./internal/issue/` ; `GOMEMLIMIT=2GiB go test -count=1 -run 'Close|Reopen|Dep' ./internal/cli/`
**Commit:** `feat(cli): close, reopen and dependency edges`

## 10. Rename and move: `capcom mv`

Depends on item 9.
**What:**
**Goal:** `capcom mv <id> <new-id>` renames an issue (file and its children directory), adds the old id to `aliases`, and rewrites every `deps` entry in the issue tree that named the old id or any of its aliases to the new id; `capcom mv <id> --parent <id|.>` moves the node (and its subtree) under another parent or to the root, refusing a move under its own descendant. Body text is never rewritten; plain filesystem rename, never `git mv`.
**Approach (assumed at the header base):** `internal/issue/move.go` computes the full set of file writes and renames first, then applies them (each write atomic); a failure before applying changes nothing.
**Regression guard.** Supersedes ADR 0001 ("a move is a `git mv`") and README "renaming one is a `git mv`": every prose site claiming capcom moves or renames via `git mv` (`grep -rn 'git mv' README.md CONTEXT.md docs/adr docs/format.md`) states the plain-rename behaviour. Renames apply in a fixed order; a failed later rename or write rolls back the applied ones.
**Files:** `internal/issue/move.go`, `internal/issue/move_test.go`, `internal/cli/mv.go`, `internal/cli/mv_test.go`, `docs/adr/0001-the-directory-tree-is-the-hierarchy.md`, `README.md`
**Read first:** internal/tree/tree.go — Tree.Load, Tree.PathFor, ChildrenOf; internal/tree/atomic.go — WriteFileAtomic; internal/issue/file.go — parse/serialise, aliases/deps key order; internal/tree/resolve.go — Tree.Resolve; internal/index/refresh.go — Refresh; docs/adr/0001-the-directory-tree-is-the-hierarchy.md
**Tests:** rename keeps alias chain (rename twice → two aliases); deps in other files rewritten; move with children; move under own child refused; rename onto an existing id or alias refused; injected failure on the second rename leaves the tree unchanged; `show <old-id>` resolves after rename.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run 'Move|Rename' ./internal/issue/` ; `GOMEMLIMIT=2GiB go test -count=1 -run Mv ./internal/cli/`
**Commit:** `feat(cli): rename and re-parent issues with aliases`
