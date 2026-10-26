---
title: Group commands accept --json and print prose with exit 0
status: open
type: bug
priority: 2
created: 2026-10-26T20:06:45Z
---

Group commands accept `--json` and print prose with exit 0, which is the worst combination for a
script that parses JSON.

    capcom know --json -> full help text, exit 0
    capcom dep --json  -> full help text, exit 0
    capcom lock --json -> full help text, exit 0
    capcom        --json -> full help text, exit 0

`capcom --help` says "Every command except capcom ui prints plain text, or with --json a
versioned JSON envelope". The reference exempts only `help` and `completion`; `know`, `dep`,
`lock` and bare `capcom` are tracker commands.

`capcom ui --json` refuses cleanly with a JSON error and exit 2, so the pattern to copy exists.

## Acceptance

- A command that cannot produce a JSON envelope refuses `--json` with a JSON error and a non-zero
  exit, as `capcom ui` does, or emits an envelope.
- Bare `capcom --json` obeys the same rule.
- A test asserts no `--json` invocation writes non-JSON to stdout with exit 0.
