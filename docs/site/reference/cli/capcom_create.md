---
title: "capcom create"
description: "Create an issue with a chosen id"
sidebar:
  order: 1
---

## capcom create

Create an issue with a chosen id

### Synopsis

Create an Issue under the id you choose: a kebab-case slug of 1 to 4 words that names the work.

--parent files it below another Issue, and --dep makes it wait for the Issues named; a --dep that names no Issue fails the create before anything is written.

The body comes from --body, or --body-file (- reads stdin); --why adds a "## Why" section and each --acceptance a bullet under "## Acceptance". An Issue created without a body draws a warning.

--standing marks it a standing Parent that only groups its children: it is never Ready, and it is not claimed or closed while marked, so it cannot be created in the doing category.

The first standing Issue in a store below format 5 raises the store format to 5, with a warning that older capcom releases can no longer read the store.

It prints the new Issue's id, path and etag.

```
capcom create --id <slug> --title <title> [flags]
```

### Examples

```
  capcom create --id login-timeout-race --title "Session refresh races the login timeout" --type bug
  capcom create --id refresh-retry --title "Retry a failed token refresh" --dep login-timeout-race   # waits for it
  capcom create --id login-copy --title "Reword the login errors" --parent login-flow --priority 1
  capcom create --id cache-warmup --title "Warm the cache on deploy" --body "Deploys start cold." --why "The first request times out." --acceptance "Warm after deploy"
  capcom create --id cache-evict --title "Evict stale cache entries" --body-file - < body.md   # body from stdin
  capcom create --id feature-requests --title "Feature requests" --standing   # only groups its children: never Ready
```

### Options

```
      --acceptance stringArray   a bullet of the "## Acceptance" section added last; repeat for more
      --body string              the markdown body, taken literally
      --body-file string         file holding the markdown body; - reads stdin
      --dep strings              issues the new one depends on; repeat or separate with commas
  -h, --help                     help for create
      --id string                the new issue's id: a kebab slug of 1 to 4 words (required)
      --kv                       print KEY=VALUE lines ending in STATUS=
      --label strings            labels; repeat or separate with commas
      --parent string            create the issue below this one
      --plan string              basename of the plan the issue belongs to
      --priority int             priority, 0 (most urgent) to 4 (default 2)
      --standing                 mark the issue standing: it only groups its children
      --status string            initial status, not in a closed category (default: the first todo status)
      --title string             the issue's title (required)
      --type string              the issue's type (default task, or the first configured type)
      --why string               text of a "## Why" section added after the body
```

### Options inherited from parent commands

```
      --json   print a versioned JSON envelope
```

### SEE ALSO

* [capcom](capcom.md)	 - A git-native issue tracker and knowledge store for coding agents

