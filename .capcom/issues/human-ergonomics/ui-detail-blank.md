---
title: capcom ui sometimes leaves the detail pane empty for the selected issue
status: done
type: bug
priority: 1
plan: 2026-11-08 - 00 - ui-detail-blank-plan
created: 2026-11-08T16:40:08Z
closed: 2026-11-08T17:57:20Z
reason: "Resolved by plan 2026-11-08 - 00 - ui-detail-blank-plan (archived in 0f33e9a): tracked detail reads with a loading line, slow-read status notice, ctrl+l repaint, CAPCOM_UI_TRACE, randomized interleaving test."
---

`capcom ui` sometimes leaves the detail pane empty: an issue or Note is selected in the sidebar,
but the content side shows nothing. Seen several times in daily use; no reliable reproduction yet.

## Why

The detail pane is what a human reads an issue in. A blank pane looks like a missing or empty
issue, and the only way back is guessing which action repaints it.

## What is known

Research on v0.15.2 (`internal/ui`):

- The pane is blanked on purpose in `openTab` (`internal/ui/keys.go`) and when a tab's selection
  disappears, then filled when the read `loadDetail`/`loadNoteDetail` started returns as a
  `detailMsg`. Until it returns the pane is blank, with no sign that a read is pending.
- `applyDetail` (`internal/ui/detail.go`) drops a read silently when the selection or the tab has
  moved on, and `show*Selection` skips a new read when the selected id equals `m.issue.id`. Nothing
  records which read is in flight, so a dropped or never-answered read leaves no trace and no retry.
- Every read waits on the session's one mutex (`session.do`, `internal/ui/session.go`), which the
  one-second index poll (`session.poll`, `internal/ui/refresh.go`) also holds while it refreshes
  the index and reads the Tree lock. A read stuck behind a slow or blocked holder (an SQLite busy
  wait, a lock-guard flock held by another process) keeps the pane blank for as long as it waits,
  and every later read queues behind it.
- A randomized probe that delivered reads out of order across keys, clicks, wheel notches, tab
  switches, the dependency view, help, layout changes and on-disk edits with live reloads found no
  model state with a selection, no read pending and an empty pane. So the cause is either an
  interleaving the probe did not reach, a read that takes very long or never returns, or the
  terminal not showing a frame the model drew (Bubble Tea's cell renderer diffing against a screen
  that something else changed, such as an editor run through `tea.ExecProcess`).

## Observations (2026-11-08)

- Once blank, the pane stays blank after selecting other issues, on every tab.
- Dragging the divider to an extreme width sometimes brings the content back. A divider drag
  re-renders the detail text at a new width; unlike a window resize, it does not make Bubble Tea
  clear and repaint the whole screen.
- Seen in tmux over SSH to the Pi, with nothing special done before (no editor, no copy).
- A probe over terminal sizes 10–260 by 4–80, split and stacked layouts and divider ratios
  0.01–0.99 found the model drawing the detail every time. The two causes left are reads stuck
  behind the session mutex (every new selection queues too, until the holder lets go), or the
  terminal not showing cells the renderer believes it drew.

## Acceptance

- A blank detail pane is never silent: while a read is pending the pane says it is loading, and a
  read that fails or is dropped leaves the pane showing why or re-reads the selection.
- Once every pending read has returned, the detail pane shows the selected issue or Note on every
  tab, pinned by a test that drives randomized interleavings of input, reloads and out-of-order
  read delivery.
- A read cannot wait forever behind the session mutex: a pending read past a deadline is reported
  in the status bar.
- ctrl+l clears the terminal and repaints the whole frame, so a screen the terminal lost is
  recoverable without restarting `capcom ui`.
