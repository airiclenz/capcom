---
title: Elsewhere scan reads other-branch blobs with no size cap
status: done
type: bug
priority: 2
deps: [elsewhere-failure-tests]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:14Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

The Elsewhere scan reads other-branch blobs with no size cap. `gitx.ShowFile` runs
`git cat-file blob`, `gitx`'s `run` buffers all of stdout and copies it twice, and
`branchStatus` hands it to `issue.Parse`. Severity: Medium (security).

Trigger: a fetched remote-tracking branch (a hostile fork or pull request branch) commits a
highly compressible multi-GB `.capcom/issues/x.md`. Every reading command and `prime` within
the day window then pulls it into memory about three times and is OOM-killed.

Wanted: `ShowFile` takes a size limit; it checks the blob size first (`git cat-file -s
<rev>:<path>` or `--batch-check`) and returns a sentinel above 1 MiB. The scan turns that into a
"larger than 1 MiB" warning reusing the `tree.ErrTooLarge` wording. Optionally, `ChangedFiles`
uses `diff --raw -z --no-renames` and carries the destination mode, so the scan skips anything
that is not `100644` or `100755`.

## Acceptance

- An elsewhere or gitx test commits a blob over 1 MiB on a side branch and asserts one warning
  naming the path and the branch, and no entry.
- `GOMEMLIMIT=2GiB go test ./internal/gitx/`, then `./internal/elsewhere/`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-02(d).
