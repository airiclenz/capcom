---
title: Reading an issue's deps, plan and Notes takes many calls
status: done
type: feature
priority: 1
deps: [search-any-word]
plan: 2026-10-26 - 01 - p1-p2-ergonomics-plan
created: 2026-10-19T14:54:45Z
closed: 2026-10-26T13:30:36Z
reason: Resolved by plan 2026-10-26 - 01 - p1-p2-ergonomics-plan (archived 6c800bf)
---

Before working an Issue an agent should read its dependencies, its plan and the Knowledge
Notes that apply. AGENTS.md asks for `capcom know tree --depth 2`, then `know tree <id>`, then
`know show` on every Note whose title matches. Small models skip this loop, and frontier models
spend several calls on it.

Wanted: `capcom context <id>` prints, in one output and in this order:

1. the Issue in full, as `show` prints it;
2. its Dependencies and dependents one level deep, with status;
3. its plan's resolved path when it has one;
4. the Notes that apply: Notes whose refs name a file the Issue's body mentions, then the top
   Notes from a Note search on the Issue's title (using the any-word fallback), each with id
   and title, and the bodies of the top three in full.

`--brief` prints ids and titles only. Everything is available with `--json`.

## Acceptance

- One call returns everything above for an Issue with deps, dependents, a plan and matching Notes.
- An Issue with none of these prints just the Issue and one line per empty section.
- Output stays under a size limit documented in `--help`; long sections are cut with a line
  naming the command that prints the rest.
- The Managed block tells agents to run `capcom context <id>` before starting an Issue.

## Log

- 2026-10-26 Airic Lenz: Field evaluation (capcom 0.9.1, eval-gather-context): gathering an issue's deps and dependents with status plus its applicable Notes took six show calls; show prints deps and dependents without status, and nothing lists the Notes that apply to an issue. The report ranks a context view second by calls saved (about five). Confirms the wanted output, including statuses on deps and dependents.
