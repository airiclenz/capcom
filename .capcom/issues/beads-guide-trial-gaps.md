---
title: Migration guide leaves notes dating and spec_id cases open
status: open
type: task
priority: 3
created: 2026-10-12T05:39:56Z
---

The apogee trial migration (2026-10-12, 312 issues, clean `check` on the first run) hit cases
`docs/migrating-from-beads.md` does not settle. Each needed a converter judgment call.

- **Mixed notes.** A `notes` value with an undated lead followed by dated entries (apogee-czf:
  "Blocked on apogee-tvw…" then "2026-09-13 …"). The guide covers only all-dated or no-dated notes.
  Suggest: the undated lead is dated by `created_at`.
- **What counts as a dated entry.** A date at the start of a line (`2026-09-09 (owner):`,
  `2026-09-06 later:`, `2026-09-08 DECIDED by…`) vs one mid-sentence ("SCOPE NARROWED by the
  2026-09-08 …"). Say which starts an entry, and whether the date and a following colon are
  removed from the entry text.
- **Non-plan `spec_id` Log entry.** The guide says "a Log entry" but gives no date, author or
  wording. Suggest `- <created_at date> beads: spec: <path>`.
- **Plan `spec_id` without the `planned` label.** Such an issue becomes Planned under capcom
  (10 closed apogee issues). Say so explicitly next to the `planned` label rule.
- **A literal close reason.** 16 apogee rows have the reason `Closed`; the `Closed in beads.` rule
  covers only an empty one. Say a non-empty reason is kept verbatim.
- **Unnamed export fields.** `created_by`, `dependency_count`, `dependent_count`,
  `comment_count`, `_type`, and a dependency's `created_by` / `metadata` all fall under "anything
  else"; listing them saves the next migrator a lookup.

## Acceptance

- Each bullet above is answered in `docs/migrating-from-beads.md`.
