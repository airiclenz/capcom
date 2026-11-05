---
title: No fixture store per older store format proves capcom still reads it
status: done
type: task
priority: 2
labels: [maturity, testing, compat]
plan: 2026-11-05 - 00 - p2-maturity-plan
created: 2026-11-04T18:27:23Z
closed: 2026-11-05T10:56:22Z
reason: "Resolved by plan '2026-11-05 - 00 - p2-maturity-plan' (archived in 1a6b7b0) items 6-7: fixture stores for formats 1-6 read by every listed command, a newer-format store refused by each, and a guard requiring a fixture for FormatVersion."
---

`internal/config/config.go` defines `FormatVersion = 6` and documents what each format added
(2: `related` on issues; 3: `retired` and `superseded-by` on Notes; 4: `agents.guides`;
5: `standing` on issues; 6: `agents.file`). Unit tests check that the config parser accepts
each format number, but no test opens a whole store written at an older format and runs the
commands on it, so a change can silently stop reading stores in the wild.

Add one fixture store per format, 1 to 6, under `internal/cli/testdata/stores/format-<n>/`,
each a `.capcom/` tree (config.yaml declaring that format, a handful of issues with deps,
parents and Log entries, a few Notes) using only the keys that format allows. Build each by
hand from the format descriptions above, or by checking out the release tag that introduced
the format and running its `capcom init` and writes.

A test copies each fixture into a temp git repository and asserts, through `runCLI`:
`capcom check` exits 0; `capcom reindex`, `list --all --json`, `ready`, `prime` and
`know tree` succeed and return the fixture's issues and Notes; a write that needs no newer
format (`capcom log`) leaves `format:` unchanged; and a write that needs a newer one (a
`standing` issue on a format-4 store) behaves as the store-format rules say at the time
(today: raises the format with a warning; after `explicit-format-upgrade`, its new behaviour).

Also add a fixture declaring `format: 7` (one beyond `FormatVersion`) and assert every
command refuses it with the message from `internal/config/parse.go`
(`store format 7 is newer than this capcom reads (up to 6); upgrade capcom`).

When a future change bumps `FormatVersion`, a test should fail until a fixture for the new
format exists: assert that `testdata/stores/format-<FormatVersion>` is present.

## Why

Old stores live in other people's repositories; capcom must keep reading every format it
ever wrote, and only a test against real old stores proves that.

## Acceptance

- Fixture stores for formats 1 to 6 exist and every listed command succeeds on each.
- A format-7 fixture is refused by every command with the documented message.
- Raising `FormatVersion` without adding a fixture makes a test fail.
