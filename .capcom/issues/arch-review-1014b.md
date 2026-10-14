---
title: Architecture review 2026-10-14, second pass
status: done
type: task
priority: 2
plan: 2026-10-14 - 02 - architecture-review-2-plan
created: 2026-10-14T16:53:33Z
closed: 2026-10-14T19:03:49Z
reason: resolved by plan 2026-10-14 - 02 - architecture-review-2-plan (7123d17f)
---

Parent of the issues filed from the second architecture review of 2026-10-14
(`docs/reviews/architecture-review-2026-10-14.html`), scoped to its Top recommendation:
candidate 3 (one node reader and descriptor per kind, carrying items 4 and 5 of the earlier
review) and candidate 5 (the Tree lock state of a worktree, answered by `lock`). The defects
candidate 3 exposes are filed as bugs that depend on the reader.

Ratified design calls (2026-10-14):

- The Managed block hides a top-level Note that the index leaves out, as `know tree` does.
- Format-key rule: any key line needs its format, empty lists included (ADR 0009: older binaries
  reject an unknown key whatever it holds); the codec and check read the rule from one place.
- The unread warning of `mv`/`rm` names every ref key of the kind.
- Messages say "Note" (capitalised, per the glossary) everywhere, issue stays lowercase.
- The lock overview reports an unreadable or invalid foreign record as a problem of that record
  and still returns every other lock and this worktree's state.
- One lock state vocabulary, `free · held · stale · invalid · unreadable`, shared by the
  Briefing, `prime` advice and `capcom lock status`.
- New glossary terms: Kind reader and Lock overview. Each user-visible change gets a CHANGELOG
  `Unreleased` line.

Out of scope: review candidates 1, 2, 4 and 6 to 11, and the open children of `arch-review-1014`.

## Acceptance

- every child issue is closed

Source: docs/reviews/architecture-review-2026-10-14.html, Top recommendation.
