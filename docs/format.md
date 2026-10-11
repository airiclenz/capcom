# capcom file format

Everything capcom keeps for a repository lives in plain files under `.capcom/` at the root of the
working tree, versioned with the code (ADR 0004). The SQLite index and the lock records live under
the git directory instead; they are never committed, and deleting them loses nothing.

```text
.capcom/
  config.yaml        repository settings (optional keys, see Config)
  issues/            the Issue tree
  knowledge/         the Knowledge tree
```

## The tree

The directory tree is the hierarchy (ADR 0001). The same rules hold in `issues/` and
`knowledge/`:

- A node is the markdown file `<id>.md`. Its id is the file name without `.md`; the id is never
  written inside the file.
- A node's children live in the sibling directory `<id>/`, next to `<id>.md`. A directory without
  the node file of the same name is an error.
- Names starting with a dot are ignored, and so are files that do not end in `.md`.

```text
.capcom/issues/
  login-flow.md
  login-flow/
    login-timeout-race.md
    session-refresh-bug.md
  release-checklist.md
```

### Ids

An id is a kebab-case slug of two to four words of `a-z` and `0-9`, joined by single hyphens, at
most 40 characters long (`login-timeout-race`). The author always chooses it; capcom never
generates one, and there is no project prefix (ADR 0002). An id is unique across the whole tree,
whatever directory it sits in.

### Aliases

An **Alias** is a former id that still resolves to its node, kept after a rename or an import so
old citations stay valid. A node has any number of them, listed in its `aliases:` field. An alias
uses only `a-z`, `0-9`, dots and hyphens, the wider set legacy ids need (`apogee-kk0.4`). No id and
no alias may appear twice anywhere in one tree, and a new id may not reuse an existing id or alias.
Renaming a node adds its old id to its aliases: `capcom mv` does so for an issue and
`capcom know mv` for a Note.

### Naming a node

A command argument resolves to a node by, in order: an exact id, an exact alias, then the one id
the argument is a prefix of. A prefix matching several ids is an error that lists them.

## Issue files

An issue file is markdown with a YAML frontmatter between two `---` lines. The first line of the
file is `---`, and the frontmatter ends at the next line that is exactly `---`. Everything after
that line is the body, kept byte for byte, so a body may hold `---` lines of its own (a thematic
break, a fenced YAML example).

```markdown
---
title: Session refresh races the login timeout
status: done
type: feature
priority: 1
labels: [owner-run, invariant]
deps: [token-store-rework, clock-skew-guard]
plan: 2026-10-11 - 00 - capcom-v1-issues-plan
aliases: [apogee-kk0.4]
created: 2026-10-11T09:30:00Z
closed: 2026-10-12T17:05:42Z
reason: "Fixed in 3f2a1c9: refresh now waits on the timeout lock."
---

The refresh path and the timeout path both clear the session.

## Acceptance

- Two concurrent refreshes never log the user out.

## Log

- 2026-10-11 airic: reproduced with two tabs.
```

### Fields

| Key        | Value                                                                       | Required                    |
|------------|-----------------------------------------------------------------------------|-----------------------------|
| `title`    | One line that reads on its own.                                             | yes                         |
| `status`   | A status declared in `config.yaml`.                                         | yes                         |
| `type`     | A type declared in `config.yaml` (`bug`, `feature`, `task`, `chore` by default). | yes                    |
| `priority` | A whole number from 0 (most urgent) to 4. A file without one reads as 2.    | always written              |
| `labels`   | A list of Labels: cross-cutting markers such as `owner-run`.                | no                          |
| `deps`     | A list of the ids of the issues this one depends on.                        | no                          |
| `plan`     | The basename of a Plan document, resolved against the config's `plans` directories. An issue with a plan is Planned. | no |
| `aliases`  | A list of the issue's former ids.                                           | no                          |
| `created`  | When the issue was created: an RFC3339 timestamp.                           | yes                         |
| `closed`   | When the issue was last closed: an RFC3339 timestamp.                       | when Closed                 |
| `reason`   | Why the issue was closed.                                                   | when Closed                 |

An issue is **Closed** when its status is in the `done` or `dropped` category; a Closed issue
needs both `closed` and `reason`. Any other key is an error, including `id` (the id is the file
name) and `updated` (git and the index know when a file changed).

The body is free markdown. By convention it holds an `## Acceptance` section and a `## Log`
section, the dated, append-only record of progress and remarks.

### Canonical form

capcom writes every issue file in one canonical form, and reading a canonical file and writing it
back yields the same bytes, so a write only changes what it means to change:

- the keys in the order of the table above;
- empty fields left out, except `priority`, which is always written (0 is a value, not a gap);
- lists in flow style on one line: `labels: [a, b]`;
- timestamps in RFC3339, UTC, to the second: `2026-10-11T09:30:00Z`;
- text unquoted wherever YAML reads it back unchanged, and in double quotes otherwise (a value
  holding `: `, a leading `#`, a number such as `"123"`, a keyword such as `"true"`, a line break
  written as `\n`), always on one line;
- LF line endings; a file with CRLF line endings reads the same and is written back with LF;
- the body byte for byte.

Hand-written files may use any valid YAML for these keys (block-style lists, quoted text, comments,
another time zone); capcom reads them and writes them back in the canonical form.

`capcom fmt` rewrites every issue file and every Note file whose bytes differ from its canonical
form, lists each one relative to the working tree root, then prints `formatted <n> files`; a second
run changes nothing. A file that does not parse or validate, or that the tree cannot place, is left
untouched and reported as a `warning:` without failing the command; `capcom check` is the
validator. `capcom fmt --check` writes nothing, lists each file that would change, and exits 1, as
`capcom check` does on a problem, when any would. With `--json` the result is kind `fmt` with the
data `{"paths": [...]}`.

### Errors

Every problem with an issue file names the file, and the line when one is to blame, as
`path:line: message`, so an agent can fix files in a loop (ADR 0006). An unknown or repeated key
cites the key's line; a value of the wrong shape cites the value's line; a value the config does not
allow (an undeclared status or type, a priority out of range) cites its key's line; a missing
`closed` or `reason` on a Closed issue cites the `status` line.

A command does not fail on a bad issue file. The index (`<git-dir>/capcom/index.db`) leaves the file
out and every command that reads it reports the file as a warning, after its result, until the file
is fixed; the other issues stay usable. `create` and `mv` fail on a problem with the tree only when it
concerns the issue they write or move, its new path or the parent they resolve; any other is a
warning. `know add`, `know mv` and `know rm` treat the Knowledge tree the same way for the Note
they write, move or remove. Two files that answer to the same id or alias are both left
out, each warning naming the other. The index refreshes itself on every command that reads it,
re-reading only the files whose size or modification time changed; `capcom reindex` rebuilds it
from scratch.

### Merge conflicts in the Log

Two branches that each log on the same issue append to the same place in its `## Log`, so merging
them stops on a git conflict that leaves markers in the file. `capcom check` reports every git
conflict marker in an issue or Note file as a problem with the code `conflict`, at the marker's
line:

- every line that starts with `<<<<<<< `, `||||||| ` or `>>>>>>> `;
- a line that is exactly `=======`, but only between a `<<<<<<< ` line and its `>>>>>>> ` line,
  since a bare `=======` is a setext heading underline.

The frontmatter is always scanned, and markers there are reported beside the frontmatter problem
they cause. Body lines inside a fenced code block (opened by three backticks or three tildes) are
not scanned, so a Note may show a conflict as an example. CRLF files give the same problems.

To resolve a Log conflict:

1. Keep both sides' entries, in date order.
2. Delete the marker lines.
3. Run `capcom check` to confirm no `conflict` problem is left, then finish the merge as usual.

## Note files

A **Note** is one fact about the repository that an agent pulls up on demand: a convention, a
gotcha, why something is the way it is. Notes live in the Knowledge tree under
`.capcom/knowledge/`, which follows the tree rules above: the id is the file name, a Note's
children sit in the sibling directory of the same name, and no id or alias may appear twice in the
Knowledge tree. The Knowledge tree and the Issue tree are checked apart from each other, so a Note
may share an id with an issue.

A Note file is markdown with a YAML frontmatter between two `---` lines, read the same way as an
issue file:

```markdown
---
title: CLI tests run through runCLI and never in parallel
labels: [testing]
refs: [internal/cli/testharness_test.go#func runCLI, docs/adr/0007-tests-use-real-temp-files-sqlite-and-git.md]
aliases: [cli-tests]
---

CLI tests drive every command in process through `runCLI`, in a real temp git repository.
```

| Key       | Value                                                                      | Required |
|-----------|----------------------------------------------------------------------------|----------|
| `title`   | One line that reads on its own.                                            | yes      |
| `labels`  | A list of Labels.                                                          | no       |
| `refs`    | A list of the files, directories and symbols the Note depends on.          | no       |
| `aliases` | A list of the Note's former ids.                                           | no       |

Any other key is an error, the id included. capcom writes a Note with the keys in the order of the
table, empty fields left out, lists in flow style on one line, text unquoted wherever YAML reads it
back unchanged and quoted otherwise, and the body byte for byte. The body is free markdown.

Notes are changed through capcom, never by hand-editing `.capcom/knowledge/`:

- `capcom know add` writes a new Note, at the top of the tree or below `--parent`.
- `capcom know update` changes a Note's title, labels, refs or body; only the flags given change
  anything. The body takes the same edit flags as `capcom update` for an issue, plus `--body-file`
  to replace the whole body.
- `capcom know mv` renames a Note, moves it below another (`--parent`, or `.` for the top of the
  tree), or both. A rename keeps the old id as an alias, and the Note's children move with it.
- `capcom know rm` deletes a Note file together with its aliases. A Note with children is refused;
  move or remove them first.

An **etag** is a hash of a Note file's bytes: `capcom know show` prints it, and `know add`,
`know update` and `know mv` print the new one after a write. `know update`, `know mv` and
`know rm` take `--if-match <etag>` and refuse with a conflict, changing nothing, when the file's
etag differs, so an agent never overwrites a Note that changed since it last read it.

### Refs

A **Ref** is either a path or `path#symbol`:

- the path is relative to the root of the working tree, with forward slashes, and names a file or a
  directory (`docs/format.md`, `internal/lock`);
- an absolute path, or one that climbs out of the working tree (`../other`), is an error, and so is
  a path that leaves the tree through a symbolic link;
- after the first `#` comes a symbol: text the file must contain, found by a plain text search
  (`internal/lock/lock.go#func Open`). It is not parsed as code, so any text works, and a symbol
  needs a file, not a directory.

A Ref that no longer resolves leaves the Note usable. `capcom know verify` lists every such Ref as
`path:line: ref "<ref>": <why>`, citing the Ref's line in the Note file, and fails when it finds
one; `capcom check` reports them too.

## Config

`.capcom/config.yaml` is optional, and so is every key in it: a missing key takes the value shown
below, which is the file `capcom init` writes. An unknown key or a bad value is an error citing its
line.

```yaml
# The store format version; left as a comment, since a missing key means format 1.
# format: 1

# Issue statuses in order, each mapped to its category: todo, doing, waiting, done or dropped.
# capcom's rules read only the category. The first status listed in a category is its default.
statuses:
  open: todo
  active: doing
  parked: waiting
  done: done
  dropped: dropped

# Issue types.
types: [bug, feature, task, chore]

# Directories, relative to the repository root, that an issue's plan: field is resolved against.
plans: [docs/plans, docs/plans/archived]

ready:
  # Issues carrying any of these labels are left out of ready unless asked for.
  hide: []

lock:
  # A Tree lock whose heartbeat is older than this is stale.
  ttl: 2h

elsewhere:
  # Branches whose tip is older than this many days are not scanned.
  days: 30
```

- **format** — the store format version, a whole number of 1 or more; a missing key means 1
  ([ADR 0009](adr/0009-the-store-declares-its-format-version.md)). It is read before every other
  key: a store whose format is newer than the binary reads fails every command with
  `.capcom/config.yaml:<line>: store format <n> is newer than this capcom reads (up to <max>); upgrade capcom`,
  except `capcom prime`, which reports it as one `warning: store:` line and exits 0. Unknown keys
  stay errors, and any new key in an issue, a Note or this file raises the format. `capcom init`
  writes the key only as a comment while the format is 1, because v0.1.0 rejects it as an unknown key.
- **statuses** — an ordered mapping of status name to Status category. Every rule in capcom reads
  the category, never the name (ADR 0003). At least one status must be in `todo` and one in
  `done`. When capcom moves an issue into a category (create, close, reopen) it uses the first
  status listed in that category.
- **types** — the issue types; at least one, no duplicates.
- **plans** — the directories a `plan:` basename is looked up in, in order.
- **ready.hide** — labels whose issues `ready` leaves out by default.
- **lock.ttl** — a positive duration (`2h`, `90m`).
- **elsewhere.days** — a whole number of days, 0 or more. The Elsewhere scan reads local and
  remote-tracking branches whose tip is newer than this. capcom never fetches, so a
  remote-tracking branch is as fresh as the last `git fetch`.

## The lock record

A Tree lock belongs to one worktree (ADR 0005). Its record is a JSON file under the git common
dir, shared by every worktree of the clone and never committed:

```text
<git-common-dir>/capcom/locks/
  _main.json         the primary worktree's lock
  _main.lck          the guard file of the record next to it
  wt-<name>.json     the lock of the linked worktree named <name>
```

A worktree's key is `_main` for the primary worktree and `wt-` plus the linked worktree's name
otherwise (the last element of its git dir, `<git-common-dir>/worktrees/<name>`). A free lock has
no record file.

```json
{
  "token": "ff6d642521c9bff460590d1469040ae1",
  "purpose": "implementing the release plan",
  "holder": "airic@devbox",
  "host": "devbox",
  "user": "airic",
  "branch": "main",
  "since": "2026-10-11T14:08:52.330264702Z",
  "heartbeat": "2026-10-11T15:02:10.118520331Z"
}
```

- **token** — the secret `lock acquire` hands the holder: 128 random bits in hex. The holder
  presents it to `lock touch`, `lock release` and a re-acquire; capcom never shows it to anyone
  else.
- **purpose** — what the holder is doing, from `--purpose`.
- **holder** — who holds the lock, from `--holder`; `user@host` by default.
- **host**, **user**, **branch** — where the lock was acquired: the machine, the user and the
  branch checked out.
- **since** — when the lock was acquired; resuming it with the token keeps this time.
- **heartbeat** — when the lock was last acquired, resumed or touched.

Both times are RFC3339 in UTC. A lock whose heartbeat is older than `lock.ttl` in `config.yaml`
(2 h by default) is **stale**: it is still held, but another session may take it over with
`lock acquire --take-stale`, and `lock break` frees it whoever holds it. The lock is advisory;
capcom installs no git hooks, and `lock check` lets a repository opt in to a check of its own.
Every change to a record runs under an OS file lock on its `.lck` file, held only for that read and
write, and the record is replaced atomically.

## The Managed block in AGENTS.md

`AGENTS.md` at the root of the working tree stays a hand-written agent guide. capcom owns only the
**Managed block**: the lines from `<!-- BEGIN CAPCOM -->` to `<!-- END CAPCOM -->`, each marker on a
line of its own. The block holds a short paragraph naming capcom as the issue register of record,
the handful of commands an agent needs, and the top level of the Knowledge tree, one
`` - `<id>`: <title> `` line per top-level Note in id order. It is rendered from the Note files
alone, so the same tree always yields the same block.

- `capcom agents` writes the block: it replaces the lines between the markers, or appends the
  block to the end of the file when it has none, and creates `AGENTS.md` when there is none. Every
  byte outside the markers is kept, and so are the file's line endings (LF or CRLF).
- `capcom agents --check` fails when the block is out of date, and `capcom check` reports an
  out-of-date block as an `agents-block` problem at the begin marker's line. A file without the
  block is neither: the block is opt-in.
- A Note write that changes the top level of the Knowledge tree refreshes the block when
  `AGENTS.md` already holds one and it went out of date: a top-level `capcom know add`, a
  `capcom know update` of a top-level Note's title, a `capcom know mv` into, out of or within the
  top level, and a `capcom know rm` of a top-level Note. It never adds a block to a file without
  one. A write below the top level changes nothing, since the block lists only the top level.
- A marker that appears twice, an end marker before the begin marker, or a begin marker that is
  never closed is an error at that marker's line, for `agents` and `check` alike.

Edits inside the markers are overwritten on the next refresh; write anything else outside them.
