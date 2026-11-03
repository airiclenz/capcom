---
title: The UI write path's failure branches are untested
status: open
type: task
priority: 3
created: 2026-11-03T08:23:42Z
---

The post-write failure branches of `applyWritten` and `applyConflict` (`internal/ui/write.go:343`)
have no test: `withRefreshFailure`, the `reloadAfterFormat` path when the conflicted issue cannot
be re-read (about `:375`), and the generic non-conflict write error notice
(`writeFailurePrefix`). These are the error paths of the UI's write path.

## Acceptance

- A test makes `Refresh` fail and asserts the notice reads `<notice> (refresh: ...)`.
- A test deletes the issue file between the conflict and the re-read and asserts `write: ... (reload: ...)`, no prompt opened, and the on-disk file unchanged.
- A test covers the generic non-conflict write error notice.
