---
title: "ADR 0017 rejected options: a --token flag on every write, blocking writes, a stderr-only warning, leaving the CLI silent"
labels: [background]
---

- **A `--token` flag on every mutating command.** Exact: the holder passes its token and is never
  warned. But every `log`, `update` and `create` would need it, which is the token recall that
  ADR 0016 removed, and a write without it would warn the holder anyway.
- **Blocking writes while the lock is held.** Rejected for the UI in ADR 0013 for the same reasons:
  a supervisor often needs to act exactly then, and `$EDITOR` bypasses any block.
- **The warning on stderr only.** It would keep stdout's documented shape, but text-mode warnings
  already go to stdout and `--json` callers read `warnings`; a stderr-only line reaches neither.
- **Leaving the CLI silent.** `capcom lock check` remains the gate an agent runs before a
  multi-file change, but a second agent that never runs it gets no signal at all.
