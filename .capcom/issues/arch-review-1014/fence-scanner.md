---
title: One frontmatter fence predicate
status: open
type: task
priority: 3
plan: 2026-10-14 - 00 - architecture-review-fixes-plan
created: 2026-10-14T08:35:11Z
---

Three places find where a file's frontmatter ends: `rawfm`'s `bodyStartIndex` (conflict
scanning) and `frontmatterEndIndex` (alias recovery), and `frontmatter.Split`. Each has its own
fence test, and they disagree on an edge: a closing `---\r` at the end of the file (a lone-CR
line ending) closes the frontmatter for one reader but not for another, so the same file can
read as having frontmatter or not depending on who reads it.

Wanted: `rawfm` exports one fence predicate that all three use. Plan author call (2026-10-14):
a closing `---\r` at end of file closes the frontmatter in `frontmatter.Split` too.

## Acceptance

- `bodyStartIndex`, `frontmatterEndIndex` and `frontmatter`'s fence split call the one
  predicate; no other fence test remains.
- A test shows a closing `---\r` at end of file closes the frontmatter in `rawfm` and in
  `frontmatter.Split`.
- `go vet ./...` is clean and the touched packages' tests pass.

Source: docs/reviews/architecture-review-2026-10-14-0848.html, Smaller locality fixes.
