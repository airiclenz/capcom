// Package ui is capcom's terminal UI, the program behind capcom ui (docs/design/tui.md): a Bubble
// Tea v2 model over a store and its index that internal/cli has already opened. This package never
// checks for a terminal and never closes the index it is handed; the caller does both.
//
// Every store and index access from a tea.Cmd goes through the session, which runs one access at
// a time; nothing reaches the index outside it. About once a second the session polls: it
// refreshes the index and re-reads the Tree lock, and the UI reloads its tabs when anything
// changed. A changed config.yaml makes the session re-open the store and an index of its own on
// it, which it closes itself.
//
// # Files
//
// Every non-test file of the package, with its responsibility. TestDocFileMap_MatchesDirectory
// fails when a file is missing from this map or the map names a file that does not exist.
//
//   - `clipboard.go` — copyText: the copy over OSC 52, the native clipboard and tmux, and its notice
//   - `closed.go` — the Closed tab: loading the closed issues and painting their dated rows
//   - `depview.go` — the dependency view d opens over the detail pane: its rows, keys and painting
//   - `detail.go` — the detail pane: reading an issue or Note, its header, rendering and links
//   - `doc.go` — the package comment and this file map
//   - `editor.go` — e: opening the selected issue or Note file in the editor and checking it after
//   - `entrylist.go` — entryList: a flat tab's issue list, its filter, kept selection and moves
//   - `filter.go` — the / filter over a tab's sidebar: parsing, matching and the filtered trees
//   - `help.go` — helpLines: the help screen ? opens, the full key map
//   - `issues.go` — the Issues tab: loading the issues and painting their tree
//   - `keys.go` — handleKey: the key map, tab switching and detail scrolling
//   - `knowledge.go` — the Knowledge tab: loading the Notes and painting their tree
//   - `layout.go` — the frame: tab bar, rules, panes, the sidebar's width and screen hit-testing
//   - `lineinput.go` — lineInput: the one-row text entry of the filter line and the prompts
//   - `listcursor.go` — listCursor: a sidebar list's clamped selection and its key and wheel moves
//   - `model.go` — model: the Bubble Tea model, its Init, Update and View, and loading the
//     user config, state file and worktree state file
//   - `mouse.go` — clicks, divider and selection drags, links followed and wheel scrolling
//   - `prompt.go` — the status bar's yes-or-no and text prompts before a write or an edit
//   - `ready.go` — the Ready tab: loading the queue and painting its rows and columns
//   - `refresh.go` — live refresh: the poll tick, reloading the tabs and keeping the selection
//   - `scrollbar.go` — the scrollbar cells beside a pane's rows
//   - `selection.go` — detailSel: the detail pane's drag selection, its shading and copied text
//   - `session.go` — session: the one-at-a-time store and index access and config.yaml reloads
//   - `statusbar.go` — the status bar: the Tree lock, notices and key hints
//   - `statuspick.go` — statusPicker: the list of statuses s opens over the detail pane
//   - `theme.go` — palette and theme: the colour schemes, the selection marks and every style
//   - `tree.go` — treeWidget: the expandable sidebar tree, its connectors and painting
//   - `ui.go` — Tab, Options and Run: the package's entry point
//   - `uistate.go` — saving the tab and each tab's selection to the worktree state file
//   - `write.go` — the writes on an issue (claim, status, close, reopen, log) and their conflicts
package ui
