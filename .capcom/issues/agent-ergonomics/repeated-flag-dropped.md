---
title: A repeated single-value flag silently keeps only its last value
status: open
type: bug
priority: 1
plan: 2026-10-19 - 00 - repeated-flag-dropped-plan
created: 2026-10-19T15:08:40Z
---

Giving a single-value flag twice keeps only the last value, drops the others without a word and
exits 0. Seen with capcom built from `main` after v0.9.0:

```
$ capcom update x-one --body-replace-old alpha --body-replace-new A \
    --body-replace-old gamma --body-replace-new G       # body "alpha beta gamma"
exit 0, body now "alpha beta G"                          # the alpha edit is lost
$ capcom update x-one --body-append one --body-append two
exit 0, only "two" appended
$ capcom update x-one --title T1 --title T2
exit 0, title "T2"
```

`capcom know update` behaves the same way: with two replace pairs it reports only on the last.
The flags are plain string flags (`internal/cli/update.go`, the Note verbs in
`internal/cli/node_edit.go`), and a repeated string flag overwrites the earlier value.

## Why it matters

The command reports success, so neither the agent nor the human knows an edit was lost.
Agents batch several edits into one call to save turns, so they hit it most. It was found
while editing two Issue bodies in this repository: one of two replacements was dropped and
only a later read of the file showed it.

## Wanted

A repeated flag is never silently dropped. Body edits (`--body-append`, `--body-replace-old`
with `--body-replace-new`) may be given several times and are applied in the order given, all
or none. Every other single-value flag given twice is refused before anything is written,
naming the flag.

## Acceptance

- Two `--body-replace-old`/`--body-replace-new` pairs on `update` and `know update` apply both
  edits in order; if either old text does not occur exactly once, nothing is written and the
  error names that pair.
- An unequal number of `--body-replace-old` and `--body-replace-new` is refused.
- Two `--body-append` values are both appended, in order.
- `--title`, `--status`, `--type`, `--priority`, `--plan` and every other single-value flag of a
  write command, given twice, are refused before writing, naming the flag.
- Help text and examples of `update` and `know update` show a repeated body edit.
- CLI tests cover each case for Issues and Notes.
