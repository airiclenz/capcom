---
title: A guessed command name gets an unrelated suggestion
status: done
type: feature
priority: 3
plan: 2026-11-02 - 00 - lock-hint-and-synonyms-plan
created: 2026-11-02T20:58:47Z
closed: 2026-11-02T21:39:22Z
reason: "Resolved by plan 2026-11-02 - 00 - lock-hint-and-synonyms-plan (item 2), archived in ee7aeb6: guessed synonyms suggest the meant command"
---

`capcom get foo` fails with `unknown command "get" for "capcom"; did you mean dep?`. The suggestion
comes from edit distance only, so a word an agent guesses from other tools gets a command that
shares letters, not the one it meant. capcom sets no cobra `SuggestFor` synonyms
(`unknownCommandError` in `internal/cli/root.go`).

## Why

Agents guess command names from other CLIs. Seqcom's apogee eval (Note `apogee-eval-report`,
experiment run-stats) guessed `capcom get` and lost a call; a `show` suggestion would have
recovered it in the same call. Seen on capcom 0.14.0.

## Acceptance

- `capcom get`, `capcom view`, `capcom read` and `capcom cat` suggest `show`; `capcom ls` suggests `list`; `capcom new` and `capcom add` suggest `create`; `capcom done` suggests `close`.
- An unknown word with no synonym still gets the edit-distance suggestion it gets today.
- The synonyms never run the command: they only feed the suggestion, exit code 2 as now.
