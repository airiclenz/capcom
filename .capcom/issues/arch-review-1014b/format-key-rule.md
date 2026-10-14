---
title: Codec and check disagree on which key lines need a store format
status: done
type: bug
priority: 2
deps: [kind-reader]
plan: 2026-10-14 - 02 - architecture-review-2-plan
created: 2026-10-14T16:53:33Z
closed: 2026-10-14T19:03:49Z
reason: resolved by plan 2026-10-14 - 02 - architecture-review-2-plan (7123d17f)
---

The rule "a key needs store format N" is coded twice and the copies disagree. The nodestore codec
raises the format only when a write gains an entry (`gainsEntry`, `gainsSupersession` in
`internal/nodestore/codec.go`); check flags any key line (`internal/check/issues.go:121`,
`internal/check/notes.go:100`). On a hand-written `related: []` or `superseded-by: []` they give
different answers.

Wanted (ratified 2026-10-14): any key line needs its format, empty lists and `retired: false`
included, because ADR 0009 keeps an unknown key an error and a binary older than the key rejects
the file whatever the key holds. The descriptor holds "key → format" once; the codec raises the
format when the file it writes would carry the key line, and check flags the same lines.

## Acceptance

- One table in nodestore maps each key to the format it needs; codec and check both read it.
- A write that leaves a key line in a file of a store below the key's format raises the format;
  check flags `related: []` and `superseded-by: []` below their formats, and tests pin both.
- The CHANGELOG `Unreleased` section has a line if a user-visible result changes.

Source: docs/reviews/architecture-review-2026-10-14.html, 3 · One node reader and descriptor per kind.
