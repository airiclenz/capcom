---
title: Compose panic recovery (SectionInternal) has no test
status: done
type: task
priority: 2
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:13Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`briefing.Compose` recovers a panic in any section reader and turns it into a single
`SectionInternal` Failure: the guarantee that a Briefing never costs a session its start
(ADR 0009). Nothing anywhere references `SectionInternal` or forces a panic inside `Compose`,
and `capcom prime` calls `Compose` outside `renderBriefing`'s own recover, so a regression would
fail session start silently. Severity: Medium. This test must land before the `AGENTS.md` read
changes, because it pins the same guarantee.

## Acceptance

- A source seam (for example an unexported package variable for one section reader, set in
  `export_test.go`) makes that reader panic.
- The test asserts that the Briefing holds exactly one `SectionInternal` Failure and that every
  section is nil.
- The test fails when the deferred `recover` in `Compose` is removed (bite check).
- `GOMEMLIMIT=2GiB go test ./internal/briefing/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, T-02.
