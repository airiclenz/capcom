---
title: Store owns worktree-relative paths
status: done
type: task
priority: 3
plan: 2026-10-14 - 00 - architecture-review-fixes-plan
created: 2026-10-14T08:35:11Z
closed: 2026-10-14T11:17:48Z
reason: Resolved by plan "2026-10-14 - 00 - architecture-review-fixes-plan" (b1ff6b8).
---

Turning an absolute path into a worktree-relative one for messages is written five times:
in `store`, `check`, `nodestore`'s `Kind`, the index walk and cli's issue rendering. The copies
differ in edge cases (a file whose name starts with `..`, such as `..foo`, can be taken for a
path outside the worktree), so the same file can be shown differently by different commands.

Wanted: the store owns it. `store.RelativePath(root, path)` and `(*Store).Relative(path)` return
a slash-separated worktree-relative path; outside the worktree they return the original path,
slash-separated (plan author call, 2026-10-14). check, the index, nodestore and cli use them.

## Acceptance

- `store.RelativePath` and `(*Store).Relative` exist with tests covering inside, outside and a
  file named `..foo`; no other worktree-relative helper remains.
- `go vet ./...` is clean and the touched packages' tests pass.

Source: docs/reviews/architecture-review-2026-10-14-0848.html, Smaller locality fixes.
