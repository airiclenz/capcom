---
title: show with several unknown ids blurs whose suggestions they are
status: open
type: bug
priority: 2
created: 2026-10-26T09:46:10Z
---

`capcom show` with several unknown ids joins their messages with "; "
(internal/cli/show.go:127) and the suggestions of the first failure are appended after the
joined text, as in `id "a" not found; id "b" not found; did you mean ...?`. In text output it
is ambiguous which id the suggestions belong to.

## Acceptance

- Text output of `capcom show a b` with both ids unknown makes clear which id each suggestion
  is for (or names suggestions per id).
- `--kv` and `--json` output are unchanged or stay consistent with the text.
- A test in internal/cli covers two unknown ids that each have a close match.
