---
title: "Code audit 2026-10-13: fix the findings"
status: open
type: task
priority: 1
plan: 2026-10-13 - 01 - code-audit-fixes-plan
created: 2026-10-13T20:24:39Z
---

The code audit of 2026-10-13 (`docs/reviews/code-audit-2026-10-13.md`) found 20 Medium
findings: correctness bugs with realistic triggers, guard bypasses, a quadratic parse reachable
from the git merge driver, and critical paths that have no test. Left alone they can leave a
store unreadable or declaring a newer format than it needs, run an unverified binary during a
release smoke, lose user files on Windows or on the copy fallback, and let a crafted body hide
conflict markers from `check`. This issue tracks fixing every one of them.

Findings, in the review's order:

- Medium: the store format is raised before the write can still fail, so a lost race leaves
  `config.yaml` raised with no node needing it (`internal/nodestore/edit.go`, `internal/nodestore/create.go`)
- Medium: the `hooks` installers read working-tree files without the `nodefile` guard
  (`internal/hooks/settings.go`, `internal/hooks/gitattributes.go`)
- Medium: exclusive create breaks its "no file or the whole new one" promise on the copy
  fallback (`internal/nodefile/atomic.go`)
- Medium: `know update --remove-superseded-by` ignores aliases and silently does nothing
  (`internal/cli/know_update.go`)
- Medium: `ReadNodeFile` is a second name for `ReadRegularFile` (`internal/nodefile/read.go`)
- Medium: `RaiseFormat` writes a duplicate `format:` key and leaves the store unreadable
  (`internal/config/format.go`)
- Medium: `release smoke` runs a host binary whose hash did not match (`cmd/release/smoke.go`)
- Medium: `release smoke` upgrades the Homebrew install when the release lookup fails
  (`cmd/release/homebrew.go`)
- Medium: `Install` makes an existing settings file world-readable or replaces its symlink
  (`internal/hooks/settings.go`)
- Medium: on Windows a save fails while another process has the node file open
  (`internal/nodefile/atomic.go`)
- Medium: the issue Log parser rescans fences quadratically, reachable from the git merge driver
  (`internal/issue/mutate.go`)
- Medium: `NameClashes` produces k×(k−1) findings for k files sharing a name
  (`internal/diagnose/clashes.go`)
- Medium: a body line like "```inline``` …" opens a fence and hides every conflict marker below
  it (`internal/rawfm/fence.go`)
- Medium: `init` can leave an empty or truncated `config.yaml` that a re-run will not repair
  (`internal/store/store.go`)
- Medium: no test that a `komac update` failure keeps the GitHub token out of the output
  (`cmd/release/winget.go`)
- Medium: no test for the winget listing-read error that chooses `submit` or `update`
  (`cmd/release/winget.go`)
- Medium: `check` does not test its AGENTS.md block stage (`internal/check/agents.go`)
- Medium: no test for the related-links self-reference rule in `check`
  (`internal/check/issues.go`)
- Medium: no test for the merge driver's fallback when a file already contains the Log
  placeholder (`internal/issue/merge.go`)
- Medium: no tests for the verbatim write pair that puts user files back unchanged
  (`internal/nodefile/atomic.go`)

Each finding's trigger, impact and suggested fix are in the review.

## Acceptance

- every finding above is fixed, or for a Critical-Path Tests finding, covered by a test that
  fails when the path breaks
- `go vet ./...` and `go test ./...` pass

Source: docs/reviews/code-audit-2026-10-13.md
