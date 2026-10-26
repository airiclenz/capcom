---
title: list --tree and dep list --json drop child counts, cuts and repeats
status: open
type: bug
priority: 2
created: 2026-10-26T20:06:45Z
---

`list --tree --json` and `dep list --json` silently drop the tree information the text shows, so an
agent reading JSON believes it has the whole picture when it does not.

`capcom list --tree --depth 1` in text:

    epic  open  P2  Epic  (0/2 closed)
      ... 2 more (--depth 2)

The same call with `--json` gives one row with `"depth": 0` and nothing else: no child count, no
cut marker, and a second child hidden entirely.

`capcom dep list --depth 0 --json` loses the `(see above)` repeat marker, so the shared dependency of a
diamond appears twice as two ordinary rows (only `via` differs) with no mark that the second is a
repeat, and a `--depth` cut
is invisible for the same reason.

An agent cannot tell "this parent has two children, none closed" from "I was shown a truncated
view", which is the difference between working an issue and working the wrong one.

## Acceptance

- `list --tree --json` carries the children total, the closed count and a `more` count when
  `--depth` cut the view, matching the text.
- `dep list --json` marks a repeated node, and carries the `more` count per section.
- A test asserts the JSON and text convey the same counts for a cut tree.
