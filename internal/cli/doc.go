// Package cli implements capcom's command line: the cobra command tree, the output contract
// (plain text, the versioned --json envelope, --kv KEY=VALUE lines) and the mapping from typed
// errors to exit codes. Run is the single entry point, used by the binary and by tests alike.
//
// # Files
//
// Every non-test file of the package, with its responsibility. TestDocFileMap_MatchesDirectory
// fails when a file is missing from this map or the map names a file that does not exist.
//
//   - `agents.go` — capcom agents: writes, refreshes or checks the capcom block of AGENTS.md
//   - `check.go` — capcom check: validates every config, issue and Note file of the store
//   - `close.go` — capcom close and capcom reopen, each recording why
//   - `create.go` — capcom create: writes a new issue file under the caller's id
//   - `dep.go` — capcom dep add, rm, list and graph: the dependencies between issues
//   - `doc.go` — the package comment and this file map
//   - `errors.go` — the typed Error, its Codes and their exit codes
//   - `flags.go` — the flag names two or more commands share, and unlimitedDepth
//   - `fmt.go` — capcom fmt: rewrites every issue and Note file in canonical form
//   - `init.go` — capcom init: creates the store and installs the Claude Code hooks
//   - `issue_render.go` — the text, --json and --kv forms of an issue shared by list, show and search
//   - `know.go` — capcom know and the Note data, resolution and grouping its subcommands share
//   - `know_add.go` — capcom know add: writes a new Note, and the Note write result and AGENTS.md refresh
//   - `know_mv.go` — capcom know mv: renames a Note or moves it below another
//   - `know_rm.go` — capcom know rm: removes a Note that has no children
//   - `know_search.go` — capcom know search: finds Notes whose title or body contains every word
//   - `know_show.go` — capcom know show: prints a Note in full, optionally with its children
//   - `know_tree.go` — capcom know tree: the outline of Note ids and titles
//   - `know_update.go` — capcom know update: changes a Note through the node store
//   - `know_verify.go` — capcom know verify: lists the Note refs that no longer resolve
//   - `list.go` — capcom list: lists issues by filter
//   - `lock.go` — capcom lock acquire, touch, release, break, check and status: the Tree lock
//   - `log.go` — capcom log: appends an entry to an issue's Log
//   - `merge_driver.go` — capcom merge-driver: the git merge driver for issue files, Log by entry
//   - `mv.go` — capcom mv: renames an issue or moves it below another
//   - `outline.go` — the indented outline rows and their text, with the hint where --depth cut
//   - `output.go` — the output contract: Result, the --json Envelope, --kv records and Renderers
//   - `prime.go` — capcom prime: the session Briefing
//   - `ready.go` — capcom ready: the issues ready to work on, most urgent first
//   - `reindex.go` — capcom reindex: rebuilds the index from the issue and Note files
//   - `related.go` — capcom related add and rm: the related links between issues, raising the store format
//   - `rm.go` — capcom rm: removes an issue that has no children
//   - `root.go` — Run, the root command, capcom version and the index lifecycle of every command
//   - `search.go` — capcom search: finds issues whose title or body contains every word
//   - `show.go` — capcom show and the resolution of an issue argument to its index entry
//   - `update.go` — capcom update and editIssue, which every issue edit verb goes through to the node store
package cli
