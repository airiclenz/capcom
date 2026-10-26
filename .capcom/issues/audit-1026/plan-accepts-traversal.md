---
title: --plan accepts a traversal or absolute path and prime counts it
status: open
type: bug
priority: 3
created: 2026-10-26T20:06:45Z
---

`--plan` accepts any string, including a path traversal or an absolute path, and stores it
verbatim. `prime` then counts the issue as planned although nothing can resolve the value.

Reproduced:

    capcom update a --plan ../../etc/passwd  -> exit 0, plan: ../../etc/passwd
    capcom update a --plan /etc/passwd       -> exit 0, plan: /etc/passwd
    capcom check  -> .capcom/issues/a.md:6: plan "/etc/passwd" is in none of the plans directories  exit 1
    capcom prime  -> ready, planned: 1

The writer accepts what the reader refuses: `config.PlanPath` rejects any value whose basename
differs from the whole value, so `context` and `prime` can never resolve these, but `update` and
`create --plan` store any non-empty string.

Nothing is exploitable for reads, since the resolver refuses it. The damage is a store that fails
`check` and a `prime` that miscounts. Contrast `--title ""`, which is refused.

## Acceptance

- `update --plan` and `create --plan` refuse a value that is absolute, contains `..`, or is not a
  basename, with a message naming the plans directories.
- A test asserts the refusal and that `prime` no longer counts it as planned.
