---
title: Pad the lock badge and drop the colon after it
status: open
type: task
priority: 2
plan: 2026-10-16 - 06 - ui-wrap-styles-and-lock-pad-plan
created: 2026-10-16T20:07:07Z
---

The status bar's lock badge (`ui-lock-badge`) fills only the four letters of `lock`, and a colon
follows it: `lock: free`, `lock: held by <holder>: <purpose> (heartbeat <age> ago)`. The fill sits
tight against the letters, so it reads as a highlighted word rather than a badge, and the colon is
redundant once the fill sets the word apart.

Wanted:
- The badge's fill spans one space on each side of the word: ` lock ` is painted, not `lock`.
- The colon after `lock` goes: `lock free`, `lock held by <holder>: <purpose> (heartbeat <age>
  ago)`, `lock stale: <purpose> (…)`, `lock invalid`, `lock unreadable`, `lock reading`. One
  plain space separates the badge from the text after it.
- With no badge (invalid, unreadable, the `reading` placeholder, or no colour for a free lock), the
  segment drops the colon the same way; whether the padding spaces show there is the
  implementer's call, but the row must not look misaligned between states.

## Acceptance

- With the lock held, stale or free, the fill covers ` lock ` (a space each side), and the text
  after it is unstyled and starts after one plain space.
- No state's text has a colon directly after `lock`.
- The Reverse fallback without colour covers the same padded span.
- The status bar tests and the golden files under `internal/ui/testdata/golden/` are updated to the
  new text, and the cut-before-hints still measures the row by its visible width.
