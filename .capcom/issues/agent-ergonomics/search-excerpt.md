---
title: Search hits print no body, so each needs a show
status: done
type: feature
priority: 2
plan: 2026-10-26 - 01 - p1-p2-ergonomics-plan
created: 2026-10-26T07:51:29Z
closed: 2026-10-26T13:30:36Z
reason: Resolved by plan 2026-10-26 - 01 - p1-p2-ergonomics-plan (archived 6c800bf)
---

`capcom know search` prints one line per hit: id and title. Whether a hit holds the answer is only
known after a `know show` on it, and with several hits there is no telling which to show first.
`capcom search` has the same shape for Issues.

Why: the field evaluation (capcom 0.9.1, experiment `eval-find-answer`, "which Note says X")
spent eight searches plus a tree listing on one question; one distinctive-word search whose output
carried the matched text would have answered it in one call. The evaluation report ranks this the
change that would have saved the most calls (about eight).

Wanted: a flag on `know search` and `search` (for example `--excerpt`) that prints, under each
hit, the lines of its body that hold the matched words, a few lines of context around them, cut to
a fixed size per hit. `--json` carries the excerpt as a field. Without the flag the output stays
as it is today.

## Acceptance

- `capcom know search <word> --excerpt` prints each hit's id and title followed by the body lines
  that match, so the answer to "which Note says X" is readable without a `show`.
- The same flag works on `capcom search` for Issues.
- Each excerpt is bounded in size; the limit is stated in `--help`.
- `--json` includes the excerpt; tests cover text and `--json` for Issues and Notes.

## Log

- 2026-10-26 Airic Lenz: Partly landed: --excerpt on search and know search (text and --json, bounded, limit in --help) via field-eval-ergonomics plan (archived 39bac45), item 4. Left open: index.Excerpt does not fold diacritics like FTS5 unicode61, so a hit on 'cafe' for 'café' gets an empty excerpt.
