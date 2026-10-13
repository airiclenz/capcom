---
title: publish rerun for an older version downgrades the tap and the bucket
status: done
type: bug
priority: 2
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:14Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

Rerunning `publish` for an older version downgrades the Homebrew tap and the Scoop bucket.
`pinnedVersionStep` reports the channel as pending whenever the pinned version differs from the
requested one, including when the pinned version is newer, and converge then overwrites it with
the older formula or manifest. A tag that already exists skips `checkPushable`, so no gate stops
it. Severity: Medium.

Trigger: `publish v0.5.0` stops after its release step, v0.5.1 is then published fully, and the
maintainer reruns `publish v0.5.0` to finish winget. That silently downgrades
`airiclenz/homebrew-tap` and `airiclenz/scoop-bucket` for every user.

Wanted: a channel pinned to a newer version than the one being published reports done ("pins
newer X") and is never rewritten. Compare parsed versions with the existing version helper in
`cmd/release`; report pending only when the pin is lower or absent.

## Acceptance

- In `probes_test.go`, a newer pin reads done with the "pins newer" detail.
- In `publish_test.go`, a converge with a newer pin issues no clone, commit or push on the
  `memRepo` and `fakeRunner` seams.
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run 'Pinned|Publish'` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-03.
