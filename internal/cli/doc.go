// Package cli implements capcom's command line: the cobra command tree and the commands that
// hand their results to internal/cli/output, which owns the output contract (plain text, the
// versioned --json envelope, --kv KEY=VALUE lines) and decides the exit code. Run is the single
// entry point, used by the binary and by tests alike: it only wires cobra and passes the
// command's final error to output.Exit.
//
// # Files
//
// Every non-test file of the package, with its responsibility. TestDocFileMap_MatchesDirectory
// fails when a file is missing from this map or the map names a file that does not exist.
//
//   - `agents.go` — capcom agents: writes, refreshes or checks the capcom block of AGENTS.md
//   - `check.go` — capcom check: validates every config, issue and Note file of the store
//   - `close.go` — capcom close: closes issues, recording why
//   - `context.go` — capcom context: an issue with its deps and dependents, plan and the Notes that apply
//   - `create.go` — capcom create: writes a new issue file under the caller's id
//   - `dep.go` — capcom dep add and rm: adds and removes the dependencies between issues through internal/ops
//   - `dep_graph.go` — capcom dep graph and the Dependency graph that dep list and related load from the index
//   - `dep_list.go` — capcom dep list: an issue's deps and dependents as indented trees
//   - `doc.go` — the package comment and this file map
//   - `errors.go` — kindFailure, which types a nodestore failure, and the tree-problem warnings
//   - `finish.go` — capcom finish: closes an Issue, frees this worktree's Tree lock and names the next Ready Issue
//   - `flag_once.go` — refuses a single-value flag given more than once
//   - `flags.go` — the flag names two or more commands share, unlimitedDepth, and valueChecker, which lists the valid values
//   - `fmt.go` — capcom fmt: rewrites every issue and Note file in canonical form
//   - `init.go` — capcom init: creates the store and installs the Claude Code hooks
//   - `issue_render.go` — the text, --json and --kv forms of an issue shared by list, show and search
//   - `know.go` — capcom know and the Note data and resolution its subcommands share
//   - `know_add.go` — capcom know add: writes a new Note, and the Note write result and AGENTS.md refresh
//   - `know_search.go` — capcom know search: finds Notes whose title or body contains every word, else some
//   - `know_show.go` — capcom know show: prints a Note in full, optionally with its children
//   - `know_tree.go` — capcom know tree: the outline of Note ids and titles
//   - `know_update.go` — capcom know update: changes a Note through internal/ops and the node store
//   - `know_verify.go` — capcom know verify: lists the Note refs that no longer resolve
//   - `list.go` — capcom list: lists issues by filter
//   - `lock.go` — capcom lock acquire, touch, release, break, check and status: the Tree lock
//   - `log.go` — capcom log: appends an entry to an issue's Log
//   - `merge_driver.go` — capcom merge-driver: the git merge driver for issue files, Log by entry
//   - `mv.go` — the destination, warnings and flags of mv and know mv, which node_verbs.go builds
//   - `next.go` — capcom next: the Issue on the first line of ready, in full as show prints it
//   - `node_edit.go` — editIssue, editNode and written, the CLI side of the internal/ops writes every issue and Note edit verb goes through, and their shared helpers
//   - `node_verbs.go` — nodeVerbs, the descriptor of a Tree kind, the mv and rm commands built from it for both kinds, and the write path of create and know add
//   - `outline.go` — indented outline rows and their text, with the hint where --depth cut; internal/outline computes the rows
//   - `output.go` — the Elsewhere scan of a result and the pathspec of the whole issue tree
//   - `prime.go` — capcom prime: the session Briefing
//   - `ready.go` — capcom ready: the issues ready to work on, most urgent first
//   - `reindex.go` — capcom reindex: rebuilds the index from the issue and Note files
//   - `related.go` — capcom related add and rm: the related links between issues, raising the store format
//   - `reopen.go` — capcom reopen: moves a closed issue back to an open status, logging why
//   - `rm.go` — the results, failure typing and warnings of rm and know rm, which node_verbs.go builds
//   - `root.go` — Run, Command, the root command, capcom version and the index lifecycle of every command
//   - `search.go` — capcom search: finds issues whose title or body contains every word, else some
//   - `show.go` — capcom show and the resolution of an issue argument to its index entry
//   - `start.go` — capcom start: claims a Ready Issue and this worktree's Tree lock, through internal/ops
//   - `suggest.go` — the closest flag named for an unknown flag and the closest ids for an unknown id, beside the command suggestions root.go makes
//   - `ui.go` — capcom ui: checks for a terminal and hands the opened store and index to internal/ui
//   - `update.go` — capcom update: changes an issue's fields or body
package cli
