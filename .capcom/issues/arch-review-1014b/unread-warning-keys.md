---
title: Unread warning names deps only, and messages say note
status: open
type: bug
priority: 2
deps: [kind-reader]
plan: 2026-10-14 - 02 - architecture-review-2-plan
created: 2026-10-14T16:53:33Z
---

When `capcom mv` or `rm` finds a bystander issue file that does not parse, it warns
"deps not checked", because cli `issueVerbs` sets `linkKey: "deps"`
(`internal/cli/node_verbs.go:69`), but `Moved.Unread`/`Removed.Unread` cover deps and related
alike (`internal/nodestore/codec.go`). The warning understates what went unchecked, and
`TestMv_UnparseableBystander_WarnsDepsNotChecked` pins it. The noun also differs by package: cli
says "a Note", the codec and the frontmatter unknown-key hint say "a note", while the glossary
capitalises Note.

Wanted (ratified 2026-10-14): the unread warning names every ref key the kind's descriptor
declares ("deps and related not checked" for issues), generated from the descriptor. Every
message says "Note"/"a Note"; "issue" stays lowercase.

## Acceptance

- `mv`/`rm` on an issue with an unparseable bystander warn that deps and related were not
  checked; the pinning test is renamed and repinned.
- No message in the binary says "a note" or "note" for the node kind; tests pinning the
  lowercase form are repinned.
- The CHANGELOG `Unreleased` section has a line for each wording change.

Source: docs/reviews/architecture-review-2026-10-14.html, 3 · One node reader and descriptor per kind.
