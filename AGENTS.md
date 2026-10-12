# capcom — agent guide

capcom is a git-native issue tracker and knowledge store for coding agents, written in Go. Issues
and knowledge notes are markdown files under `.capcom/`, versioned with the code; a disposable
SQLite index lives under the git directory and is never committed. The binary is built from
`cmd/capcom`; everything else is in `internal/`, and `cli.Run` drives every command in process.
`cmd/release` is the release dev tool and is never shipped.

Project knowledge lives in capcom's Knowledge tree (listed at the end of this file). Run
`capcom know tree` before starting work and `capcom know show <id>` on every Note whose title
matches the task.

## Working rules

- Commit to `main`; no feature branches or pull requests unless asked.
- No AI attribution anywhere: no co-author trailers, no "generated with" footers, not in commits,
  pull requests, code or docs.
- Never tag, bump `VERSION` or edit a CHANGELOG release heading by hand (Note `release-process`).

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

- `capcom ready`: the issues ready to work on, most urgent first
- `capcom show <id>`: one issue in full
- `capcom create --id <slug> --title <title>`: file a new issue
- `capcom close <id> --reason <text>`: close an issue
- `capcom know tree`: the outline of the Knowledge tree; `capcom know show <id>` reads a Note

Knowledge tree, top level:

- `cli-test-harness`: CLI tests run through runCLI, hermetic and never parallel
- `coding-conventions`: Before writing code: house conventions, gofmt and capcom-specific rules
- `decisions`: Decision records: why capcom works the way it does, one Note per record with its sections as children
- `docs-layout`: Before reading or changing a design: where the glossary, ADRs, plans and handoffs live
- `glossary`: Domain glossary: the terms to use in code, docs and messages, one Note per term
- `release-process`: Before cutting a release: use release-capcom, never tag or bump by hand
- `store-layout`: What capcom keeps where: files in .capcom, index and locks under the git dir
<!-- END CAPCOM -->
