---
title: The UI tabs do not show how many items they hold
status: done
type: feature
priority: 2
plan: 2026-10-26 - 01 - p1-p2-ergonomics-plan
created: 2026-10-26T08:02:56Z
closed: 2026-10-26T13:30:36Z
reason: Resolved by plan 2026-10-26 - 01 - p1-p2-ergonomics-plan (archived 6c800bf)
---

The tab bar in `capcom ui` shows only the tab names (Ready, Issues, Knowledge). A human has to
open each tab and count rows to see how much is ready, open or recorded, and nothing shows how
far a filter has narrowed a list.

Wanted: each tab label carries its item count, e.g. `Ready [5]`.

- The count is what the tab lists with no filter on: Ready counts the Ready issues, Issues counts
  the issues not in a closed category, Knowledge counts the Notes that are not retired.
- While a filter is in effect on a tab, its label shows matched over total, e.g. `Ready [3/5]`.
- The count follows every reload of the tab (after an edit, a status change, a refresh).

## Acceptance

- With no filter, the tab bar reads `Ready [n]  Issues [n]  Knowledge [n]` with the counts above.
- With a filter kept on a tab, that tab's label reads `[matched/total]`; clearing the filter
  restores `[total]`.
- Closing, reopening or creating an issue updates the affected counts without restarting the UI.
- Clicking a tab label still selects that tab (mouse hit-testing uses the new label widths).
- Tests cover the labels with and without a filter.
