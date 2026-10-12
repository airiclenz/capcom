---
title: show --kv prints warnings before the result
status: done
type: bug
priority: 3
created: 2026-10-12T05:39:56Z
closed: 2026-10-12T06:30:58Z
reason: Resolved by plan 2026-10-12 - 00 - open-issues-plan (item 5, archived in 70b91be)
---

`docs/format.md` says a command reports a bad file "as a warning, after its result". Plain-text
`create` does (the `warning:` line follows `id:`/`path:`/`etag:`), but `capcom show <id> --kv`
prints `WARNING=...` before the issue's `ID=` record:

```
WARNING=.capcom/issues/renamed-issue.md:3: invalid YAML frontmatter: ...
ID=other-one
ETAG=...
```

A `--kv` consumer that reads records from `ID=` to `STATUS=` is fine either way, but the order
contradicts the documented contract. Found in the apogee beads-migration trial (2026-10-12).

## Acceptance

- Either every `--kv` command prints `WARNING=` lines after its records (before nothing but the
  final `STATUS=` if that is the contract), or `docs/format.md` states where `--kv` puts them.
- A CLI test pins the chosen order for `show --kv` and one write verb.
