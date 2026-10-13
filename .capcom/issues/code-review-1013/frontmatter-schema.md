---
title: Generic frontmatter field table (speculative)
status: done
type: task
priority: 3
deps: [yaml-multi-document]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`internal/issue/file.go` and `internal/knowledge/note.go` both repeat a `field` type, `listField`,
`fieldKeys` and a Parse loop ending in an "unreachable" return. Strength: speculative.

Wanted: a generic `frontmatter.Schema[T]` with `Text`, `List` and `Raw` builders, used by both
packages; it saves about 80 lines. It touches the parsing that `yaml-multi-document` fixes, which
lands first.

Risks: the canonical key order and the round trip must stay byte-identical.

## Acceptance

- `issue` and `knowledge` declare their fields through `frontmatter.Schema`, and the duplicated
  `field`, `listField` and `fieldKeys` are gone.
- The round-trip and key-order tests pass unchanged.
- `GOMEMLIMIT=2GiB go test ./internal/frontmatter/`, then `./internal/issue/` and
  `./internal/knowledge/`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-13.
