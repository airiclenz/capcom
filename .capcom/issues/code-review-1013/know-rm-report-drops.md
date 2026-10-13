---
title: know rm does not report the superseded-by entries it removed
status: done
type: bug
priority: 3
deps: [reference-rewrite-engine]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`know rm` does not report the `superseded-by` entries it removed from other Notes. The doc on
`nodestore.Removed` promises "the entries of other nodes the removal dropped", but `Removed` has
no field for them: `Remove` copies only the deps and related drops, although `planNoteRemove`
fills `supersededByDropped`, and `know rm`'s result carries only the id and path, unlike `rm`'s
`issueRemoveResult`. Severity: Low.

Trigger: `know rm adr-0011` rewrites `adr-0003.md` silently, and `--json` and `--kv` consumers
cannot see it. The rewrite itself is documented in `docs/format.md` and visible in `git diff`.

Wanted: add `SupersededByDropped []string` to `Removed`, fill it in `Remove`, and emit it in
`noteRemoveResult` as `superseded_by_dropped`, mirroring `issueRemoveResult`. Update
`docs/format.md` and the README's result-key list where they list `rm` keys. With
`reference-rewrite-engine` drops are reported keyed by field, and this falls out generically.

## Acceptance

- A CLI test of `know rm` on a Note that another Note lists as `superseded-by` asserts the
  `superseded_by_dropped` key in `--json` output and the dropped entry in text.
- `GOMEMLIMIT=2GiB go test ./internal/nodestore/`, then `./internal/cli/ -run KnowRm`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-24.
