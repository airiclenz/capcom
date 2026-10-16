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

- No argument opens the Ready tab. A tab name opens that tab; an id after it also selects that
  Issue or Note there (resolving aliases as `show` does). An id the tab does not hold is a
  status-bar notice.
- It needs a terminal: when stdin or stdout is not a TTY it fails with an error and exit code 1.
- `--json` does not apply and is refused.

## Layout

```
 Issues  │  Ready  │  Knowledge
─────────────────────┬───────────────────────────
▼ capcom-ui        ┃ │ Ready queue view         ┃
┝ ▶ ready-view     ┃ │                          ┃
┕ ▼ issue-detail   ┃ │ status: active · P1      ┃
  ┕ · deep-links   ┃ │ deps: knowledge-view     │
▶ release-x        │ │                          │
· docs-pass        │ │ Body markdown...         │
─────────────────────┴───────────────────────────
lock: free · ? help · q quit
```

- **Tab bar**: Issues, Ready, Knowledge, in that order for `1 2 3`. Each tab keeps its own
  selection, expanded nodes and filter. A `─` rule in the border colour sets it off from the panes,
  with `┬` above the divider, mirroring the `┴` rule above the status bar.
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
  which clears it; a failed write is a status-bar notice and the width still applies. The narrow
  layout has no divider to move.
- **Scrollbars**: each pane ends in a blank gutter column and a bar, apogee's: a heavy `┃` thumb
  marks the shown rows on a light `│` track, and the bar is blank while the pane's rows fit.
- **Detail pane**: the selected Issue or Note: its frontmatter as a header (an Issue's title,
  status, priority, labels, parent and deps; a Note's title, labels and refs), then the body
  rendered as markdown. Followable links are Issue and Note ids only: markdown links whose target
  is an id, inline-code spans exactly matching an existing id, and the parent, deps and refs
  naming an id. URLs and file refs are styled but inert. Body links are mouse-only.
- **Status bar**: the Tree lock (free, held by this worktree's other session with its purpose,
  or stale), the filter in effect, transient messages, and key hints.
- **Narrow terminals** (below about 90 columns): the layout collapses to drill-down. The sidebar
  fills the screen, Enter opens the detail full width, Esc goes back.

## Views

### Issues tab

- **Tree**: the outline `capcom list --tree` prints, collapsible, with each row's status and
  priority and a parent's `(<closed>/<total> closed)` progress. Closed issues follow the CLI's
  default visibility; a `status:<name>` filter naming a closed-category status shows them (see
  Filter). The tree opens collapsed to the top level; selecting an id expands its ancestors, and
  a selected node the outline hides (a Closed issue) shows muted, with its ancestors, while it
  stays selected.
- **Dependency view**: `d`, on the Ready and Issues tabs, swaps the detail pane for the selected
  issue's "waits on" and "blocks" lists, both sides as `capcom dep list` prints them, a dep naming
  no issue marked missing. It has its own cursor (↑↓, j k); Enter selects that issue in the
  Issues tab; Esc or `d` returns to the detail. Moving the sidebar's selection closes it.

### Ready tab

The queue `capcom ready` prints, most urgent first, through `briefing.ReadyList`.

### Knowledge tab

The Note tree `capcom know tree` prints, and the selected Note in the detail pane with its labels
and refs. Retired and Background Notes are hidden, as in `know tree`, and revealed muted while
selected, as in the Issues tree; the tree opens collapsed to the top level. The only write here is
`e` (`$EDITOR`).

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
filter   /
status   s        claim   a
close    c        reopen  R
log      L        editor  e
deps     d
help     ?        quit    q
```

`?` shows the full map. Prompts (reason, log text, confirmations) take Enter to submit and Esc
to cancel; a confirmation also takes y and n.

**Detail scroll**: PgUp/PgDn scroll the detail pane a page and ctrl+u/ctrl+d half a page, from
anywhere; the sidebar keeps focus. In the narrow detail view j/k and the arrows also scroll.

**Mouse**: a click selects a tab, a sidebar row, a dependency-view row or follows a link;
clicking a tree arrow expands or collapses. A drag on the divider resizes the sidebar and a
double-click on it resets the width (see Layout). The wheel over the sidebar moves the selection a
row, over the detail pane it scrolls three rows (or moves the dependency view's cursor). The mouse
does nothing while the help screen, the filter line, the status picker or a prompt is open. With
mouse reporting on, terminal text selection needs shift-drag (or the terminal's own modifier).

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
- Overrides name roles, not elements: `accent`, `border`, `muted`, `link`, `selection`, `heading`,
  `code`, and one per status category: `status-todo`, `status-doing`, `status-waiting`,
  `status-done`, `status-dropped`. A value is a quoted `"#rgb"`/`"#rrggbb"` or a number 0-255.
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
- **`internal/userconfig`**: loads and creates the user-level config (ADR 0011), and loads and
  saves the state file `state.yaml` (ADR 0014).
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
- **Written new**: the tree sidebar, the tab bar, mouse hit-testing, the layout. apogee's
  `mouse.go` and `panes.go` serve its own model; capcom borrows their patterns, not their code.
- A shared module comes later, if ever: only once both UIs exist and show what they share.

## Testing

Per the Note `cli-test-harness` and ADR 0007:

- Tests drive the model with key, mouse and window-size messages at a fixed size, against a
  temporary store built the way `runCLI` builds one. No real terminal, never parallel.
- `View()` is compared with golden files for each tab, the narrow layout, the help screen, prompts
  and a conflict.
- Behaviour tests cover each write, the ETag conflict, the lock confirmation, live refresh keeping
  the selection, and the first-start creation of the user config.
- The markdown renderer keeps its own table tests from apogee, plus links and blockquotes.

## Docs

- The manual page `docs/site/getting-started/terminal-ui.md`: tabs, keys, mouse, writes, the lock
  prompt, conflicts, the theme config.
- `go run ./cmd/gendocs` for the CLI reference page `docs/site/reference/cli/capcom_ui.md`.
- The file-format reference's section The user config describes the user-level config, and its
  section The state file describes `state.yaml`.

## Out of scope for v1

Creating, renaming, moving or removing nodes; editing deps, labels, titles or bodies inside the
UI; Note writes other than `$EDITOR`; acquiring the Tree lock; a file watcher; a shared module
with apogee; a VSCode extension.
