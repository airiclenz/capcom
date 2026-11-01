---
title: ready prints nothing when every ready issue is hidden
status: open
type: bug
priority: 2
plan: 2026-11-01 - 00 - ready-notice-and-block-file-plan
created: 2026-11-01T19:45:09Z
---

When every Ready issue carries a label listed under `ready.hide`, `capcom ready` prints
nothing on stdout or stderr and exits 0. That output is byte-identical to a store where
nothing is Ready, so an agent concludes there is no work while Ready issues sit hidden.

Reported by seqcom (request `ready-empty-when-hidden`, run 20261031T1731Z), re-checked on
capcom 0.12.1 (run 20261101T1714Z); `internal/cli/ready.go` in 0.13.0 has no such notice.

Wanted: when the Ready set is non-empty but all of it is hidden, one line naming the count
and the hiding labels, with `--show-hidden` as the hint, for example
`ready: 12 hidden (bug, ops); nothing shown; --show-hidden lists them`.

## Why

The silent empty output tells an agent the opposite of the truth; the notice costs one line
and saves a run that stops on "nothing to do".

## Acceptance

- With a store where every Ready issue is hidden, `capcom ready` prints a notice naming the
  count and the hiding labels and pointing at `--show-hidden`, instead of an empty body.
- The notice does not appear when nothing is Ready, nor when at least one Ready issue is shown.
- `--show-hidden` output is unchanged; the notice is a result warning, so `--json` carries it in its warnings array.
- `capcom next` with nothing shown for the same reason carries the same notice and keeps its nothing-ready exit code.
