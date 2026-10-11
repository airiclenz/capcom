---
title: internal/cli has no doc.go file map or structural test
status: open
type: chore
priority: 2
created: 2026-10-11T14:07:46Z
---

`internal/cli` has grown well past ten non-test files (`create.go`, `update.go` and `log.go` first
pushed it past the threshold) but has no `doc.go` file map and no structural test. The
coding-standards Go rule calls for both once a package grows past ~10 files.

Found 2026-10-11, implementing plan `2026-10-11 - 00 - capcom-v1-issues-plan`, item 8 (Write
commands).

## Acceptance

- `internal/cli/doc.go` maps each file to its responsibility.
- A structural test fails when a file is added without updating the map.
