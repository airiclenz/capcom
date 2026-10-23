---
title: "The Managed block"
description: "capcom agents writes and refreshes a marked section of AGENTS.md telling agents how to use capcom and listing the top of the Knowledge tree; the rest of the file stays yours."
sidebar: { order: 3 }
---

`AGENTS.md` at the root of a repository is the instructions file most coding agents read when a
session starts. capcom owns one marked section of it, the **Managed block**: a pointer to capcom,
the work loop as commands in the order an agent runs them (take the most urgent issue in full, or
find one and read it, claim it and take the Tree lock with `capcom start`, log progress, close it,
or close it and free the lock with `capcom finish`, reopen it), a few more core commands, a
pointer to each command's `--help`, a map of the store, a warning never to claim or close a [Standing Parent](../concepts/issues.md#standing-parent), a
recommendation to take the [Tree lock](../concepts/tree-locks.md) before an extensive change, and
the top level of the [Knowledge](../concepts/knowledge.md) Tree. Everything outside the markers is
your own hand-written guide, and capcom never touches it.

`AGENTS.md` is the block's only home. An agent that reads another guide, such as Claude Code's
`CLAUDE.md` or Gemini CLI's `GEMINI.md`, reaches the block through that guide, which links to
`AGENTS.md` or imports it with an `@AGENTS.md` line; no other file holds a copy of it (see
[Other agent guides](#other-agent-guides)).

## Writing it: `capcom agents`

```console
$ capcom agents
wrote the capcom block to AGENTS.md
```

`capcom agents` replaces the lines between the markers, appends the block to the end of the file
when it has none, and creates `AGENTS.md` when there is no such file. Every byte outside the
markers is kept, line endings included. On an `AGENTS.md` that held only a heading and one line of
its own, the result is:

```markdown
# My project

Hand-written guide.

<!-- BEGIN CAPCOM -->
<!-- Managed by capcom: run "capcom agents" to refresh. Edits inside these markers are overwritten. -->
## Issues and knowledge: capcom

capcom is the issue register of record for this repository. Issues and Knowledge Notes are
markdown files under `.capcom/`, versioned with the code; track work there and nowhere else.

Without a `capcom` binary, install it from https://github.com/airiclenz/capcom#install.

- `capcom next`: the most urgent ready issue, in full
- `capcom ready`: the issues ready to work on, most urgent first
- `capcom show <id>`: one issue in full, with what blocks it
- `capcom start <id>`: claim it and take the Tree lock before you start
- `capcom log <id> <text>`: note progress as work lands
- `capcom close <id> --reason <text>`: close an issue; it names the issues this made ready
- `capcom finish <id> --reason <text>`: close it and free the Tree lock; it names the issue to work on next
- `capcom reopen <id> --reason <text>`: undo a close
- `capcom create --id <slug> --title <title> [--dep <id>]`: file a new issue
- `capcom dep add <id> <dep-id>`: make an issue wait for another; `capcom dep list <id>` shows both sides
- `capcom list --tree --depth 2`: the outline of the issues
- `capcom know tree`: the outline of the Knowledge tree; `capcom know show <id>` reads a Note

Each command's flags are in `capcom <cmd> --help`.

An issue's body says what is wrong or wanted, why, and how to tell it is done (`## Acceptance`),
readable on its own; a plan says how and in what order.

Where things are: an issue is `.capcom/issues/<id>.md`, and its children sit in the directory
`.capcom/issues/<id>/` beside it; Notes follow the same shape under `.capcom/knowledge/`.
Prefer capcom commands; after a hand edit run `capcom check`.

A standing issue (`standing: true` in its file) only groups its children: never claim or close it.

Tree lock: before a change that spans several files or commits, it is recommended to claim this
worktree with `capcom lock acquire --purpose <text>`. capcom keeps the Lock token for the session:
running the same acquire again resumes the lock after a restart or compaction and prints the token
again (or pass `--token <token>`).
`capcom start <id>` takes the lock for an issue, and running the same start again resumes it too.
`capcom lock touch --token <token>` renews it during long work,
`capcom lock release --token <token>` frees it when done. When `capcom lock check --token <token>` exits 3, or
`capcom lock check` does while you hold no token, another session holds this worktree: ask the user
before editing it.

Knowledge tree, top level: no Notes yet.
<!-- END CAPCOM -->
```

The claim line names `capcom start`, which claims the issue in the repository's own doing status,
the first status of the `doing` category in [`config.yaml`](../reference/file-format.md#config),
and takes the Tree lock in the same step. capcom keeps the Lock token for the session, so the
block never asks an agent to keep it: running the same `capcom lock acquire` or `capcom start`
again resumes the lock after a restart or compaction, and `capcom finish` frees it. A config that
declares no doing status leaves the claim line and the Tree lock paragraph's `capcom start`
sentence out; what the doing status is called does not change the block.

Write anything you want agents to know outside the markers: edits inside them are overwritten on
the next refresh.

## The Knowledge tree in the block

The block ends with the top level of the Knowledge tree, one line per top-level Note in id order,
so an agent sees at a glance which branches exist and can open the one that matches its task:

```markdown
Knowledge tree, top level:

- `conventions`: House conventions: gofmt, error wrapping, naming
- `testing`: Tests run through runCLI and never in parallel
```

A [Retired](../concepts/knowledge.md#retired-notes-and-supersession) top-level Note is left out,
and so is a [Background](../concepts/knowledge.md#background-note) one. So is a top-level Note that
does not parse or validate (a blank title, a ref that names no path), or that shares its id or an
alias with another Note: `capcom know tree` hides it too, and `capcom check` reports it. The block
is rendered from the Note files and whether the config declares a doing status alone, so the same
Tree and config always produce the same block.

## Keeping it current

Once `AGENTS.md` holds the block, capcom keeps it current: a Note write that changes the top
level of the Knowledge tree refreshes it in the same command, and says so:

```console
$ capcom know add --id conventions --title "House conventions: gofmt, error wrapping, naming"
id: conventions
path: .capcom/knowledge/conventions.md
etag: 091cdfcb692d501e
refreshed: AGENTS.md
```

That covers adding, renaming, moving, retiring and removing a top-level Note, and changing its
title or its `background` label. A write below the top level changes nothing, since the block
lists only the top level. A Note write never adds a block to a file that has none: the block is
opt-in, through `capcom agents`.

A hand edit of a Note refreshes nothing, and neither does adding or removing the doing status in
`config.yaml`. `capcom check` then reports the block as out of date, at
the line of its begin marker, and `capcom agents --check` fails the same way:

```console
$ capcom agents --check
capcom: AGENTS.md:5: the capcom block is out of date; run capcom agents to refresh it
```

Run `capcom agents` to rewrite it. In CI, `capcom agents --check` (or `capcom check`) catches a
block that has drifted, and an agent guide that does not reach it (see
[Other agent guides](#other-agent-guides)). A file without the block passes both: nothing is out of
date when there is nothing to keep current.

## Other agent guides

Some agents read their own guide instead of `AGENTS.md`: Claude Code reads `CLAUDE.md`, Gemini CLI
reads `GEMINI.md`. The block is never copied into them; each reaches the one in `AGENTS.md`, in one
of two ways:

- the guide is a symbolic link to `AGENTS.md` (`CLAUDE.md -> AGENTS.md`), or
- one of its lines, white space trimmed, is the import line `@AGENTS.md` (or `@./AGENTS.md`). A
  guide in a subdirectory imports the root file by its relative path, `@../AGENTS.md` and so on.

The `agents.guides` key of [`config.yaml`](../reference/file-format.md#config) names the guides
that must reach the block, as paths relative to the repository root. It defaults to
`[CLAUDE.md, GEMINI.md]`, and `guides: []` turns the rule off:

```yaml
format: 4
agents:
  guides: [CLAUDE.md, GEMINI.md, docs/CLAUDE.md]
```

`capcom agents` appends the import line, after a blank line, to every listed guide that exists but
does not reach the block, and names each one:

```console
$ capcom agents
wrote the capcom block to AGENTS.md
added @AGENTS.md to CLAUDE.md
```

On a `CLAUDE.md` that held a heading and one line of its own, the result is:

```markdown
# Claude

House rules.

@AGENTS.md
```

A guide that does not exist is never created, and one that links outside the working tree is left
alone (`skipped CLAUDE.md: links outside the working tree`). Commit the guides it changed together
with `AGENTS.md`.

While `AGENTS.md` carries the block, `capcom check` reports each listed guide that exists but does
not reach it, as an `agents-guide` problem, and `capcom agents --check` fails on the first one:

```console
$ capcom agents --check
capcom: CLAUDE.md: does not reach the capcom block in AGENTS.md; run capcom agents to add @AGENTS.md
```

An `AGENTS.md` without the block reports no guide: there is nothing yet to reach.

## AGENTS.md as a link

When `AGENTS.md` is a symbolic link to another file inside the working tree
(`AGENTS.md -> CLAUDE.md`), capcom writes the block into the file it points to and keeps the link.
That file then reaches the block as it is, with no import line.
A link that points outside the working tree is neither read nor written. The full rules, including
what counts as a broken marker, are in
[The Managed block in AGENTS.md](../reference/file-format.md#the-managed-block-in-agentsmd).

## Next

- [Agent setup](agent-setup.md): the Managed block together with the session hooks.
- [Knowledge](../concepts/knowledge.md): the Notes the block lists.
