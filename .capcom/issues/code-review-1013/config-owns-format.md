---
title: Store format and per-key thresholds owned by config
status: done
type: task
priority: 3
deps: [split-config-go, split-check-go]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:43Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

Two store-format rules have no single owner (ADR 0009). "A missing `config.yaml` means format 1"
is written three times (`keylessFormat` in `config`, `missingConfigFormat` in `check`, and a
comment on `store.RaiseFormat`), while `Store.Config.Format` says 3 for a missing file because
`loadConfig` falls back to `config.Default()`. The per-key minimum formats (`related` needs 2, the
supersession keys need 3) are copied into `cli` and `check`, and `check` lost one of them.
Strength: Strong.

Wanted: `config` owns the store format. Add `config.MissingFormat` (= 1) and have `loadConfig`
return it for a missing file; export `config.FormatRelated` and `config.FormatSupersession` and
have `cli` and `check` read them; `check` reads `Config.Format` directly instead of re-statting
`config.yaml`. `config` stays a leaf package, importing nothing from `internal/`. ADR 0009 is
implemented more faithfully, not changed. This absorbs `missing-config-format` and
`check-related-format`, which carry their own acceptance.

## Acceptance

- `config.MissingFormat`, `config.FormatRelated` and `config.FormatSupersession` are the only
  definitions of those values: `grep -rn 'FormatVersion = [23]' internal/` finds none outside
  `config`, and `check` has no `missingConfigFormat` and no second stat of `config.yaml`.
- Opening a store with no `config.yaml` gives `Config.Format == 1`.
- `go list -f '{{join .Imports "\n"}}' ./internal/config/` lists no `internal/` package.
- `GOMEMLIMIT=2GiB go test ./internal/config/`, then `./internal/store/`, `./internal/check/` and
  `./internal/cli/`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-07.
