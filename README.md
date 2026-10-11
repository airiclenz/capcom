# capcom: a git-native issue tracker and knowledge base for AI coding agents

[![Release](https://img.shields.io/github/v/release/airiclenz/capcom)](https://github.com/airiclenz/capcom/releases/latest)
[![CI](https://github.com/airiclenz/capcom/actions/workflows/ci.yml/badge.svg)](https://github.com/airiclenz/capcom/actions/workflows/ci.yml)
[![Go version](https://img.shields.io/github/go-mod/go-version/airiclenz/capcom)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**Mission control for a repository and the coding agents working in it.**

capcom is a command-line issue tracker, task manager and project knowledge base built for AI
coding agents such as Claude Code, OpenAI Codex, Cursor, Gemini CLI and Aider, and for the people
working beside them. Issues and knowledge notes are plain markdown files under `.capcom/`,
versioned in git with your code. There is no server, no account and no database to sync: one
binary for Linux, macOS and Windows, written in Go.

In spaceflight, CAPCOM is the one voice on the ground that talks to the crew. `capcom` plays that
role for a code repository: it keeps the issue list, says who is currently working on the tree,
and answers the agents' questions about the project, all from plain files that live in the repo
itself.

**Contents:** [Why](#why) · [Features](#what-it-does) · [Works with](#works-with-any-coding-agent) ·
[Install](#install) · [Quick start](#quick-start) · [Commands](#usage) · [FAQ](#faq) ·
[License](#license)

## Why

Issue trackers built for agents tend to keep their data in two places: a database and a file
exported from it for git. The two drift apart, and syncing them becomes a chore of its own.
capcom keeps a single source of truth, and that truth is files under git: issues branch, merge
and show up in code review like the code they describe.

## What it does

- **Issues as files.** One markdown file per issue under `.capcom/issues/`, with a short YAML
  header for status, priority, labels and dependencies. Git does the syncing, the history and the
  merging, and the issues travel with the branch they were changed on. Ids are readable slugs
  you choose (`login-timeout-race`); the directory tree is the hierarchy, so a child issue is a
  file in its parent's directory. Renaming one (`capcom mv`) is a plain file rename that keeps the
  old id as an alias and rewrites the dependencies that named it.
- **A fast index that can't drift.** A local SQLite index under the git directory is built from
  those files for quick queries ("what's ready to work on?") and refreshes itself on every
  command. It is a cache: delete it and capcom rebuilds it.
- **Configurable statuses.** Each status maps to one of five categories (`todo`, `doing`,
  `waiting`, `done`, `dropped`), and capcom's rules read only the category, so a repository can
  call its statuses whatever it likes.
- **A Tree lock.** One agent session claims the working tree while it edits ("implementing plan
  X") and gets a token; every other agent can see who holds it, why and since when. A heartbeat
  keeps it live, a lock whose heartbeat is older than the TTL is stale, and each worktree has a
  lock of its own. The lock is advisory: capcom installs no git hooks.
- **Project knowledge on demand.** The facts agents need about a repo (conventions, gotchas, why
  something is the way it is) live as small Notes under `.capcom/knowledge/`, so an agent pulls up
  just the part it needs instead of reading one huge instructions file. Each Note names the files
  and symbols it talks about, and `capcom know verify` flags the ones that have gone. Notes are
  changed with `capcom know update`, renamed or moved with `capcom know mv` (the old id stays an
  alias, as with `capcom mv`) and deleted with `capcom know rm`.
- **Changes on other branches.** `ready`, `list`, `show` and `prime` warn when an issue was
  changed on another local branch that is not merged yet; capcom never merges them itself.
- **A briefing at session start.** `capcom prime` prints the ready work, the issues in progress,
  the lock holders, unmerged changes elsewhere and the health of the store. capcom also keeps a
  short block in `AGENTS.md` up to date, between `<!-- BEGIN CAPCOM -->` and `<!-- END CAPCOM -->`,
  and leaves the rest of the file alone.
- **One validator.** `capcom check` validates every config, issue and Note file and lists each
  problem as `path:line: message`, so an agent can fix files in a loop.

Every command prints plain text by default and a versioned JSON envelope with `--json`. `show`,
`close`, `reopen`, every `lock` subcommand, `version` and the commands that write an issue
(`create`, `update`, `log`, `mv`, `dep add`, `dep rm`) also take `--kv`, after the command name,
and print `KEY=VALUE` lines ending in `STATUS=`; the other commands do not offer it.
Exit codes: 0 ok, 1 error, 2 usage, 3 not found or locked. capcom runs on Linux, macOS and Windows.

## Works with any coding agent

capcom is a plain CLI, so any agent that can run a shell command can use it: Claude Code, OpenAI
Codex, Cursor, Gemini CLI, Aider, GitHub Copilot's agent mode or your own scripts. Three things
make that easy:

- **`AGENTS.md`.** `capcom agents` writes a short block telling agents how to use capcom into
  `AGENTS.md`, the instructions file most coding agents read on start-up.
- **Claude Code hooks.** `capcom init --claude-hooks` runs `capcom prime` from Claude Code's
  session hooks, so every session starts with a briefing.
- **Machine-readable output.** `--json` and `--kv` give agents stable output to parse, and
  `capcom check` gives them `path:line: message` errors to fix in a loop.

## Install

capcom is a single binary and runs inside a git repository.

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

From source, with Go 1.24 or later:

```sh
go install github.com/airiclenz/capcom/cmd/capcom@latest
```

Prebuilt archives for Linux, macOS and Windows on amd64 and arm64 are attached to each
[GitHub release](https://github.com/airiclenz/capcom/releases). How a release is cut is described
in [docs/releasing.md](docs/releasing.md).

## Quick start

```sh
cd your-repo
capcom init                     # create .capcom/ in this git repository
capcom agents                   # tell your agents about capcom in AGENTS.md
capcom create --id first-task --title "Try capcom"
capcom ready                    # what to work on next
git add .capcom AGENTS.md && git commit -m "chore: track issues with capcom"
```

## Usage

```sh
capcom init                     # create .capcom/ in this git repository
capcom init --claude-hooks      # ...and run capcom prime from Claude Code's session hooks

capcom create --id login-timeout-race --title "Session refresh races the login timeout" --type bug
capcom ready                    # issues that are open and unblocked, most urgent first
capcom list --label owner-run   # open issues, filtered
capcom show login-timeout-race
capcom search refresh timeout
capcom update login-timeout-race --priority 1 --add-label owner-run
capcom log login-timeout-race "reproduced with two tabs"
capcom dep add login-timeout-race token-store-rework
capcom mv login-timeout-race --parent login-flow
capcom close login-timeout-race --reason "Fixed in 3f2a1c9"
capcom reopen login-timeout-race --reason "Came back on Safari"

capcom lock acquire --purpose "implementing the release plan"   # prints the token to keep
capcom lock touch --token <token>
capcom lock status --all
capcom lock release --token <token>

capcom know add --id release-steps --title "How a release is cut" --ref docs/release.md
capcom know tree
capcom know search release
capcom know update release-steps --add-label release --if-match <etag>   # etag from know show
capcom know mv release-steps cutting-a-release   # the old id stays an alias
capcom know rm cutting-a-release                 # refuses a Note with children
capcom know verify              # Note refs that no longer resolve

capcom agents                   # write or refresh the capcom block of AGENTS.md
capcom check                    # validate the whole store
capcom prime                    # the session-start briefing
capcom reindex                  # rebuild the index from scratch
```

`capcom <command> --help` lists every flag. The file formats are described in
[docs/format.md](docs/format.md); moving a repository off beads is described in
[docs/migrating-from-beads.md](docs/migrating-from-beads.md).

The binary is `capcom`. If you want a shorter command, a **shell alias** such as
`alias cc=capcom` works well. Don't install a binary or symlink named `cc`, though: that name
belongs to the system C compiler, and build tools call it.

## FAQ

**Does capcom need a server, an account or a database?**
No. The issues and Notes are files in your repository. The SQLite index is a local cache under
the git directory, never committed, and rebuilt on demand.

**How does a team or several agents share issues?**
Through git, like the code: push, pull and merge. Each issue is its own file, so changes to
different issues merge without conflicts, and `capcom check` validates the result after a merge.

**Can two agents work in the same repository at once?**
Yes. The Tree lock tells each agent session whether another one is editing the working tree, and
every git worktree has a lock of its own, so agents can work in parallel worktrees.

**How is it different from beads?**
beads keeps a database and exports it to a file for git; capcom keeps only the files, so there is
nothing to sync. [docs/migrating-from-beads.md](docs/migrating-from-beads.md) moves a repository
over.

**Is it only for AI agents?**
No. The files are readable markdown and every command prints plain text, so people can use
capcom on its own or alongside their agents.

## License

MIT, see [LICENSE](LICENSE).
