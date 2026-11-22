---
title: The AGENTS.md capcom block never names search, so agents grep .capcom instead
status: open
type: bug
priority: 1
labels: [agent-ergonomics]
plan: 2026-11-22 - 00 - lock-holder-and-agent-guide-plan
created: 2026-11-22T17:47:25Z
---

The managed block (`internal/agents/block.go`) lists next, ready, show, context, start, log,
close, finish, reopen, create, apply, dep, list and know tree/show. It does not mention
`capcom search` or `capcom know search`, and it gives the file layout
(`.capcom/issues/<id>.md`), so agents search the files with shell tools.

Seen in the reviewed sessions: about 1,900 direct reads of `.capcom/` (`ls` 550, `grep` 495,
`cat` 480, `sed -n` 330, `head` 150, `find` 45) against about 780 `show` and `know show`.
`capcom search` ran 5 times and `capcom know search` once. Typical bypasses:
`ls .capcom/issues | grep -i '<word>\|<word>'`, `grep -rli '<two words>' .capcom/issues`,
`ls .capcom/knowledge/glossary/ | grep -i <word>`,
`find .capcom/knowledge/glossary -iname '*<word>*'`. The one agent that used search found
its issue in one call.

Why: direct reads skip aliases, retired marks, readiness and Elsewhere, cost more calls,
and bake the file layout into agent habits.

Wanted: the block names both commands with `--excerpt`, in the same one-line style as its
other entries, for example "`capcom search <word>... --excerpt`: find issues by words in
the title or body" and the `know search` twin. Check whether search also matches words of
the id (the `ls | grep` cases search ids); if it does not, say so in the plan and consider
it there.

## Acceptance

- `capcom agents` writes a block that names `capcom search` and `capcom know search`.
- The block's size stays within whatever budget its tests pin.
- This repository's AGENTS.md is refreshed with the new block.
