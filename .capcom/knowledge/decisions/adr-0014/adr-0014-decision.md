---
title: "ADR 0014 decision: <os.UserConfigDir>/capcom/state.yaml holds what capcom ui remembers; only capcom ui writes it, and config.yaml is never rewritten"
---

The sidebar width is set by dragging the divider or pressing a key, not by typing it into a file,
and it has to survive a restart. ADR 0011 makes `config.yaml` the user's own file: `capcom ui`
writes it once, as a commented template, and never again. Saving the width there would mean
rewriting a file the user edits by hand on every drag. A value the program sets belongs in a file
the program owns.

**The file.** `<os.UserConfigDir>/capcom/state.yaml`, in the same directory as `config.yaml`:
`~/.config/capcom/state.yaml` on Linux (or `$XDG_CONFIG_HOME`),
`~/Library/Application Support/capcom/state.yaml` on macOS, `%AppData%\capcom\state.yaml` on
Windows. Like the user config it is never inside a repository and never part of the store. Its
first line says it is written by capcom ui and names this record; it holds `ui.sidebar_width`, the
sidebar's width in columns.

**Only `capcom ui` reads and writes it.** It reads the file once, when it starts, and does not
create it then. It writes it once per finished drag (on the release, when the width changed), on
each `<` or `>` that changed the width, and on a double-click reset of the divider, which clears
the stored width. Each write creates the directory when missing and replaces the whole file
through a temp file renamed into place, so a reader never sees a half-written file. Every other
command neither reads nor writes it.

**Missing means the defaults.** No file, or no `sidebar_width` key, is the default width: a third
of the terminal, between 28 and 48 columns. A stored width is clamped to the limits (20 columns
for the sidebar, 40 for the detail pane) on every terminal size without being rewritten, so a
narrow window does not overwrite the width set on a wide one. Deleting the file is safe: it only
forgets the width.

**Unknown keys are ignored on read and dropped on save.** Every capcom binary on the machine
shares the one file, so a key a newer release wrote is ignored silently, never an error. A save
writes only the keys the running binary knows, so such a key does not survive it: the file holds
nothing hand-written, and losing a newer release's value costs only that value, where keeping it
would need a writer that round-trips YAML it does not understand.

**An unusable file is a notice.** Malformed YAML, a width that is not a whole number of 1 or
more, or a file that cannot be read is a status-bar notice naming the file, and `capcom ui` starts
on the default width. A failed write is a notice too, and the new width still applies for the
session.
