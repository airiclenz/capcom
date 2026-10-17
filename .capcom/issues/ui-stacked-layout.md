---
title: capcom ui stacks the panes on a narrow, tall terminal
status: open
type: feature
priority: 2
created: 2026-10-17T18:20:20Z
---

Below `narrowWidth` (90 columns) `capcom ui` stops showing both panes: the sidebar fills the
screen and Enter swaps in the detail pane (`internal/ui/layout.go`). On a terminal that is narrow
but tall, a split pane in a portrait window for example, there is room for both, one above the
other, yet the person has to keep swapping between the list and what it selects.

Wanted:

- **Automatic flip.** When the terminal is narrower than the side-by-side layout needs but tall
  enough for a list and a detail pane, the panes stack: the sidebar on top, a horizontal divider,
  the detail pane below. The thresholds (the fewest rows each pane keeps) are a design call for the
  plan. A terminal that is narrow and too short keeps today's one-pane-at-a-time layout, and a
  wide one keeps the side-by-side split. Resizing switches between the layouts live.
- **Moving the horizontal divider.** It moves like the vertical one: a mouse drag, and the keys
  that move the divider (`<` and `>` today, or keys the plan picks) move it a step.
- **Its own remembered position.** The horizontal divider's position is a fraction of the body's
  height and is stored separately from `ui.sidebar_ratio`, for example as `ui.stacked_ratio` in
  `state.yaml`. A width fraction and a height fraction mean different things, so one shared
  value would put the divider in the wrong place in one of the two layouts. Adding a key does
  not break anything: `LoadState` ignores unknown keys, so an older capcom reads the new file and
  a newer one reads the old file and uses the default. ADR 0014 gains the key.

## Acceptance

- At a narrow, tall size the panes are stacked, with a horizontal rule between them. At a narrow,
  short size the one-pane layout is shown, and at a wide size the side-by-side split. Each case
  has a test, and so does a resize that crosses the boundary.
- Dragging the horizontal divider and pressing the divider keys move it within the bounds the
  plan sets. Its position is saved to its own key and restored on the next start, and the
  vertical divider's `sidebar_ratio` is unchanged by it.
- A `state.yaml` without the new key loads, with the stacked layout using its default.
- The manual's `capcom ui` page and ADR 0014 describe the stacked layout and the new key.
- A CHANGELOG line under Unreleased.
