---
title: Keep the sidebar/content ratio when the terminal is resized
status: open
type: feature
priority: 2
plan: 2026-10-16 - 01 - ui-drag-divider-plan
created: 2026-10-16T15:39:56Z
---

In the split layout of `capcom ui` the sidebar keeps the column width the user set, by dragging
the divider or with `<` and `>`, whatever the terminal's width. When the terminal window is made
wider or narrower, the sidebar stays the same number of columns and only the detail pane grows or
shrinks, so the split the user chose is lost. `state.yaml` stores the width in columns, so a
restart in a terminal of a different width loses the proportion too.

Wanted (decided by the user, 2026-10-16):

- The width the user sets is kept as a ratio: the sidebar's columns over the terminal's columns.
  When the terminal's width changes, the sidebar takes that ratio of the new width, so the
  sidebar/detail proportion holds.
- `state.yaml` stores that ratio, not a column count, so the proportion also holds across
  restarts in a terminal of a different size. The `ui.sidebar_width` key, which no release ever
  wrote, is no longer read.
- `<` and `>` and the divider drag still move the divider in columns; the ratio they store is the
  one the resulting width gives.
- A double-click on the divider still returns to the default width (a third of the terminal,
  28-48 columns) and clears the stored ratio.
- The limits still apply on every frame (sidebar at least 20 columns, detail pane at least 40)
  without rewriting the stored ratio.
- The resize concerns the terminal's width only; its height does not move the divider.

## Acceptance

- After the user sets a width, a `tea.WindowSizeMsg` to a different width gives the sidebar the
  same ratio of the new width (rounded to a column, clamped to the limits), covered by an
  `internal/ui` test; the stored ratio is unchanged by the resize.
- `state.yaml` holds `ui.sidebar_ratio`; a saved state loads back equal, covered by
  `internal/userconfig` tests, and a file holding only `ui.sidebar_width` starts on the default.
- The user docs (`docs/site/reference/file-format.md`, `docs/site/getting-started/terminal-ui.md`),
  `docs/design/tui.md` and ADR 0014 describe the ratio; `capcom check` is clean.
