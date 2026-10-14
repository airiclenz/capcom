---
title: One owner for Tree membership and resolution
status: open
type: task
priority: 2
deps: [claimed-name-prefix]
plan: 2026-10-14 - 00 - architecture-review-fixes-plan
created: 2026-10-14T08:35:10Z
---

The Tree membership rules (who holds an id or an Alias, repeated-id subtrees, orphan
directories, unparsed files, prefix resolution) are implemented three times: in `tree`
(`Load`, `Claimant`, `Resolve`, `repeatedIDs`, `orphanClaim`, `unparsedClaim`), again in index Go
(`leaveOutRepeatedIDSubtrees`, `orphanDirectoryClaims`) and again in index SQL
(`excludeDuplicates`, `resolveID`, `duplicateClaim`), with a third alias fallback in
`nodestore` (parsed aliases, else `rawfm`). Comments admit the copying ("the way tree.Load
does", "as tree.Resolve counts"), no test checks that the copies agree, and they have drifted:
the index reads an orphan file's Aliases with `rawfm.Aliases` while tree's `orphanClaim` only
parses; an alias clash leaves both nodes out instead of refusing only the shared name. Every
other change that reads a Tree builds on these rules, so they need one owner.

Wanted: one module in `tree` decides membership and resolution; the index persists its verdicts
and answers queries over them. Ratified calls (2026-10-14):

- Depth is full: a pure membership function over per-file facts decides kept nodes, left-out
  files with their reason, and every name's claimants with its refusal reason; `tree.Load` is
  built on it; the index stores its verdicts after each refresh; `tree.Resolve` is the only
  resolution algorithm, written against a small names interface that tree's maps and the index's
  SQL both implement.
- Alias clash: both nodes stay usable; only the shared alias is refused, in tree and index.
- Prefix candidates: every file's id counts, unusable ones included; an ambiguous prefix is
  refused naming the candidates.
- Resolution order: a name that is one node's exact id and another file's alias or unusable
  claim is refused as a duplicate naming both files, by every verb (tree's order).
- A repeated id counts unparsed and invalid files too, warning about and leaving out both.
- One alias reader: parsed YAML when a file parses, `rawfm` only when it does not; one listing
  of the files under orphan directories, used by tree, the index and nodestore.

Index warning wording stays as it is except where a call above changes what is reported.

## Acceptance

- `tree` exports one pure membership function and `tree.Load` uses it; `internal/index` holds no
  repeated-id, orphan-listing or alias-reading logic of its own and `excludeDuplicates` is gone.
- `tree.Resolve` is the only resolution algorithm; a test runs every membership fixture through
  tree and the index and asserts they agree, with no expected-divergence table.
- When two files list the same alias, `capcom show <id>` of either node succeeds and resolving
  the alias fails as a duplicate naming both files.
- A prefix matching the id of any file, unusable ones included, and another id is refused as
  ambiguous naming every candidate.
- `go vet ./...` is clean and the touched packages' tests pass.

Source: docs/reviews/architecture-review-2026-10-14-0848.html, 1 · One owner for Tree membership and resolution.
