---
title: show and know show print everything, so agents cut the output with head and sed
status: open
type: feature
priority: 2
labels: [agent-ergonomics]
created: 2026-11-22T17:47:25Z
---

`capcom show` and `capcom know show` print the fields, the whole body and, for an issue,
the Log. Agents that want part of it pipe through shell tools.

Seen: 92 `capcom show ... | head -N`, 32 `| sed -n A,Bp`, and on the files directly
`sed -n '/## Acceptance/,$p' .capcom/issues/<id>.md`,
`sed -n '/## Log/,$p' .capcom/issues/<id>.md` and
`sed -n 60,70p .capcom/knowledge/<path>.md`.

Why: `head` cuts blindly (it drops warnings, which print last, and the Acceptance section,
which sits at the end of the body), and the wish to read a part pushes agents to the files.

Wanted, on both commands:
- `--brief`: the fields only, no body and no Log.
- `--section <heading>`: the fields and the one body section whose heading matches
  (case-insensitive, without the `#` marks), for example `--section acceptance` or
  `--section log`; repeatable. A heading that does not exist is an error naming the
  headings the body has.

## Acceptance

- `capcom show <id> --brief` prints no body line.
- `capcom show <id> --section acceptance` prints the fields and that section only.
- Both work on `capcom know show`, in text and `--json`, and `--help` shows them.
