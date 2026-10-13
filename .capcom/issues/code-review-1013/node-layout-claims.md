---
title: tree owns the node-file layout and Tree.Claimant
status: open
type: task
priority: 3
deps: [nodefile-package, duplicate-id-subtree, split-nodestore-create, replace-missing-target, rename-no-replace]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:43Z
---

The node-file layout and the id-claim concept have no owner:

- Six packages re-derive the `.md` extension, "id = base name" and "children directory = path
  minus `.md`".
- `nodestore` walks orphan directories again with its own copy of `tree.Walk`'s skip rules
  (hidden prefix, extension, irregular files).
- The ordering constraints between claim checks are scattered across the verbs ("each later
  refusal relies on the earlier ones" in Create; the orphan check before triage and the unparsed
  check after it in Move).
- The Alias resolution rule is implemented three times (`tree.Resolve`, `index.resolveID` in SQL,
  `issue`'s `readyTree.resolve`), and the three treat a doubly-claimed Alias slightly differently.

Strength: Strong for the layout, worth exploring for the claims.

Wanted: `tree` exports `IDOf(path)` and `ChildrenDir(path)`, and `Load` takes a second,
recovering `AliasFunc`. `Tree.Claimant(name)` returns any holder of a name: a live Node, a dropped
duplicate, a file in an orphan directory or an unparseable file; `nodestore` claims names through
it, so Create and Move shrink to load, resolve, claim, triage, write. The file-IO half is
`nodefile-package`. `Claimant` covers the dropped duplicates of `duplicate-id-subtree`, and the
refuse-missing replace and no-replace rename of `replace-missing-target` and `rename-no-replace`
live in `nodefile`. ADR 0001 (the directory is the hierarchy) is unchanged, only centralised.

## Acceptance

- `cli`, `index`, `knowledge` and `nodestore` derive ids and children directories through
  `tree.IDOf` and `tree.ChildrenDir`; `nodestore` has no directory walk of its own.
- A `tree` test asserts `Claimant` for each holder kind: live Node, dropped duplicate, orphan file
  and unparseable file.
- Create and Move refusal wording and codes are unchanged.
- `GOMEMLIMIT=2GiB go test ./internal/tree/`, then `./internal/nodestore/`, `./internal/index/`,
  `./internal/knowledge/` and `./internal/cli/`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-05.
