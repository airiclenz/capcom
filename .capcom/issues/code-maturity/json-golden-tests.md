---
title: No golden test pins the shape of each --json kind and the exit codes
status: open
type: task
priority: 2
labels: [maturity, testing, compat]
plan: 2026-11-05 - 00 - p2-maturity-plan
created: 2026-11-04T18:27:23Z
---

Agents parse capcom's `--json` envelope (`{"schema":1,"kind":...,"data":...,"elsewhere":[],
"warnings":[]}`, documented in `docs/site/reference/json-output.md`) and branch on its exit
codes. A refactor that renames a field, changes `[]` to `null` or shifts an exit code breaks
every agent integration without failing a test written for something else.

Add golden tests in `internal/cli` (driven by `runCLI`, Note `cli-test-harness`) that, against
a fixed seeded store with a fixed clock and fixed git identity, run one representative command
per `kind` listed in `json-output.md` (at least `issue`, `issue-write`, `close`, `apply`,
`ready`, `next`, `prime`, `context`, `recap`, `check`, `version`, `search`, `know-search`,
`error`, and the partial-failure form with both `data` and `error`) and compare stdout with a
file under `internal/cli/testdata/json/<kind>.golden.json`. Normalise only what is truly
volatile (etags, timestamps, temp paths) with a documented replacement such as `<ETAG>`.
Do the same for the `--kv` form of the commands that offer it
(`testdata/kv/<command>.golden`).

Add a table test of exit codes: one row per documented exit code (success, invalid input,
not found, `next` with nothing Ready = 4, lock held = 3, and the rest the reference lists),
each with the command that produces it.

Provide an `-update` flag (`go test ./internal/cli -run Golden -update`) that rewrites the
golden files, following how `internal/ui/testdata/golden` is maintained, so an intended
change is one reviewed diff.

## Why

The JSON envelope and exit codes are capcom's API for agents; a golden test turns an
accidental change into a failing test and an intended one into a visible diff.

## Acceptance

- Every `kind` named in `docs/site/reference/json-output.md` has a golden file and a test comparing against it.
- Every exit code the reference documents has a table row that produces it.
- Renaming one JSON field in the code makes a golden test fail; `-update` regenerates the files.
