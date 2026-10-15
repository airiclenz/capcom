---
title: Document capcom ui in the manual and references
status: open
type: task
priority: 2
deps: [ui-ready-tab, ui-issue-tree, ui-knowledge-tab, ui-dep-view, ui-filter, ui-live-refresh, ui-writes, ui-editor, ui-mouse, ui-theme]
created: 2026-10-15T20:03:24Z
---

Users need to find and learn `capcom ui`.

## Acceptance

- A manual page under `docs/site/` covers tabs, keys, mouse, writes, the lock prompt, conflicts
  and the theme config, with the frontmatter the site requires.
- `go run ./cmd/gendocs` has been run and `TestReferenceUpToDate` passes.
- `docs/site/reference/file-format.md` describes the per-user config file.
- `docs/design/tui.md` is updated wherever the build diverged from it.
