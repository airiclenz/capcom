---
title: show --kv leaves out labels, type, priority and deps
status: done
type: feature
priority: 2
created: 2026-10-26T07:51:30Z
closed: 2026-10-26T09:47:07Z
reason: "Resolved by field-eval-ergonomics plan (archived 39bac45), item 8: show --kv prints TYPE, PRIORITY, LABELS, PARENT, DEPS and PLAN, keys documented in json-output.md"
---

`capcom show <id> --kv` prints `ID`, `ETAG`, `ISSUE_STATUS`, `CATEGORY`, `TITLE` and `STATUS`
only. Labels, type, priority, parent and dependencies are missing, and `create` prints just id,
path and etag, so checking that a write landed takes a full `show` and reading the body around the
fields.

Why: in the field evaluation (capcom 0.9.1) two experiments paid for this. `eval-unfamiliar-commands`
changed labels, type and priority and needed full `show` calls to read them back;
`eval-linked-issues` filed three linked issues and needed one `show` per issue plus a `dep list`
to confirm labels and deps. With the fields in `--kv`, each check is one line per field and can be
scripted (about three calls saved across the two).

Wanted: `show --kv` adds one line per field: `TYPE=`, `PRIORITY=`, `LABELS=` (comma-separated),
`PARENT=`, `DEPS=` (comma-separated), and `PLAN=` when set. Empty fields print with an empty value,
so a consumer can rely on every key being present.

## Acceptance

- After `capcom update <id> --add-label alpha --type bug --priority 3`, `capcom show <id> --kv`
  prints `TYPE=bug`, `PRIORITY=3` and `LABELS=` holding `alpha`.
- An issue with deps and a parent prints them as `DEPS=` and `PARENT=`.
- `docs/format.md` lists the keys; a CLI test pins them.
