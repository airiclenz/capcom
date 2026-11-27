---
title: Colour the priority in capcom ui by its level
status: done
type: feature
priority: 2
labels: [ui]
created: 2026-11-27T12:06:12Z
closed: 2026-11-27T13:09:11Z
reason: P<n> coloured by priority-0..4 roles in Ready/Issues/Closed rows, dependency view and detail header; Closed rows gain P<n>
---

capcom ui prints an issue's priority as plain `P<n>` text: in the Ready and Issues sidebar rows
and the dependency view (all drawn by `readyRow` in `internal/ui/ready.go`), and in the detail
header line (`status: … · P<n>`, `internal/ui/detail.go`). The Closed tab's rows
(`closedRow` in `internal/ui/closed.go`, `date  id  type  title`) show no priority at all. Every
priority looks the same, so an urgent issue does not stand out in a long list.

The colouring applies in every tab that shows issues: Ready, Issues and Closed, plus the
dependency view and the detail pane opened from any of them. The Knowledge tab holds Notes, which
have no priority, and is unchanged. The Closed tab gains a priority column, so its rows read
`date  id  P<n>  type  title`, aligned like the other tabs.

Paint the `P<n>` cell in a colour per priority level, through the existing theme: one new colour
role per level, `priority-0` to `priority-4` (priorities run 0 to 4, 0 the most urgent, 2 the
default), each with a colour in both preset schemes (`internal/ui/schemes/dark.yaml`,
`light.yaml`), overridable under `ui.colors` in the user config like every other role, and listed
in `userconfig.Roles` and the config template.

Proposed colours: an urgency ramp that leaves the default priority uncoloured, so a list of mostly
P2 issues stays calm and only the outliers draw the eye. Red and amber avoid the hues the status
column already uses (blue doing, orange waiting, green done).

| Role         | Dark        | Light       | Reads as                     |
|--------------|-------------|-------------|------------------------------|
| `priority-0` | `#f85149`   | `#cf222e`   | red, bold: drop everything   |
| `priority-1` | `#e3b341`   | `#9a6700`   | amber: next up               |
| `priority-2` | none        | none        | terminal default: the norm   |
| `priority-3` | `#8a8a8a`   | `#656d76`   | the muted colour             |
| `priority-4` | `#5c5c5c`   | `#8c959f`   | dimmer than muted            |

A role without a colour paints nothing, as today; P0 is also bold, so it stands out without colour
too.

## Why

Priority is the first thing to scan a list for, and `capcom ready` already ranks by it; colour lets
the eye find the P0 and P1 issues without reading every row.

## Acceptance

- The `P<n>` cell is painted in the `priority-<n>` colour of the active scheme, P0 also bold, in
  every tab that shows issues: the Ready, Issues and Closed sidebar rows, the dependency view, and
  the detail header of an issue opened from any tab. A test per tab checks it.
- Closed tab rows read `date  id  P<n>  type  title`, the priority column aligned across rows.
- The Knowledge tab is unchanged.
- `priority-2` has no colour in either preset, so P2 shows in the terminal's default colour.
- Both preset schemes give every `priority-*` role a key (P2's left empty or omitted as the scheme
  loader allows), and the theme tests that check each scheme covers every role still pass.
- `ui.colors` in the user config overrides any `priority-*` role; the config template lists the new
  roles.
- A selected row's fill runs unbroken beneath the coloured cell.
- With NO_COLOR nothing is coloured, and P0 is still bold.

## Log

- 2026-11-27 Airic Lenz: Implemented, uncommitted: priority-0..4 roles, P<n> coloured in Ready/Issues/Closed rows, dependency view and detail header; Closed rows gain P<n>. internal/ui and internal/userconfig tests, vet and staticcheck pass; independently verified.
