---
title: A first apply file fails on shape and key guesses the error could have answered
status: open
type: feature
priority: 3
labels: [agent-ergonomics]
plan: 2026-11-22 - 01 - session-friction-followups-plan
created: 2026-11-22T17:47:25Z
---

Writing a first apply file takes several tries.

Seen: apogee session `20261122T111318Z-26c504e8` (a small model) needed four runs for one
create and one log: first "the file is a mapping; give a list of operations" (it wrote a
top-level map), then `unknown key "labels"; create takes ... label ...` (the CLI flag is
`--label`, but the issue file's frontmatter key is `labels`, which is what an agent that
read a file copies), then a wrong directory.

Why: apply is the recommended way to file linked issues in one call, and small models pay
most for each failed try.

Wanted:
- `labels` is accepted for `label` in create and update entries (and any other key where
  the frontmatter name and the flag name differ).
- The "file is a mapping" error prints a minimal valid file: a two-line list with one
  create.
- `apply --help` carries a complete small example file, with a create that has a body and
  a dep, and a log.

## Acceptance

- An apply file with `labels: [a, b]` on a create applies.
- The mapping error shows the expected shape.
- `capcom apply --help` shows an example file that applies as printed.
