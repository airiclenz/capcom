---
title: The UI heading renderer strips the author's leading hash
status: open
type: bug
priority: 3
plan: 2026-11-03 - 01 - remaining-small-bugs-plan
created: 2026-11-03T08:23:42Z
---

`headingText` (`internal/ui/markdown/markdown.go:327`) uses `TrimLeft(line, " #")`, which removes
every leading space and `#` after the marker, not just the marker. `## #42 follow-up` renders as
`42 follow-up` and `# #hashtag` as `hashtag`.

Wanted: slice off the marker (the n `#` that `isHeading` counted), then `TrimSpace` the rest.

Source: docs/reviews/code-audit-2026-11-03.md.

## Why

The issue-number hash the author wrote disappears from the rendered heading.

## Acceptance

- `## #42 follow-up` renders as `#42 follow-up` and `# #hashtag` as `#hashtag`.
- Ordinary headings, including ones with trailing `#`, render as before.
