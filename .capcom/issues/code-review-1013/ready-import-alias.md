---
title: cli/ready.go imports internal/briefing under the alias readylist
status: done
type: bug
priority: 3
deps: [split-large-files]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

Package `cli` imports `internal/briefing` under two names: `briefing` in `prime.go` and the
misleading alias `readylist` in `ready.go`, so a search for `briefing.` misses the `ready` path.
No behaviour is affected. Severity: Low (cosmetic).

Wanted: drop the alias and write `briefing.ReadyList` and `briefing.ReadyOptions` in `ready.go`,
alongside the Phase 2 moves.

## Acceptance

- `grep -rn readylist internal/` finds nothing.
- `go vet ./internal/cli/` is clean.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, U-09.
