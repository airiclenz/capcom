---
title: Resize the sidebar by dragging the divider
status: open
type: feature
priority: 2
created: 2026-10-16T13:54:30Z
---

In the split layout the sidebar is a fixed third of the terminal (28–48 columns). Long ids and
titles are cut, and on a wide terminal the detail pane takes space the sidebar could use. The
user wants to set the sidebar's width with the mouse, within limits, and keep it across runs.

Wanted:

- Drag the divider (the `│` column and the blank margin either side, on any body row) with the
  left button; the panes reflow live and the divider and its joints show in the accent style
  while the button is held.
- Limits: the sidebar never under 20 columns, the detail pane never under 40.
- `<` and `>` shrink and grow the sidebar by 4 columns; double-clicking the divider returns it
  to the default width.
- The width, in columns, persists in a machine-written state file
  `<os.UserConfigDir>/capcom/state.yaml`; `config.yaml` is never rewritten (ADR 0011).

## Acceptance

- A drag, `<`/`>` and a double-click change the sidebar width within the limits, covered by
  `internal/ui` tests.
- The width survives a restart through `state.yaml`; a failed write is a status-bar notice.
- The help screen, `docs/site/getting-started/terminal-ui.md`, `docs/design/tui.md` and
  `docs/site/reference/file-format.md` describe the feature and the state file; a decision
  record explains the state file.
