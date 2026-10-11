# capcom — agent guide

capcom is a git-native issue tracker and knowledge store for coding agents, written in Go. Issues
and knowledge notes are markdown files under `.capcom/`, versioned with the code; a disposable
SQLite index lives under the git directory and is never committed. The binary is built from
`cmd/capcom`; everything else is in `internal/`, and `cli.Run` drives every command in process.

## Where knowledge lives

- `CONTEXT.md` — the domain glossary (Issue, Tree, Alias, Ready, Lock, ...). Use its terms.
- `docs/adr/` — architectural decisions; read the relevant ADR before changing what it covers.
- `docs/design/` — research and design notes.
- `docs/plans/` — implementation plans; finished ones move to `docs/plans/archived/`.
- `docs/handoffs/` — handoff documents between sessions.

## Working rules

- Commit to `main`; no feature branches or pull requests unless asked.
- No AI attribution anywhere: no co-author trailers, no "generated with" footers, not in commits,
  pull requests, code or docs.
- Follow the house coding standards; format with `gofmt`.
- Every error message names the file, and the line when known.
- Tests use real temp dirs, SQLite and git, hermetically (ADR 0007); CLI tests go through the
  `runCLI` helper in `internal/cli` and do not run in parallel.

## Tests on the Raspberry Pi 5 host

Memory is tight. Run single packages or single tests with `GOMEMLIMIT=2GiB`, for example
`GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`. Never run `./...` with `-race`, and never run
two test commands at once. CI runs `go vet ./...` and `go test ./...` on Linux, macOS and Windows.

<!-- BEGIN CAPCOM -->
<!-- Managed by capcom: run "capcom agents" to refresh. Edits inside these markers are overwritten. -->
## Issues and knowledge: capcom

capcom is the issue register of record for this repository. Issues and Knowledge Notes are
markdown files under `.capcom/`, versioned with the code; track work there and nowhere else.

- `capcom ready`: the issues ready to work on, most urgent first
- `capcom show <id>`: one issue in full
- `capcom create --id <slug> --title <title>`: file a new issue
- `capcom close <id> --reason <text>`: close an issue
- `capcom know tree`: the outline of the Knowledge tree; `capcom know show <id>` reads a Note

Knowledge tree, top level:

- `cli-test-harness`: CLI tests run through runCLI, hermetic and never parallel
- `store-layout`: What capcom keeps where: files in .capcom, index and locks under the git dir
<!-- END CAPCOM -->
