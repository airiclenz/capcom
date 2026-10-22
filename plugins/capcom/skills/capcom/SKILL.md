---
name: capcom
description: Track work and project knowledge with capcom, the git-native issue tracker whose issues and Notes are markdown files under .capcom/. Use when the repository has a .capcom/ directory, when the user mentions capcom, issues, tickets, the backlog or what to work on next, or asks to set up issue tracking or shared memory for coding agents in a git repository.
license: MIT
---

# capcom

capcom keeps Issues and Knowledge Notes as markdown files under `.capcom/`, versioned with the
code. A disposable SQLite index under the git directory is rebuilt as needed and never committed.
Manual: https://airiclenz.com/capcom/

## 1. Check the binary

Run `capcom version`. If the command is not found, install it, asking the user first:

- macOS or Linux: `brew install airiclenz/tap/capcom`
- Windows: `scoop bucket add airiclenz https://github.com/airiclenz/scoop-bucket` then
  `scoop install capcom`, or `winget install AiricLenz.Capcom`
- With Go: `go install github.com/airiclenz/capcom/cmd/capcom@latest`
- Otherwise a release archive: https://github.com/airiclenz/capcom/releases

## 2. Set up a repository

Only when the user asks for capcom in a repository that has no `.capcom/` yet. From the
repository root:

1. `capcom init` creates `.capcom/` with `issues/`, `knowledge/` and `config.yaml`.
2. `capcom agents` writes the capcom block into `AGENTS.md` and links it from `CLAUDE.md` and
   `GEMINI.md` when they exist, so every agent learns the commands.
3. Optional: `capcom init --claude-hooks` adds the `capcom prime` hooks to
   `.claude/settings.json`, so collaborators without this plugin get the Briefing too. This
   plugin's own hook then stands down, and the Briefing never prints twice.
4. Commit `.capcom/`, `AGENTS.md` and every file the commands changed.

## 3. Work loop

This plugin runs `capcom prime` at session start in any repository with a `.capcom/`: it lists the
Ready Issues, the work in progress and who holds the Tree locks. Then:

- `capcom next`: the most urgent Ready Issue in full. `capcom ready` lists all of them.
- `capcom show <id>`: one Issue in full, with what blocks it.
- `capcom start <id>`: claim the Issue and take the Tree lock before starting; keep the Lock
  token it prints. `capcom start <id> --token <token>` resumes the lock after a restart.
- `capcom log <id> <text>`: note progress as work lands.
- `capcom finish <id> --reason <text> --token <token>`: close it and free the Tree lock; the output
  names the Issues this made Ready and the one to work on next.
- `capcom create --id <slug> --title <title> [--dep <id>]`: file new work. The body says what is
  wrong or wanted, why, and how to tell it is done, under `## Acceptance`.
- `capcom know tree`, then `capcom know show <id>`: read project Knowledge before starting.

When `capcom start --help` fails, the binary predates these two commands: claim with
`capcom update <id> --status active` plus `capcom lock acquire --purpose <id>`, and close with
`capcom close <id> --reason <text>` plus `capcom lock release --token <token>`.

A standing Issue only groups its children: never claim or close it. Every command takes `--json`,
and `capcom <cmd> --help` lists its flags. After editing a file under `.capcom/` by hand, run
`capcom check`.

## 4. Tree lock

`capcom start` and `capcom finish` take and free the lock with an Issue. For a change that spans
several files or commits outside an Issue, claim the worktree with
`capcom lock acquire --purpose <text>` and keep the token it prints. Renew it with
`capcom lock touch --token <token>` and free it with `capcom lock release --token <token>`. When
`capcom lock check` exits 3, another session holds this worktree: ask the user before editing.

The capcom block in `AGENTS.md` is the repository's own word on these commands; when it differs
from this skill, follow `AGENTS.md`.
