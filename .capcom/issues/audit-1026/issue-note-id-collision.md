---
title: An issue and a Note can share one id and nothing reports it
status: open
type: bug
priority: 2
created: 2026-10-26T20:06:45Z
---

An issue and a Note can hold the same id, and nothing reports it.

Reproduced:

    capcom create --id shared --title "An issue" ...     -> .capcom/issues/shared.md
    capcom know mv notex shared                          -> exit 0
    ls .capcom/issues/shared.md .capcom/knowledge/shared.md  -> both exist
    capcom check                                          -> exit 0, no problem

The file format reference says "An id is unique across the whole tree", and ADR 0002 says a new
id may not reuse an existing id or alias. Claims are checked per tree, so the two never meet.

The result is that `capcom show shared` resolves to the issue and `capcom know show shared` to
the Note, and `rm`, `mv` and `dep` in one tree silently ignore the collision. For an agent
following the "one id, one thing" mental model that AGENTS.md teaches, this is a confusing
failure that no command reports.

## Acceptance

- `create`, `know add`, `mv` and `know mv` refuse an id, or alias, already claimed by the other
  tree, with a message naming the file that holds it.
- `capcom check` reports an existing collision as a problem.
- A test creates the collision by hand and asserts both the refusal and the check finding.
