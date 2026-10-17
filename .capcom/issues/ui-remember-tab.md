---
title: capcom ui reopens on the last tab and selection, per worktree
status: done
type: feature
priority: 2
plan: 2026-10-17 - 00 - ui-remember-tab-plan
created: 2026-10-17T14:20:34Z
closed: 2026-10-17T15:08:08Z
reason: Resolved by plan 2026-10-17 - 00 - ui-remember-tab-plan (archived in 700744b)
---

`capcom ui` with no argument always opens on the Ready tab with the first row selected, so a
person who reopens it to go on watching the same issue or Note has to find it again each time.
The divider position already survives a restart (`ui.sidebar_ratio` in the global `state.yaml`,
ADR 0014); the tab and the selection do not.

Wanted (decided by the user, 2026-10-17):

- `capcom ui` remembers, per worktree, the tab it was on and the issue or Note selected on each
  tab, and reopens there when started with no argument.
- That state lives under the git directory, at `<git-dir>/capcom/ui-state.yaml` beside the index:
  per worktree, never committed, gone with the clone. It is not keyed by path in the global
  `state.yaml`, which would grow without bound and lose the state when a repository moves.
- The divider ratio stays in the global `state.yaml`: it is about the person and the terminal,
  not the repository.
- Arguments win: `capcom ui <tab>` opens that tab and `capcom ui <tab> <id>` selects that id,
  whatever is stored.
- A stored id that no longer exists, or is no longer on its tab (an issue closed off Ready),
  selects the first row, as today; a stored tab name the binary does not know opens Ready.
- Only `capcom ui` reads and writes the file, through a temp file renamed into place, written when
  the tab or selection changes (not only on quit, so a killed UI keeps it). Missing, malformed or
  unwritable is a status-bar notice at most, and the UI runs as today. Deleting it is safe.

## Acceptance

- Quitting on the Knowledge tab with a Note selected and starting `capcom ui` again in the same
  worktree opens the Knowledge tab with that Note selected; another worktree of the same clone
  keeps its own state. Covered by `internal/ui` tests.
- `capcom ui issues` and `capcom ui issues <id>` override the stored tab and selection.
- A stored id that is gone selects the first row; a malformed file gives a notice and the
  defaults. Covered by tests.
- No other command reads or creates `<git-dir>/capcom/ui-state.yaml`.
- ADR 0014 is amended (or a new ADR records the per-worktree file), and
  `docs/site/reference/file-format.md`, `docs/site/getting-started/terminal-ui.md`,
  `docs/design/tui.md` and the `store-layout` Note describe it; `capcom check` is clean.
