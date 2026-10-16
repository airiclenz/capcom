---
title: Show every frontmatter key in the detail header
status: done
type: task
priority: 2
labels: [ui]
plan: 2026-10-16 - 03 - ui-fill-tests-and-file-map-plan
created: 2026-10-16T18:44:01Z
closed: 2026-10-16T19:07:35Z
reason: closed by plan 2026-10-16 - 03 - ui-fill-tests-and-file-map-plan (bf3fb87), items 4-5
---

The detail pane's header shows only part of the frontmatter: an Issue's title, status, priority,
labels, parent and deps, and a Note's title, labels and refs (`issueHeader` and the Note header in
internal/ui/detail.go; docs/design/tui.md, "Detail pane"). An Issue's type, related, plan,
aliases, created, closed and reason, and a Note's retired, superseded-by and aliases, never show.
No ADR or Note gives a reason for the subset. The user has to open the file to see, for example,
which plan covers an issue or why it was closed.

Wanted (ratified by the user, 2026-10-16):
- **Fields:** every frontmatter key the file can hold, plus parent, each line left out when empty.
  Derived facts `capcom show` adds (path, category, ready, etag) stay out.
- **Notes too:** the Note header adds retired, superseded-by and aliases.
- **Layout:** one `key: value` line per key, as today, in the file's key order. Status, priority and
  type share the first line (`status: open · P2 · task`). Keys that name an id (parent, deps,
  related, superseded-by) are code spans, so the renderer makes them links. Dates read
  `2026-10-16 17:22` (UTC, as stored). `retired` shows only when true.

## Acceptance

- An Issue holding every key shows each one in the header in the file's key order. An Issue
  holding none of the optional keys shows only title, status, priority and type.
- A Note holding retired, superseded-by and aliases shows all three. A Note without them shows
  none of the three.
- parent, deps, related and superseded-by naming an existing id are followable links.
- The header reads the fields from the file, as it reads labels and deps today, so an edit shows
  without a reindex.
- docs/design/tui.md's "Detail pane" bullet describes the full header.
- Affected goldens are rewritten and read; `GOMEMLIMIT=2GiB go test -count=1 ./internal/ui/` passes.
