---
title: Command help buries the examples under a long paragraph
status: open
type: feature
priority: 2
created: 2026-10-19T14:54:45Z
---

Every command's `--help` opens with one long paragraph (see `capcom update --help` or
`capcom create --help`) and the examples come after it. Small models copy examples far more
reliably than they apply rules buried in prose, and often read only the top of the output.

Wanted: help output in this order: the one-line `Short`, the usage line, the examples, the
flags, then the `Long` text split into short paragraphs or bullets, one rule each. The wording
of the rules stays the same.

`internal/cli/help_text_test.go` checks every command has `Long` and `Example`; the CLI
reference generator (`go run ./cmd/gendocs`, `internal/clidoc`) reads the same fields, so the generated reference under
`docs/site/` must be regenerated and stay correct.

## Acceptance

- For every command, the examples appear before the first line of `Long` in `--help` output.
- No `Long` paragraph is longer than four lines at 100 columns.
- A test enforces both rules for every available command.
- The CLI reference is regenerated with `go run ./cmd/gendocs` and `internal/clidoc` tests pass.
