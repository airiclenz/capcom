---
title: apply outside a repository blames the wrong path and hints at apply --help
status: open
type: bug
priority: 3
labels: [agent-ergonomics]
plan: 2026-11-22 - 01 - session-friction-followups-plan
created: 2026-11-22T17:47:25Z
---

Run from a directory that is not in a git repository, `capcom apply <file>` printed:

    capcom: /path/to/scratch: capcom requires a git repository
    hint: capcom apply --help

The path is the working directory, shown in the place where apply's other errors show the
apply file, and the hint leads to help that says nothing about where to run.
(`ErrNotGitRepository`, `internal/store/store.go:38`.)

Seen: apogee session `20261122T111318Z-26c504e8`, after a `cd` into its scratch directory
to fix the file.

Wanted: the message says the working directory is not inside a git repository and to run
the command from the repository, and carries no `--help` hint. Check the other commands
for the same hint on this failure.

## Acceptance

- The failure names the working directory as such and tells the reader to run capcom from
  inside the repository.
- No command adds a `<command> --help` hint to this failure; a test pins it.
