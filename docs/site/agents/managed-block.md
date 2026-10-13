---
title: "The Managed block"
description: "capcom agents writes and refreshes a marked section of AGENTS.md telling agents how to use capcom and listing the top of the Knowledge tree; the rest of the file stays yours."
sidebar: { order: 3 }
---

`AGENTS.md` at the root of a repository is the instructions file most coding agents read when a
session starts. capcom owns one marked section of it, the **Managed block**: a pointer to capcom,
the core commands, a map of the store and the top level of the [Knowledge](../concepts/knowledge.md)
Tree. Everything outside the markers is your own hand-written guide, and capcom never touches it.

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

- `capcom ready`: the issues ready to work on, most urgent first
- `capcom show <id>`: one issue in full, with what blocks it
- `capcom create --id <slug> --title <title> [--dep <id>]`: file a new issue
- `capcom dep add <id> <dep-id>`: make an issue wait for another; `capcom dep list <id>` shows both sides
- `capcom close <id> --reason <text>`: close an issue; it names the issues this made ready
- `capcom list --tree --depth 2`: the outline of the issues
- `capcom know tree`: the outline of the Knowledge tree; `capcom know show <id>` reads a Note

An issue's body says what is wrong or wanted, why, and how to tell it is done (`## Acceptance`),
readable on its own; a plan says how and in what order.

Where things are: an issue is `.capcom/issues/<id>.md`, and its children sit in the directory
`.capcom/issues/<id>/` beside it; Notes follow the same shape under `.capcom/knowledge/`.
Prefer capcom commands; after a hand edit run `capcom check`.

Knowledge tree, top level: no Notes yet.
<!-- END CAPCOM -->
```

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
and so is a [Background](../concepts/knowledge.md#background-note) one. The block is rendered from
the Note files alone, so the same Tree always produces the same block.

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

A hand edit of a Note refreshes nothing. `capcom check` then reports the block as out of date, at
the line of its begin marker, and `capcom agents --check` fails the same way:

```console
$ capcom agents --check
capcom: AGENTS.md:5: the capcom block is out of date; run capcom agents to refresh it
```

Run `capcom agents` to rewrite it. In CI, `capcom agents --check` (or `capcom check`) catches a
block that has drifted. A file without the block passes both: nothing is out of date when there
is nothing to keep current.

## AGENTS.md as a link

When `AGENTS.md` is a symbolic link to another file inside the working tree
(`AGENTS.md -> CLAUDE.md`), capcom writes the block into the file it points to and keeps the link.
A link that points outside the working tree is neither read nor written. The full rules, including
what counts as a broken marker, are in
[The Managed block in AGENTS.md](../reference/file-format.md#the-managed-block-in-agentsmd).

## Next

- [Agent setup](agent-setup.md): the Managed block together with the session hooks.
- [Knowledge](../concepts/knowledge.md): the Notes the block lists.
