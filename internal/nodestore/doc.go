// Package nodestore is the node store of capcom's two Tree kinds, the issue tree and the Knowledge
// tree: a Kind binds the kind's Codec to its directory in the store, so the verbs of both kinds
// read, change and write node files through one module, report one set of typed failures and look
// up Aliases the same way. It reads the files itself and never consults the index: where a verb
// needs the index's resolution of an argument, the caller injects it as a Resolver.
//
// # Files
//
// Every non-test file of the package, with its responsibility. TestDocFileMap_MatchesDirectory
// fails when a file is missing from this map or the map names a file that does not exist.
//
//   - `claim.go` — the refusals of a new id that a node, or a file the tree leaves out, holds
//   - `codec.go` — Codec: how one Tree kind parses, validates, serializes and plans moves of its nodes
//   - `create.go` — Create: writes a new node below its parent
//   - `doc.go` — the package comment and this file map
//   - `edit.go` — Edit: changes a node and writes it back under its ETag
//   - `errors.go` — FileError, ConflictError and ExistsError: the typed failures every verb reports
//   - `format.go` — Format: rewrites every node file in canonical form
//   - `kind.go` — Kind, Issues and Notes, Written, and the Alias reads and paths every verb shares
//   - `load.go` — the tree load and the triage of its problems into blocking failures and warnings
//   - `move.go` — Move: renames a node or moves it below another
//   - `remove.go` — Remove: removes a node that has no children
//   - `write.go` — the validated write of a node file, replacing it or creating it exclusively
package nodestore
