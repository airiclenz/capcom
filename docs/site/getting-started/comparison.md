---
title: "capcom compared"
description: "How capcom compares with beads, GitHub Issues, Task Master and a markdown TODO or agent instructions file for tracking the work of AI coding agents."
sidebar: { order: 5 }
---

capcom is one of several ways to track the work of AI coding agents. What sets it apart is that
the files in git are the only store: no database to sync, no hosted service, no background process
and no LLM calls of its own. This page compares it with the alternatives people most often weigh.

## At a glance

| | capcom | beads | GitHub Issues | Task Master | `TODO.md` or agent memory |
| --- | --- | --- | --- | --- | --- |
| Where the data lives | Markdown files in the repository | A database, with a JSONL export in git | GitHub's servers | A JSON task file in the repository | One markdown file |
| Branches with the code | Yes | Through the export | No | Yes | Yes |
| Works offline, no account | Yes | Yes | No | Needs an LLM API key to generate tasks | Yes |
| Dependencies and a ready queue | Yes | Yes | Partial | Yes | No |

Beyond the table, capcom validates its files in CI with `capcom check`, keeps project knowledge
as small Notes beside the Issues, and shows every agent session who holds the Tree lock on a
worktree.

## capcom vs beads

beads (`bd`) is an issue tracker for coding agents that keeps its issues in a database and exports
a JSONL copy for git. capcom has no database to keep in step: each Issue is its own markdown file,
the SQLite index is a cache rebuilt from the files and never committed, and an Issue branches,
merges and shows up in code review with the code it describes. Ids are readable slugs you choose
(`login-timeout-race`) instead of hashes. To move a repository over, follow
[Migrating from beads](migrating-from-beads.md).

## capcom vs GitHub Issues

GitHub Issues lives on GitHub's servers, so an agent needs network access and credentials to read
it, and an issue cannot change on a branch and merge with the code. capcom lives in the
repository: an agent reads it offline with no account, a pull request shows the Issue change next
to the code change, and `capcom ready` answers "what is unblocked" from Dependencies. GitHub
Issues remains the better place for bug reports from outside contributors; capcom is for the work
the team and its agents plan and do.

## capcom vs Task Master

Task Master is an AI task manager that keeps tasks in a JSON file and uses an LLM, with your API
key, to break a product requirements document into tasks. capcom calls no LLM: the agent you
already run writes Issues as markdown, and capcom validates, indexes and orders them. Each Issue is
its own file, so two branches touching different Issues merge without conflict.

## capcom vs a TODO.md or an agent instructions file

A single `TODO.md`, or a growing `CLAUDE.md` or `AGENTS.md`, works for a short list but has no
Dependencies, no ready queue, no validation, and conflicts whenever two branches edit it. capcom
splits work into one file per Issue and knowledge into small Notes an agent opens only when it
needs them, and keeps `AGENTS.md` to a short block that points at them. See
[Knowledge](../concepts/knowledge.md) and [The Managed block](../agents/managed-block.md).

## When capcom is not the right fit

- You need a hosted board for people outside the repository, with notifications: capcom sends
  nothing and runs no server.
- Your issues must live apart from any one repository: capcom keeps each repository's issues in
  that repository.
