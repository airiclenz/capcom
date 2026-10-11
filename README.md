# capcom

**Mission control for a repository and the coding agents working in it.**

In spaceflight, CAPCOM is the one voice on the ground that talks to the crew. `capcom` plays that
role for a code repository: it keeps the issue list, says who is currently working on the tree,
and answers the agents' questions about the project, all from plain files that live in the repo
itself.

> **Status: v1, pre-release.** Every command below works; there are no tagged releases yet, so
> build it from source.

## Why

Issue trackers built for agents tend to keep their data in two places: a database and a file
exported from it for git. The two drift apart, and syncing them becomes a chore of its own.
capcom keeps a single source of truth, and that truth is files under git.

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
  and symbols it talks about, and `capcom know verify` flags the ones that have gone.
- **Changes on other branches.** `ready`, `list`, `show` and `prime` warn when an issue was
  changed on another local branch that is not merged yet; capcom never merges them itself.
- **A briefing at session start.** `capcom prime` prints the ready work, the issues in progress,
  the lock holders, unmerged changes elsewhere and the health of the store. capcom also keeps a
  short block in `AGENTS.md` up to date, between `<!-- BEGIN CAPCOM -->` and `<!-- END CAPCOM -->`,
  and leaves the rest of the file alone.
- **One validator.** `capcom check` validates every config, issue and Note file and lists each
  problem as `path:line: message`, so an agent can fix files in a loop.

Every command prints plain text by default and a versioned JSON envelope with `--json`; `show`,
`close`, `lock` and the commands that write an issue also print `KEY=VALUE` lines with `--kv`.
Exit codes: 0 ok, 1 error, 2 usage, 3 not found or locked. capcom runs on Linux, macOS and Windows.

## Install

capcom needs Go 1.24 or later and runs inside a git repository.

```sh
go install github.com/airiclenz/capcom/cmd/capcom@latest
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

## License

MIT, see [LICENSE](LICENSE).
