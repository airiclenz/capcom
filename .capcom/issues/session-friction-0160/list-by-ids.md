---
title: No one-call way to get the status of several issues or learn whether an id exists
status: open
type: feature
priority: 2
labels: [agent-ergonomics]
created: 2026-11-22T17:47:25Z
---

To learn the status of a set of issues, or whether a slug is free before `create`, agents
loop over `capcom show`.

Seen: 65 shell loops such as
`for i in a b c; do printf '%s: ' $i; capcom show $i 2>/dev/null | grep -m1 '^status:'; done`
and `capcom show $s >/dev/null 2>&1 && echo "EXISTS $s"`, plus about 25 single probes
`capcom show <new-id> 2>&1 | head -2` before a `create`. The probe exits 3 (not found),
which is the wanted answer, yet apogee's fail-fast shell stops the rest of the line on it,
so the `create` chained after it never ran and needed a second call.

Why: each loop prints whole issues to read one field, and the existence probe turns the
good case into an error.

Wanted: `capcom list <id|alias|prefix>...` prints the usual one line per issue (id, status,
priority, title) for exactly the ids named, closed ones included, and one
`<id>  not found` line for each id that names no issue. Decide in the plan whether a
missing id changes the exit code; an existence check should be answerable without a
failure. `--json` carries the same rows.

## Acceptance

- One call answers "what is the status of these N issues", closed ones included.
- One call answers "is this id free" without printing an issue body.
- `--help` shows both uses as examples; text and `--json` are tested.
