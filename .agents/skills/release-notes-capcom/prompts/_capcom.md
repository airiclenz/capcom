# capcom facts — the repository this run describes

A shared include, verified against the tree when the skill was written. Read it
after `_shared.md`; when the tree contradicts a line here, trust the tree and
say so on your `FLAG` line.

## The baseline (`BASELINE`) and the diff base

The file is JSON. Use `baseline.tag` (the last **published** GitHub release,
e.g. `v0.3.1`), `baseline.published` (its publish time), `baseline.range`
(`<tag>..HEAD`) and `baseline.commits`. Ignore `gate`, `steps` and
`highestTag`. `version` is the checkout's `VERSION` file with a `v` in front;
it is never the release's number — that is the dispatch's `VERSION:` key,
`unreleased` when absent.

**No published release yet.** Then `baseline.tag` and `baseline.published` are
`""` and `baseline.range` is `HEAD`: the baseline is the **whole history**.
This is a normal state, never a reason to stop. In it:

- **The log range** is `baseline.range` either way: `git log HEAD` lists every
  commit when there is no tag.
- **The diff base** is `baseline.tag`, or with no tag the root commit the
  dispatch passes as `ROOT:` (from `git rev-list --max-parents=0 HEAD`).
  Every `git diff <diff base>` compares that commit with the working tree;
  `git diff <diff base> HEAD` covers the committed range alone.
- Where a prompt writes `<baseline.tag>` into a heading or title, write
  `the first commit` instead.

## Changelog

- File: `CHANGELOG.md`, in the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
  shape. Unreleased heading, literally: `## [Unreleased]`.
- Release heading shape: `## [X.Y.Z] - YYYY-MM-DD` (no `v`, a hyphen), e.g.
  `## [0.3.1] - 2026-11-02`. Entries collect under `[Unreleased]` until a
  release cut rolls them under a new heading.
- Under each heading the entries are grouped into `### Added`, `### Changed`
  and `### Fixed` subsections (`### Deprecated`, `### Removed` and
  `### Security` are read the same way when present).
- An entry is one `- ` bullet: plain prose naming commands, flags and files in
  backticks, often several sentences long. There is no bold lead.

## Commit conventions

Conventional commits, `type(scope): subject` — types `feat`, `fix`, `docs`,
`ci`, `build`, `refactor`, `test`, `chore`; scopes are package or area names
(`cli`, `issue`, `index`, `release`, …). Bookkeeping that ships nothing:
`chore(plans)`, `docs(plans)`, `docs(plan)`, `chore(issues)`,
`chore(release)`, `chore(version)`, and any commit touching only
`.capcom/issues/`, `.capcom/knowledge/` (except the glossary,
`.capcom/knowledge/glossary/`), `docs/plans/` or `docs/handoffs/`.
Ticket ids are capcom issue ids: the slug of a file under `.capcom/issues/`
(e.g. `release-pipeline`), with no prefix. No AI attribution trailers. Plans
live in `docs/plans/`, decision records in `.capcom/knowledge/decisions/`.

## User surfaces — where a change becomes visible

| Surface | Paths |
|---|---|
| CLI commands & flags | `internal/cli/root.go`, `internal/cli/*.go` (one file per command; skip `_test.go`) |
| Output modes & exit codes | `internal/cli/output.go`, `internal/cli/output/errors.go` |
| Config keys & defaults | `internal/config/config.go` (`Default()`, the `.capcom/config.yaml` that `capcom init` writes) |
| File formats | `docs/format.md` (issue files, Note files, the tree, `config.yaml`, the lock record, the Managed block) |
| Agent integration | `internal/agents/block.go` (the AGENTS.md Managed block `capcom agents` writes), `internal/cli/prime.go` (the `capcom prime` Briefing), `internal/cli/init.go` (the Claude Code hooks `--claude-hooks` adds) |
| Docs | `README.md`, `docs/format.md`, `docs/migrating-from-beads.md`, `.capcom/knowledge/glossary/` |
| Install & packaging | `.github/workflows/`, `cmd/release/templates/` |
| Emitted strings | `internal/cli/` user messages and usage text, `internal/agents/block.go`, `internal/check/` (what `capcom check` reports) |

`VERSION` moves with every release and is not a surface.
