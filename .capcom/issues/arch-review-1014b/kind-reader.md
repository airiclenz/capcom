---
title: One node reader and descriptor per kind
status: done
type: task
priority: 2
plan: 2026-10-14 - 02 - architecture-review-2-plan
created: 2026-10-14T16:53:33Z
closed: 2026-10-14T19:03:49Z
reason: resolved by plan 2026-10-14 - 02 - architecture-review-2-plan (7123d17f)
---

"Read one node file" is assembled six ways, each with its own usability verdict: cli `show.go`,
`know_show.go`, `know_verify.go`, `close.go`, `update.go` (`readIssueFile`,
`frontmatterFailure`), index `parseIssue`/`parseNote`/`aliasParser`, check `readAliases`,
diagnose `IssueFile`/`NoteFile`, agents `topNotes` (parses, never validates) and the nodestore
codec. What a kind is (its noun, its ref keys, the format each key needs) is spelled separately in
the codec, in cli `nodeVerbs` (`nounPhrase`, `linkKey`) and in check. Three defects already ship
from that drift (`block-left-out-notes`, `format-key-rule`, `unread-warning-keys`), and review
candidates 1 and 9 need one reader to plug into.

Wanted: nodestore exports, per kind, a descriptor (noun, ref fields, the format each key needs)
and a Kind reader: file → node, etag, aliases, findings, and one usable verdict. Every reader
listed above goes through it. This carries items 4 and 5 of the earlier review
(`docs/reviews/archived/architecture-review-2026-10-14-0848.html`). A glossary Note "Kind
reader" under `glossary-structure` names it.

## Acceptance

- nodestore exports one descriptor and one Kind reader per kind; a table test per kind covers
  parse failure, validation findings, refused files (not regular, too large) and a clean file.
- cli, index, check, diagnose and agents read node files only through the Kind reader;
  `aliasParser`, check `readAliases` and the `tree.Load` alias funcs that re-implement a reader
  are gone.
- cli `nodeVerbs` takes its noun and ref keys from the descriptor, not its own fields.
- The glossary Note "Kind reader" exists and `capcom check` is clean.
- `go vet ./...` is clean and the touched packages' tests pass.

Source: docs/reviews/architecture-review-2026-10-14.html, 3 · One node reader and descriptor per kind.
