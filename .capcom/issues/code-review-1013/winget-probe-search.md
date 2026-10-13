---
title: Winget done-probe relies on the lagging GitHub search index
status: done
type: bug
priority: 3
deps: [channel-repo-seam]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

The winget "done" probe in `cmd/release` lists pull requests with `gh pr list --search "... in:title"`,
which depends on GitHub's lagging search index; komac's own existing-PR check is search-based too.
Severity: Low.

Trigger: the smoke step fails right after winget submitted its pull request, and the maintainer
reruns `publish` at once. Search has not yet indexed the new pull request, so the probe reads
pending and `komac update --submit` opens a duplicate pull request on `microsoft/winget-pkgs`, a
third-party repository.

Wanted: probe through a read that does not use the search index, such as `gh api graphql` on the
viewer's open pull requests filtered to winget-pkgs, or a listing of the branches komac creates in
the maintainer's fork. `OpenPulls` on the `channel-repo-seam` interface is its natural home.

## Acceptance

- A probe test asserts that the non-search read is used (through `fakeRunner`), or a fake reports
  an open pull request through the channel-repository seam and the probe reads done.
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run Winget` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-14.
