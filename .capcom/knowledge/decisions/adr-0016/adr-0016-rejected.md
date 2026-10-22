---
title: "ADR 0016 rejected options: a --token-file flag, a token from a harness session id, a printed resume line alone"
labels: [background]
---

- **A `--token-file <path>` flag.** capcom would write and read the token at a path the agent
  names, but the agent must then carry the path across turns and compactions instead of the token,
  and remember to pass the flag on every call: the same recall the Issue is about, moved one step.
  A path fixed in AGENTS.md so the agent need not remember it is the token file of the decision with
  a flag to forget added.
- **Deriving the token from a harness session id.** Where a harness exposes a session id in the
  environment, capcom could key or derive the token from it, and a compaction would keep it. Most
  harnesses expose none, so a second mechanism is needed anyway; the variable's name differs per
  harness, so capcom would grow a list of them; and a sub-agent may run under its own id, so it
  would lose the session's lock, which breaks the sub-agent constraint.
- **A printed resume line alone.** `lock acquire` could print a one-line resume command for the
  agent's own memory to keep. It costs nothing and capcom already prints the token, but it rests
  on the agent's memory, which is exactly what a compaction or a small model loses; it helps a
  session that keeps it and does nothing for one that does not.
