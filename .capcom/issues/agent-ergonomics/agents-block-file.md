---
title: The capcom block cannot live outside AGENTS.md
status: open
type: feature
priority: 2
plan: 2026-11-01 - 00 - ready-notice-and-block-file-plan
created: 2026-11-01T19:45:09Z
---

capcom keeps its managed block in `AGENTS.md`, and the config's `agents:` section accepts
only `guides`. A workspace whose `AGENTS.md` is immutable (`chattr +i`, or a directory capcom
may not write) cannot carry the block: `capcom know add` of a top-level Note, `know update`,
`know mv` and `know rm` fail to refresh it with only a warning, and `capcom check` then exits
1 with "the capcom block is out of date". So such a workspace goes without the block and
its generated Knowledge outline.

Reported by seqcom (request `capcom-block-target`, capcom 0.7.0), reproduced on 0.9.0 and
0.12.1 (runs 20261018T2200Z, 20261030T1738Z, 20261101T1714Z): with the repo dir mode 555,
`know add` warns `open .AGENTS.md.tmp-…: permission denied; the capcom block was not
refreshed` and `check` exits 1.

Wanted: a config key naming the file that holds the block (for example
`agents.file: .capcom/AGENTS-capcom.md`), which `AGENTS.md` links to, so the block lives in
a file the agent may write while `AGENTS.md` stays locked.

## Why

A locked `AGENTS.md` is a reasonable guard for an autonomous agent's own instructions; today
it forces a choice between that guard and capcom's generated outline.

## Acceptance

- A config key names the file holding the managed block; `capcom agents`, the Note writes
  that refresh it, `capcom agents --check` and `capcom check` all use that file.
- With the key set and `AGENTS.md` immutable, `capcom know add` refreshes the block without a
  warning and `capcom check` passes.
- `capcom check` reports when `AGENTS.md` does not reference the configured file.
- Without the key, behaviour is unchanged.
