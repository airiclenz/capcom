---
title: Per-user config file, created only by capcom ui
status: done
type: feature
priority: 2
created: 2026-10-15T20:03:23Z
closed: 2026-10-16T10:28:24Z
reason: Resolved by the capcom-ui plan (docs/plans/archived/2026-10-16 - 00 - capcom-ui-plan.md, archived in 65127ff)
---

capcom has no per-user config; the UI theme needs one (ADR 0011). Add
`<os.UserConfigDir>/capcom/config.yaml` with a `ui:` section, loaded by a new package (working
name `internal/userconfig`).

Rules from ADR 0011: only `capcom ui` reads it; the first `capcom ui` start creates it with every
key as a commented default; no other command reads or creates it. Unknown keys are a warning, not
an error. Malformed YAML or a wrong-typed value is an error with file and line, after which the UI
runs on defaults. A directory that cannot be written is a warning, and the UI runs on defaults.

## Acceptance

- Loading returns the `ui` settings (theme preset, colour overrides) with defaults filled in.
- First load with no file creates it, all keys commented; a second load leaves it unchanged.
- No command other than `capcom ui` touches the path: a test runs a sample of commands with
  `UserConfigDir` pointed at a temp dir and finds it empty.
- Tests cover an unknown key (warning), bad YAML (error with line), and an unwritable directory.
