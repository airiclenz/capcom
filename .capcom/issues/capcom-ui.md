---
title: "capcom ui: a terminal UI for issues and Knowledge"
status: done
type: feature
priority: 2
created: 2026-10-15T20:03:23Z
closed: 2026-10-17T14:20:34Z
reason: All 28 children closed; shipped in v0.8.0; CI green on Linux, macOS and Windows; the manual and CLI reference describe capcom ui. Remembering the tab and selection is filed separately as ui-remember-tab.
---

A terminal UI, `capcom ui [issues|ready|knowledge] [id]`, for a person working next to coding
agents: watch the issues and Knowledge change while agents work, and make the small changes a
supervisor makes (claim, status, close, reopen, a log line, `$EDITOR`) without leaving the
terminal. It ships inside the capcom binary through the existing release channels.

The design is `docs/design/tui.md`; the decisions behind it are ADR 0011 (per-user config),
ADR 0012 (shared write path) and ADR 0013 (the UI checks the Tree lock). The children below are
the work, each with its own acceptance.

## Acceptance

- Every child issue is closed.
- `capcom ui` opens on the Ready tab in a real terminal on Linux, macOS and Windows, and CI
  (`go vet ./...`, `go test ./...` on all three) passes.
- The manual and CLI reference describe `capcom ui`.
