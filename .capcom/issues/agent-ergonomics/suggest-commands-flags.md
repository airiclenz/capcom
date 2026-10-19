---
title: Mistyped commands and flags get no suggestion
status: open
type: feature
priority: 1
plan: 2026-10-19 - 01 - suggest-next-plan
created: 2026-10-19T14:54:45Z
---

A mistyped command or flag fails with no hint: `capcom shwo x` prints
`unknown command "shwo" for "capcom"` and `capcom update x --statsu active` prints
`unknown flag: --statsu`. Suggestions are switched off with `DisableSuggestions: true` in
`internal/cli/root.go`. Small models then guess again, often wrongly, and burn turns.

Wanted: an unknown command or subcommand names the closest valid one, and an unknown flag names
the closest flag of that command, as `did you mean ...?` on the same error. Nothing is suggested
when no candidate is close. The `--json` error envelope carries the suggestion in its own field.

## Acceptance

- `capcom shwo x` suggests `show`; `capcom know shwo x` suggests `know show`.
- `capcom update x --statsu active` suggests `--status`.
- A string with no close candidate gets no suggestion.
- CLI tests cover each case in text and `--json` output.
