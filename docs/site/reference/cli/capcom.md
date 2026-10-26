---
title: "capcom"
description: "A git-native issue tracker and knowledge store for coding agents"
sidebar:
  order: 0
---

## capcom

A git-native issue tracker and knowledge store for coding agents

### Synopsis

capcom is a git-native issue tracker and knowledge store for coding agents. Issues and Knowledge Notes are markdown files under .capcom/, versioned with the code; a disposable SQLite index lives under the git directory and is never committed. Every command except capcom ui prints plain text, or with --json a versioned JSON envelope. Run capcom <command> --help for a command's flags and examples.

```
capcom [flags]
```

### Examples

```
  capcom init                  # create .capcom/ in this git repository
  capcom prime                 # the session-start briefing
  capcom ready                 # the Issues ready to work on, most urgent first
  capcom know tree --depth 2   # the top two levels of the Knowledge tree
  capcom ready --json          # the same list as a JSON envelope
```

### Options

```
  -h, --help   help for capcom
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom agents](capcom_agents.md)	 - Write or refresh the capcom block of AGENTS.md
* [capcom check](capcom_check.md)	 - Validate every config, issue and Note file of the store
* [capcom close](capcom_close.md)	 - Close issues, recording why
* [capcom create](capcom_create.md)	 - Create an issue with a chosen id
* [capcom dep](capcom_dep.md)	 - Manage dependencies between issues
* [capcom finish](capcom_finish.md)	 - Close an Issue and free this worktree's Tree lock
* [capcom fmt](capcom_fmt.md)	 - Rewrite every issue and Note file in canonical form
* [capcom init](capcom_init.md)	 - Create the capcom store (.capcom/) in this git repository
* [capcom know](capcom_know.md)	 - Read, search, add, update, move, remove and verify Knowledge Notes
* [capcom list](capcom_list.md)	 - List issues; without flags, every issue that is not closed
* [capcom lock](capcom_lock.md)	 - Acquire, refresh, release and inspect this worktree's Tree lock
* [capcom log](capcom_log.md)	 - Append an entry to an issue's Log
* [capcom merge-driver](capcom_merge-driver.md)	 - Merge three versions of an issue file, its Log by entry (a git merge driver)
* [capcom mv](capcom_mv.md)	 - Rename an issue or move it below another
* [capcom next](capcom_next.md)	 - Show the top Ready Issue in full
* [capcom prime](capcom_prime.md)	 - Print the session Briefing: ready work, locks, other branches and store health
* [capcom ready](capcom_ready.md)	 - List the issues ready to work on, most urgent first
* [capcom reindex](capcom_reindex.md)	 - Rebuild the index from the issue and Note files
* [capcom related](capcom_related.md)	 - Manage related links between issues
* [capcom reopen](capcom_reopen.md)	 - Reopen a closed issue, logging why
* [capcom rm](capcom_rm.md)	 - Remove an issue that has no children
* [capcom search](capcom_search.md)	 - Find issues whose title or body contains every word, else some of them
* [capcom show](capcom_show.md)	 - Show issues in full
* [capcom start](capcom_start.md)	 - Claim a Ready Issue and this worktree's Tree lock
* [capcom ui](capcom_ui.md)	 - Open the terminal UI
* [capcom update](capcom_update.md)	 - Change an issue's fields or body
* [capcom version](capcom_version.md)	 - Print the capcom version

