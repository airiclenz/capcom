---
title: "Dependencies"
description: "How Dependencies decide which Issues are Ready, how Related links differ from them, the order capcom ready lists work in, and the dep graph."
sidebar: { order: 2 }
---

Issues rarely stand alone: a migration has to land before the code that uses it, and a bug fix
and its documentation belong together. capcom has two ways to tie Issues to each other. A
**Dependency** blocks; a **Related link** does not.

## Dependency

A **Dependency** is a directed edge between two Issues: the dependent Issue cannot be worked until
the one it depends on is Closed. It is stored in the dependent's file, as the Id of the Issue it
depends on in its `deps` list.

Set Dependencies when creating an Issue, or add and remove them later:

```sh
capcom create --id session-refresh --title "Refresh the session token" --dep db-schema
capcom dep add write-docs db-schema     # write-docs now waits for db-schema
capcom dep rm write-docs db-schema
```

Either argument may be an Id, an Alias or a unique prefix. An Issue cannot depend on itself, and
an edge that would close a cycle is refused, with the cycle named:

```console
$ capcom dep add login-flow write-docs
capcom: .capcom/issues/login-flow.md: login-flow cannot depend on write-docs: that would create the cycle login-flow -> write-docs -> login-flow
```

`capcom dep list <id>` shows both sides of one Issue: what it depends on, and what depends on it.

```console
$ capcom dep list db-schema
deps:
dependents:
  session-refresh  open  P2  Refresh the session token
```

`capcom show` gives the same picture on its `deps:` and `dependents:` lines, and for a `todo`
Issue its `ready:` line names each Dependency still in the way:

```console
$ capcom show session-refresh
...
ready: no (blocked by db-schema (todo))
...
deps: [db-schema]
```

A Dependency on an Issue that is `done` or `dropped` is satisfied: dropping an Issue releases
whatever waited for it. A Dependency naming no Issue at all is never satisfied; `ready --all`
reports it as `(missing)` and `capcom check` lists it as a problem.

## Related link

A **Related link** is an undirected tie between two Issues that belong together. It blocks neither
of them and never affects `ready`, `dep` or the graph. It is stored in the `related` list of one of
the two Issues and shown on both:

```console
$ capcom related add write-docs login-error-copy
id: write-docs
path: .capcom/issues/write-docs.md
etag: c11b053800607aeb
$ capcom show login-error-copy
...
related: [write-docs]
```

`capcom related rm <a> <b>` removes the link from whichever Issue holds it. Use a Related link for
"look at this one too"; use a Dependency only when the order of the work matters. The first
Related link in an older store raises its format version; see
[Related links](../reference/file-format.md#related-links) in the file format reference.

## How `capcom ready` orders work

`capcom ready` lists only the [Ready](issues.md#ready) Issues, sorted by:

1. priority, most urgent first (`P0` before `P4`);
2. then the Issue that **unblocks** more Issues first, where an Issue unblocks exactly the `todo`
   Issues that would become Ready if it alone were closed;
3. then the order of the Tree.

An Issue that unblocks others says how many at the end of its line:

```console
$ capcom ready
login-copy  open  P1  Reword the login errors
db-schema  open  P2  Add the sessions table  -- unblocks 1
```

So among Issues of the same priority, the one holding up the most work comes first. When you close
an Issue, `capcom close` names what it made Ready:

```console
$ capcom close db-schema --reason "Added in the sessions migration."
db-schema  closed
now ready: session-refresh
```

Issues carrying a label listed under `ready.hide` in `.capcom/config.yaml` stay out of `ready`
unless you pass `--show-hidden`; use it for work only a person should pick up.

## The dependency graph

`capcom dep graph` prints the Dependencies as a [Mermaid](https://mermaid.js.org/) flowchart,
with an edge from each Dependency to its dependent. By default it shows every Issue that is not
Closed; `--all` adds the Closed ones, and `capcom dep graph <id>` narrows it to one Issue and
everything it transitively depends on or is depended on by:

```console
$ capcom dep graph
flowchart LR
    n1["db-schema (active)"]
    n2["login-flow (open)"]
    n3["login-error-copy (open)"]
    n4["session-refresh (open)"]
    n5["write-docs (open)"]
    n1 --> n4
    n2 --> n5
```

Paste it into anything that renders Mermaid (GitHub markdown does, inside a `mermaid` fenced
block) to see the shape of the work.

The exact output of `show`, `close`, `ready` and `dep graph`, in text and `--json`, is specified
under [Dependency output](../reference/file-format.md#dependency-output).

## Next

- [Issues](issues.md): Status categories, Ready and Closed.
- [Branches and merging](branches-and-merging.md): what happens when two branches change the same
  Issues.
