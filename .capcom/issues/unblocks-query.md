---
title: No query for which issues a close would make Ready, and no bounded dependency walk
status: open
type: feature
priority: 2
plan: 2026-10-12 - 04 - dependency-graph-and-knowledge-map-plan
created: 2026-10-12T18:13:07Z
---

capcom can tell whether an issue is Ready, but not which issues would become Ready if some issues
were closed, and it has no walk over dependencies beyond direct neighbours. Package `issue` gains
both: a query for the todo issues that become Ready when a set of issues is closed with given
categories, and a depth-limited walk over an issue's deps or dependents that stops on cycles and
counts what the depth cap left out.

This is the shared base for `close` naming what it made ready, `ready` ranking by what an issue
unblocks, `dep list --depth` and `dep graph`.

## Acceptance

- Closing an issue's only blocker (done or dropped) reports that issue as newly Ready; closing one
  of two blockers reports nothing; closing both together reports it.
- Closing the last open child reports its parent; a dependency written as an alias resolves.
- The walk visits each id once, terminates on a cycle, and reports how many neighbours the depth
  cap cut.
