---
title: Names claimed by files left out of the index fall through to prefix matching
status: done
type: bug
priority: 2
deps: [refresh-race-tests]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:14Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

Names claimed by files the index leaves out fall through to unique-prefix matching. The
2026-10-12 fix made the id of a parsed-but-invalid file resolve to "… is invalid", never a
prefix match, but it did not cover that file's Aliases, nor files refused at walk time (symlink,
FIFO, over 1 MiB, and the subtree such a refusal orphans). A file with problems gets only a
`files` row, so its Aliases are never stored, and `unusableClaim` compares only the file's base
name. Severity: Medium for Aliases of invalid files, Low for walk-refused files.

Trigger: (a) `capcom mv auth auth-rework` leaves `auth-rework.md` with `aliases: [auth]` beside a
sibling `auth-tests`. `auth-rework.md` then becomes invalid (a status removed from
`config.yaml`, an empty label added by hand). `capcom close auth` finds nothing exact and
prefix-matches `auth-tests`: the write lands on the wrong Issue. `ResolveNote` shares the same
lookup. (b) `foo.md` is a symlink or too large beside `foo-bar.md`; `capcom close foo` acts on
`foo-bar`.

Wanted: every name a left-out file claims (its id and its Aliases), whether refused at walk time
or at read and validate time, resolves to `ErrNotFound` "… is invalid" and never to a prefix
match. Keep the claimed names of a file with problems (Alias rows marked unusable, or a claims
table, from the parse or from raw front-matter Alias recovery), and record walk-refused node
files and the subtree they orphan as `files` rows with a problem. Bump the index schema.

## Acceptance

- `query_test.go`: `Resolve("auth")`, where `auth` is an Alias of an invalid file beside
  `auth-tests`, and `Resolve("foo")`, where `foo.md` is a symlink beside `foo-bar.md` (unix),
  both return `ErrNotFound` naming the file.
- A CLI `close` test shows `auth-tests` untouched.
- `GOMEMLIMIT=2GiB go test ./internal/index/`, then `./internal/cli/ -run Close`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-09.
