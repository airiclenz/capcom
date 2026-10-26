---
title: No summary of what changed since a human last looked
status: open
type: feature
priority: 2
plan: 2026-10-26 - 01 - p1-p2-ergonomics-plan
created: 2026-10-19T14:54:45Z
---

A human coming back to a repository after agents have worked on it has no quick answer to
"what changed since I last looked". They read `git log` and open Issue files one by one.

Wanted: `capcom recap --since <date|git-ref>` lists, from the git history of `.capcom/`:
Issues closed (with reason), Issues filed, Issues whose status changed, Log entries added, and
Notes added, changed or retired, grouped in that order, each with id and title. Default
`--since` is seven days. `--json` returns the same data. capcom runs no writing git command
(see the `store-layout` Note); reading history is fine.

## Acceptance

- After a scripted series of commits that close, create, log and change Notes, `recap` lists
  each change once in the right group.
- A `--since` ref that does not exist fails with a hint.
- Tests use a real temp git repository (ADR 0007).
