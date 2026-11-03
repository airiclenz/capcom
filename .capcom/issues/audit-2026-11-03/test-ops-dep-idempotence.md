---
title: Dependency idempotence and alias matching are untested in ops
status: open
type: task
priority: 3
plan: 2026-11-03 - 02 - eval-harness-and-test-gaps-plan
created: 2026-11-03T08:23:42Z
---

In `internal/ops/deps.go:46-77`, AddDep's no-op for a dep already present and RemoveDep's
matching of a dep written as an alias (through `graph.Resolve`) have no test. The AddDep closure
also appends without re-checking presence in the file, so a stale Catalog can duplicate a dep.

Wanted: the tests below, and skip the append in the closure when the file already names the dep.

## Acceptance

- AddDep twice, and AddDep with an alias of an existing dep, leave Deps and the file bytes unchanged.
- RemoveDep with the id of an issue stored as an alias removes that entry.
- AddDep with a stale Catalog that misses an existing dep does not duplicate it in the file.
