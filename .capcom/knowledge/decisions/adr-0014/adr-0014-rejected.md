---
title: "ADR 0014 rejected options: rewriting config.yaml in place, storing columns, writing on quit"
labels: [background]
---

- **Rewrite `config.yaml` in place.** One file instead of two, but it breaks ADR 0011's rule that
  the file, once created, is the user's: every drag would rewrite a hand-edited file and would have
  to keep its comments, order and commented defaults intact, or destroy them, and a write could
  land while the user has the file open in an editor.
- **Store columns** (the record's first choice, replaced 2026-10-16). Columns are what the user
  drags and sees, but a fixed width loses the split on every resize of the terminal, and on a
  restart in a terminal of another width: a sidebar set to a third of a narrow window becomes a
  sliver of a wide one. The ratio keeps the split, and the clamp still keeps both panes usable.
- **Write on quit.** One write per session, but a crash, a killed terminal or a dropped SSH session
  loses the width, and quitting would gain a failure with no status bar left to report it on.
  Writes at each finished change are rare enough to cost nothing.
