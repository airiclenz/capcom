---
title: "ADR 0015 decision: <git-dir>/capcom/ui-state.yaml holds capcom ui's tab and each tab's selection for one worktree; only capcom ui reads and writes it"
---

Which tab was open and which issue or Note was selected belong to one worktree: two worktrees of
a clone, or two repositories, are different work, and reopening one on the other's selection would
be noise. The sidebar's width is a taste in layout that holds everywhere, so it stays in the
per-user `state.yaml` (ADR 0014); what capcom ui remembers per worktree needs a file of its own
that moves, and goes away, with the worktree.

**The file.** `<git-dir>/capcom/ui-state.yaml`, in the same directory as the index `index.db`: per
worktree, under the git directory, so never committed and never part of the store. It is the one
file of capcom ui's own inside a repository; the user config `config.yaml` (ADR 0011) and the state
file `state.yaml` (ADR 0014) stay outside it. Its first line says it is written by capcom ui and
names this record. It holds a `ui:` mapping with `tab`, the name of the open tab (`ready`,
`issues` or `knowledge`), and `selected`, a mapping of those same names to the id selected on that
tab. Every value is double-quoted, so an id such as `2026` or `true` loads back as a string; an
empty value is left out.

**Only `capcom ui` reads and writes it.** It reads the file once, when it starts, and does not
create it then. A change of tab saves at once. A change of selection saves one second after the
last change, so a run of moves writes the file once. Quitting writes a selection change still
waiting for its second. Each write creates the directory when missing and replaces the whole file
through a temp file renamed into place, so a reader never sees a half-written file. Every other
command neither reads nor writes it.

**The command line wins.** With no argument `capcom ui` opens on the stored tab. `capcom ui <tab>`
opens that tab and still selects each tab's stored id; `capcom ui <tab> <id>` selects that id on
that tab, and the other tabs their stored ids.

**What is gone falls back without a word.** No file, no `tab` key or no stored id is today's
start: the tab the command line names, Ready when it names none, and the first row. A stored tab
name this binary does not know opens Ready. A stored id that is gone, or not on its tab, selects
the first row. A stored id the Issues or Knowledge tree still holds, closed or retired among them,
is selected and revealed, as an id named on the command line is. Deleting the file is safe: it
only forgets the tab and the selections.

**Unknown keys are ignored on read and dropped on save.** A newer capcom's key loads silently in an
older one, and a save writes only the keys the running binary knows, as for `state.yaml`.

**An unusable file is one notice per session.** Malformed YAML, a value that is not a string, or a
file that cannot be read is a status-bar notice naming the file, and capcom ui starts as with no
file. The first failed write of a session is a notice too, and later ones are silent; the tab and
the selections still apply for the session. A failed write at quit has no status bar left to report
it on and is dropped.
