---
title: "capcom dep graph"
description: "Print the dependencies as a Mermaid flowchart"
sidebar:
  order: 1
---

## capcom dep graph

Print the dependencies as a Mermaid flowchart

### Synopsis

Print the Dependency graph as a Mermaid flowchart: one node per Issue, labelled with its id and status, and one arrow per Dependency, from the Issue depended on to the Issue that waits for it. Without an argument it shows every Issue that is not closed; with one, that Issue and every Issue its deps and dependents reach, transitively. --all adds the closed Issues. A dep that names no Issue draws no arrow. Paste the output into a mermaid code block to render it.

```
capcom dep graph [id|alias|prefix] [flags]
```

### Examples

```
  capcom dep graph                       # the open Issues' dependencies
  capcom dep graph login-timeout-race    # only the Issues connected to it
  capcom dep graph --all                 # closed Issues too
```

### Options

```
      --all    include closed issues
  -h, --help   help for graph
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom dep](capcom_dep.md)	 - Manage dependencies between issues

