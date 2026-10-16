---
title: "ADR 0014 rejected options: rewriting config.yaml in place, a share-of-width unit, writing on quit"
labels: [background]
---

- **Rewrite `config.yaml` in place.** One file instead of two, but it breaks ADR 0011's rule that
  the file, once created, is the user's: every drag would rewrite a hand-edited file and would have
  to keep its comments, order and commented defaults intact, or destroy them, and a write could
  land while the user has the file open in an editor.
- **Store a share of the width.** A fraction of the terminal scales with the window, but the
  sidebar holds ids and titles whose length does not: a share grows the sidebar past what it needs
  on a wide terminal and starves it on a narrow one. Columns are what the user drags and sees, and
  the clamp already handles a small terminal.
- **Write on quit.** One write per session, but a crash, a killed terminal or a dropped SSH session
  loses the width, and quitting would gain a failure with no status bar left to report it on.
  Writes at each finished change are rare enough to cost nothing.
