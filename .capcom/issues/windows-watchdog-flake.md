---
title: Agents test watchdogs time out on slow Windows CI
status: done
type: bug
priority: 2
created: 2026-10-18T05:57:13Z
closed: 2026-10-18T05:57:14Z
reason: Raised the guidesWithin and syncWithin watchdogs from 5 s to 30 s.
---

`TestGuides_GuideInSubdirectory_ImportsFromItsDirectory` failed on `windows-latest` in CI run
39101919153 (the v0.9.0 version bump): "Guides did not return within 5 s". The watchdogs in
`guidesWithin` (`internal/agents/guides_test.go`) and `syncWithin` (`internal/agents/block_test.go`)
exist to catch a hang opening a named pipe, not to bound speed; under `t.Parallel()` on the slow
Windows runner a normal call can pass 5 s.

## Acceptance

- Both watchdogs allow 30 s, still far below a real hang's `go test` timeout.
- `go test ./internal/agents/` passes.
