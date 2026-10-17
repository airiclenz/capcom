---
title: "ADR 0014 decision: <os.UserConfigDir>/capcom/state.yaml holds what capcom ui remembers for every worktree (the per-worktree tab and selections are ADR 0015); only capcom ui writes it, and config.yaml is never rewritten"
---

The sidebar width, and the sidebar's height in the stacked layout, are set by dragging the
divider or pressing a key, not by typing them into a file, and they have to survive a restart. ADR 0011 makes `config.yaml` the user's own file: `capcom ui`
writes it once, as a commented template, and never again. Saving them there would mean
rewriting a file the user edits by hand on every drag. A value the program sets belongs in a file
the program owns.

**The file.** `<os.UserConfigDir>/capcom/state.yaml`, in the same directory as `config.yaml`:
`~/.config/capcom/state.yaml` on Linux (or `$XDG_CONFIG_HOME`),
`~/Library/Application Support/capcom/state.yaml` on macOS, `%AppData%\capcom\state.yaml` on
Windows. Like the user config it is never inside a repository and never part of the store; what
capcom ui remembers per worktree, the tab and each tab's selection, lives under the repository's
git dir in `ui-state.yaml` (ADR 0015). Its first line says it is written by capcom ui and names
this record; it holds `ui.sidebar_ratio`, the sidebar's width as a ratio of the terminal's width,
and `ui.stacked_ratio`, the stacked layout's sidebar height as a ratio of the body's height.

**Amended 2026-10-16: the width is a ratio.** `ui.sidebar_ratio` is a decimal more than 0 and less
than 1, the sidebar's columns over the terminal's when the user set it; the drag and the keys
still move the divider in columns. At every terminal width the sidebar is the ratio times the
width, rounded to a column, so the split holds when the terminal is resized and when capcom ui
starts in a terminal of another width. It replaces `ui.sidebar_width`, a width in columns, which
no release wrote: that key is ignored as an unknown one, never converted, since the terminal's
width it was set at is unknown.

**Amended 2026-10-17: the stacked layout's divider is `ui.stacked_ratio`.** Below 90 columns, on a
terminal tall enough for both panes, the sidebar sits above the detail pane and a horizontal divider
between them moves by drag and by `<` `>`, in rows. `ui.stacked_ratio` keeps that position: a
decimal more than 0 and less than 1, the sidebar's rows over the body's (the rows between the rules
under the tab bar and above the status bar) when the user set it, validated as `sidebar_ratio` is.
At every terminal size the sidebar's rows are the ratio times the body's, rounded and clamped to the
minimums (8 rows for the sidebar, 12 for the detail pane); without the key the sidebar takes a third
of the body's rows, rounded down, and a double-click on the divider clears the key. It is a key of
its own, not `sidebar_ratio` read on the other axis, because the two measure different things: a
width a third of the way across says nothing about a good height, and one shared ratio would make a
move on a narrow terminal shift the split on a wide one. Every save writes both ratios the running
capcom ui holds, so moving one divider never drops the other.

**Only `capcom ui` reads and writes it.** It reads the file once, when it starts, and does not
create it then. It writes it once per finished drag (on the release, when the divider's position changed), on
each `<` or `>` that changed it, and on a double-click reset of a divider, which clears that
divider's ratio and keeps the other. A drag abandoned by a resize that changes the layout is not
saved and restores the ratio held at the press. Each write creates the directory when missing and replaces the whole file
through a temp file renamed into place, so a reader never sees a half-written file. Every other
command neither reads nor writes it.

**Missing means the defaults.** No file, or no `sidebar_ratio` key, is the default width: a third
of the terminal, between 28 and 48 columns; no `stacked_ratio` key is the default stacked height,
a third of the body's rows. The width a stored ratio gives is clamped to the limits
(20 columns for the sidebar, 40 for the detail pane) on every terminal size, and neither the clamp
nor a resize changes the ratio, so a narrow window does not overwrite the split set on a wide one. Deleting the file is safe: it only
forgets the divider positions.

**Unknown keys are ignored on read and dropped on save.** Every capcom binary on the machine
shares the one file, so a key a newer release wrote is ignored silently, never an error. A save
writes only the keys the running binary knows, so such a key does not survive it: the file holds
nothing hand-written, and losing a newer release's value costs only that value, where keeping it
would need a writer that round-trips YAML it does not understand.

**An unusable file is a notice.** Malformed YAML, a ratio that is not a decimal more than 0
and less than 1, or a file that cannot be read is a status-bar notice naming the file, and `capcom ui` starts
on the default width and height. A failed write is a notice too, and the new position still
applies for the session.
