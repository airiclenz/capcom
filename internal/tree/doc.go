// Package tree loads one store directory (`.capcom/issues/`, `.capcom/knowledge/`) as a Tree of
// nodes and writes node files back atomically. The directory tree is the hierarchy (ADR 0001): a
// node is the markdown file `<id>.md`, and its children live in the sibling directory `<id>/`.
// Ids are chosen kebab slugs, unique across the whole tree together with every node's Aliases
// (ADR 0002). The package is generic over the node kind; reading a node's Aliases out of its file
// is left to the caller through an AliasFunc.
//
// # Files
//
// Every non-test file of the package, with its responsibility. TestDocFileMap_MatchesDirectory
// fails when a file is missing from this map or the map names a file that does not exist.
//
//   - `claims.go` — the facts Load reads for DecideMembership and the problems it renders from the
//     verdict, and Claimant: the file that holds a name, live, orphan, duplicate or unparsed
//   - `doc.go` — the package comment and this file map
//   - `errors.go` — the sentinel errors, Error and Errors, and the problem types they wrap
//   - `ids.go` — ValidateID, ValidateAlias, CheckAliases and RenameAliases: what an id or alias may be
//   - `membership.go` — DecideMembership: the pure membership function over per-file facts
//   - `move.go` — PlanMove: renames a node or moves it below another, with its reference rewrites
//   - `plan.go` — Plan: the steps, rewrites and cleanups of a move or removal, applied with undo
//   - `remove.go` — PlanRemove: removes a node that has no children
//   - `resolve.go` — Lookup and Resolve: an argument to its node by id, alias or id prefix
//   - `tree.go` — Tree, Node, Load and the accessors, and PathFor
//   - `walk.go` — Walk: the one directory walk of ADR 0001 and the entries it leaves out
package tree
