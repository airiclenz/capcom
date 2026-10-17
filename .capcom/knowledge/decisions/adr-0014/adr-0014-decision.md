---
title: "ADR 0014 decision: <os.UserConfigDir>/capcom/state.yaml holds what capcom ui remembers for every worktree (the per-worktree tab and selections are ADR 0015); only capcom ui writes it, and config.yaml is never rewritten"
---

The sidebar width is set by dragging the divider or pressing a key, not by typing it into a file,
and it has to survive a restart. ADR 0011 makes `config.yaml` the user's own file: `capcom ui`
writes it once, as a commented template, and never again. Saving the width there would mean
rewriting a file the user edits by hand on every drag. A value the program sets belongs in a file
the program owns.

**The file.** `<os.UserConfigDir>/capcom/state.yaml`, in the same directory as `config.yaml`:
`~/.config/capcom/state.yaml` on Linux (or `$XDG_CONFIG_HOME`),
`~/Library/Application Support/capcom/state.yaml` on macOS, `%AppData%\capcom\state.yaml` on
Windows. Like the user config it is never inside a repository and never part of the store; what
capcom ui remembers per worktree, the tab and each tab's selection, lives under the repository's
git dir in `ui-state.yaml` (ADR 0015). Its first line says it is written by capcom ui and names
this record; it holds `ui.sidebar_ratio`, the sidebar's width as a ratio of the terminal's width.

**Amended 2026-10-16: the width is a ratio.** `ui.sidebar_ratio` is a decimal more than 0 and less
than 1, the sidebar's columns over the terminal's when the user set it; the drag and the keys
still move the divider in columns. At every terminal width the sidebar is the ratio times the
width, rounded to a column, so the split holds when the terminal is resized and when capcom ui
starts in a terminal of another width. It replaces `ui.sidebar_width`, a width in columns, which
no release wrote: that key is ignored as an unknown one, never converted, since the terminal's
width it was set at is unknown.

**Only `capcom ui` reads and writes it.** It reads the file once, when it starts, and does not
create it then. It writes it once per finished drag (on the release, when the width changed), on
each `<` or `>` that changed the width, and on a double-click reset of the divider, which clears
the stored width. Each write creates the directory when missing and replaces the whole file
through a temp file renamed into place, so a reader never sees a half-written file. Every other
command neither reads nor writes it.

**Missing means the defaults.** No file, or no `sidebar_ratio` key, is the default width: a third
of the terminal, between 28 and 48 columns. The width a stored ratio gives is clamped to the limits
(20 columns for the sidebar, 40 for the detail pane) on every terminal size, and neither the clamp
nor a resize changes the ratio, so a narrow window does not overwrite the split set on a wide one. Deleting the file is safe: it only
forgets the width.

**Unknown keys are ignored on read and dropped on save.** Every capcom binary on the machine
shares the one file, so a key a newer release wrote is ignored silently, never an error. A save
writes only the keys the running binary knows, so such a key does not survive it: the file holds
nothing hand-written, and losing a newer release's value costs only that value, where keeping it
would need a writer that round-trips YAML it does not understand.

**An unusable file is a notice.** Malformed YAML, a ratio that is not a decimal more than 0
and less than 1, or a file that cannot be read is a status-bar notice naming the file, and `capcom ui` starts
on the default width. A failed write is a notice too, and the new width still applies for the
session.
