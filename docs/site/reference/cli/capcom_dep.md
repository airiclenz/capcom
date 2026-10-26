---
title: "capcom dep"
description: "Manage dependencies between issues"
sidebar:
  order: 1
---

## capcom dep

Manage dependencies between issues

### Synopsis

Manage the Dependencies between Issues.

A Dependency is a directed edge: the dependent Issue is not Ready until every Issue it depends on is closed.

Each edge is stored in the dependent's file, under the canonical id of the Issue it depends on.

dep add and dep rm change an existing Issue's deps, create --dep sets a new Issue's, and dep list and dep graph show them.

```
capcom dep [flags]
```

### Examples

```
  capcom dep add login-timeout-race token-store-rework   # login-timeout-race waits for token-store-rework
  capcom dep list login-timeout-race                     # its deps and dependents
  capcom dep graph                                       # the open Issues' dependencies as a Mermaid flowchart
```

### Options

```
  -h, --help   help for dep
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents
* [capcom dep add](capcom_dep_add.md)	 - Make an issue depend on another
* [capcom dep graph](capcom_dep_graph.md)	 - Print the dependencies as a Mermaid flowchart
* [capcom dep list](capcom_dep_list.md)	 - List an issue's dependencies and dependents
* [capcom dep rm](capcom_dep_rm.md)	 - Remove a dependency of an issue

