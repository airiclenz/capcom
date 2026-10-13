---
title: Rebuilt SHA256SUMS uploaded unchecked beside existing archives
status: open
type: bug
priority: 3
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:15Z
---

When a release exists without SHA256SUMS, the rebuilt sums are uploaded unchecked. The check
against published sums runs only when SHA256SUMS is already uploaded; otherwise the rebuilt file
is uploaded as is, and the build uses whatever `go` is on PATH. Severity: Low.

Trigger: `gh release create` uploads the archives and dies before SHA256SUMS lands, and the
rerun comes after a Go upgrade or from another host. The rebuilt SHA256SUMS describes different
archive bytes, and Homebrew, Scoop and winget render their hashes from it and push. Smoke fails
the run at the end, by which time every channel carries bad hashes and installs fail their
checksum. When SHA256SUMS is published, archives about to be uploaded beside it are never
checked against it either.

Wanted: no SHA256SUMS is uploaded that does not match the archives already on the release. When
it is absent but archives exist, download and hash them and require a match with the rebuild,
otherwise refuse with the `gh release delete-asset` way out; when it exists, check every rebuilt
archive against it before uploading.

## Acceptance

- In `publish_test.go` with the `memRepo` seam, the release holds archives A and B and no sums,
  and the rebuild yields a different A: `publish` refuses and uploads nothing.
- A case with SHA256SUMS published and a rebuilt archive that does not match it refuses before
  uploading.
- `GOMEMLIMIT=2GiB go test ./cmd/release/ -run Publish` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-13.
