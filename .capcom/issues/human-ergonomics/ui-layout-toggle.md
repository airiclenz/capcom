---
title: No way to choose the stacked layout on a terminal that fits both
status: done
type: feature
priority: 2
plan: 2026-10-27 - 02 - ui-layout-toggle-plan
created: 2026-10-27T16:49:18Z
closed: 2026-10-27T17:32:19Z
reason: Resolved by plan 2026-10-27 - 02 - ui-layout-toggle-plan (all 8 items done; archived in e02c59e)
---

`capcom ui` picks its layout from the terminal's size alone: side by side (the split layout, a
vertical `│` separator) at 90 columns or more, the sidebar above the detail pane (the stacked
layout, a horizontal `─` rule) below that when the body has `stackBodyMinRows` rows or more, and
one pane at a time otherwise (`internal/ui/layout.go`). On a terminal big enough for both the
split layout always wins; the user cannot choose the stacked one.

Wanted: a manual switch between the split and the stacked layout, offered only when the
terminal fits both, remembered across runs.

**When it is offered.** Both layouts fit when the terminal is at least `narrowWidth` (90)
columns wide and the body has at least `stackBodyMinRows` rows (the terminal at least 25 rows):
the existing stacked threshold, unchanged, at any width. Otherwise the layout stays automatic as
today and the switch is not offered.

**The choice.** A layout preference, split or stacked. With no stored preference (first run, or a
state file written before this) the split layout applies, as today. When the terminal shrinks so
only one layout fits, that layout is used, but the stored preference is kept and applies again
once both fit; a forced fallback never rewrites it. Each layout keeps its own divider position
(`sidebar_ratio`, `stacked_ratio`) as today.

**Persistence.** The preference is written to the global state file
`<os.UserConfigDir>/capcom/state.yaml` beside `sidebar_ratio` and `stacked_ratio`
(`internal/userconfig/state.go`, ADR 0014), one choice for every worktree. It is written only
when the user toggles; a failed save shows through the existing state-file notice.

**The `v` key.** `v` toggles the layout while both fit. It is inert while the switch is not
offered, while the help screen is open, and while a confirmation or text prompt replaces the
status bar. The help screen lists it.

**The status-bar button.** A mouse-clickable button at the right edge of the status bar, last,
after the key hints (`? help · q quit` then the button). It shows the layout a click switches
to, not the active one: ` ─ ` while the split layout is active, ` │ ` while the stacked one is.
No bracket characters: the glyph is padded by one space on each side (3 columns) and set on a
background colour, a new theme style defined for every colour scheme (like the lock badge),
otherwise muted like the hints. Its columns are reserved like the hints', so a long status line
is cut before it. A left click on it toggles the layout exactly as `v` does. No flash notice: the
layout change is the feedback.

The button is not drawn (and takes no columns) when the switch is not offered, while the help
screen covers the panes, and while a confirmation or text prompt replaces the status bar. It is
drawn while the filter line or the status picker is open.

## Why

On a big terminal some users prefer the sidebar above the detail pane, with full-width lines in both; today only shrinking the terminal gets them that layout.

## Acceptance

- On a terminal of at least 90 columns and 25 rows, `v` and a click on the status-bar button switch between the split and the stacked layout
- Below either threshold the button is not drawn, takes no columns, and `v` does nothing; the layout is chosen by size as today
- The button shows ` ─ ` while split and ` │ ` while stacked, on a background colour with no bracket characters, at the right edge after the key hints, in every colour scheme
- The button is hidden while the help screen or a confirmation/text prompt is open, and shown with the filter line or status picker open
- The chosen layout is saved to state.yaml on toggle and restored on the next `capcom ui` start; with none stored the split layout applies
- Shrinking the terminal to fit one layout and growing it back restores the stored choice; the fallback never rewrites state.yaml
- The help screen lists `v`; tests cover the toggle, the click, visibility at each threshold, and the state round-trip
