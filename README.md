# **capcom** - a git-native issue tracker and knowledge base for AI coding agents

[![Release](https://img.shields.io/github/v/release/airiclenz/capcom)](https://github.com/airiclenz/capcom/releases/latest)
[![CI](https://github.com/airiclenz/capcom/actions/workflows/ci.yml/badge.svg)](https://github.com/airiclenz/capcom/actions/workflows/ci.yml)
[![Go version](https://img.shields.io/github/go-mod/go-version/airiclenz/capcom)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Mission control for your repository and the AI coding agents working in it.**

capcom is a command-line issue tracker, task manager and project knowledge base for AI coding
agents such as Claude Code, OpenAI Codex, Cursor, Gemini CLI, Aider and GitHub Copilot, and for
the people working beside them. Issues and notes are plain markdown files in your git repository.
No server, no account, no database to sync. One binary for Linux, macOS and Windows, written in
Go, with a command line for agents and a terminal UI for people.

**Contents:** [Why](#why) · [Features](#features) · [Terminal UI](#terminal-ui) ·
[Works with](#works-with-any-coding-agent) · [Install](#install) · [Quick start](#quick-start) ·
[Commands](#commands) · [Non-goals](#non-goals) · [FAQ](#faq) · [License](#license)

## Why

Most agent issue trackers keep a database and export a copy to git. The two drift apart, and
keeping them in sync becomes a chore. capcom keeps one source of truth: files in git. Issues
branch, merge and show up in code review, just like the code they describe.

## Features

- **Issues as markdown files.** One file per issue under `.capcom/issues/`, with a short YAML
  header for status, priority, labels and dependencies. Readable ids you choose
  (`login-timeout-race`). Folders make the hierarchy.
- **Ready-work queue.** `capcom ready` lists the open, unblocked issues, most urgent first.
- **Dependencies and links.** Make an issue wait for another, or just link related issues.
- **Fast local index.** A SQLite cache under `.git/` answers queries quickly and rebuilds
  itself. It is never committed and never drifts.
- **Your own statuses.** Name statuses as you like; each maps to `todo`, `doing`, `waiting`,
  `done` or `dropped`.
- **Tree lock for parallel agents.** An agent claims the working tree while it edits. Others see
  who holds it, why and since when. One lock per git worktree.
- **Project knowledge on demand.** Conventions, gotchas and design decisions live as small
  Notes under `.capcom/knowledge/`. Agents read only what they need instead of one huge
  instructions file. `capcom know verify` flags Notes whose files or symbols are gone.
- **Session briefing.** `capcom prime` shows ready work, work in progress, locks, unmerged
  changes on other branches and store health.
- **Validator and formatter.** `capcom check` reports problems as `path:line: message`;
  `capcom fmt` keeps files in canonical form. Both work in CI.
- **Machine-readable output.** Every command prints plain text, or JSON with `--json`. Many also
  take `--kv` for `KEY=VALUE` lines.

## Terminal UI

`capcom ui` opens a full-screen terminal UI to browse and update issues and Notes without
leaving the terminal.

- **Three tabs:** Ready, Issues (a collapsible tree) and Knowledge.
- **Detail pane** with rendered markdown and clickable issue and Note links.
- **Filter** with `/`: words, `status:<name>` and `label:<name>`.
- **Quick edits:** claim (`a`), change status (`s`), close (`c`), reopen (`R`), add a log line
  (`L`) or open the file in your editor (`e`).
- **Dependency view** (`d`): what an issue waits on and what it blocks.
- **Mouse support:** click tabs, rows and links, drag the divider to resize, drag over text to
  copy it to the clipboard.
- **Live refresh:** changes made by agents or other tools show up within a second.
- **Dark and light themes,** picked from your terminal, with custom colours.
- **Tree lock badge** in the status bar shows whether another session is editing.

Full guide: [docs/site/getting-started/terminal-ui.md](docs/site/getting-started/terminal-ui.md).

## Works with any coding agent

capcom is a plain CLI, so any agent that can run a shell command can use it.

- **`AGENTS.md`.** `capcom agents` writes a short how-to block into `AGENTS.md` and links it
  from `CLAUDE.md` and `GEMINI.md` when they exist.
- **Claude Code and Codex hooks.** `capcom init --claude-hooks` or `--codex-hooks` runs
  `capcom prime` at session start, so every session begins with a briefing.
- **Stable output.** `--json` and `--kv` are easy to parse; `capcom check` errors are easy to fix
  in a loop.

## Install

Homebrew (macOS, Linux):

```sh
brew install airiclenz/tap/capcom
```

Scoop (Windows):

```sh
scoop bucket add airiclenz https://github.com/airiclenz/scoop-bucket
scoop install capcom
```

winget (Windows):

```sh
winget install AiricLenz.Capcom
```

From source (Go 1.25 or later):

```sh
go install github.com/airiclenz/capcom/cmd/capcom@latest
```

Prebuilt archives for Linux, macOS and Windows (amd64, arm64) are on the
[releases page](https://github.com/airiclenz/capcom/releases).

## Quick start

```sh
cd your-repo
capcom init                     # create .capcom/ in this git repository
capcom agents                   # tell your agents about capcom in AGENTS.md
capcom create --id first-task --title "Try capcom"
capcom ready                    # what to work on next
capcom ui                       # browse it all in the terminal UI
git add .capcom AGENTS.md       # plus CLAUDE.md or GEMINI.md if capcom agents changed them
git commit -m "chore: track issues with capcom"
```

## Commands

```sh
# Issues
capcom create --id login-timeout-race --title "Login times out during refresh" --type bug
capcom ready                    # open, unblocked issues, most urgent first
capcom list --tree              # the issue outline
capcom show login-timeout-race
capcom search refresh timeout
capcom update login-timeout-race --priority 1 --status active
capcom log login-timeout-race "reproduced with two tabs"
capcom dep add login-timeout-race token-store-rework   # wait for another issue
capcom related add login-timeout-race session-bug      # link without blocking
capcom mv login-timeout-race --parent login-flow       # the old id stays an alias
capcom close login-timeout-race --reason "Fixed in 3f2a1c9"
capcom reopen login-timeout-race --reason "Came back on Safari"

# Tree lock
capcom lock acquire --purpose "implementing the release plan"   # prints a token
capcom lock touch --token <token>
capcom lock release --token <token>
capcom lock status --all

# Knowledge
capcom know add --id release-steps --title "How a release is cut" --ref docs/release.md
capcom know tree
capcom know show release-steps
capcom know search release
capcom know verify              # Notes whose files or symbols are gone

# Store and agents
capcom ui                       # the terminal UI
capcom prime                    # the session-start briefing
capcom agents                   # write or refresh the capcom block of AGENTS.md
capcom check                    # validate every file
capcom fmt                      # rewrite files in canonical form
```

`capcom <command> --help` lists every flag. The full manual is at
[airiclenz.com/capcom](https://airiclenz.com/capcom/) (source: [docs/site/](docs/site/)), with the
[file format reference](docs/site/reference/file-format.md) and a guide to
[migrating from beads](docs/site/getting-started/migrating-from-beads.md).

Want a shorter command? Use a shell alias such as `alias cc=capcom`, not a binary named `cc`:
that name belongs to the system C compiler.

## Non-goals

capcom stays one binary over files in git. These would bring back a second store, a background
process or a sync chore, so they will not be added:

- **A server or hosting.** The repository is the only store; git already handles many writers.
- **A daemon or background sync.** capcom runs only while a command runs. It never fetches,
  pushes or merges.
- **Notifications.** capcom sends nothing; `capcom prime` shows what changed when asked.

## FAQ

**Does capcom need a server, an account or a database?**
No. Issues and Notes are files in your repository. The SQLite index is a local cache under the
git directory, never committed.

**How do a team and several agents share issues?**
Through git: push, pull and merge. Each issue is its own file, so edits to different issues
merge cleanly. Run `capcom check` after a merge
([branches and merging](docs/site/concepts/branches-and-merging.md)).

**Can two agents work in the same repository at once?**
Yes. The Tree lock shows each session whether another one is editing, and each git worktree has
its own lock, so agents can work side by side in separate worktrees.

**What if two branches add log entries to the same issue?**
`capcom init --merge-driver` sets up a git merge driver that merges the `## Log` by entry, so
there is no conflict to resolve by hand
([details](docs/site/reference/file-format.md#merge-conflicts-in-the-log)).

**Can CI check the issue files?**
Yes. `capcom check` and `capcom fmt --check` fail on problems and write nothing. A minimal
GitHub Actions job:

```yaml
name: capcom
on: [push, pull_request]
permissions:
  contents: read
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - run: go install github.com/airiclenz/capcom/cmd/capcom@latest
      - run: capcom check
      - run: capcom fmt --check
```

**How is it different from beads?**
beads keeps a database and exports it to git; capcom keeps only the files, so there is nothing
to sync. See [migrating from beads](docs/site/getting-started/migrating-from-beads.md).

**Is it only for AI agents?**
No. The files are readable markdown, and the terminal UI and plain-text output are made for
people too.

## License

MIT, see [LICENSE](LICENSE).
