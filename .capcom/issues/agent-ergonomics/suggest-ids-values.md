---
title: Unknown ids and bad flag values get no suggestion
status: open
type: feature
priority: 2
created: 2026-10-19T14:54:45Z
---

A mistyped Issue or Note id fails with `id "x" not found` and nothing else, and a wrong value
for an enumerated flag gives no list of valid values. Small models then loop on guesses.

Wanted:
- An id that resolves to nothing names up to three closest ids or aliases (edit distance and
  shared words), as `did you mean ...?`.
- A `--status`, `--type` or `--priority` value that is not valid lists the valid ones from
  `.capcom/config.yaml` (statuses with their category).
- The `--json` error envelope carries the suggestions as a list field.

## Acceptance

- `capcom show standng-parents` suggests `standing-parents`.
- `capcom update <id> --status doing` lists the configured statuses with categories.
- `capcom update <id> --type bugg` lists the configured types.
- No suggestion is printed when nothing is close.
- Tests cover issues and Notes, text and `--json`.

## Log

- 2026-10-26 Airic Lenz: Field evaluation (capcom 0.9.1): two of nine experiments hit this. eval-full-cycle ran update --status doing and eval-help-discovery ran list --status closed (a category word); both errors say only 'not a status declared in config.yaml', so the agent read config.yaml or retried blind. Still the same on 0.10.0. Listing the declared statuses in the error saves about one call each.
