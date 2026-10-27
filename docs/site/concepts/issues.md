---
title: "Issues"
description: "What an Issue is in capcom: its file, its Id and Aliases, its place in the Tree, its Status, when it is Ready, how it is Closed and its Log."
sidebar: { order: 1 }
---

An **Issue** is one unit of tracked work or one known defect in the repository. Each Issue is a
markdown file under `.capcom/issues/`: a YAML header with its title, status, type, priority,
labels and dependencies, then a free markdown body. The file is the Issue; there is no database
behind it. capcom keeps a SQLite index under the git directory to answer queries quickly, but the
index is a cache it rebuilds from the files whenever they change.

This page explains the ideas. The exact keys, their order and the validation rules are in the
[file format reference](../reference/file-format.md#issue-files).

## The Tree

Issues are organised in a **Tree**: every Issue has at most one Parent and any number of
children, and the directory layout is the hierarchy. An Issue is the file `<id>.md`; its children
live in the directory `<id>/` beside it:

```text
.capcom/issues/
  db-schema.md
  login-flow.md
  login-flow/
    login-copy.md
    session-refresh.md
  write-docs.md
```

Create a child with `--parent`, and move an Issue anywhere in the Tree with `capcom mv`:

```console
$ capcom create --id login-copy --title "Reword the login errors" --parent login-flow --priority 1
id: login-copy
path: .capcom/issues/login-flow/login-copy.md
etag: ee3c737826bc9fb6
warning: login-copy has no body: add what, why and acceptance with capcom update login-copy --body-append <text>; create takes --body, --why and --acceptance
```

`capcom list --tree` shows the outline, with how many of each Parent's children are closed:

```console
$ capcom list --tree
db-schema  open  P2  Add the sessions table
login-flow  open  P2  Rework the login flow  (0/2 closed)
  login-copy  open  P1  Reword the login errors
  session-refresh  open  P2  Refresh the session token
write-docs  open  P2  Document the login flow
```

A row filter such as `--status`, `--priority` or `--label` narrows the outline to the Issues it
matches, keeping each match's ancestors so it still shows where it sits; an ancestor that does not
match itself ends `[context]`:

```console
$ capcom list --tree --priority 1
login-flow  open  P2  Rework the login flow  (0/2 closed)  [context]
  login-copy  open  P1  Reword the login errors
```

## Parent

The **Parent** of an Issue is the node directly above it in the Tree. Any Issue with children is a
Parent; there is no separate kind of Issue for grouping work, so a large piece of work is simply
an Issue whose children are its steps. A Parent is not Ready while any of its children is open,
so closing the children is what brings the Parent itself up in `capcom ready`.

### Standing Parent

Some Parents are never the work themselves: a bucket of feature requests, a list of known bugs.
Mark such an Issue a **Standing Parent** with `capcom create --standing` or
`capcom update <id> --standing`, which write `standing: true` into its file. A Standing Parent is
never Ready, even with every child closed, and while it is marked it is never claimed or closed:
a status in the `doing` category and `capcom close` are refused, naming the way out:

```console
$ capcom update feature-requests --status active
capcom: .capcom/issues/feature-requests.md: the issue is standing: it only groups its children and is never claimed or closed
hint: capcom update feature-requests --no-standing
```

`capcom update <id> --no-standing` clears the mark, and is applied before `--status`, so
`capcom update <id> --no-standing --status active` clears it and claims the Issue in one call. An
Issue that is Closed or in the `doing` category cannot be marked. `capcom show` prints a
`standing: true` line for a marked Issue, `capcom ready --all` lists it as held back by
`standing: holds children`, and `capcom check` reports a Standing Parent whose Status was moved
into the `doing` category or a closed one by hand. The `standing` key needs store format 5 (see
[Fields](../reference/file-format.md#fields)).

## Id

Every Issue has an **Id**: a short, permanent, human-meaningful name that says in a few words what
the Issue is about. It is a kebab-case slug of one to four words (`login-timeout-race`), unique
across the whole Tree. You always choose it; capcom never generates one, and there is no project
prefix.

The Id is the file name without `.md`, and it is never written inside the file. Any command that
takes an Id also accepts an Alias or a prefix that matches exactly one Id, so `capcom show sess`
finds `session-refresh` when nothing else starts with `sess`.

## Alias

An **Alias** is a former Id that still resolves to its Issue, kept so that old citations in
commits, plans and other Issues stay valid. Renaming an Issue with `capcom mv` records the old Id
as an Alias, and every `deps` and `related` entry naming the old Id is rewritten:

```console
$ capcom mv login-copy login-error-copy
id: login-error-copy
path: .capcom/issues/login-flow/login-error-copy.md
etag: 0448aa34315f4823
$ capcom show login-copy
id: login-error-copy
path: .capcom/issues/login-flow/login-error-copy.md
etag: 0448aa34315f4823
parent: login-flow
title: Reword the login errors
status: open
category: todo
ready: yes
type: task
priority: 1
aliases: [login-copy]
created: 2026-10-13T18:54:28Z
```

An Issue may have any number of Aliases, and no Id may reuse one. Imports use them too: an Issue
brought over from another tracker keeps its old identifier as an Alias (see
[Migrating from beads](../getting-started/migrating-from-beads.md)).

## Status and Status category

An Issue's **Status** is its workflow state, named by the repository. Each Status belongs to
exactly one of five fixed **Status categories**, and capcom's behaviour depends only on the
category, never on the name:

| Category | Meaning | Default status |
| --- | --- | --- |
| `todo` | Not started. Only `todo` Issues can be Ready. | `open` |
| `doing` | Someone is working on it. | `active` |
| `waiting` | Parked: neither it nor any Issue below it is Ready. | `parked` |
| `done` | Closed, the work is complete. | `done` |
| `dropped` | Closed, the work will not be done. | `dropped` |

A repository declares its own statuses in `.capcom/config.yaml`, in order, each mapped to a
category: a `review` status in the `doing` category, say. When capcom moves an Issue into a
category (on `create`, `close` or `reopen`) it uses the first status listed for that category. See
[Config](../reference/file-format.md#config).

Claim an Issue by moving it into the `doing` category. `capcom start` does that and takes the
worktree's [Tree lock](tree-locks.md#start-and-finish-the-lock-with-an-issue) in one step, then
prints the Issue in full and the Lock token, which capcom keeps for the session:

```console
$ capcom start db-schema
id: db-schema
path: .capcom/issues/db-schema.md
etag: 31ea8488cc1aa362
title: Add the sessions table
status: active
category: doing
type: task
priority: 2
dependents: [session-refresh]
created: 2026-10-22T09:45:59Z

_main: acquired by claude@devbox since 2026-10-22T09:45:59Z (heartbeat 2026-10-22T09:45:59Z): db-schema
token 670fa2ee31064cc626fff5a087e22d95
```

It refuses, writing nothing, an Issue that is Closed, standing or not Ready, and a lock another
session holds. `capcom update db-schema --status active` moves the Issue alone and leaves the lock
untouched. Running `capcom start db-schema` again, after a restart or a compaction, resumes the
lock and prints the token again.

## Ready

An Issue is **Ready** when all of these hold:

- its Status is in the `todo` category;
- every Issue it depends on is Closed;
- none of its children is open;
- none of its ancestors is in the `waiting` or `dropped` category;
- it is not a [Standing Parent](#standing-parent).

`capcom ready` lists the Ready Issues, most urgent first; `capcom ready --all` adds the `todo`
Issues that are not Ready, each with what holds it back:

```console
$ capcom ready --all
login-copy  open  P1  Reword the login errors
db-schema  open  P2  Add the sessions table  -- unblocks 1
login-flow  open  P2  Rework the login flow  -- not ready: open children: login-copy, session-refresh
session-refresh  open  P2  Refresh the session token  -- not ready: blocked by db-schema (todo)
write-docs  open  P2  Document the login flow  -- not ready: blocked by login-flow (todo)
```

`capcom show` prints the same verdict on its `ready:` line for any `todo` Issue. How the list is
ordered, and what `-- unblocks 1` means, is covered in [Dependencies](dependencies.md).

## Closed

An Issue is **Closed** when its Status is in the `done` or `dropped` category. Closing always
records a reason, which is kept in the file beside the time it was closed:

```console
$ capcom close db-schema --reason "Added in the sessions migration."
db-schema  closed
now ready: session-refresh
```

The `now ready:` line names the Issues this close made Ready, so the next step is in front of you.
`capcom finish` closes the Issue and frees the Tree lock `capcom start` took, then names the Ready
Issue to work on next:

```console
$ capcom finish db-schema --reason "Added in the sessions migration."
db-schema  closed
now ready: session-refresh
_main: released
next: session-refresh  Refresh the session token
```

Close as dropped with `--status dropped`. `capcom reopen <id> --reason <text>` moves a Closed Issue
back to `todo` and appends the reason to its Log.

## Log

The **Log** is the dated, append-only record of progress and remarks kept inside an Issue, under
the `## Log` heading of its body. `capcom log` appends one entry, with today's date and your git
author name:

```console
$ capcom log login-copy "draft wording agreed with design"
id: login-copy
path: .capcom/issues/login-flow/login-copy.md
etag: 8edd976cfe0d214a
$ capcom show login-copy
...
## Log

- 2026-10-13 you: draft wording agreed with design
```

Because the Log lives in the file, it branches and merges with the code. Two branches that both
log on the same Issue merge cleanly with capcom's merge driver; see
[Branches and merging](branches-and-merging.md).

## The body

Everything after the header is free markdown, kept byte for byte. By convention an Issue's body
says what is wrong or wanted and why, then holds an `## Acceptance` section saying how to tell it
is done, then the `## Log`.

`capcom create` takes the body inline: `--body <text>` (or `--body-file`, where `-` reads
standard input), then `--why <text>` for a `## Why` section and a repeatable `--acceptance <text>`
for each bullet of the `## Acceptance` section. `--body` and `--body-file` cannot be combined:

```console
$ capcom create --id login-copy --title "Reword the login errors" --body "The errors name no fix." \
    --why "Users retry blindly." --acceptance "Each error names its fix"
```

`capcom create` warns when a new Issue has no body; add one with
`capcom update <id> --body-append <text>`.

## Next

- [Dependencies](dependencies.md): Dependencies, Related links and the order of `capcom ready`.
- [Branches and merging](branches-and-merging.md): Issues on other branches, and merging Logs.
- [File format](../reference/file-format.md): every key of an Issue file.
