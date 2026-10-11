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

### Errors

Every problem with an issue file names the file, and the line when one is to blame, as
`path:line: message`, so an agent can fix files in a loop (ADR 0006). An unknown or repeated key
cites the key's line; a value of the wrong shape cites the value's line; a value the config does not
allow (an undeclared status or type, a priority out of range) cites its key's line; a missing
`closed` or `reason` on a Closed issue cites the `status` line.

## Config

`.capcom/config.yaml` is optional, and so is every key in it: a missing key takes the value shown
below, which is the file `capcom init` writes. An unknown key or a bad value is an error citing its
line.

```yaml
# Issue statuses in order, each mapped to its category: todo, doing, waiting, done or dropped.
# capcom's rules read only the category. The first status listed in a category is its default.
statuses:
  open: todo
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

- **statuses** — an ordered mapping of status name to Status category. Every rule in capcom reads
  the category, never the name (ADR 0003). At least one status must be in `todo` and one in
  `done`. When capcom moves an issue into a category (create, close, reopen) it uses the first
  status listed in that category.
- **types** — the issue types; at least one, no duplicates.
- **plans** — the directories a `plan:` basename is looked up in, in order.
- **ready.hide** — labels whose issues `ready` leaves out by default.
- **lock.ttl** — a positive duration (`2h`, `90m`).
- **elsewhere.days** — a whole number of days, 0 or more.
