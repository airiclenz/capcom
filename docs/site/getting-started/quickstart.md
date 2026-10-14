---
title: "Quick start"
description: "Set capcom up in a git repository and take one Issue from creation to close, with the real output of each command."
sidebar: { order: 2 }
---

This walk-through sets capcom up in an existing git repository, files two Issues, works one of
them and closes it. Every output below is what capcom printed; only the paths, the etags (content
hashes), the dates and the author will differ on your machine. It assumes capcom is
[installed](install.md).

## 1. Create the store

Run `capcom init` from anywhere inside the repository:

```console
$ capcom init
initialized capcom store at /home/you/your-repo/.capcom
created .capcom
created .capcom/issues
created .capcom/knowledge
created .capcom/config.yaml
run capcom agents to write the capcom block into AGENTS.md

To brief Claude Code at every session start and before compaction, add to .claude/settings.json (or run capcom init --claude-hooks):
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume|clear|compact",
        "hooks": [
          {
            "type": "command",
            "command": "capcom prime"
          }
        ]
      }
    ],
    "PreCompact": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "capcom prime"
          }
        ]
      }
    ]
  }
}
```

`.capcom/config.yaml` lists every setting with its default value and a comment. The snippet at
the end is for Claude Code users: `capcom init --claude-hooks` writes it for you, and
`capcom init --codex-hooks` does the same for Codex. The [agent setup](../agents/agent-setup.md)
page covers both.

## 2. Tell your agents

```console
$ capcom agents
wrote the capcom block to AGENTS.md
```

`AGENTS.md` is the instructions file most coding agents read on start-up. capcom writes a short
block into it, between `<!-- BEGIN CAPCOM -->` and `<!-- END CAPCOM -->`, that tells agents how to
use capcom, and leaves the rest of the file alone. Run `capcom agents` again after upgrading capcom
to refresh it.

## 3. File Issues

An Issue's id is a readable slug you choose. Its body says what is wrong or wanted, why, and how
to tell it is done under `## Acceptance`; `--body-file -` reads it from standard input:

```console
$ printf 'Install capcom and file the first issue.\n\n## Acceptance\n\n- `capcom ready` lists this issue.\n' \
    | capcom create --id first-task --title "Try capcom" --body-file -
id: first-task
path: .capcom/issues/first-task.md
etag: 59c2c01c603f850b
```

A second Issue that waits for the first one, through `--dep`:

```console
$ printf 'Write down how the team installs capcom.\n\n## Acceptance\n\n- The README links the install steps.\n' \
    | capcom create --id write-docs --title "Document the setup" --dep first-task --body-file -
id: write-docs
path: .capcom/issues/write-docs.md
etag: 2eed913e840b77d8
```

Without a body, `capcom create` still files the Issue and prints a `warning:` line asking for one.

## 4. Find the next piece of work

```console
$ capcom ready
first-task  open  P2  Try capcom  -- unblocks 1
```

`capcom ready` lists the open Issues with nothing open in their way, most urgent first. `write-docs`
is missing because it depends on `first-task`; `-- unblocks 1` says that closing `first-task` frees
one other Issue.

## 5. Record progress

The Log is the append-only part of an Issue: progress, findings and remarks go there, while the
body stays the statement of the work.

```console
$ capcom log first-task "installed with brew; init and agents ran clean"
id: first-task
path: .capcom/issues/first-task.md
etag: 8c57c550988a0ccc
```

## 6. Close the Issue

```console
$ capcom close first-task --reason "capcom works in this repo"
first-task  closed
now ready: write-docs
```

Closing names the Issues it made ready. The Issue file now holds the whole story:

```console
$ cat .capcom/issues/first-task.md
---
title: Try capcom
status: done
type: task
priority: 2
created: 2026-10-13T18:51:24Z
closed: 2026-10-13T18:51:24Z
reason: capcom works in this repo
---

Install capcom and file the first issue.

## Acceptance

- `capcom ready` lists this issue.

## Log

- 2026-10-13 Ada Lovelace: installed with brew; init and agents ran clean
```

The Log entry is dated and attributed to the author git would record for a commit.

## 7. Brief a session

`capcom prime` is the briefing an agent reads at the start of a session: the ready work, the
Issues in progress, the Tree lock holders and the health of the store.

```console
$ capcom prime
ready, planned: 0
ready, unplanned: 1
  write-docs  open  P2  Document the setup
doing: 0
locks: 0
this worktree: free; before a multi-file change run capcom lock acquire --purpose <text> and keep the token
broken Note refs: 0
check: no problems
```

With the Claude Code or Codex hooks installed, every session starts with this output.

## 8. Commit

Issues are files, so they are committed like code:

```sh
git add .capcom AGENTS.md && git commit -m "chore: track issues with capcom"
```

The SQLite index capcom builds for its queries lives under the git directory and is never
committed; capcom rebuilds it from the files whenever they change.

## Next

- [Issues](../concepts/issues.md) and [dependencies](../concepts/dependencies.md) in depth.
- [Knowledge](../concepts/knowledge.md): keep project facts as Notes agents can look up.
- [Agent setup](../agents/agent-setup.md): hooks and instructions for each coding agent.
- [CLI reference](../reference/cli/capcom.md): every command and flag.
