---
title: Missing config.yaml treated as format 3; init re-run raises the format
status: done
type: bug
priority: 3
deps: [config-owns-format]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

A missing `config.yaml` is treated as format 3, and re-running `init` raises the store format.
ADR 0009 and `docs/format.md` say a missing `config.yaml` or `format` key means format 1, but
`store`'s `loadConfig` uses `config.Default()` on a missing file, so `Store.Config.Format` is 3;
only `check` reads it today, and it compensates with a second stat (`missingConfigFormat`).
`Init` on an existing store with no `config.yaml` writes `DefaultFile`, which has an active
`format: 3`, and prints only "created .capcom/config.yaml". Severity: Low.

Trigger: a store with Issues and no `config.yaml`. Someone runs `capcom init --merge-driver` or
`--claude-hooks` to add a feature, and the format jumps from 1 to 3 with no warning; teammates on
v0.1.x or format-2 binaries then fail every command. It also breaks Init's promise that a re-run
is a no-op.

Wanted (decided 2026-10-13): `loadConfig` sets `Format` to `config.MissingFormat` (= 1) when the
file is missing, and `check`'s second stat goes. `createConfig` decides by whether this call
created the store root: on an existing store without `config.yaml`, `init` writes the template
with the `format:` line commented out, so the store stays at format 1; on a new store it keeps
writing `format: 3`.

## Acceptance

- A `store` test: opening a store with no `config.yaml` gives `Config.Format == 1`.
- A `cli` init test: a re-run on an existing store without `config.yaml` writes `config.yaml` with
  `format:` commented out, the store stays at format 1, and `check` still reports format 1.
- `init` on a new store still writes `format: 3`.
- `GOMEMLIMIT=2GiB go test ./internal/store/`, then `./internal/check/` and
  `./internal/cli/ -run Init`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-16.
