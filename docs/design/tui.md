# capcom ui: a terminal UI

Status: design agreed 2026-10-15; implemented, and this document matches the build. Decisions
recorded in ADR 0011 (user-level config), ADR 0012 (shared ops package) and ADR 0013 (the UI
checks the Tree lock). The user manual page is `docs/site/getting-started/terminal-ui.md`.

## Purpose

`capcom ui` is a terminal UI for a person working next to coding agents: they watch the issues
and Knowledge change while agents work, and make the small changes a supervisor makes (claim,
status, close, a log line) without leaving the terminal. Anything larger opens the file in
`$EDITOR`. It ships in the capcom binary, through the same release channels.

It is a viewer with light writes, not an editor: it never creates, renames, moves or removes a
node, and it never edits dependencies, labels or titles.

## Command

```
capcom ui [issues|ready|knowledge] [id]
```

- No argument reopens on the tab and each tab's selection last saved for this worktree in
  `<git-dir>/capcom/ui-state.yaml` (ADR 0015), and on Ready with no file. A tab name opens that
  tab; an id after it also selects that Issue or Note there (resolving aliases as `show` does).
  The arguments win: with a tab name every tab still selects its stored id, except the one an id
  names. An id the tab does not hold is a status-bar notice; a stored id that is gone or not on its
  tab, or a stored tab name this binary does not know, falls back to the first row or Ready
  silently. A stored id the Issues or Knowledge tree still holds, closed or retired among them, is
  selected and revealed.
- It needs a terminal: when stdin or stdout is not a TTY it fails with an error and exit code 1.
- `--json` does not apply and is refused.

## Layout

```
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

- **Tab bar**: Issues, Ready, Knowledge, in that order for `1 2 3`. Each tab keeps its own
  selection, expanded nodes and filter. The open tab and each tab's selection persist across runs
  in `ui-state.yaml`, per worktree (ADR 0015): a tab change saves at once, a selection change one
  second after the last (`internal/ui/uistate.go` debounces), and quitting flushes a pending save;
  the first failed load or save of a session is a status-bar notice. A `─` rule in the border colour sets it off from the panes,
  with `┬` above the divider, mirroring the `┴` rule above the status bar. The bar's right end reads
  `v<version> · capcom` in the muted hints style, its last character in the frame's last column,
  with the version `capcom ui` was built with; the label is left out when it would leave fewer
  than two columns after the last tab, so the tabs are never cut for it, and a click on it opens
  no tab.
- **Sidebar**: the tab's list or tree; the selected row drives the detail pane. A tree row's arrow
  is `▼` for a node whose children are shown, `▶` for one whose children are hidden and `·` for a
  leaf. Below the top level, gray connectors in the border colour lead it: `┝` for a child a later
  sibling follows, `┕` for the last child, and `│` under a non-last ancestor (blanks under a last
  one). The selection and muted highlights start after the connectors.
- **Divider**: ` │ `, the pane separator with a blank column either side. It moves: a drag on
  the `│` or a margin column, on any body row, or `<`/`>` (4 columns a press) set the sidebar's
  width, moving it in columns, at least 20 for the sidebar and 40 for the detail pane (from
  `detailLeft` to the right edge, scrollbar included). The panes reflow on every motion, and while the button is
  held the divider and its `┬`/`┴` joints draw bold in the accent colour. A double-click resets
  the default, a third of the width within 28-48. The width set is kept as `sidebarRatio`, its
  ratio of the terminal's width, so the split holds when the terminal's width changes: at every
  width the sidebar is the ratio times the width, rounded and clamped to the limits, and neither a
  resize nor the clamp changes the ratio. The ratio persists across runs as `ui.sidebar_ratio` in
  `<os.UserConfigDir>/capcom/state.yaml` (ADR 0014), beside the user config, which stays
  hand-owned: it is written once per finished drag that changed it, per resize key and per reset,
  which clears it; a failed write is a status-bar notice (`cannot save the divider position: `) and
  the width still applies. In the stacked layout (see Narrow terminals) the divider is the full-width
  `─` rule row between the panes, drawn in the border style and in `dividerActive` while dragged;
  that row alone is its hit area. A drag on it or `<`/`>` (2 rows a press, `<` up, shrinking the
  sidebar) set the sidebar's height in rows, at least 8 for the sidebar and 12 for the detail pane,
  and a double-click resets the default. The height set is kept as `stackedRatio`, the sidebar's
  rows over the body's, and persists as `ui.stacked_ratio` in the same file, apart from
  `ui.sidebar_ratio`: every save writes both ratios the model holds, so moving one divider never
  drops the other, and a divider change that ends unsaved (a release with no net move, a layout
  change mid-drag) restores the ratio held at the press. The one-pane layout has no divider to
  move.
- **Scrollbars**: each pane ends in a blank gutter column and a bar, apogee's: a heavy `┃` thumb
  marks the shown rows on a light `│` track, and the bar is blank while the pane's rows fit.
- **Detail pane**: the selected Issue or Note: its frontmatter as a header, then the body rendered
  as markdown. An Issue's header is its title, then status, priority and type on one line, then
  labels, parent, deps, related, plan, aliases, created, closed and reason; a Note's is its title,
  then labels, refs, retired, superseded-by and aliases. Each is a `key: value` line in the
  file's key order, left out when empty (`retired` only when true); times read in UTC to the
  minute. Followable links are Issue and Note ids only: markdown links whose target is an id,
  inline-code spans exactly matching an existing id, and the parent, deps, related, refs and
  superseded-by naming an id. URLs and file refs are styled but inert. Body links are mouse-only.
- **Status bar**: the Tree lock (free, held by this worktree's other session with its purpose,
  or stale), the filter in effect, transient messages, and key hints. It opens with ` lock `,
  the word padded with a space each side, then one plain space and the state (` lock  free`,
  ` lock  held by <holder>: <purpose> (heartbeat <age> ago)`, ` lock  stale: …`, ` lock  invalid`),
  no colon after the word. The padded word is a badge: filled with the `lock-held` colour while
  the lock is held or stale, with `lock-free` while it is free, its text black or white, whichever
  reads on the fill, and the space and state after it unstyled; invalid, unreadable and the first
  read's `reading` leave it plain. Under `NO_COLOR` a held or stale badge is drawn in reverse and
  a free one plain.
- **Narrow terminals** (below 90 columns): with a body of 21 rows or more (a terminal of 25 or
  more; the sidebar's 8, the rule's 1 and the detail pane's 12) the panes stack: the sidebar on
  top, a full-width `─` rule, the detail pane below, each full width with its own scrollbar, and
  every key and click but the divider's acting as side by side. The sidebar takes `stackedRatio`
  of the body's rows, rounded, or a third of them rounded down when unset, clamped to the
  minimums. A shorter body collapses to drill-down: the sidebar fills the screen, Enter opens the
  detail full width, Esc goes back. The layout is chosen from the terminal's size alone, live on
  every resize, and a resize changes neither stored ratio.

## Views

### Issues tab

- **Tree**: the outline `capcom list --tree` prints, collapsible, with each row's status and
  priority and a parent's `(<closed>/<total> closed)` progress. The statuses start at one column
  and the priorities at another, at every depth, measured over every visible row so scrolling
  never shifts them; the CLI's output stays unaligned. Closed issues follow the CLI's
  default visibility; a `status:<name>` filter naming a closed-category status shows them (see
  Filter). The tree opens collapsed to the top level; selecting an id expands its ancestors, and
  a selected node the outline hides (a Closed issue) shows muted, with its ancestors, while it
  stays selected.
- **Dependency view**: `d`, on the Ready and Issues tabs, swaps the detail pane for the selected
  issue's "waits on" and "blocks" lists, both sides as `capcom dep list` prints them, a dep naming
  no issue marked missing. It has its own cursor (↑↓, j k); Enter selects that issue in the
  Issues tab; Esc or `d` returns to the detail. Moving the sidebar's selection closes it.

### Ready tab

The queue `capcom ready` prints, most urgent first, through `briefing.ReadyList`, its statuses
and priorities aligned in columns as on the Issues tab.

### Knowledge tab

The Note tree `capcom know tree` prints, and the selected Note in the detail pane with its labels,
refs, retired flag, superseded-by and aliases. Retired and Background Notes are hidden, as in
`know tree`, and revealed muted while selected, as in the Issues tree; the tree opens collapsed to
the top level. The only write here is `e` (`$EDITOR`).

### Filter

`/` opens a filter line over the current sidebar: free text matches the title and body (as
`search` or `know search` does), and `status:<name>` and `label:<name>` narrow by field; every
term must hold. Enter keeps the filter, Esc on the line or from the sidebar clears it. A status
`config.yaml` does not declare, a label nothing carries and `status:` on the Knowledge tab are
reported instead. In the tree a match keeps its ancestors, expanded, so the shape stays readable.

Text and label filters keep the default visibility. A `status:<name>` filter naming a
closed-category status (e.g. `status:done`) includes Closed issues, shown with their ancestors:
that is the route to a Closed issue, to read or reopen it. There is no toggle key.

## Writes

| Key | Action | Same as |
|---|---|---|
| `a` | claim: set the first status of the doing category (`active` in the default config) | `capcom update <id> --status active` |
| `s` | pick a status from the configured statuses outside the closed categories | `capcom update <id> --status <s>` |
| `c` | close; prompts for a reason, then names the issues the close made ready | `capcom close <id> --reason <r>` |
| `R` | reopen; prompts for a reason | `capcom reopen <id> --reason <r>` |
| `L` | add a log line | `capcom log <id> <text>` |
| `e` | open the Issue or Note file in `$EDITOR` (`$VISUAL` first); the UI suspends, then runs the `capcom check` checks and shows any problem | editing by hand |

Every write goes through the shared ops package (ADR 0012), the same code path the CLI uses.
The writes act on the Ready and Issues tabs; a blank reason or log line is refused, and an issue
whose file does not parse takes no write.

**Conflicts.** Each write sends the ETag of the version on screen as if-match. When the file
changed in the meantime, the write is refused; the UI reloads the issue and asks
`changed on disk: <fields> — run <action> again? (y/n)`, naming the changed frontmatter fields,
then `body` and `log`, or `file` when nothing but the formatting changed. y re-runs the action on
the new ETag, n cancels.

**The Tree lock.** The status bar always shows it. When another session holds this worktree's
lock, live or stale, each write first asks "locked by <purpose>, write anyway? (y/n)"; `$EDITOR`
asks the same, and a lock record that cannot be read asks with its error in place of the purpose.
The UI never acquires, touches or releases the lock: its writes are single-file edits, not the
multi-file changes the lock exists for (ADR 0013).

## Keys

```
move     ↑↓←→  j k h l
expand   → / l   collapse ← / h
tabs     Tab  1 2 3
open     Enter    back Esc
scroll   PgUp PgDn  ctrl+u ctrl+d
resize   < >      drag the divider, double-click resets
copy     drag over the detail text
filter   /        deps    d
status   s        claim   a
close    c        reopen  R
log      L        editor  e
help     ?        quit    q
```

`?` shows the full map. Prompts (reason, log text, confirmations) take Enter to submit and Esc
to cancel; a confirmation also takes y and n.

**Detail scroll**: PgUp/PgDn scroll the detail pane a page and ctrl+u/ctrl+d half a page, from
anywhere; the sidebar keeps focus. In the one-pane narrow detail view j/k and the arrows also
scroll.

**Mouse**: a click selects a tab, a sidebar row, a dependency-view row or follows a link;
clicking a tree arrow expands or collapses. A drag on the divider resizes the sidebar and a
double-click on it resets it, the width side by side or the height when stacked (see Layout). The wheel over the sidebar moves the selection a
row, over the detail pane it scrolls three rows (or moves the dependency view's cursor). The mouse
does nothing while the help screen, the filter line, the status picker or a prompt is open.

**Selecting and copying**: on the Ready, Issues and Knowledge tabs a left press in the detail pane
anchors a selection and a drag extends it, across lines; the selected cells are painted in the
`selection` role over the rendered markdown. The selection is held in content coordinates
(rendered line, column), so a wheel scroll keeps it; motion outside the detail pane does not
extend it. Release copies the selected text right away, with no copy key: what is shown, each
line cut by display cell through `markdown.Measure`, ANSI codes stripped, trailing spaces
trimmed, lines joined with `\n`. The copy goes out through OSC 52 (`tea.SetClipboard`), the native
clipboard (best effort: `pbcopy`, `wl-copy`, `xclip`, `xsel`, `termux-clipboard-set` or
`clip.exe`, the Win32 API on Windows) and, when `TMUX` is set, `tmux load-buffer -w -`; a
clipboard or tmux program that `PATH` resolves inside the working tree is refused and never runs.
The status bar shows `copied N chars` for about two seconds, even when a native or tmux write
fails. A bare click (press and release on one cell) selects nothing and copies nothing: it
follows a link as before, and a drag over blank cells only copies nothing. A keypress, a press
elsewhere, loading another Issue or Note, or a re-render that changes a line the selection spans
clears it. Elsewhere on the screen (the sidebar, the dependency view, the help screen), terminal
text selection still needs shift-drag (or the terminal's own modifier), as mouse reporting is on.

## Live refresh

The UI polls about once a second: it refreshes the index (the existing incremental refresh, which
stats every file and re-parses only changed ones), re-reads the Tree lock for the status bar, and
redraws when any Issue or Note changed. Selection,
expanded nodes and scroll position survive a refresh; when the selected node vanished, the
selection moves to its parent or nearest sibling.

Each poll first compares `config.yaml`'s modification time and size with what the UI loaded: on a
change it re-opens the store and the index on it, so a refresh never rebuilds the index under a
stale config; a `config.yaml` it cannot use keeps the old one in effect, with the error in the
status bar, until the file changes again. An index another capcom version rebuilt (its schema
version is not this binary's) stops polling with `index changed by another capcom version: restart
capcom ui`; the UI never rebuilds it back. The filter in effect is matched again on every reload.

The index is not safe for concurrent use, so one session owns it: the poll, reads and writes
all run as Bubble Tea commands serialised through its mutex. `Index.Refresh` is the exported
refresh for this: it runs the full incremental refresh every call, as `Open` does, and reports
whether any Issue or Note changed.

No file watcher: polling needs no new dependency, behaves the same on every OS, and reuses the
refresh the CLI already trusts.

## Theme

Set in the user-level config (ADR 0011), `<os.UserConfigDir>/capcom/config.yaml`:

```yaml
ui:
  theme: auto        # auto | dark | light
  colors:            # optional overrides, per role
    accent: "#7aa2f7"
    status-doing: "#9ece6a"
```

- `auto` (the default) asks the terminal for its background colour and picks the light or dark
  preset; until the terminal answers the background counts as dark.
- The presets' colours live in `internal/ui/schemes/dark.yaml` and `light.yaml`, embedded in the
  binary: one quoted `"#rrggbb"` per role.
- Overrides name roles, not elements: `accent`, `border`, `muted`, `link`, `selection`, `heading`,
  `code`, one per status category: `status-todo`, `status-doing`, `status-waiting`,
  `status-done`, `status-dropped`, and the status bar's lock badge: `lock-held`, `lock-free`. A
  value is a quoted `"#rgb"`/`"#rrggbb"` or a number 0-255.
- The selected row of a list is filled with the `selection` colour, its text in its usual colours
  (a status keeps its category's); under `NO_COLOR` it is drawn in reverse instead.
- `NO_COLOR` turns colour off, whatever the config says.
- The file is created the first time `capcom ui` starts, with every key present as a commented
  default; no other command reads or creates it. It is read once, at start.
- It is lenient: an unknown key or role is a warning in the status bar; a file that cannot be used
  is an error there, and the UI runs on the defaults.

## Architecture

- **`internal/ui`**: the Bubble Tea v2 model, views and widgets. `cli` adds a `ui` command that
  hands it the opened store and index.
- **`internal/ops`**: the write glue moved out of `internal/cli` (`editIssue`, `editNode`,
  `closeResult`, the dep and log helpers), so the CLI and the UI share one path (ADR 0012). Reads
  go straight to `index`, `briefing`, `tree` and `issue`.
- **`internal/outline`**: the issue and Note outline builders (`outline.Issues`, `outline.Notes`),
  moved out of `internal/cli`, so `list --tree`, `know tree` and the UI's trees build one outline.
- **`internal/ui/markdown`**: the markdown renderer of the detail pane.
- **`internal/userconfig`**: loads and creates the user-level config (ADR 0011), loads and
  saves the state file `state.yaml` (ADR 0014), and loads and saves the per-worktree
  `ui-state.yaml` (ADR 0015), whose path under the git dir `internal/ui` passes in.
- **Dependencies**: `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2`.
  They declare go 1.25.0 and need `golang.org/x/sys` v0.46, so `go.mod` declares go 1.25.0 and
  `x/sys` v0.46.0 (up from go 1.24.0 and v0.37). CI and release read `go.mod` and build with
  `CGO_ENABLED=0`; the charm libraries are pure Go, so the six release targets are unaffected.

### Taken from apogee

- **Copied** into `internal/ui/markdown`: `internal/tui/markdown.go` and `internal/tui/mdtable.go`
  (headings, fenced code, bullet and numbered lists, GFM pipe tables, inline code and bold), with
  the wrap and width helpers they use and a small capcom theme in place of apogee's `theme`.
- **Added to the copy**: links (rendered and clickable, for issue and Note ids and refs) and
  blockquotes.
- **Ported as is**: `listCursor` from `internal/tui/listsurface.go` (move, clamp, keys, wheel),
  as `internal/ui/listcursor.go`.
- **Ported**: the scrollbar, from `Model.renderScrollbar` in `internal/tui/model.go` and
  `scrollbarThumb` in `internal/tui/boxdraw.go`, as `internal/ui/scrollbar.go`, with its `┃`
  thumb and `│` track glyphs and the blank gutter before the bar.
- **Ported**: the detail pane's drag selection, from `transcriptSel`, `spanUnchanged`,
  `shadeCells` and `transcriptSelectionText` in `internal/tui/mouse.go`, as
  `internal/ui/selection.go`, measuring through `markdown.Measure` (ADR 0030's one width
  authority).
- **Ported**: the clipboard routes, from `internal/tui/clipboard.go`,
  `internal/present/clipboard.go` and the working-tree fence in `internal/security/execsafety.go`:
  OSC 52, the native clipboard and tmux, as `internal/clipboard` and `internal/ui/clipboard.go`.
- **Written new**: the tree sidebar, the tab bar, mouse hit-testing, the layout. apogee's
  `panes.go` serves its own model, and capcom borrows its patterns, not its code; from apogee's
  `mouse.go` only the selection is ported, the rest of capcom's mouse handling is its own.
- A shared module comes later, if ever: only once both UIs exist and show what they share.

## Testing

Per the Note `cli-test-harness` and ADR 0007:

- Tests drive the model with key, mouse and window-size messages at a fixed size, against a
  temporary store built the way `runCLI` builds one. No real terminal, never parallel.
- `View()` is compared with golden files for each tab, the narrow one-pane and stacked layouts,
  the help screen, prompts and a conflict.
- Behaviour tests cover each write, the ETag conflict, the lock confirmation, live refresh keeping
  the selection, and the first-start creation of the user config.
- The markdown renderer keeps its own table tests from apogee, plus links and blockquotes.

## Docs

- The manual page `docs/site/getting-started/terminal-ui.md`: tabs, keys, mouse, writes, the lock
  prompt, conflicts, the theme config.
- `go run ./cmd/gendocs` for the CLI reference page `docs/site/reference/cli/capcom_ui.md`.
- The file-format reference's section The user config describes the user-level config, its
  section The state file describes `state.yaml`, and its section The worktree state file describes
  `ui-state.yaml`.

## Out of scope for v1

Creating, renaming, moving or removing nodes; editing deps, labels, titles or bodies inside the
UI; Note writes other than `$EDITOR`; acquiring the Tree lock; a file watcher; a shared module
with apogee; a VSCode extension.
