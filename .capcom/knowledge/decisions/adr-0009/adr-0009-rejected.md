---
title: "ADR 0009 rejected options: strict parsing alone, ignored unknown keys, a version per file, format 1 from init and a failing prime"
labels: [background]
---

- **No format version; rely on strict parsing.** An older binary reports every file that uses a
  new key as bad, one by one, and drops those issues from its results instead of saying once that
  it is too old. A rewrite by that binary can lose them.
- **Ignore unknown keys.** Old binaries would keep running, but they would rewrite files without
  the keys they skipped, and a mistyped key in a hand-edited file would be silently ignored.
- **A version per file.** Every issue and Note would carry its own marker, so one store could mix
  formats and a binary would only find out file by file. The format is a property of the store.
- **Writing `format: 1` from `init` now.** Every v0.1.0 binary would reject the new config as an
  unknown key, failing on a store it can read perfectly well.
- **Failing `capcom prime` too.** `prime` runs from a session-start hook; a non-zero exit there
  costs the agent its Briefing and can block the session, while a warning line carries the same
  instruction.
