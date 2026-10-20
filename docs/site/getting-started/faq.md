---
title: "FAQ"
description: "Short answers to the questions people ask about capcom: what it is, which agents it works with, how agents share a repository, and how it differs from other trackers."
sidebar: { order: 6 }
---

## What is capcom?

capcom is a command-line issue tracker and project knowledge base for AI coding agents and the
people working beside them. Issues and knowledge Notes are plain markdown files under `.capcom/`
in your git repository, versioned with the code. It is one binary for Linux, macOS and Windows,
written in Go, with a CLI for agents and a terminal UI for people.

## Is capcom related to Capcom, the video game company?

No. capcom is an open-source developer tool and is not affiliated with Capcom Co., Ltd. The name
comes from spaceflight, where CAPCOM is the one voice on the ground that talks to the crew.

## Which coding agents does capcom work with?

capcom works with any agent that can run a shell command: Claude Code, OpenAI Codex, Cursor,
Gemini CLI, Aider, GitHub Copilot and others. `capcom agents` writes a short how-to block into
`AGENTS.md` and links it from `CLAUDE.md` and `GEMINI.md`, and `capcom init --claude-hooks` or
`--codex-hooks` runs the session briefing when a session starts. In Claude Code,
`/plugin install capcom --marketplace airiclenz/capcom` installs a capcom plugin that does the same
in every repository with a `.capcom/`. See [Agent setup](../agents/agent-setup.md).

## How does an agent know what to work on next?

`capcom next` prints the most urgent Ready Issue in full: open, with no open Dependency in its
way. `capcom ready` lists all of them, most urgent first, and `capcom prime` prints a briefing of
ready work, work in progress and Tree locks at the start of a session. See
[Dependencies](../concepts/dependencies.md) and [The session briefing](../agents/briefing.md).

## How do I give a coding agent project knowledge that lasts across sessions?

Write it as Knowledge Notes: small markdown files under `.capcom/knowledge/`, committed with the
code, that an agent opens one at a time with `capcom know tree` and `capcom know show <id>`
instead of reading one huge instructions file. Each Note names the files and symbols it talks
about, and `capcom know verify` flags a Note whose files or symbols are gone. An agent's own
memory stays private to one machine; Knowledge is shared by everyone who clones the repository.
See [Knowledge](../concepts/knowledge.md).

## Can two agents work in the same repository at once?

Yes. An agent claims the working tree with `capcom lock acquire` while it edits, and every other
session sees who holds the Tree lock, why and since when. Each git worktree has its own lock, so
agents can work side by side in separate worktrees. See [Tree locks](../concepts/tree-locks.md).

## Does capcom need a server, an account or a database?

No. Issues and Notes are files in your repository. The SQLite index capcom queries is a local
cache under the git directory, rebuilt from the files and never committed. capcom makes no network
calls: it never fetches, pushes or merges.

## How do a team and several agents share issues?

Through git: push, pull and merge. Each Issue is its own file, so edits to different Issues merge
cleanly, and `capcom init --merge-driver` merges two branches' log entries on the same Issue
without a conflict. Run `capcom check` after a merge. See
[Branches and merging](../concepts/branches-and-merging.md).

## Can CI check the issue files?

Yes. `capcom check` reports problems as `path:line: message` and `capcom fmt --check` reports
files out of canonical form; both exit non-zero on a problem and write nothing.

## How is capcom different from beads, GitHub Issues or Task Master?

capcom keeps only files in git, with no database to sync, no hosted service and no LLM calls of
its own. beads keeps a database and exports a copy to git; GitHub Issues lives on a server, apart
from your branches; Task Master generates tasks with an LLM. The full comparison is in
[capcom compared](comparison.md), and a move off beads is in
[Migrating from beads](migrating-from-beads.md).

## Is capcom only for AI agents?

No. The files are readable markdown, `capcom ui` is a terminal UI made for people, and every
command prints plain text as well as `--json`.
