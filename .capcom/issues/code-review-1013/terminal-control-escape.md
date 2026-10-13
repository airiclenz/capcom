---
title: Text output passes terminal control sequences from repository content
status: done
type: bug
priority: 2
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:14Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

Text output passes terminal control sequences from repository content. A YAML double-quoted
`"\e[..."` or `"\x1b..."` parses to a raw ESC, and the text output of `show`, `list`,
`know tree`, `know search`, `know show` and `dep list` writes titles, field values and bodies
to the terminal unfiltered; the only filter (`flatten`) handles line breaks. Severity: Medium
(security).

Trigger: a hostile repository or branch commits an Issue titled `"\e[2J\e[31mx"`, or a body
carrying OSC sequences. A human runs `capcom show`, and the screen is cleared or rewritten to
hide text, the window title is set, or, on some terminals, the clipboard is written.

Wanted (decided 2026-10-13): in text mode, no C0 control other than TAB and LF, no DEL and no
C1 control from repository content reaches stdout; each is shown as a visible escape in the
`\x1b` form. `--json` and `--kv` are byte-for-byte unchanged. One sanitising function in the
text renderer, used by `flatten` and by every body write.

## Acceptance

- A CLI test creates an Issue and a Note whose title and body contain `\x1b[2J`, `\x9b` and
  `\x07`, and asserts that text output carries the visible escapes while `--json` round-trips
  the raw content.
- `GOMEMLIMIT=2GiB go test ./internal/cli/ -run 'Show|List|Know'` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-07.
