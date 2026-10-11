# capcom

**Mission control for a repository and the coding agents working in it.**

In spaceflight, CAPCOM is the one voice on the ground that talks to the crew. `capcom` plays that
role for a code repository: it keeps the issue list, says who is currently working on the tree,
and answers the agents' questions about the project, all from plain files that live in the repo
itself.

> **Status: design phase.** There is no code and nothing to install yet. This README describes
> what capcom is meant to become.

## Why

Issue trackers built for agents tend to keep their data in two places: a database and a file
exported from it for git. The two drift apart, and syncing them becomes a chore of its own.
capcom keeps a single source of truth, and that truth is files under git.

## What it will do

- **Issues as files.** One markdown file per issue, with a short header for status, labels and
  dependencies. Git does the syncing, the history and the merging. Ids are readable slugs
  (`login-timeout-race`). Renaming one (`capcom mv`) is a plain file rename that keeps the old
  id as an alias and updates the dependencies that named it.
- **A fast index that can't drift.** A local SQLite index is built from those files for quick
  queries ("what's ready to work on?"). It is a cache: delete it and capcom rebuilds it.
- **A repo lock.** One agent can claim the working tree while it edits ("implementing plan X"),
  and every other agent can see who holds it, why, and since when. A lock left by a crashed agent
  is detected as stale and can be cleared.
- **Project knowledge on demand.** The facts agents need about a repo (conventions, gotchas,
  why something is the way it is) live as small, tagged entries, so an agent pulls up just the
  part it needs instead of reading one huge instructions file. Each entry names the files it
  talks about, and capcom flags entries whose files have gone.
- **A briefing at session start.** One command prints the ready work, the current lock holder
  and the key facts. That's what an agent sees first.

Every command will also offer JSON output, so agents and scripts can drive it as easily as people.

## Usage (planned)

```sh
capcom ready                    # issues that are open and unblocked
capcom show login-timeout-race
capcom lock acquire "implementing the release plan"
capcom know search release
capcom prime                    # the session-start briefing
```

The binary is `capcom`. If you want a shorter command, a **shell alias** such as
`alias cc=capcom` works well. Don't install a binary or symlink named `cc`, though: that name
belongs to the system C compiler, and build tools call it.

## License

MIT, see [LICENSE](LICENSE).
