---
title: Any write silently deletes YAML comments from frontmatter
status: open
type: bug
priority: 2
created: 2026-10-26T20:06:45Z
---

Any write silently deletes hand-written YAML comments from an issue's frontmatter.

Reproduced: an issue carrying `# why: customer outage` above `status:`, then:

    capcom close a --reason fixed   -> a closed, exit 0
    grep '#' .capcom/issues/a.md    -> 0 comment lines left

`fmt` does the same, and it lists the file it rewrote, but `close`, `update`, `log`, `dep add`
and `related add` say nothing at all. The canonical form is lossy, so the presence of a comment
makes a file "non-canonical" and every write path rewrites it from the parsed node.

The file format reference admits comments are read and rewritten in canonical form, but no
mutating command's help or output says a write will eat them, and there is no preview.

This matters for the "notes to myself" habit the format invites: an agent or human annotating why
an issue is urgent loses the note on the next status change, without a warning.

## Acceptance

- Either comments survive a write, or a write that would drop one warns and names the file.
- The lossy behaviour, if kept, is stated in `capcom update --help` and in the file-format
  reference, and `fmt --check` reports a file whose comments a rewrite would drop.
- A test writes an issue with a comment, closes it, and asserts the documented behaviour.
