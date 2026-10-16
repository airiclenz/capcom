---
title: Select text in the detail pane with the mouse and copy it
status: open
type: task
priority: 2
created: 2026-10-16T19:25:30Z
---

`capcom ui` captures the mouse (`view.MouseMode = tea.MouseModeCellMotion` in `View`,
`internal/ui/model.go`) for clicks, links, divider drags and the wheel. That turns off the
terminal's own selection, so the text of an issue or Note in the detail pane cannot be selected and
copied. The terminal's bypass (shift-drag in most terminals) is not discoverable, and in the split
layout it selects whole screen rows, sidebar included.

Wanted: an in-app selection in the detail pane, on the Issues and Knowledge tabs, built the way
apogee does it (`/home/airic/repos/apogee`, design comment at the top of `internal/tui/mouse.go`;
ADR `docs/adr/adr-0030.md` for the width rules):

- **Select:** a left press in the detail pane anchors the selection and a drag extends it, across
  lines. The pointer is mapped to a cell of the rendered detail lines. The selection is stored in
  content coordinates (rendered line, column), not screen rows, so a wheel scroll during or after
  the drag keeps it. Motion outside the detail pane does not extend it. A drag never starts one;
  only a press does. A bare click (press and release on one cell) keeps today's behaviour: it
  follows a link and copies nothing.
- **Show:** the selected cells are painted last, in the existing `selection` colour role, over the
  rendered markdown. Text outside the span keeps its styling. A collapsed selection paints nothing.
- **Copy:** release copies the selected text right away; there is no copy key. The text is what is
  shown: each line cut by display cell, ANSI codes stripped, trailing spaces trimmed, lines joined
  with `\n`. A drag over blank cells only copies nothing. The copy goes out through:
  - `tea.SetClipboard` (OSC 52), the primary route
  - the native clipboard, best effort: `pbcopy` on macOS; `wl-copy` (only with `WAYLAND_DISPLAY`),
    `xclip`, `xsel`, `termux-clipboard-set` or `clip.exe` elsewhere; the Windows API on Windows.
    A program found inside the working tree on `PATH` is refused, as apogee refuses it.
  - `tmux load-buffer -w -`, only when `TMUX` is set
  The status bar then shows `copied N chars` (`char` for one) for about two seconds, even when a
  native or tmux write fails.
- **Clear:** a press elsewhere, any keypress, loading another issue or Note, or a re-render that
  changes a line the selection spans (a width change, a refresh that rewrites the body). Text
  changing outside the span keeps it.
- **Widths:** hit-testing, painting and copying all measure through the one `markdown.Measure`, so
  a wide rune (CJK, emoji) under the pointer is the one highlighted and copied.

Out of scope: selection in the sidebar, the dependency view, the help screen and the prompts;
keyboard selection; double- and triple-click.

## Acceptance

- Dragging over two lines of an issue's body highlights exactly the dragged cells and, on release,
  sets the clipboard (OSC 52) to that text without ANSI codes or trailing spaces; the status bar
  shows `copied N chars`.
- The same works on a Note in the Knowledge tab, and over the frontmatter header.
- A bare click on a link still follows it and copies nothing; a drag over blank cells copies
  nothing.
- The selection survives a wheel scroll and is cleared by a keypress, a press elsewhere, opening
  another issue or Note, and a resize that rewraps it.
- A drag over a line with wide runes copies the runes it highlights.
- With `TMUX` set the tmux route is called; a failed native or tmux write still shows the notice;
  a clipboard program inside the working tree is not run. Tests swap the writers, so none touch
  the real clipboard.
- Divider drags, link clicks and wheel scrolling behave as before, and their tests still pass.
