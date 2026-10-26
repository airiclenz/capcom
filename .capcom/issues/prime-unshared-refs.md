---
title: prime does not name unshared Note refs
status: open
type: bug
priority: 2
plan: 2026-10-26 - 01 - p1-p2-ergonomics-plan
created: 2026-10-26T09:46:10Z
---

`capcom check` now reports `unshared-ref` problems (a Note Ref into the git directory or to an
ignored file), but the `capcom prime` briefing lists only `broken-ref` problems under
`broken Note refs:`: `readHealth` keeps only `diagnose.BrokenRef`
(internal/briefing/briefing.go:183). An unshared Ref counts in the problem total but is never
named in the briefing.

## Acceptance

- `capcom prime` names each `unshared-ref` problem, either in the Note refs list or a list of
  its own.
- A test in internal/briefing covers a Note with an unshared Ref.
