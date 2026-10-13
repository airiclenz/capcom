---
title: A draft release left by an interrupted create is probed as done
status: done
type: bug
priority: 2
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:14Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

A draft release left by an interrupted `gh release create` is counted as published. Given
assets, `gh release create` makes a draft, uploads, then publishes; a Ctrl-C or kill during the
upload leaves a draft. On a rerun, `convergeRelease` uploads only the missing assets and never
un-drafts the release, `probeRelease` reports done, and the Homebrew, Scoop and winget steps
push URLs that return 404 for everyone else. `smoke` downloads through authenticated `gh`, which
can read draft assets, so it does not notice. Severity: Medium.

Wanted: a draft release is never reported done, and a rerun publishes it once its assets are
complete. `execRepo.ReleaseAssets` asks for `isDraft` and the repo seam (and `memRepo`) carry it;
`probeRelease` reports a draft as pending ("draft"); `convergeRelease` runs
`gh release edit <tag> --draft=false --latest --repo airiclenz/capcom` after the asset upload.
Every `gh release` call names `--repo airiclenz/capcom` (ADR 0008).

## Acceptance

- `probes_test.go` shows a draft reading pending.
- `publish_test.go` converges a draft with all assets present and asserts exactly one un-draft
  call, after which the probe reads done.
- `repo_test.go` pins the argv of the new `execRepo` call.
- `GOMEMLIMIT=2GiB go test ./cmd/release/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-04.
