---
title: The UI has no way to browse Closed issues
status: open
type: task
priority: 2
deps: [ui-tab-counts]
created: 2026-10-26T08:02:56Z
---

`capcom ui` gives no way to browse Closed issues. The Issues tree hides them; they show only
through a `/status:closed` filter, or muted while selected. A human reviewing what agents
finished has to fall back to the CLI.

Wanted: a fourth tab, Closed, listing every Closed issue for browsing finished work.

- Placed last in the tab bar, after Knowledge, on key `4`; the existing keys 1 to 3 are unchanged.
- Label carries its count like the other tabs, `Closed [n]` (or `[matched/total]` while filtered).
- A flat list, most recently closed first. Each row: closed date, id, title.
- The detail pane shows the selected issue in full, close reason included.
- Actions: `R` reopens the selected issue (it leaves the tab), `/` filters (text words, `label:`,
  `status:` over closed statuses), `d` opens the dependency view, `e` opens the editor.
  Claim, close and status change do not apply here.
- The `/status:closed` filter on the Issues tab keeps working as today.

## Acceptance

- `4` or a click on the label opens the Closed tab; Tab cycles through all four tabs.
- The list holds exactly the issues in a closed category, newest close first, rows as
  `<closed date>  <id>  <title>`.
- `R` reopens the selected issue, which then leaves the Closed tab and shows on Issues
  (and Ready when it is ready); the counts update.
- `/`, `d` and `e` work as on the Ready tab; `a`, `c` and `s` do nothing on this tab.
- The help screen lists the Closed tab and key `4`.
- Tests cover the list order, reopen and filtering on the tab.
