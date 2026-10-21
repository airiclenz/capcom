---
title: Name the real minimum apogee version for --apogee-hooks
status: open
type: task
priority: 3
created: 2026-10-21T14:40:40Z
---

`capcom init --apogee-hooks` and its docs name the minimum apogee only as a placeholder, "an apogee release after v0.27.0", because apogee had not yet cut the release carrying `advise:` on `session-started` and the `compaction-finished` moment (apogee main 4b290a8b) when the feature landed. Once apogee cuts that release, every placeholder should name the real version, so a user can check their apogee against it.

Placeholder text today:

- `internal/cli/init.go:53` (initLong, "it needs an apogee release after v0.27.0")
- `internal/cli/init.go:72` (apogeeVersionNote, printed after the added/present line)
- `internal/cli/init_test.go:631` (wantApogeeVersion)
- `docs/site/reference/cli/capcom_init.md:14` (generated from initLong)
- `docs/site/agents/agent-setup.md:175` and `docs/site/agents/agent-setup.md:204`
- `CHANGELOG.md:18` (the `[Unreleased]` entry, while it is still unreleased)

## Acceptance

- `capcom init --apogee-hooks` prints `needs apogee <version> or later` (or equivalent) with the real released apogee version, and `capcom init --help` names it too.
- `docs/site/agents/agent-setup.md` and `docs/site/reference/cli/capcom_init.md` name the same version; `grep -rn 'after v0.27.0' internal docs/site` prints nothing.
- `go test ./internal/cli` passes.
