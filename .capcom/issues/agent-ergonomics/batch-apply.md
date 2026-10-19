---
title: Filing a linked set of issues takes one call per issue and edge
status: open
type: feature
priority: 3
created: 2026-10-19T14:54:45Z
---

Turning a plan into Issues takes one `create` per Issue and one `dep add` per edge. A frontier
model filing fifteen linked Issues makes thirty calls, and a failure midway leaves a half-filed
set behind.

Wanted: `capcom apply <file>` (and `-` for stdin) reads a YAML list of operations (create,
update, dep add, close, log) and applies all of them or none. Ids created earlier in the file
can be used by later operations. `--dry-run` validates the whole file against the store and
prints what would change without writing. Writes go through `internal/ops`.

Settle the file format in the plan before coding, and keep it a plain list that mirrors the
CLI flag names.

## Acceptance

- A file that creates three Issues with dependencies between them applies in one call.
- An invalid operation anywhere in the file leaves the store untouched and names the operation
  by its position and the file line.
- `--dry-run` reports the same outcome as a real run and writes nothing.
- The file format is documented under `docs/site/reference/`.
