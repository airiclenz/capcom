# **capcom** - agent guide

capcom is a git-native issue tracker and knowledge store for coding agents, written in Go. Issues
and knowledge notes are markdown files under `.capcom/`, versioned with the code; a disposable
SQLite index lives under the git directory and is never committed. The binary is built from
`cmd/capcom`; everything else is in `internal/`, and `cli.Run` drives every command in process.
`cmd/release` (the release tool) and `cmd/gendocs` (generates the CLI reference) are dev tools,
never shipped.

Project knowledge lives in capcom's Knowledge tree (listed at the end of this file). Before
starting work, run `capcom know tree --depth 2`, then `capcom know tree <id>` to open a branch,
and `capcom know show <id>` on every Note whose title matches the task. `--depth 2` prints the
top level and its direct children only.

## Working rules

- Commit to `main`; no feature branches or pull requests unless asked.
- No AI attribution anywhere: no co-author trailers, no "generated with" footers, not in commits,
  pull requests, code or docs.
- Never tag, bump `VERSION` or edit a CHANGELOG release heading by hand (Note `release-process`).
- When a plan under `docs/plans/` is saved, link every issue it will close with
  `capcom update <id> --plan "<plan basename without .md>"`, so `capcom prime` lists it as planned.

## Tests on the Raspberry Pi 5 host

Memory is tight. Run single packages or single tests with `GOMEMLIMIT=2GiB`. Never run `./...`
with `-race`, and never run two test commands at once. CI runs `go vet ./...` and
`go test ./...` on Linux, macOS and Windows.

<!-- BEGIN CAPCOM -->
<!-- Managed by capcom: run "capcom agents" to refresh. Edits inside these markers are overwritten. -->
## Issues and knowledge: capcom

capcom is the issue register of record for this repository. Issues and Knowledge Notes are
markdown files under `.capcom/`, versioned with the code; track work there and nowhere else.

Without a `capcom` binary, install it from https://github.com/airiclenz/capcom#install.

- `capcom next`: the most urgent ready issue, in full
- `capcom ready`: the issues ready to work on, most urgent first
- `capcom show <id>`: one issue in full, with what blocks it
- `capcom context <id>`: the issue with its deps, plan and the Notes that apply; run it before you start
- `capcom start <id>`: claim it and take the Tree lock before you start
- `capcom log <id> <text>`: note progress as work lands
- `capcom close <id> --reason <text>`: close an issue; it names the issues this made ready
- `capcom finish <id> --reason <text>`: close it and free the Tree lock; it names the issue to work on next
- `capcom reopen <id> --reason <text>`: undo a close
- `capcom create --id <slug> --title <title> [--dep <id>]`: file a new issue
- `capcom dep add <id> <dep-id>`: make an issue wait for another; `capcom dep list <id>` shows both sides
- `capcom list --tree --depth 2`: the outline of the issues
- `capcom know tree`: the outline of the Knowledge tree; `capcom know show <id>` reads a Note

Each command's flags are in `capcom <cmd> --help`.

An issue's body says what is wrong or wanted, why, and how to tell it is done (`## Acceptance`),
readable on its own; a plan says how and in what order.

Where things are: an issue is `.capcom/issues/<id>.md`, and its children sit in the directory
`.capcom/issues/<id>/` beside it; Notes follow the same shape under `.capcom/knowledge/`.
Prefer capcom commands; after a hand edit run `capcom check`.

A standing issue (`standing: true` in its file) only groups its children: never claim or close it.

Tree lock: before a change that spans several files or commits, it is recommended to claim this
worktree with `capcom lock acquire --purpose <text>`. capcom keeps the Lock token for the session:
running the same acquire again resumes the lock after a restart or compaction and prints the token
again (or pass `--token <token>`).
`capcom start <id>` takes the lock for an issue, and running the same start again resumes it too.
`capcom lock touch --token <token>` renews it during long work,
`capcom lock release --token <token>` frees it when done. When `capcom lock check --token <token>` exits 3, or
`capcom lock check` does while you hold no token, another session holds this worktree: ask the user
before editing it.

Knowledge tree, top level:

- `cli-test-harness`: CLI tests run through runCLI, hermetic and never parallel
- `coding-conventions`: Before writing code: house conventions, gofmt and capcom-specific rules
- `decisions`: Decision records: why capcom works the way it does, one Note per record with its sections as children
- `docs-layout`: Before reading or changing a design: where the glossary, ADRs, plans and handoffs live
- `glossary`: Domain glossary: the terms to use in code, docs and messages, one Note per term
- `issue-workflow`: Before filing, starting or closing an issue: its body, claiming it, dependencies
- `release-process`: Before cutting a release: use release-capcom, never tag or bump by hand
- `store-layout`: What capcom keeps where: files in .capcom, index and locks under the git dir
<!-- END CAPCOM -->
