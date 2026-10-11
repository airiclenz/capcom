---
title: mv warns about a moved child's bad file with its pre-move path
status: open
type: bug
priority: 3
created: 2026-10-11T20:44:59Z
---

Deferred from the capcom-maturity plan, item 4: `mv` (and `know mv`) of a node whose children
directory holds an unrelated bad file warns with that file's pre-move path, which the rename makes
stale. The tree problems are triaged into warnings before the move is applied
(internal/cli/mv.go:75, internal/cli/know_mv.go:82), so a problem under the moved node's children
directory still names the old directory once `move.Apply()` has renamed it.

## Acceptance

- A warning about a file inside the moved node's children directory names the file's path after
  the move, for both `mv` and `know mv`.
- internal/cli/mv_test.go and internal/cli/know_mv_test.go cover a move whose children directory
  holds an unrelated bad file.
