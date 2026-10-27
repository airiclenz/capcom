---
title: --plan stores any string and prime counts an unresolvable plan as planned
status: done
type: bug
priority: 3
plan: 2026-10-27 - 00 - audit-p3-fixes-plan
created: 2026-10-26T20:06:45Z
closed: 2026-10-27T08:53:53Z
reason: Resolved by plan '2026-10-27 - 00 - audit-p3-fixes-plan' (items 4-5), archived in 0e1c89b
---

`--plan` accepts any string, including a path traversal, an absolute path or a name with no file
behind it, and stores it verbatim. `prime` then counts the issue as planned although nothing can
resolve the value.

Reproduced:

    capcom update a --plan ../../etc/passwd  -> exit 0, plan: ../../etc/passwd
    capcom update a --plan /etc/passwd       -> exit 0, plan: /etc/passwd
    capcom update a --plan no-such-plan      -> exit 0, plan: no-such-plan
    capcom check  -> .capcom/issues/a.md:6: plan "/etc/passwd" is in none of the plans directories  exit 1
    capcom prime  -> ready, planned: 1

The writer accepts what the reader refuses: `config.PlanPath` (`internal/config/config.go:261`)
rejects any value that is not a bare basename, so `context` can never resolve these, but `update`
and `create --plan` store any non-empty string (`internal/cli/update.go:121`), and `prime` counts
any non-empty plan as planned (`internal/briefing/briefing.go:154`).

Nothing is exploitable for reads, since the resolver refuses it. The damage is a store that fails
`check` and a `prime` that miscounts. AGENTS.md links an issue to a plan after the plan is saved,
so a plan with no file is a mistake, not a step in the workflow.

## Acceptance

- `update --plan` and `create --plan` refuse a value that is not a bare basename (absolute, or
  containing a separator or `..`), with a message naming the plans directories.
- A basename with no plan file is accepted with a warning, as an unresolved `--ref` is.
- `prime` counts an issue as planned only when its plan resolves to a file.
- A test asserts the refusal, the warning, and that `prime` does not count an unresolvable plan.
