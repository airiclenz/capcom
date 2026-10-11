---
title: know update --add-label writes an empty label that breaks the Note
status: open
type: bug
priority: 2
created: 2026-10-11T18:36:10Z
---


`capcom know update --add-label "x,,y"` writes `labels: [x, "", y]` into the Note.
`knowledge.Validate` (`internal/knowledge/note.go:129`) accepts the empty label, but the parser
rejects it (`internal/knowledge/note.go:337`: "every entry must be non-empty text"), so the Note
stops resolving (`id not found`) and `capcom check` fails. The labels are merged by `edit.Labels`
(`internal/edit/edit.go:21`), called from `internal/cli/know_update.go:160`, which keeps empty
entries; `capcom know add` drops them (`internal/cli/know.go:391`). Issue `capcom update` already
had this bug before, through `edit.Labels`.

Found 2026-10-11, implementing plan `2026-10-11 - 03 - know-edit-verbs-plan`, item 3
(capcom know update).

## Acceptance

- `capcom know update --add-label "x,,y"` writes `labels: [x, y]`, or refuses the empty label;
  the Note still resolves and `capcom check` stays green.
