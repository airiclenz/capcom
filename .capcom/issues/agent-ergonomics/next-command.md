---
title: No single command shows the issue to work on next
status: open
type: feature
priority: 1
created: 2026-10-19T14:54:44Z
---

An agent starting a session runs `capcom ready`, picks the first line, then runs
`capcom show <id>`. Small models often stop after `ready`, or show the wrong issue. The common
case, "what do I work on now", should be one command.

Wanted: `capcom next` prints the top Ready Issue in full, exactly as `capcom show` would, using
the same order as `capcom ready` (priority, then how many issues it unblocks). It accepts the
same filters as `ready` (labels, `ready.hide`), supports `--json`, and when nothing is Ready it
says so and exits with a distinct non-zero code documented in `--help`, so a script can tell
"nothing to do" from an error.

Look at `internal/cli/ready.go` and `internal/cli/show.go`; reuse their ordering and rendering
rather than copying them.

## Acceptance

- `capcom next` prints the same Issue as the first line of `capcom ready`, in `show` format.
- With nothing Ready it prints one plain sentence and exits with the documented code.
- `--json` returns the `show` envelope for that Issue.
- The Managed block written by `capcom agents` mentions `capcom next`.
- CLI tests cover: one ready, several ready with a priority tie, none ready, a label filter.
