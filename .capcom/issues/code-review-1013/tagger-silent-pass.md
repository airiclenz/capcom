---
title: version-bump.sh passes green without tagging (fallback range, v prefix)
status: open
type: bug
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:15Z
---

`.github/scripts/version-bump.sh` passes green without tagging. Severity: Low.

- (a) When `github.event.before` is unusable (a force-push or branch creation), the scan narrows
  to the head commit. A bump buried under later commits is missed, the job passes green, and a
  re-run gets the same `before`.
- (b) A `v`-prefixed VERSION is accepted by `publish` (`checkPushable`) and `dist` but refused by
  the tagger, which writes to stderr and exits 0. `publish` then waits out its full
  `--tag-timeout` and fails pointing at a green workflow run, which breaks "never swallow errors
  silently".

Wanted (decided 2026-10-13): a bare `X.Y.Z` VERSION everywhere: `checkPushable` and `dist`
reject a leading `v`, naming the VERSION file, and the script exits non-zero after any refusal.
In its fallback, the scan starts at the newest reachable release tag
(`git describe --tags --abbrev=0 --match 'v[0-9]*'`) before falling back to the parent commit.

## Acceptance

- `versionbump_test.go`: with `before` absent from the clone and the bump under a later commit,
  the tag is created; with VERSION `v0.6.0`, the script exits non-zero. Both are real-git tests,
  isolated and skipped on Windows per ADR 0008's bump-test amendment.
- `dist_test.go` and `publish_test.go` assert that a `v`-prefixed VERSION is rejected.
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'VersionBump|Dist|Pushable'` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-15.
