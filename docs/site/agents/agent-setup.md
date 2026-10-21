---
title: "Agent setup"
description: "Wire capcom into a coding agent: the Managed block in AGENTS.md, session hooks that run capcom prime for Claude Code, Codex and apogee, and the Claude Code plugin."
sidebar: { order: 1 }
---

capcom is a plain command-line tool, so any agent that can run a shell command can use it: Claude
Code, OpenAI Codex, Cursor, Gemini CLI, Aider, GitHub Copilot's agent mode or your own scripts.
Wiring it in takes two things:

1. **Tell the agent capcom exists.** `capcom agents` writes a short [Managed block](managed-block.md)
   into `AGENTS.md`, the instructions file most coding agents read on start-up.
2. **Brief the agent when a session starts.** `capcom prime` prints the
   [session briefing](briefing.md): the Ready work, the Issues in progress, who holds the Tree
   locks, changes on other branches and the health of the store. A session hook runs it for you.

The rest of this page sets both up, with Claude Code as the worked example.

## Step 1: the Managed block

From the repository root:

```console
$ capcom agents
wrote the capcom block to AGENTS.md
```

This adds a marked section to `AGENTS.md` (creating the file when there is none) and leaves the
rest of the file alone. [The Managed block](managed-block.md) explains what it holds and how it
stays current.

`AGENTS.md` is the block's only home. An agent that reads a different instructions file reaches it
through that file: Claude Code reads `CLAUDE.md`, Gemini CLI reads `GEMINI.md`, and a guide holding
the line `@AGENTS.md` makes the agent read `AGENTS.md` too; capcom's own repository does exactly
that. The `agents.guides` key of `config.yaml` lists the guides that must reach the block,
`[CLAUDE.md, GEMINI.md]` by default, and `capcom agents` appends `@AGENTS.md` to each one that
exists without reaching it:

```console
$ capcom agents
wrote the capcom block to AGENTS.md
added @AGENTS.md to CLAUDE.md
```

It never creates a guide that does not exist. A guide that is a symbolic link to `AGENTS.md`
reaches the block as it is, and so does the reverse link `AGENTS.md -> CLAUDE.md`, where capcom
writes the block into the file the link points to. While `AGENTS.md` carries the block,
`capcom check` and `capcom agents --check` report a listed guide that does not reach it
([Other agent guides](managed-block.md#other-agent-guides) has the details). Commit `AGENTS.md`
and every guide `capcom agents` changed.

## Step 2: Claude Code hooks

Claude Code runs commands from hooks declared in `.claude/settings.json`. capcom needs two:

- **`SessionStart`**, so every new, resumed or cleared session, and every session continuing after
  compaction, starts with the briefing;
- **`PreCompact`**, which runs it again just before Claude Code compacts a long conversation.

`capcom init --claude-hooks` merges both into `.claude/settings.json`, creating it when needed and
keeping every other setting:

```console
$ capcom init --claude-hooks
initialized capcom store at /home/you/your-repo/.capcom
created .capcom
created .capcom/issues
created .capcom/knowledge
created .capcom/config.yaml
run capcom agents to write the capcom block into AGENTS.md
added the capcom prime hooks to .claude/settings.json
```

It works on a store that already exists too (it then says `capcom store already initialized`),
and a re-run reports `.claude/settings.json already runs capcom prime` and changes nothing. The
hooks it writes are:

```json
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

Plain `capcom init`, without `--claude-hooks`, prints this same snippet after its own output,
headed "To brief Claude Code at every session start and before compaction, add to
.claude/settings.json (or run capcom init --claude-hooks):", so you can add it by hand, to
`.claude/settings.json` or to your user-level settings. Add both hooks, not `SessionStart` alone:
together they brief the agent at the start of every session and again around every compaction.

Commit `.claude/settings.json` so every clone gets the hooks.

## Claude Code plugin

capcom's repository is also a Claude Code plugin marketplace. In Claude Code 2.1.275 or later:

```console
/plugin install capcom --marketplace airiclenz/capcom
```

On an older Claude Code, add the marketplace first:

```console
/plugin marketplace add airiclenz/capcom
/plugin install capcom@airiclenz
```

The plugin installs once for you, not per repository, and brings two things:

- **A `capcom` skill.** It tells Claude how to install the binary, set a repository up with
  `capcom init` and `capcom agents`, and work an Issue from `capcom next` to `capcom close`.
- **The same two hooks as above**, running `capcom prime` at session start and before compaction.
  They stay silent when the `capcom` binary is missing, when the repository has no `.capcom/`, and
  when `.claude/settings.json`, `.claude/settings.local.json` or your user-level settings already
  run `capcom prime`, so a repository set up with `--claude-hooks` is never briefed twice.

The plugin does not install the binary or replace Step 1: the Managed block in `AGENTS.md` is
still what tells every agent, and every collaborator, how the repository uses capcom. Keep
`--claude-hooks` for collaborators who do not have the plugin.

## Codex hooks

`capcom init --codex-hooks` does the same for OpenAI Codex, merging `SessionStart` and
`PreCompact` hooks that run `capcom prime` into `.codex/hooks.json`:

```console
$ capcom init --codex-hooks
...
added the capcom prime hooks to .codex/hooks.json
Codex runs hooks only with [features] hooks = true in its config.toml
```

As the last line says, Codex ignores its hooks file until hooks are switched on: add this to
Codex's `config.toml`:

```toml
[features]
hooks = true
```

The hook flags are independent and combine with each other and with `--merge-driver`:
`capcom init --claude-hooks --codex-hooks --apogee-hooks` sets up all three agents at once.

## apogee hooks

`capcom init --apogee-hooks` does the same for [apogee](https://github.com/airiclenz/apogee),
adding a reaction that runs `capcom prime` to the repository's `.apogee/config.yaml`:

```console
$ capcom init --apogee-hooks
...
added the capcom prime reaction to .apogee/config.yaml
apogee asks to adopt reaction capcom-prime at the next session start
needs apogee v0.28.0 or later; an older apogee ignores the capcom-prime entry
```

It creates the file when it is missing and otherwise adds one entry under `reactions:`, keeping
everything else in the file as it was:

```yaml
reactions:
  - id: capcom-prime
    on: [session-started, compaction-finished]
    advise: ["capcom", "prime"]
```

An `advise:` reaction is one whose output the model reads. On `session-started` the Briefing lands
ahead of the session's first prompt, and on `compaction-finished` it lands again after every
compaction fold. That is the one difference from Claude Code and Codex: their `PreCompact` hook
runs before the fold, while apogee's `compaction-finished` briefs the model after it. The run
starts as the fold begins, and its answer lands once the fold has replaced the conversation.

**Adopt it once.** apogee treats a reaction in a project's config as proposed until each user
adopts it, and keeps that answer on the user's own machine, never in the repository. Until then the
entry stays inert. apogee offers it in a pane at the next session start; the `/settings` screen and
`apogee project adopt`, run from a shell, answer it too. An edit to the entry asks again.

**Where it runs.** apogee runs `capcom prime` in the Project root. With `confine-to-workspace` on,
it runs inside apogee's workspace box, and on a host that cannot build that box it does not run.
The run has a default `timeout:` of 10 s, and the model reads at most 8 KiB of its output.

**Which apogee.** Briefing on `session-started` and `compaction-finished` needs apogee v0.28.0 or
later. An older apogee ignores the capcom-prime entry, with one start-up line naming it, and loads
the rest of the project config. apogee's retracted 1.x releases (v1.0.0 to v1.8.0) predate it
despite their higher numbers.

Commit `.apogee/config.yaml` so every clone gets the reaction.

## Other agents

An agent without session hooks still reads `AGENTS.md`, and the Managed block tells it which
commands to run. To give it the briefing as well, run `capcom prime` as the first step of the
session yourself, or add a line asking for it to the hand-written part of your `AGENTS.md`.

Agents that parse output should use `--json`, or `--kv` where a command offers it; see
[JSON and KV output](../reference/json-output.md). Two habits help an agent work safely beside
others:

- claim the working tree with a [Tree lock](../concepts/tree-locks.md) before editing it, and
  release it on every exit path;
- run `capcom check` after a hand edit or a merge, and fix what it reports, file and line, in a
  loop.

## Next

- [The session briefing](briefing.md): what `capcom prime` prints and how to read it.
- [The Managed block](managed-block.md): what capcom writes into `AGENTS.md`.
- [Quick start](../getting-started/quickstart.md): a whole session from `capcom init` to close.
