---
title: show and know show pair index content with a later disk etag
status: done
type: bug
priority: 2
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:14Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

`show` and `know show` pair index content with an etag read later from disk. The fields and body
come from the index, while the etag comes from a separate, later read of the file. ADR 0010 says
the etag derives from the file bytes and is what a later `--if-match` is checked against, so the
two must describe the same bytes. Severity: Medium (a lost update).

Trigger: (a) agent A runs `capcom show X`; agent B runs `capcom update X --status doing` after
A's index refresh and before A's file read. A sees `status: todo` with the etag of B's version,
and A's `update --if-match <etag>` then passes and overwrites a change A never saw. (b) A write
that keeps size and mtime is skipped by the refresh by design, so the stale pairing repeats with
no race needed.

Wanted: every field, body and etag that `show` and `know show` print derive from one read of the
file's bytes. Parse the bytes read for the etag with `issue.Parse` or `knowledge.Parse` and
render from that; use the index only to resolve the name and for index-only data such as
readiness and children. A file that no longer parses fails with the error a write would give.

## Acceptance

- One CLI test each for `show` and `know show` rewrites the file between index open and the body
  read (through a seam, or a same-size edit after a refresh with the mtime restored by
  `os.Chtimes`) and asserts that the printed body and status match the etag's bytes.
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Show'` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-05.
