---
title: "What capcom keeps where: files in .capcom, index and locks under the git dir"
labels: [architecture]
refs: [docs/site/reference/file-format.md, internal/index/index.go#index.db, internal/lock/lock.go#locksDir, internal/store/store.go#func Open]
---

Committed, versioned with the code: `.capcom/config.yaml`, `.capcom/issues/` and
`.capcom/knowledge/` (ADR 0004). Never committed, under the git directory: the SQLite index at
`<git-dir>/capcom/index.db` (per worktree, disposable, rebuilt by `capcom reindex`) and the Tree
lock records at `<git-common-dir>/capcom/locks/<key>.json` (shared by every worktree of a clone,
ADR 0005). The store root is resolved from the working directory, never hard-coded, and capcom
never runs a writing git command. `docs/site/reference/file-format.md` is the full reference.
