---
title: File map in internal/ui/doc.go with a structural test
status: open
type: task
priority: 2
labels: [ui]
plan: 2026-10-16 - 03 - ui-fill-tests-and-file-map-plan
created: 2026-10-16T10:26:47Z
---

`internal/ui` has no `doc.go` file map and no structural test that checks it. The package's
only package comment is in `internal/ui/ui.go:1`, and it maps none of the files. The package now
holds 23 non-test `.go` files; the deferral was filed when it held 12.

Why: the Go coding standard requires a `doc.go` that maps every non-test file (half a line each)
once a package passes about 10 files, plus a structural test that fails when a file is missing
from the map. The later UI work keeps adding files, so the map should exist before more land.
`internal/tree/doc.go` with `internal/tree/doc_test.go` (`TestDocFileMap_MatchesDirectory`) and
`internal/cli/doc.go` with `internal/cli/doc_test.go` show the house pattern.

## Acceptance

- `internal/ui/doc.go` carries the package comment and maps every non-test `.go` file in
  `internal/ui` with a half-line role each.
- A structural test in `internal/ui` fails when a non-test file is missing from the map (and when
  the map names a file that does not exist), following `internal/tree/doc_test.go`.
- `go vet ./internal/ui` and `go test ./internal/ui` pass.
