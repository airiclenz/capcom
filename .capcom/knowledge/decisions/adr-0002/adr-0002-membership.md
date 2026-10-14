---
title: "ADR 0002 membership rules of 2026-10-14: one owner decides which files are nodes and which names resolve"
---

Ratified 2026-10-14. Which files are nodes of a tree, and which names an argument may resolve to,
is decided in one place: `tree.DecideMembership` gives the verdicts, the index stores them on each
refresh, and `tree.ResolveIn` resolves an argument over either tree's maps or the index's stored
verdicts, so `show`, `close` and the other index readers answer every name as `rm` and `mv` do.

- **Repeated id.** Every file of a repeated id is left out, with every node file below the
  directory beside it; each claimant gets one finding, saying both are left out. Their names are
  refused as duplicates naming both files.
- **Alias clash.** Two nodes that list the same alias both stay usable; only the shared alias is
  refused, naming both files.
- **Unusable claims.** A name only an unparsed, invalid, orphan or left-out file claims is refused
  by every verb and never matched as a prefix of another id. The one exception: `rm` and
  `know rm` by the exact id of an unparsed live file still remove it (the tree's verbs resolve that
  id; the index never does, since it holds no usable node for it).
- **Prefix candidates.** Every file's id counts as a candidate, unusable ones included, so a prefix
  two such ids share is refused as ambiguous, naming the candidates.
- **Resolution order.** A name that is one node's exact id and another file's alias or unusable
  claim is refused as a duplicate naming both files, before any exact match; a unique prefix of a
  usable node's id resolves to it even when another file also claims that id as an alias.
