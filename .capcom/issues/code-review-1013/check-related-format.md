---
title: check has no store-format gate for the related key
status: open
type: bug
priority: 3
deps: [config-owns-format]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
---

`check` has no store-format gate for the Issue `related` key: `checkSupersession` emits
`format-too-old` for the Note keys only, and the Issue checks never compare against the declared
format. The per-key thresholds are duplicated as well: the supersession minimum (3) is defined in
both `check` and `cli`, and the `related` minimum (2) only in `cli`. Severity: Low.

Trigger: a hand edit or a merge leaves `related: [x]` in a store that declares format 1. `check`
passes, but every format-1 binary rejects the unknown key and leaves the Issue out.

Wanted: read the per-key minimum formats from `config` (`config.FormatRelated`,
`config.FormatSupersession`, from `config-owns-format`) in both `cli` and `check`, and add a
`format-too-old` problem at the Issue's `related` line (`RelatedLine()`) when the declared format
is below the minimum. Update `docs/format.md` where it describes the gate.

## Acceptance

- A `check` test with format 1 and an Issue carrying `related` produces `format-too-old` at that
  line.
- `grep -rn 'FormatVersion = [23]' internal/` finds a single definition per key.
- `GOMEMLIMIT=2GiB go test ./internal/check/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-17.
