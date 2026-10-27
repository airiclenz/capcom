---
title: A write drops YAML comments from frontmatter without a warning
status: done
type: bug
priority: 3
plan: 2026-10-27 - 00 - audit-p3-fixes-plan
created: 2026-10-26T20:06:45Z
closed: 2026-10-27T08:53:53Z
reason: Resolved by plan '2026-10-27 - 00 - audit-p3-fixes-plan' (items 8-9), archived in 0e1c89b
---

Any write silently deletes hand-written YAML comments from an issue's frontmatter.

Reproduced: an issue carrying `# why: customer outage` above `status:`, then:

    capcom close a --reason fixed   -> a closed, exit 0
    grep '#' .capcom/issues/a.md    -> 0 comment lines left

`log`, `update`, `dep add` and `related add` do the same. The canonical encoder
(`internal/frontmatter/encode.go`) writes only the keys it knows, so a comment does not survive a
rewrite.

This is documented, not a surprise to the design: the file format reference
(`docs/site/reference/file-format.md:298-299`) says comments are read and written back in canonical
form, and `fmt --check` already lists such a file. Keeping comments would fight the canonical form,
so the gap is the silence: a write that drops a comment says nothing, and an agent or human who
annotated why an issue is urgent loses the note on the next status change.

## Acceptance

- A write that drops a frontmatter comment adds a warning naming the file.
- `capcom update --help` says comments in frontmatter are not kept.
- A test writes an issue with a comment, closes it, and asserts the warning.
