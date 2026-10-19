---
title: Make capcom easier for a human reviewing agent work
status: open
type: feature
priority: 2
created: 2026-10-19T14:54:44Z
---

Groups the work that makes capcom easier for a human who reviews what agents did and steers
them. The human's main job with capcom is reading the result of agent sessions, so these
Issues favour review and navigation over editing.

`capcom ui` already has a help overlay (`internal/ui/help.go`), a dependency view
(`internal/ui/depview.go`) and editing in `$EDITOR` (`internal/ui/editor.go`); those are not
part of this group.

## Acceptance

- Every child Issue is closed.
