---
title: Five-word ids are refused, and the error does not say how many words it counted
status: open
type: bug
priority: 3
labels: [agent-ergonomics]
created: 2026-11-22T17:47:25Z
---

An id is 1 to 4 kebab words (`internal/tree/ids.go:25`). Agents regularly pick five.

Seen: four distinct five-word ids refused in five calls, of the shape
`<component>-<component>-<noun>-<noun>-<state>` (for example `login-form-token-refresh-untested`),
one of them twice in an apply file. Three of them sat in one shell call that created several
issues, so the others were created and the batch was left partial. The message "must be 1
to 4 words of a-z and 0-9 joined by single hyphens" does not say which rule the id broke.

Why: the limit is tight for names that carry a component prefix, and the error costs a
second look to find the cause.

Wanted: decide whether the limit rises to five words (check the glossary and any ADR that
set four, and what ids of five words would do to list and UI column widths). Either way
the error names the broken rule, for example "5 words, at most 4" or "upper-case letter".

## Acceptance

- A refused id's error says which rule it broke.
- The limit is either raised, with help texts, the block and docs updated, or kept, with
  the reason logged here.
