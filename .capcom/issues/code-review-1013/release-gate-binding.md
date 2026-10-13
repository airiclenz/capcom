---
title: Release gate is skipped once the tag exists, and the push uses symbolic HEAD
status: open
type: bug
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:15Z
---

The release gate does not bind to the commit it judged. Severity: Low.

- (a) Once the version's tag exists on origin, `publish` skips the gate entirely, and the tag
  workflow tags every VERSION bump on main without waiting for CI. A bump that reached main
  outside `publish` is then built and released from a commit whose CI may be red or still
  running. A test pins the skip, but nothing tells a resume apart from a tag made outside
  `publish`.
- (b) The gate judges a resolved SHA, but the push uses the symbolic `HEAD:main`. A commit made
  by another session between them reaches `origin/main` without a green run (the release still
  builds from the tag).

Wanted (decided 2026-10-13): while the tag is done but the release step is still pending,
`publish` keeps the CI half of the gate: it peels the bumps from the tag's commit and requires a
green run for it, dropping only the clean-tree and HEAD-contains-`origin/main` conditions, which
matter only for the push. The resolved SHA is recorded and used for every later read and for the
push (`git push origin <sha>:refs/heads/main`), and HEAD is re-checked against it before
pushing. The ADR 0008 Note is amended to match.

## Acceptance

- A `publish_test.go` case where the tag exists, the release is pending and CI is red on the tag
  commit asserts that `publish` refuses with the gate error.
- A case where HEAD moves between the gate and the push asserts a refusal, and a passing case
  asserts that `PushMain` receives the SHA.
- The ADR 0008 Note states the kept CI half and the SHA binding.
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'Publish|Gate'` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-12.
