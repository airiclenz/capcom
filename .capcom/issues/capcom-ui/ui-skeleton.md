---
title: capcom ui command, layout, keys and test harness
status: open
type: feature
priority: 2
created: 2026-10-15T20:03:24Z
---

The base every `capcom ui` view builds on (design `docs/design/tui.md`).

- The `ui` command in `internal/cli`, handing the opened store and index to a new `internal/ui`
  package; `capcom ui [issues|ready|knowledge] [id]` picks the tab and selection.
- Fails with exit code 1 when stdin or stdout is not a TTY; `--json` is refused.
- `go.mod` moves to go 1.25 and adds `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`,
  `charm.land/bubbles/v2` (with `golang.org/x/sys` v0.46).
- Layout: tab bar, sidebar, detail pane, status bar; below about 90 columns it collapses to
  drill-down (Enter opens, Esc returns).
- Keys: arrows and `j k h l`, Tab and `1 2 3`, Enter, Esc, `?` help, `q` quit.
- The status bar shows this worktree's Tree lock (free, held with purpose and heartbeat age, or
  stale).
- Index access serialised on one goroutine.
- The test harness: drive the model with key, mouse and window-size messages at a fixed size
  against a temp store built like `runCLI`'s, and compare `View()` with golden files.

## Acceptance

- `capcom ui` opens and quits cleanly; a non-TTY run exits 1 with an error.
- Golden tests cover the empty layout at wide and narrow sizes and the help screen.
- The status bar shows each lock state in a test.
- CI's cross build of the six release targets with `CGO_ENABLED=0` passes on the new go version.
