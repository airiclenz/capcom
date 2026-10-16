# capcom ui: a terminal UI

Status: design, agreed 2026-10-15. Not implemented. Decisions recorded in ADR 0011 (user-level
config), ADR 0012 (shared ops package) and ADR 0013 (the UI checks the Tree lock).

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

- No argument opens the Ready tab. A tab name opens that tab; an id also selects that Issue or
  Note (resolving aliases as `show` does).
- It needs a terminal: when stdin or stdout is not a TTY it fails with an error and exit code 1.
- `--json` does not apply and is refused.

## Layout

```
┌ Issues │ Ready │ Knowledge ───────────────┐
│ ▾ capcom-ui      │ # Ready queue view      │
│   ▸ ready-view   │ status: active · P1     │
│   ▸ issue-detail │ blocks: knowledge-view  │
│ ▸ release-x      │                         │
│                  │ Body markdown...        │
├──────────────────┴─────────────────────────┤
│ lock: free · ? help · q quit               │
└────────────────────────────────────────────┘
```

- **Tab bar**: Issues, Ready, Knowledge.
- **Sidebar**: the tab's list or tree; the selected row drives the detail pane.
- **Detail pane**: the selected Issue or Note: its frontmatter as a header (status, priority,
  labels, parent, deps), then the body rendered as markdown. Issue ids and Note ids in the body,
  deps and refs are links.
- **Status bar**: the Tree lock (free, held by this worktree's other session with its purpose,
  or stale), the filter in effect, transient messages, and key hints.
- **Narrow terminals** (below about 90 columns): the layout collapses to drill-down. The sidebar
  fills the screen, Enter opens the detail full width, Esc goes back.

## Views

### Issues tab

- **Tree**: the outline `capcom list --tree` prints, collapsible, with each row's status and
  priority. Closed issues follow the CLI's default visibility.
- **Dependency view**: for the selected issue, what it waits on and what it blocks, both sides as
  in `capcom dep list`, each row a link that selects that issue.

### Ready tab

The queue `capcom ready` prints, most urgent first, through `briefing.ReadyList`.

### Knowledge tab

The Note tree `capcom know tree` prints, and the selected Note in the detail pane with its labels
and refs. Retired Notes are hidden, as in `know tree`. The only write here is `e` (`$EDITOR`).

### Filter

`/` opens a filter line over the current sidebar: free text matches the title and body (as
`search` does), and `status:<name>` and `label:<name>` narrow by field. Esc clears it. In the tree
a match keeps its ancestors so the shape stays readable.

## Writes

| Key | Action | Same as |
|---|---|---|
| `a` | claim: set the claim status (`active`) | `capcom update <id> --status active` |
| `s` | pick a status from the configured statuses | `capcom update <id> --status <s>` |
| `c` | close; prompts for a reason, then names the issues the close made ready | `capcom close <id> --reason <r>` |
| `R` | reopen; prompts for a reason | `capcom reopen <id> --reason <r>` |
| `L` | add a log line | `capcom log <id> <text>` |
| `e` | open the Issue or Note file in `$EDITOR` (`$VISUAL` first); the UI suspends, then runs the `capcom check` checks and shows any problem | editing by hand |

Every write goes through the shared ops package (ADR 0012), the same code path the CLI uses.

**Conflicts.** Each write sends the ETag of the version on screen as if-match. When the file
changed in the meantime, the write is refused; the UI reloads the issue, says what changed, and
offers to run the action again on the new version.

**The Tree lock.** The status bar always shows it. When another session holds this worktree's
lock, each write first asks "locked by <purpose>, write anyway?"; `$EDITOR` asks the same. The UI
never acquires, touches or releases the lock: its writes are single-file edits, not the
multi-file changes the lock exists for (ADR 0013).

## Keys

```
move     ↑↓←→  j k h l
expand   → / l   collapse ← / h
tabs     Tab  1 2 3
open     Enter    back Esc
filter   /
status   s        claim   a
close    c        reopen  R
log      L        editor  e
help     ?        quit    q
```

`?` shows the full map. Prompts (reason, log text, confirmations) take Enter to submit and Esc
to cancel.

**Mouse**: a click selects a row, a tab or a link; the wheel scrolls the pane under the pointer;
clicking a tree arrow expands or collapses.

## Live refresh

The UI polls about once a second: it refreshes the index (the existing incremental refresh, which
stats every file and re-parses only changed ones) and redraws when anything changed. Selection,
expanded nodes and scroll position survive a refresh; when the selected node vanished, the
selection moves to its parent or nearest sibling.

The index is not safe for concurrent use, so one goroutine owns it: the poll, reads and writes
all run as Bubble Tea commands serialised through it. The index's `refresh` is exported for this
(today only `index.Open` reaches it).

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

- `auto` (the default) detects a light or dark background and picks that preset.
- Overrides name roles (accent, border, muted, link, each status category, ...), not elements.
- `NO_COLOR` turns colour off, whatever the config says.
- The file is created the first time `capcom ui` starts, with every key present as a commented
  default; no other command reads or creates it.

## Architecture

- **`internal/ui`**: the Bubble Tea v2 model, views and widgets. `cli` adds a `ui` command that
  hands it the opened store and index.
- **`internal/ops`** (working name): the write glue moved out of `internal/cli` (`editIssue`,
  `editNode`, `closeResult`, the dep and log helpers), so the CLI and the UI share one path
  (ADR 0012). Reads go straight to `index`, `briefing`, `tree` and `issue`.
- **`internal/userconfig`**: loads and creates the user-level config (ADR 0011).
- **Dependencies**: `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2`.
  They declare go 1.25.0 and need `golang.org/x/sys` v0.46, so `go.mod` moves from go 1.24.0 to
  go 1.25 and `x/sys` from v0.37. CI and release read `go.mod` and build with `CGO_ENABLED=0`;
  the charm libraries are pure Go, so the six release targets are unaffected.

### Taken from apogee

- **Copied**: `internal/tui/markdown.go` and `internal/tui/mdtable.go` (headings, fenced code,
  bullet and numbered lists, GFM pipe tables, inline code and bold), with the wrap and width
  helpers they use and a small capcom theme in place of apogee's `theme`.
- **Added to the copy**: links (rendered and clickable, for issue and Note ids and refs) and
  blockquotes.
- **Ported as is**: `listCursor` from `internal/tui/listsurface.go` (move, clamp, keys, wheel).
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

- A manual page under `docs/site/` for `capcom ui`: tabs, keys, mouse, writes, the theme config.
- `go run ./cmd/gendocs` for the CLI reference page.
- The file-format reference gains the user-level config.

## Out of scope for v1

Creating, renaming, moving or removing nodes; editing deps, labels, titles or bodies inside the
UI; Note writes other than `$EDITOR`; acquiring the Tree lock; a file watcher; a shared module
with apogee; a VSCode extension.
