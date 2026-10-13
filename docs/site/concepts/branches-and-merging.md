---
title: "Branches and merging"
description: "Issues branch and merge with the code: what capcom reports from other branches (Elsewhere), how the Log merge driver merges Logs, and checking the store after a merge."
sidebar: { order: 3 }
---

Issue files live in the working tree, so they branch, merge and travel with the code. The state
capcom reports is always the state of the branch you have checked out. Closing an Issue in the same
commit as its fix means the two merge together or not at all, and git is the only way Issues move
between machines: there is no server to sync and no export to keep in step.

The price is that a change made on one branch is not visible on another until it is merged. capcom
softens that by reporting such changes as **Elsewhere**, and makes the merge itself painless with a
merge driver for the Log.

## Elsewhere

An **Elsewhere** entry is a change to an Issue that exists on another local branch, another
worktree's branch or a remote-tracking branch, but not on the current branch. `capcom ready` and
`capcom prime` report these beside their result, and never merge them in:

```console
$ capcom ready
login-error-copy  open  P1  Reword the login errors
session-refresh  open  P2  Refresh the session token
elsewhere: login-error-copy: active on fix-login (unmerged)
elsewhere: rate-limit: added on fix-login (unmerged)
```

Here the branch `fix-login` has claimed `login-error-copy` and filed a new Issue, `rate-limit`,
neither of which `main` has yet. The first line is a warning not to start work someone else has
already taken. Each entry says whether the branch `added`, modified or `removed` the Issue file
since it split from the current branch; for a modified file, it gives the Status on that branch.

How the scan works:

- It compares each branch with the current one from their merge base, so it reports only commits
  the current branch lacks.
- It reads local branches (including branches checked out in other worktrees) and remote-tracking
  branches such as `origin/fix-login`. Uncommitted changes in other worktrees are not seen.
- It never fetches. A remote-tracking branch is as fresh as your last `git fetch`.
- It skips branches whose tip is older than `elsewhere.days` in `.capcom/config.yaml` (30 by
  default); `0` turns the scan off. See [Config](../reference/file-format.md#config).

Pass `--no-elsewhere` to `capcom ready` to leave the report out. In `--json` the entries are a
top-level `"elsewhere"` array beside `"data"`, each with `id`, `branch`, `change`, `path` and, for a
modified file, `status`.

## Merging Logs

Two branches that each append to the same Issue's [Log](issues.md#log) add lines at the same
place, so git's ordinary text merge stops on a conflict. capcom ships a git merge driver,
`capcom merge-driver`, that merges a Log entry by entry instead: it keeps one side's entries in
order and merges the other side's new entries in by date. The rest of the file merges as usual,
and a real conflict outside the Log still stops the merge.

Turn it on once per repository:

```console
$ capcom init --merge-driver
capcom store already initialized at /home/you/your-repo/.capcom
added the capcom-log merge driver to .gitattributes
git uses the capcom-log merge driver only once it is configured; run:
  git config merge.capcom-log.name "capcom Log merge"
  git config merge.capcom-log.driver "capcom merge-driver %O %A %B %P %S %X %Y"
```

(Plain `capcom init` also prints a snippet for Claude Code hooks after this; see
[Agent setup](../agents/agent-setup.md).)

The line it adds to `.gitattributes` is:

```text
.capcom/issues/**/*.md merge=capcom-log
```

Commit `.gitattributes`. git does not version its own config, so every clone runs the two
`git config` commands once; capcom never writes `.git/config` itself. A clone without them merges
Issue files with git's normal text merge, and a Log conflict there is resolved by hand: keep both
sides' entries in date order, delete the marker lines, and run `capcom check`.

With the driver configured, a branch that logged on an Issue merges into one that logged on the
same Issue without a conflict:

```console
$ git merge --no-edit fix-login
Auto-merging .capcom/issues/login-flow/login-error-copy.md
Merge made by the 'ort' strategy.
...
$ capcom show login-error-copy
...
## Log

- 2026-10-13 you: draft wording agreed with design
- 2026-10-13 you: checked the copy against the style guide
- 2026-10-13 you: new copy in review
```

The driver's exact rules are in
[The Log merge driver](../reference/file-format.md#the-log-merge-driver).

## Check the store after a merge

A merge without a conflict can still leave the store inconsistent: one branch removes an Issue that
the other branch made a Dependency of, for example. `capcom check` validates the whole store from
its files and lists each problem as `path:line: message`, failing with exit code 1 when there is
any:

```console
$ capcom check
.capcom/issues/write-docs.md:6: dependency "ghost-issue" names no issue by id or by an alias only one issue has
```

It also reports any git conflict marker left in an Issue or Note file. Run it after every merge and
after any hand edit. To make that automatic, install a git `post-merge` hook that runs it:

```sh
#!/bin/sh
capcom check
```

Save it as `.git/hooks/post-merge` (or in the directory `core.hooksPath` names) and make it
executable. git does not version hooks, so every clone installs it once, and git does not run it
when a merge stops on a conflict or for `git pull --rebase`: run `capcom check` yourself then. See
[Checking the store after every merge](../reference/file-format.md#checking-the-store-after-every-merge).

## Next

- [Issues](issues.md): the Log and the other parts of an Issue.
- [Tree locks](tree-locks.md): telling other agent sessions that you are editing this working
  tree.
