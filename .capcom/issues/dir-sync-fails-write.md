---
title: A directory-sync failure reports a write that landed as failed
status: open
type: bug
priority: 3
created: 2026-11-03T08:23:42Z
---

`WriteFileAtomic` (`internal/nodefile/atomic.go:153`) and `WriteFileExclusive` (`:119`) return a
`syncDir` error after a successful rename or link as a write failure, although the new content is
already in place. On a filesystem whose directory fsync fails (EINVAL or ENOTSUP on some network
or FUSE mounts, or a transient EIO) the caller sees an error for a write that took effect, and a
retry of `WriteFileExclusive` then fails with `fs.ErrExist`.

Wanted: treat a directory-sync error as best effort (ignore EINVAL and ENOTSUP), or return a
distinct "written, but not synced" error that callers can tell apart.

Source: docs/reviews/code-audit-2026-11-03.md.

## Why

A false failure makes agents retry a write that already happened, and the retry then fails.

## Acceptance

- With directory sync stubbed to fail with EINVAL or ENOTSUP, `WriteFileAtomic` and `WriteFileExclusive` succeed.
- Any other sync error is either reported as a distinct written-but-not-synced error or documented as best effort, and a test pins the choice.
