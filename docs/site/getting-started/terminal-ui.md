---
title: "Terminal UI"
description: "Watch the Issues and Knowledge change while agents work, and claim, set a status, close, reopen or log an Issue from capcom ui."
sidebar: { order: 4 }
---

`capcom ui` is a terminal UI for the person working next to the coding agents. It shows the Ready
queue, the Issue tree and the Knowledge tree, redraws them as the agents change the files, and
makes the small changes a supervisor makes: claim an Issue, set its Status, close or reopen it,
add a Log entry. Anything larger opens the file in your editor.

It is a viewer with light writes, not an editor: it never creates, renames, moves or removes an
Issue or a Note, and it never edits dependencies, labels or titles. Every write it makes is the
one the matching CLI command makes, through the same code (ADR 0012).

## Start it

```sh
capcom ui                           # the last tab and selection in this worktree
capcom ui issues                    # the Issues tab
capcom ui issues login-timeout-race # the Issues tab with that Issue selected
capcom ui knowledge store-layout    # the Knowledge tab with that Note selected
```

With no argument `capcom ui` reopens where you left it in this worktree: on the tab you were on,
with the Issue or Note you had selected on each tab selected again, a closed Issue or a retired
Note revealed in its tree. The first time, it opens on the Ready tab. The first argument names the
tab (`ready`, `issues` or `knowledge`), and an id after it selects that Issue or Note there; an
Alias resolves as it does for `capcom show`. The arguments win: a tab name opens that tab, and
every tab still selects what it last had, except where an id names the selection. An id the tab
does not hold is reported in the status bar; a remembered one that is gone just selects the first
row.

The tab and the selections are kept in `ui-state.yaml` under the repository's git directory, beside
the index, so they are never committed and each worktree of a clone keeps its own. A change of tab
is saved at once and a change of selection a second after the last one; quitting saves what is
still waiting. A file that cannot be read is reported in the status bar, and so is the first write
of a session that fails; `capcom ui` runs on as without the file. The file is described under
[The worktree state file](../reference/file-format.md#the-worktree-state-file).

`capcom ui` needs a terminal: when stdin or stdout is not one, it fails with exit code 1. `--json`
does not apply and is refused. `q` or ctrl+c quits. The flags are in the
[CLI reference](../reference/cli/capcom_ui.md).

## The screen

```text
 Issues  │  Ready  │  Knowledge   v0.7.0 · capcom
─────────────────────┬───────────────────────────
▼ capcom-ui        ┃ │ Ready queue view         ┃
┝ ▶ ready-view     ┃ │                          ┃
┕ ▼ issue-detail   ┃ │ status: active · P1      ┃
  ┕ · deep-links   ┃ │ deps: knowledge-view     │
▶ release-x        │ │                          │
· docs-pass        │ │ Body markdown...         │
─────────────────────┴───────────────────────────
 lock  free · ? help · q quit
```

- **Tab bar**: Issues, Ready and Knowledge. `Tab` moves to the next one, and `1`, `2` and `3`
  open Issues, Ready and Knowledge directly. Each tab keeps its own selection, expanded nodes and
  filter. A rule under it sets it off from the panes. Its right end shows capcom's version, as
  `v0.7.0 · capcom`, when the terminal is wide enough.
- **Sidebar**: the tab's list or tree. The selected row drives the detail pane. In a tree, `▼`
  marks a node whose children are shown, `▶` one whose children are hidden and `·` a node with no
  children; the gray `┝`, `┕` and `│` connectors before a row show where it sits under its Parent.
- **Detail pane**: the selected Issue or Note. An Issue shows its title, its Status, priority and
  type, then its labels, Parent, dependencies, related, plan, aliases, created and closed times and
  close reason; a Note shows its labels, Refs, retired, superseded-by and aliases. A field that is
  empty is left out, and `retired` shows only when true. The body follows, rendered as markdown:
  headings, lists, code, tables, blockquotes and links.
- **Status bar**: the Tree lock of this worktree (`lock  free`, `lock  held by <holder>: <purpose>`
  or `lock  stale: <purpose>`, with the heartbeat's age), the filter in effect, notices and the
  report of the last write, then the key hints. The word `lock`, with a space either side, is a
  badge: filled with the `lock-held` colour while another session holds the lock, live or stale,
  and with `lock-free` while it is free. Under `NO_COLOR` a held lock's badge is drawn in reverse.
- **Scrollbars**: each pane has one at its right edge. The heavy `┃` thumb shows where the visible
  rows sit among all of them; the bar stays blank while everything fits.

Below 90 columns the panes stop sharing the screen: the sidebar fills it, Enter opens the
selected Issue or Note full width, and Esc goes back to the sidebar.

### Resizing the sidebar

While the panes share the screen, the divider between them moves. Drag it with the mouse, on the
`│` or the blank column either side of it, on any row of the panes; or press `<` to narrow the
sidebar and `>` to widen it, 4 columns a press. The panes reflow as the divider moves, and while
the button is held it shows bold in the accent colour. The sidebar keeps at least 20 columns and
the detail pane, its scrollbar included, at least 40. A double-click on the divider returns it to
the default width: a third of the terminal, between 28 and 48 columns. Below 90 columns the
divider does not move.

The width you set is kept as a ratio of the terminal's width, so the split holds when you resize
the terminal: a sidebar a third of the way across stays a third of the way across. `capcom ui`
remembers that ratio in `capcom/state.yaml`, beside your [user config](#theme), and starts on it
the next time, in every repository; the tab and the selections are kept per worktree instead (see
[Start it](#start-it)). Where the ratio would break a limit the sidebar takes what fits, and the
ratio stays for a terminal where it fits again; a double-click reset forgets it. A width that
cannot be saved is reported in the status bar, and the new width still applies. The file is
described under [The state file](../reference/file-format.md#the-state-file).

## The tabs

- **Ready** lists the queue `capcom ready` prints with no flags, most urgent first: each Issue's
  id, Status, priority and title.
- **Issues** shows the Tree `capcom list --tree` prints, each row with its Status and priority and,
  for a Parent, how many of its children are closed. Closed Issues are hidden, as in the CLI; a
  [filter](#filter) on a closed Status shows them.
- **Knowledge** shows the Note tree `capcom know tree` prints. Retired and Background Notes are
  hidden, as in `know tree`. The only write here is `e`, the editor.

On the Ready and Issues tabs the Statuses line up in one column and the priorities in another, at
every depth of the Tree, so they stay put as you scroll.

Both trees open collapsed to the top level. Selecting an id, from the command line or by following
a link, expands its ancestors; a hidden one (a Closed Issue, a Retired or Background Note) shows
muted, with its ancestors, for as long as it stays selected.

## Keys

```text
move     ↑↓←→  j k h l
expand   → / l    collapse ← / h
tabs     Tab  1 2 3
open     Enter    back     Esc
scroll   PgUp PgDn  ctrl+u ctrl+d
resize   < >      drag the divider, double-click resets
copy     drag over the detail text
filter   /        deps     d
status   s        claim    a
close    c        reopen   R
log      L        editor   e
help     ?        quit     q
```

- The up and down arrows and `j` `k` move the sidebar's selection. On a tree, → and `l` expand the
  selected node, ← and `h` collapse it or, when it is collapsed, move to its Parent.
- PgUp and PgDn scroll the detail pane a page, ctrl+u and ctrl+d half a page, wherever the
  selection is: the sidebar keeps it. Where the detail pane fills a narrow screen, the arrows and
  `j` `k` scroll it too.
- `<` and `>` narrow and widen the sidebar; see [Resizing the sidebar](#resizing-the-sidebar).
- `?` shows the full key map; `?` or Esc closes it.
- Prompts (a reason, a Log entry, a confirmation) take Enter to submit and Esc to cancel; a
  confirmation also takes `y` and `n`.

### Dependencies

On the Ready and Issues tabs, `d` swaps the detail pane for the selected Issue's dependencies, as
`capcom dep list` prints them: what it **waits on** and what it **blocks**, each row with its id,
Status, priority and title. A dependency that names no Issue is marked missing. The arrows and
`j` `k` move a cursor over the rows, Enter selects that Issue in the Issues tab, and `d` or Esc
returns to the detail pane. Moving the sidebar's selection closes the view.

### Filter

`/` opens a filter line over the current tab's sidebar. Enter keeps the filter, Esc on the line
clears it, and Esc from the sidebar clears a kept one. The words of the line all have to match:

- free text matches the title and body, as `capcom search` (or `capcom know search` on the
  Knowledge tab) finds them;
- `status:<name>` keeps the Issues in that Status; the Status must be declared in
  `config.yaml`, and Notes have none;
- `label:<name>` keeps the Issues or Notes carrying that Label; some Issue or Note must carry it.

In a tree a match keeps its ancestors, all expanded, so the shape stays readable. Text and label
filters keep the default visibility; a `status:` filter naming a closed Status, such as
`status:done`, is the way to Closed Issues: the Issues tab then shows the Closed Issues in that
Status, with their ancestors, ready to read or reopen.

## Writes

The writes act on the Issue the detail pane shows, on the Ready and Issues tabs:

| Key | Action | Same as |
|---|---|---|
| `a` | claim: set the first Status of the doing category (`active` in the default config) | `capcom update <id> --status active` |
| `s` | pick a Status: the configured ones outside the closed categories, the cursor on the current one; Enter sets it, Esc cancels | `capcom update <id> --status <s>` |
| `c` | close: asks for a reason, then names the Issues the close made Ready | `capcom close <id> --reason <r>` |
| `R` | reopen a Closed Issue: asks for a reason | `capcom reopen <id> --reason <r>` |
| `L` | add a Log entry: asks for the line | `capcom log <id> <text>` |
| `e` | open the Issue or Note file in your editor, on every tab | editing by hand |

A blank reason or Log line is refused, as the CLI refuses one. An Issue whose file does not parse
takes no write: fix its file first, with `e`. The status bar reports each write that landed, such
as `closed login-timeout-race; now ready: session-store`.

### The editor

`e` opens the selected file in `$VISUAL`, or in `$EDITOR` when `$VISUAL` is unset; with neither
set it says so. The UI hands the terminal to the editor and comes back when it exits. It then runs
the [`capcom check`](../reference/cli/capcom_check.md) checks and reports the first problem in the
file you edited, else the first problem anywhere, with how many there are, or that check found
none.

### The Tree lock

The status bar always shows this worktree's [Tree lock](../concepts/tree-locks.md). While another
session holds it, live or Stale, every write and `e` first asks:

```text
locked by <purpose>, write anyway? (y/n)
```

When the lock record cannot be read, the question names the failure instead. `y` goes ahead, `n`
or Esc cancels. The UI never acquires, touches or releases the lock: its writes are single-file
edits, not the multi-file changes the lock exists for (ADR 0013).

### Conflicts

Each write is made on the version of the Issue on screen: it carries that version's ETag, as an
if-match. When an agent changed the file in the meantime, the write is refused and nothing is
written. The UI reloads the Issue and asks:

```text
changed on disk: status, log — run close again? (y/n)
```

The question names the frontmatter fields that changed, then `body` and `log`, or `file` when only
the formatting did. Read the new version in the detail pane: `y` runs the action again on it, `n`
cancels.

## Live refresh

The UI polls about once a second: it refreshes the index, the same incremental refresh the CLI
runs, and redraws when an Issue or a Note changed. Selection, expanded nodes, scroll position and
the filter survive a refresh; when the selected node is gone, the selection moves to its Parent
or nearest sibling. The Tree lock in the status bar is read again on every poll.

A change to `config.yaml` re-opens the store on the new config. One the UI cannot use keeps the
previous config in effect, with the error in the status bar, until the file changes again. When
another capcom version rebuilt the index, polling stops with `index changed by another capcom
version: restart capcom ui`.

## Mouse

- A click selects a tab, a sidebar row, a dependency row or a link. A click on a tree row's arrow
  expands or collapses it.
- In the detail pane, the links that are followed are the ids of Issues and Notes: a markdown link
  whose target is an id, a code span that is exactly an existing id, and the Parent,
  dependencies, related, Refs and superseded-by naming one. A click selects that Issue in the
  Issues tab or that Note in the Knowledge tab. URLs and file references are styled but not
  followed. Body links are mouse-only.
- The wheel over the sidebar moves the selection a row; over the detail pane it scrolls three rows,
  or moves the dependency view's cursor.
- A drag on the divider between the panes resizes the sidebar, and a double-click on it resets
  the width; see [Resizing the sidebar](#resizing-the-sidebar).
- A drag in the detail pane selects its text, on the Ready, Issues and Knowledge tabs, and
  releasing the button copies it; there is no copy key. See [Copying text](#copying-text).
- While the help screen, the filter line, the status picker or a prompt is open, the mouse does
  nothing.

### Copying text

Press the left button on the text of an Issue or a Note in the detail pane and drag: the text you
drag over is highlighted, across lines, and stays selected while you scroll the pane with the
wheel. When you release the button, the selected text is copied as it is shown, without colours
and without trailing spaces, and the status bar shows `copied N chars` for about two seconds.

The copy goes to the terminal's clipboard (OSC 52), to your system clipboard when a clipboard
program is installed (`pbcopy` on macOS; `wl-copy`, `xclip`, `xsel`, `termux-clipboard-set` or
`clip.exe` elsewhere; Windows needs none), and to tmux's paste buffer when you run inside tmux. A
clipboard program found inside the repository's working tree is never run.

A click without a drag copies nothing: on a link it follows the link, as before. A key press, a
click elsewhere, opening another Issue or Note, or a resize that rewraps the selected lines clears
the selection.

The UI turns on the terminal's mouse reporting, so a plain drag elsewhere on the screen (the
sidebar, the tab bar, the status bar) no longer selects text; only the detail pane selects on its
own. To select text anywhere else, hold Shift while you drag, or the modifier your terminal uses
to bypass mouse reporting (Option in iTerm2).

## Theme

The colours come from your own config file, not from the repository's: `capcom/config.yaml` in
your user config directory (`~/.config` on Linux, `~/Library/Application Support` on macOS,
`%AppData%` on Windows). `capcom ui` creates it the first time it starts, with every key present
as a commented default; no other command reads or creates it, and every repository you open shares
it.

```yaml
ui:
  theme: auto        # auto | dark | light
  colors:            # optional overrides, per role
    accent: "#7aa2f7"
    status-doing: 214
```

- `theme: auto`, the default, asks the terminal for its background colour and picks the dark or
  light preset; `dark` and `light` pick one outright.
- `colors` overrides the preset by role: `accent`, `border`, `muted`, `link`, `selection`,
  `heading`, `code`, and one per Status category, `status-todo`, `status-doing`,
  `status-waiting`, `status-done` and `status-dropped`, each covering every Status of its
  category, and `lock-held` and `lock-free`, the status bar's lock badge. A value is a quoted
  `"#rgb"` or `"#rrggbb"` colour or a 256-colour number from 0 to 255.
- `NO_COLOR` in the environment turns colour off, whatever the file says.

The file is read when `capcom ui` starts: restart it after an edit. An unknown key or role is
ignored with a warning in the status bar, and a file that cannot be used is reported there while
the UI runs on the defaults. The full format is under
[The user config](../reference/file-format.md#the-user-config).
