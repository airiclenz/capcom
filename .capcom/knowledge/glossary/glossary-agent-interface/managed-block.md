---
title: "Managed block: the marked section of AGENTS.md that capcom writes and every agent guide reaches"
---

The marked section of a repository's agent guide (AGENTS.md) that capcom writes and keeps current:
the pointer to capcom, the core commands, a recommendation to take the **Tree lock** before an
extensive change, and the top level of the **Knowledge** **Tree**. Text outside it is never
touched. AGENTS.md is its home unless `agents.file` in config.yaml names another file, which
AGENTS.md then reaches by a link or an `@<file>` import line. Every other agent guide that
`agents.guides` lists (CLAUDE.md and GEMINI.md by default) reaches AGENTS.md, by being a link to it
or by holding an `@AGENTS.md` import line, and never holds a copy of the block. `capcom agents` appends that import line
to an existing guide that does not reach the block, and `capcom check` and `capcom agents --check`
report such a guide while AGENTS.md carries the block.
_Avoid_: generated section, injected instructions
