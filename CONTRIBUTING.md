# Contributing to capcom

capcom keeps its own project knowledge as Notes under [`.capcom/knowledge/`](.capcom/knowledge/).
This guide points to them rather than repeating them. With capcom installed, `capcom know tree`
lists them and `capcom know show <id>` reads one; without it, read the markdown files directly.

## Build and test

capcom needs the Go version named in [`go.mod`](go.mod).

```sh
go build ./cmd/capcom
go vet ./...
go test ./...
```

CI runs `go vet ./...` and `go test ./...` on Linux, macOS and Windows. On a machine with little
memory, such as a Raspberry Pi, test one package at a time with `GOMEMLIMIT=2GiB`, and never run
`./...` with `-race` (see [`AGENTS.md`](AGENTS.md)).

## Conventions and tests

- Code conventions, formatting and error messages: Note
  [`coding-conventions`](.capcom/knowledge/coding-conventions.md) and the Notes below it.
- Tests: Note [`cli-test-harness`](.capcom/knowledge/cli-test-harness.md). CLI tests run every
  command in process against a real temp git repository and SQLite index, and never run in
  parallel.
- Where the glossary, design decisions, plans and handoffs live: Note
  [`docs-layout`](.capcom/knowledge/docs-layout.md). Use the glossary's terms in code, docs and
  messages.
- The user manual lives in [`docs/site/`](docs/site/). Its CLI reference under
  [`docs/site/reference/cli/`](docs/site/reference/cli/) is generated: after changing a command,
  a flag or its help text, run `go run ./cmd/gendocs` and commit the result.
  `TestReferenceUpToDate` fails while the committed reference is stale.

## Issues

Work is tracked as capcom issues under [`.capcom/issues/`](.capcom/issues/); `capcom ready` lists
the ones ready to work on, and `capcom close <id> --reason <text>` closes one when the work is
done. To report a bug without a change, open a GitHub issue.

## Commits and the changelog

- Commit messages follow Conventional Commits: `feat(know): ...`, `fix(check): ...`, `docs: ...`.
- No AI attribution in commits, pull requests, code or docs: no co-author trailers, no
  "generated with" footers.
- A change users can notice adds a line under `## [Unreleased]` in [`CHANGELOG.md`](CHANGELOG.md)
  ([Keep a Changelog](https://keepachangelog.com/en/1.1.0/)). Write it for users: what now
  behaves differently, with the exact command, flag or message.

## Releases

Releases are cut only by the maintainer, with the `release-capcom` skill. Never tag, bump
`VERSION` or edit a CHANGELOG release heading by hand; the release tool does that. See Note
[`release-process`](.capcom/knowledge/release-process.md) and
[`docs/releasing.md`](docs/releasing.md).

## Security

Report vulnerabilities privately, as described in [`SECURITY.md`](SECURITY.md).
