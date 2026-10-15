---
title: "ADR 0013 rejected options: blocking writes, ignoring the lock, and acquiring it while the UI is open"
labels: [background]
---

- **Block writes while the lock is held.** Safe, but a supervisor often needs to act exactly then
  (close the issue the agent finished, add a note), and `$EDITOR` would bypass the block anyway.
- **Show the lock, never check it.** Matches the CLI, but a click in a TUI is far easier to make
  by accident than a typed command.
- **Acquire the lock while the UI is open.** Shuts agents out for as long as a window stays open,
  for writes that do not need it.
