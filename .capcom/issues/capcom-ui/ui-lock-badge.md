---
title: Colour the word lock in the status bar by the Tree lock state
status: open
type: task
priority: 2
plan: 2026-10-16 - 04 - ui-lock-badge-plan
created: 2026-10-16T18:57:37Z
---

The status bar starts with the Tree lock as plain text (`lockText` in `internal/ui/statusbar.go`):
`lock: free`, `lock: held by <holder>: <purpose> (heartbeat <age> ago)` or `lock: stale: <purpose>
(heartbeat <age> ago)`. It looks like the notices next to it, so a glance at the UI does not show
whether this worktree is free to edit or claimed by another session, in which case an edit asks
first.

Wanted: the word `lock` at the start of the status bar becomes a badge with a background fill that
shows the state:
- **held or stale:** orange fill, colour role `lock-held`. The UI holds no token, so a held lock is
  always another session's. Stale counts too, because the tree stays claimed until someone resumes
  or releases it.
- **free:** green fill, colour role `lock-free`.
- **invalid, unreadable, or the `reading` placeholder:** no fill. The plain `lock: ` prefix stays.

The rest of the segment (`held by …`, `stale: …`, `free`) stays plain.

The two new roles:
- are added to `userconfig.Roles`, to the roles list in the user config template, and to both
  scheme files `internal/ui/schemes/dark.yaml` and `light.yaml`
- default to apogee colours the schemes already use:
  - `lock-held`: the status-waiting orange (apogee mode-auto), `#f0883e` in dark, `#bc4c00` in light
  - `lock-free`: the status-done green (apogee success), `#43d389` in dark, `#115e59` in light
- can be overridden under `ui.colors` like every other role

Without colour (NO_COLOR, or a palette lacking the role), a held or stale badge falls back to
Reverse, as the selected row does, so the warning still stands out. A free badge stays plain, so
the two states remain distinguishable.

## Acceptance

- With the lock held or stale, the status bar's word `lock` has the `lock-held` fill. With it free,
  the word has the `lock-free` fill. In both cases the text after it is unstyled, and the word's
  foreground stays readable on the fill in both schemes.
- With the lock invalid, unreadable or still being read, the status bar is unchanged.
- `lock-held` and `lock-free` are roles in `userconfig.Roles`, the template and both scheme files,
  and the theme tests that check every scheme has exactly the roles still pass.
- A `ui.colors.lock-held` or `ui.colors.lock-free` override changes that fill.
- Without colour, a held or stale `lock` is shown in Reverse and a free one is plain.
- A test covers the held, stale and free cases. The status bar's cut-before-hints still measures
  the row by its visible width, so the badge's escape codes do not shorten the row.
