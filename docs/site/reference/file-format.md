---
title: "File format"
description: "The files capcom keeps under .capcom/: issue files, Note files, the tree, config.yaml, the lock record, the Managed block, and the per-user config of capcom ui, the one file it keeps outside the repository."
sidebar: { order: 2 }
---

Everything capcom keeps for a repository lives in plain files under `.capcom/` at the root of the
working tree, versioned with the code (ADR 0004). The SQLite index and the lock records live under
the git directory instead; they are never committed, and deleting them loses nothing. One file
lives outside the repository altogether: the per-user config of `capcom ui`, in your user config
directory (see The user config).

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
- An entry that is neither a regular file nor a directory (a symlink, a named pipe, a device) is
  never read: it is left out with a `not a regular file` warning, whatever its name. A node file
  larger than 1 MiB is left out with a `file is larger than 1 MiB` warning. Both are `not-regular`
  problems in `capcom check`, and a directory beside such a node file is left out with it.
- Every node file of an id two or more files share is left out, each with a `duplicate id` problem
  naming another file of the id: `duplicate id "<id>": also the id of <file>; both are left out`,
  or `also the id of <file> and <n> more; all are left out` for three or more. A file that does not
  parse counts as a file of its id too. A directory beside any of them is left out with its whole
  subtree, as a `directory left out: its node file <id>.md has a duplicate id` problem.

```text
.capcom/issues/
  login-flow.md
  login-flow/
    login-timeout-race.md
    session-refresh-bug.md
  release-checklist.md
```

This repository's own Knowledge tree goes one level deeper. The Note
`glossary/glossary-knowledge.md` has its children, one Note per term, in
`glossary/glossary-knowledge/` (`note.md`, `ref.md`, ...), and
`decisions/adr-0001/adr-0001-rejected.md` is a child of the Note `adr-0001`, which is itself a
child of `decisions`. A child id that repeats
its parent's id, as `adr-0001-rejected` does, is a naming habit, not a rule: it keeps ids unique
across the tree and tells a reader where the Note belongs, but capcom never requires it.

### Ids

An id is a kebab-case slug of one to four words of `a-z` and `0-9`, joined by single hyphens, at
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
the argument is a prefix of. A prefix matching several ids is an error that lists them. Every
node file's id counts as a prefix candidate, that of a file with problems or of a file left out as
a duplicate included, and a prefix matching only such an id fails as that id does. A name
two files claim, an alias both list, an alias that is the other's id, or a node's name that a file
with problems or a file left out also claims, is an error naming both files, checked before any of
these; both nodes still resolve by their other names. An
argument that is exactly the id or an alias of a node left out as a duplicate, or of a node file
under the directory of a duplicate id, is an error naming the two files that clash, and one that is exactly a name claimed by a file with problems, a file
left out unread (not a regular file, or over 1 MiB) or a node file under a directory left out (its
id or an alias) is not found; neither is matched as a prefix. The exact id of a file whose
frontmatter does not parse is the one exception: `capcom rm` and `capcom know rm` still remove that
file. Before any of these, the argument is trimmed, lowercased and
each run of whitespace becomes one hyphen, so a citation spelling resolves as typed (`"ADR 0076"`
names `adr-0076`); dots and hyphens are kept, and errors quote the argument as typed.

## Issue files

An issue file is markdown with a YAML frontmatter between two `---` lines. The first line of the
file is `---`, and the frontmatter ends at the next line that is `---` with nothing else on it
but a line ending: a CRLF ending counts, and so does a lone carriage return after a closing `---`
that ends the file, while a space or any other text makes the line no fence. Everything after
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
| `related`  | A list of the ids of the issues this one is related to, without depending on them (format 2). | no      |
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

`capcom rm` deletes an issue file together with its aliases. It refuses, changing nothing, an
issue with children, an issue whose children directory still holds any file, and an issue other
issues depend on (their `deps` name its id or an alias), listing those issues; `--drop-deps`
removes their `deps` entries for it instead. A `deps` entry of the issue on itself does not count.
Like `update` and `mv`, it takes `--if-match <etag>` and refuses with a conflict, changing
nothing, when the file's etag differs. `rm` and `mv` also refuse with a conflict, taking back what
they changed, when an issue file they rewrite changes while they run. Other issues' `related` entries naming the issue never
refuse it: they are always removed, and those issues are reported as `related_dropped`. capcom
deletes the file itself and never runs git.

### Related links

A **Related link** ties two issues together without either depending on the other: it never
affects `ready`, `dep` or blocking. `capcom related add <a> <b>` writes the canonical id of `b`
into the `related` list of `a`, and only there; the other issue's file is not touched. Linking an
issue to itself is refused, and a link already held by either issue changes nothing. In a store
below format 2, any write that leaves a `related` line in the issue file raises `config.yaml` to
`format: 2` (see Config) and warns: the first link, and also an `update`, `close`, `reopen`, `log`
or `dep` of an issue that already holds one:

```text
warning: .capcom/config.yaml: store format raised to 2; capcom releases that read only format 1 can no longer read this store
```

`capcom related rm <a> <b>` removes the link from whichever issue holds it: the entries of `a`
naming `b`, or failing that, the entries of `b` naming `a`. Two issues that are not linked are
`not_found`. A `b` that is not exactly an issue's id or Alias removes the entries of `a` that are
its text (as typed or as a citation spelling normalises it) when there are any, before it is
matched as a prefix, and a `b` that names no issue is matched only that way: a link to an issue
that no longer exists can still be removed, even when its text is a prefix of another issue's id.
Both commands take `--if-match <etag>`, checked against the file they write, and `--kv`.

`capcom show` lists an issue's links on both issues: a `related: [...]` line after the `deps:`,
`dependents:` and `children:` lines in the text (see Dependency output), and `"related": [...]` in
`--json`, always present. The list holds the issue's own `related`
entries in file order, then the ids of the issues whose links name it, in id order, each once. A
renaming `capcom mv` rewrites every `related` entry naming the old id or one of its aliases, as it
does `deps`. `capcom check` reports a `related` entry that names no issue (`missing-related`) or
names the issue itself (`field`), and a `related` key in a store declaring a format below 2
(`format-too-old`), at the `related` line, whatever it holds: `related: []` included.

### Dependency output

An issue **unblocks** exactly the todo issues that become Ready when it alone is closed. The
commands below report Dependency facts in the output agents already read; their `--json` data
only gains fields (the envelope's `schema` stays 1), and their `--kv` output is unchanged.

`capcom show` prints, after `category:` and only for an issue in the todo category, a `ready:`
line: `yes`, or `no (<reasons>)` worded as `ready --all` words them. After `deps:` come a
`dependents:` line, the ids of the issues that depend on this one in tree order, and a `children:`
line, `<closed>/<total> closed` for its direct children; each is left out when empty. The text
lines run in this order:

```text
id, path, etag, parent, title, status, category, ready, type, priority, labels, deps,
dependents, children, related, plan, aliases, created, closed, reason
```

```text
category: todo
ready: no (blocked by db-schema (todo); open children: write-tests)
type: task
priority: 2
deps: [db-schema]
dependents: [write-docs]
children: 1/3 closed
```

In `--json`, `"ready"` is a boolean, present only for a todo issue; `"blocking"` lists the
unsatisfied dependencies of an issue that is not Ready, each as `{"dep": ..., "reason": ...}`, and
is left out otherwise; `"dependents"` is always present, `[]` when none; `"children"` is
`{"total": N, "closed": N}`, left out for an issue without children.

`capcom close` prints a `<id>  <outcome>` line per argument, then, when the closes made any todo
issue Ready, one last line naming them:

```text
db-schema  closed
login-flow  closed
now ready: build-pipeline, ship-it
```

Each `--json` item carries `"now_ready"`: the todo issues that became Ready once that argument
and the ones before it were closed, in tree order, each id reported once across the arguments. It
is always an array, `[]` when none. An issue that
`ready.hide` hides is never named.

`capcom ready` lists the Ready issues most urgent first (priority 0 first), then the one that
unblocks more issues first, then in tree order. A Ready issue that unblocks others ends its line
with `-- unblocks N`, the count of those issues; under `--all`, a todo issue that is not Ready ends
with `-- not ready: <reasons>`:

```text
session-refresh  open  P2  Session refresh  -- unblocks 1
```

In `--json`, `"unblocks"` lists the ids of the issues it unblocks, and is left out when there are
none.

`capcom dep graph [id] [--all]` prints the Dependency graph as a Mermaid flowchart: every issue
that is not Closed, or, with an id, that issue and its transitive deps and dependents; `--all` adds
Closed issues, and an issue named by id is always a node. `flowchart LR` comes first, then a node
`n<k>["<id> (<status>)"]` per issue in tree order, then an edge `n<a> --> n<b>` from each
dependency to its dependent, by dependent in tree order and then in its `deps` order. A dep naming
no issue, or an issue left out, has no edge:

```text
flowchart LR
    n1["build-pipeline (open)"]
    n2["db-schema (open)"]
    n3["login-flow (open)"]
    n2 --> n1
```

Its `--json` data holds `"nodes"` (`id`, `status`, `category`, `title`) in node order and `"edges"`
(`dependent`, `dependency`).

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
validator. A file that changes between `capcom fmt`'s read and its rewrite fails the command with
a conflict and keeps its new bytes; the files rewritten before it stay rewritten. `capcom fmt --check` writes nothing, lists each file that would change, and exits 1, as
`capcom check` does on a problem, when any would. With `--json` the result is kind `fmt` with the
data `{"paths": [...]}`.

### Errors

Every problem with an issue file names the file, and the line when one is to blame, as
`path:line: message`, so an agent can fix files in a loop (ADR 0006). An unknown or repeated key
cites the key's line; a value of the wrong shape cites the value's line; a value the config does not
allow (an undeclared status or type, a priority out of range) cites its key's line; a missing
`closed` or `reason` on a Closed issue cites the `status` line.

A command does not fail on a bad issue file, except when it writes or moves an id the file may hold,
as below. The index (`<git-dir>/capcom/index.db`) leaves the file out and every command that reads
it reports the file as a warning until the file is fixed; the other issues stay usable. Plain text
prints each `warning:` line after the command's result; `--kv` output puts every `WARNING=` line
before its first record, so that `STATUS=` is always the last line. `create`, `mv` and `rm` fail on
a problem with the tree only
when it concerns the issue they write, move or remove, its new path or the parent they resolve; any
other is a warning. `know add`, `know mv` and `know rm` treat the Knowledge tree the same way for the Note
they write, move or remove. A new id is refused when a file anywhere under a directory without its
node file is named for that id or records it among its aliases, since that file is outside the tree
until the directory is fixed. A new id is refused the same way when a file of a repeated id is named
for it or records it among its aliases, with `id "<id>" is already taken by this file, left out for
the duplicate id "<dup>"` (`an alias of` in place of `taken by` for an alias). A new id is also refused when an issue file or Note that does not parse
records it among its aliases, as far as its `aliases:` line can still be read (either list form,
conflict markers passed over), with `id "<id>" may be an alias of issue <holder>, whose file does not
parse: fix it first` (`note` in place of `issue` for a Note); every other problem of that file stays
a warning. Two files that answer to the same
id or alias are both left out, each warning naming the other. The index refreshes itself on every command that reads it,
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

The frontmatter is always scanned. A marker there breaks the frontmatter, and every command that
reads the file warns with the first such marker's `conflict` message in place of the YAML error it
causes, so `capcom check` reports it once, as `conflict`, with no `frontmatter` problem beside it.
Body lines inside a fenced code block (opened by three backticks or three tildes) are not scanned,
so a Note may show a conflict as an example. A backtick line with another backtick after its opening
run, such as ```` ```inline``` quoted ````, is inline code and opens no block. CRLF files give the
same problems.

To resolve a Log conflict:

1. Keep both sides' entries, in date order.
2. Delete the marker lines.
3. Run `capcom check` to confirm no `conflict` problem is left, then finish the merge as usual.

#### The Log merge driver

capcom ships a git merge driver, `capcom merge-driver`, that merges an issue's Log by entry, so
two branches that each log on the same issue merge without a conflict. It keeps one side's
entries in their order and merges the other side's new entries in by date; an entry one side
deleted stays deleted. The rest of the file merges with `git merge-file`: when both sides change
the same lines outside the Log, the driver leaves conflict markers and git stops on the conflict
as usual. The markers carry git's branch labels, or `<path> (ours)`, `<path> (base)` and
`<path> (theirs)` from a git older than 2.44. A Log the driver cannot split into entries merges
as plain text.

To turn the driver on:

1. Run `capcom init --merge-driver`. It adds this line to the `.gitattributes` at the working tree
   root, creating the file when missing; an existing file keeps its bytes and its line endings, and
   a re-run adds nothing:

   ```
   .capcom/issues/**/*.md merge=capcom-log
   ```

2. Run the two commands it prints. capcom never writes `.git/config`, and git does not version its
   config, so every clone runs them once:

   ```sh
   git config merge.capcom-log.name "capcom Log merge"
   git config merge.capcom-log.driver "capcom merge-driver %O %A %B %P %S %X %Y"
   ```

Commit `.gitattributes`. A clone without the config ignores the `merge=capcom-log` attribute and
merges issue files with git's normal text merge, so a Log conflict there is resolved by hand as
above.

#### Checking the store after every merge

A merge without a conflict can still leave the store broken: one branch removes an issue that the
other branch made a dependency of, for example. git runs a `post-merge` hook after every merge
that completes, including a `git pull` that merges, and a hook that runs `capcom check` lists each
such problem as `path:line: message`:

```sh
#!/bin/sh
capcom check
```

Save it as `.git/hooks/post-merge`, or in the directory `core.hooksPath` names when that is set,
and make it executable. git does not version hooks, so every clone installs it once; capcom
installs no hooks itself. The hook cannot undo the merge: fix what it reports in a new commit.
git does not run it when the merge stops on a conflict, or for `git pull --rebase`; run
`capcom check` yourself then, as in step 3 above.

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
refs: [internal/cli/testharness_test.go#func runCLI]
aliases: [cli-tests]
---

CLI tests drive every command in process through `runCLI`, in a real temp git repository.
```

| Key             | Value                                                                    | Required |
|-----------------|--------------------------------------------------------------------------|----------|
| `title`         | One line that reads on its own.                                          | yes      |
| `labels`        | A list of Labels.                                                        | no       |
| `refs`          | A list of the files, directories and symbols the Note depends on.        | no       |
| `retired`       | `true` or `false`: whether the Note no longer holds (see Supersession).  | no       |
| `superseded-by` | A list of the ids of the Notes that replace this one (see Supersession). | no       |
| `aliases`       | A list of the Note's former ids.                                         | no       |

Any other key is an error, the id included. `retired` and `superseded-by` need store format 3 (see
Config). capcom writes a Note with the keys in the order of the table, empty fields and a false
`retired` left out, lists in flow style on one line, text unquoted wherever YAML reads it back
unchanged and in double quotes otherwise, always on one line, and the body byte for byte. The body
is free markdown.

One Label means something to capcom: `background` marks a **Background** Note, one that still
holds but is rarely the answer, such as a decision record's rejected options. `capcom know tree`
leaves out a background Note and every Note below it unless `--background` is given, and starts
from a background Note named directly; `--background` and `--retired` are independent.
`capcom know show`, `capcom know show --with-children` and `capcom know search` treat it like any
other Note. The Managed block in AGENTS.md leaves out a background top-level Note. Every other
Label is free-form and changes nothing capcom does.

Notes may be edited by hand, in any editor, or changed through capcom's write verbs (ADR 0010).
`capcom check` and `capcom fmt --check` are the validators of a hand edit: `check` reports a file
that does not parse or validate, or a link that names no Note, as `path:line: message`, and
`fmt --check` lists a file that is not in canonical form, which `capcom fmt` then rewrites. The
write verbs are the scripted path:

- `capcom know add` writes a new Note, at the top of the tree or below `--parent`.
- `capcom know update` changes a Note's title, labels, refs, retired flag, superseded-by list or
  body; only the flags given change anything. The body takes the same edit flags as
  `capcom update` for an issue, plus `--body-file` to replace the whole body.
- `capcom know mv` renames a Note, moves it below another (`--parent`, or `.` for the top of the
  tree), or both. A rename keeps the old id as an alias, and the Note's children move with it.
  It rewrites every other Note's `superseded-by` entry naming the old id or one of its aliases.
- `capcom know rm` deletes a Note file together with its aliases, and drops every other Note's
  `superseded-by` entries naming it; those Notes are reported as `superseded_by_dropped`. A Note
  with children is refused; move or remove them first.

An **etag** is a hash of a Note file's bytes: `capcom know show` prints it, and `know add`,
`know update` and `know mv` print the new one after a write. `know update`, `know mv` and
`know rm` take `--if-match <etag>` and refuse with a conflict, changing nothing, when the file's
etag differs, so an agent never overwrites a Note that changed since it last read it. `know mv`
and `know rm` also refuse with a conflict, taking back what they changed, when a Note file they
rewrite changes while they run. A hand edit
changes the file's bytes, and so its etag, like a write verb does: `--if-match` refuses a write
over a Note edited by hand since the agent last read it.

### Supersession

A **Retired** Note is one whose content no longer holds, kept so that old citations still resolve:
`retired: true` marks it. **Supersession** links a Note to the Notes that replace it: its
`superseded-by` list holds their ids. A Note replaced in whole is retired and names its
successors; one replaced in part stays current, names its successors and says in its body what
still holds. Neither key implies the other.

- `capcom know tree`, `capcom know search` and `capcom know show --with-children` leave out a
  retired Note and every Note below it, whatever their own `retired` key says; `--retired` brings
  them back. `capcom know tree` also leaves out a background Note and every Note below it unless
  `--background` is given (see Note files). `capcom know show <id>` always prints the Note it
  names, retired or not, with its `retired` and `superseded-by` lines, and `capcom know tree <id>`
  starts from a retired Note named directly.
- The Managed block in AGENTS.md leaves out a retired or background top-level Note (see The
  Managed block in AGENTS.md), and the Ref checks of `capcom know verify`, `capcom check` and `capcom prime` leave
  out the Refs of a retired Note and of every Note below it (see Refs).
- `capcom know update --retire` sets `retired: true` and `--unretire` removes it.
  `--add-superseded-by <note>` takes an id, an alias or a prefix and writes the id it resolves to,
  unless an entry already names that Note by its id or any alias, which adds nothing.
  `--remove-superseded-by <note>` follows `capcom related rm`: a `<note>` that is exactly a Note's
  id or alias removes every entry naming that Note, by its id or any alias; one that is not removes
  the entries that are its text when there are any, before it is matched as a prefix, and one that
  names no Note is matched only that way. One that matches no entry changes nothing. A prefix of
  several ids that no entry's text matches is refused naming the candidates, and a name two files
  claim is refused naming both files, as `capcom dep rm` refuses them; nothing is written. Both
  repeat, and the removals apply after the additions. An update that leaves a `retired` or
  `superseded-by` line in the Note file in a store below format 3, whether or not the update
  added it, first raises `config.yaml` to `format: 3`, as `capcom related add` raises it to 2
  (see Config), and warns:

```text
warning: .capcom/config.yaml: store format raised to 3; capcom releases that read only format 2 can no longer read this store
```

A `superseded-by` entry is spelled as an id or alias, appears once and never names the Note
itself; anything else is an error at the entry's line. A renaming `capcom know mv` rewrites every
entry naming the old id or one of its aliases to the new id, and `capcom know rm` drops every
entry naming the removed Note; a Note file that does not parse is left as it is, with a warning
naming it. `capcom check` reports a `superseded-by` entry that names no Note by id or by an alias
(`missing-superseded-by`), at the entry's line, and a `retired` or `superseded-by` key in a store
declaring a format below 3 (`format-too-old`), at the key's line, whatever it holds:
`retired: false` and `superseded-by: []` included.

### Refs

A **Ref** is either a path or `path#symbol`:

- the path is relative to the root of the working tree, with forward slashes, and names a file or a
  directory (`docs/site/reference/file-format.md`, `internal/lock`);
- an absolute path, or one that climbs out of the working tree (`../other`), is an error, and so is
  a path that leaves the tree through a symbolic link;
- after the first `#` comes a symbol: text the file must contain, found by a plain text search
  (`internal/lock/lock.go#func Open`). It is not parsed as code, so any text works, and a symbol
  needs a file, not a directory: a regular file of at most 1 MiB. A symbol whose file is a named
  pipe or another entry that is not a regular file, or is larger than 1 MiB, is not searched, and
  the Ref does not resolve.

A Ref that no longer resolves leaves the Note usable. `capcom know verify` lists every such Ref as
`path:line: ref "<ref>": <why>`, citing the Ref's line in the Note file, and fails when it finds
one; `capcom check` reports them too, and `capcom prime` counts them in its briefing. All three
leave out the Refs of a retired Note and of every Note below it.

## Config

`.capcom/config.yaml` is optional, and so is every key in it: a missing key takes the value shown
below, which is the file `capcom init` writes, except `format`, whose missing key means 1. An
unknown key or a bad value is an error citing its
line. A `config.yaml` that is not a regular file (a symlink, a named pipe, a directory) is never
followed or read: every command fails with `.capcom/config.yaml: not a regular file`, and with
`.capcom/config.yaml: file is larger than 1 MiB` for one larger than 1 MiB, except `capcom prime`,
which reports it as one `warning: store:` line and exits 0.

```yaml
# The store format version. capcom refuses a store whose format is newer than it reads.
format: 4

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

# Agent guides that must reach AGENTS.md's Managed block by a link or an @AGENTS.md import (format 4).
# agents:
#   guides: [CLAUDE.md, GEMINI.md]
```

- **format** — the store format version, a whole number of 1 or more; a missing key means 1
  ([ADR 0009](../../../.capcom/knowledge/decisions/adr-0009.md)). It is read before every other
  key: a store whose format is newer than the binary reads fails every command with
  `.capcom/config.yaml:<line>: store format <n> is newer than this capcom reads (up to <max>); upgrade capcom`,
  except `capcom prime`, which reports it as one `warning: store:` line and exits 0. Unknown keys
  stay errors, and any new key in an issue, a Note or this file raises the format. Format 2 adds
  the `related` key to issue files; format 3 adds the Note keys `retired` and `superseded-by`;
  format 4 adds the `agents.guides` key to `config.yaml`.
  `capcom init` writes `format: 4` into a store it creates; into an existing store without
  `config.yaml` it writes the same file with the `format:` line left as the commented
  `# format: 1`, so the store stays at format 1. A key line needs its format whatever it holds,
  an empty list or `retired: false` included, because a release older than the key rejects the
  file for the key alone. So a write that creates or edits an issue or a Note file carrying a key
  line of a newer format than the store declares raises the store to that format: a `related`
  line raises a store below format 2 to `format: 2`, replacing the `format:` line, or the
  format-1 template's commented `# format: 1` line, keeping every other byte, and creating
  `config.yaml` holding only that line when the store has none; a `retired` or `superseded-by`
  line raises a store below format 3 to `format: 3` the same way (see Related links and
  Supersession). Each raise warns that capcom releases reading only the older format can no
  longer read the store. `capcom mv`, `rm`, `know mv`, `know rm` and `fmt` never raise it;
  `capcom check` reports the key line instead. Nothing raises it for `config.yaml`'s own keys:
  an `agents` key in a store declaring a format below 4 is an error at its line naming
  `format: 4` as the fix.
- **statuses** — an ordered mapping of status name to Status category. Every rule in capcom reads
  the category, never the name (ADR 0003). At least one status must be in `todo` and one in
  `done`. When capcom moves an issue into a category (create, close, reopen) it uses the first
  status listed in that category.
- **types** — the issue types; at least one, no duplicates.
- **plans** — the directories a `plan:` basename is looked up in, in order.
- **ready.hide** — labels whose issues `ready` leaves out by default.
- **lock.ttl** — a positive duration (`2h`, `90m`).
- **elsewhere.days** — a whole number of days, 0 to 36500. The Elsewhere scan reads local and
  remote-tracking branches whose tip is newer than this; 0 turns the scan off. capcom never
  fetches, so a remote-tracking branch is as fresh as the last `git fetch`. A branch copy of an
  issue file larger than 1 MiB is not read: the scan reports no entry for it and a warning names it.
- **agents.guides** — the agent guides beside `AGENTS.md` that must reach its Managed block, by a
  link to `AGENTS.md` or an `@AGENTS.md` import line; `[CLAUDE.md, GEMINI.md]` when the key is
  absent, and `[]` turns the rule off. Each entry is a slash-separated path inside the repository,
  relative to its root, never `AGENTS.md` itself, and no two alike. The key needs store format 4.

## The user config

`capcom ui` reads one file that belongs to no repository: the per-user config,
`capcom/config.yaml` under the user config directory Go's `os.UserConfigDir` names
([ADR 0011](../../../.capcom/knowledge/decisions/adr-0011.md)):

| Platform | File |
|---|---|
| Linux and other Unix | `$XDG_CONFIG_HOME/capcom/config.yaml`, or `~/.config/capcom/config.yaml` when it is unset |
| macOS | `~/Library/Application Support/capcom/config.yaml` |
| Windows | `%AppData%\capcom\config.yaml` |

It is yours, not a repository's: it is never committed, and every repository you open with
`capcom ui` shares it. Only `capcom ui` reads it; no other command reads or creates it, so normal
capcom use leaves nothing in your home directory. The first time `capcom ui` starts without the
file, it creates the directory and writes this template, every key commented out at its default;
it never rewrites an existing file.

```yaml
# capcom user config: personal settings for capcom ui (ADR 0011).
#
# This file belongs to you, not to any repository: only capcom ui reads it, and every repository
# you open shares it. Every key below is commented out at its default; remove the leading "# " of
# a line to set it. An unknown key is ignored with a warning.
#
# ui:
#   # The colour preset: auto picks dark or light from the terminal background.
#   theme: auto        # auto | dark | light
#
#   # Colour overrides by role; none by default, so each role takes the preset's colour. A value
#   # is a quoted "#rgb" or "#rrggbb" colour (an unquoted # starts a comment) or a number 0-255.
#   # Roles: accent, border, muted, link, selection, heading, code, and one per status category:
#   # status-todo, status-doing, status-waiting, status-done, status-dropped.
#   colors:
#     accent: "#7aa2f7"
#     status-doing: 214
#
# NO_COLOR in the environment turns colour off, whatever this file says.
```

- **ui.theme** — `auto`, `dark` or `light`; `auto` when absent. `auto` asks the terminal for its
  background colour and picks the light or the dark preset, dark until the terminal answers.
- **ui.colors** — a mapping of colour role to colour, overriding the preset's colour for that
  role; none when absent. The roles are `accent`, `border`, `muted`, `link`, `selection`,
  `heading`, `code`, and one per Status category: `status-todo`, `status-doing`,
  `status-waiting`, `status-done` and `status-dropped`, each covering every Status of its
  category. A colour is a quoted `"#rgb"` or `"#rrggbb"` (unquoted, `#` starts a YAML comment) or
  a 256-colour number from 0 to 255.

A non-empty `NO_COLOR` environment variable turns colour off, whatever the file says.

Since every capcom release on the machine reads the same file, it is read leniently, unlike
`.capcom/config.yaml`: an unknown key or colour role is ignored, with a `path:line:` warning in the
status bar of `capcom ui`. Malformed YAML, a theme other than the three, a colour of another form,
or a file that cannot be read is an error shown there the same way, and `capcom ui` runs on the
defaults; so it does when the file cannot be created. `capcom ui` reads the file once, when it
starts.

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
A record that does not parse or carries no token holds no lock: `lock break` discards it with a
warning, and `lock status --all` and `capcom prime` skip it with a warning naming the file.
Every change to a record runs under an OS file lock on its `.lck` file, held only for that read and
write, and the record is replaced atomically.

## The Managed block in AGENTS.md

`AGENTS.md` at the root of the working tree stays a hand-written agent guide, and the only home of
the block capcom owns in it, the **Managed block**: the lines from `<!-- BEGIN CAPCOM -->` to
`<!-- END CAPCOM -->`, each marker on a line of its own. The block holds a short paragraph naming
capcom as the issue register of record,
an install pointer for an agent without a `capcom` binary (capcom's source and the README's
install instructions, https://github.com/airiclenz/capcom#install), the commands an agent needs,
the work loop first in the order it runs (`ready`, `show`, `update <id> --status <doing>` to claim
an issue, `log`, `close`, `reopen`), then `create` with `--dep`, `dep add` and `dep list`,
`list --tree --depth 2` and `know tree`, followed by one line pointing to `capcom <cmd> --help` for
each command's flags, a paragraph on what an issue's body holds (what is wrong
or wanted, why, and an `## Acceptance` section, readable on its own; the how and the order belong
in a plan), a map of the store (an issue is `.capcom/issues/<id>.md` with its children in the
directory `.capcom/issues/<id>/` beside it, Notes the same under `.capcom/knowledge/`; prefer capcom
commands, and run `capcom check` after a hand edit), a paragraph recommending the Tree lock before
a change that spans several files or commits (`capcom lock acquire --purpose <text>`, keeping the
token it prints for `touch`, `release` and resuming, and asking the user when `capcom lock check`
reports another session's lock), and the top level of the Knowledge tree, one
`` - `<id>`: <title> `` line per top-level Note in id order, a retired one and one labelled
`background` left out, and so is one whose file a load problem names (a duplicate id or alias, an
invalid or repeated alias). The claim line names the configured doing status, the first status of
the `doing` category in `config.yaml` (`active` by default), and is left out when the config
declares none. The block is rendered from the Note files and that doing status alone, so the same
tree and config always yield the same block.

- `capcom agents` writes the block: it replaces the lines between the markers, or appends the
  block to the end of the file when it has none, and creates `AGENTS.md` when there is none. Every
  byte outside the markers is kept, and so are the file's line endings (LF or CRLF).
- `capcom agents --check` fails when the block is out of date, and `capcom check` reports an
  out-of-date block as an `agents-block` problem at the begin marker's line. A file without the
  block is neither: the block is opt-in.
- A Note write that changes the top level of the Knowledge tree refreshes the block when
  `AGENTS.md` already holds one and it went out of date: a top-level `capcom know add`, a
  `capcom know update` of a top-level Note's title, retired flag or `background` label, a
  `capcom know mv` into, out of or within the top level, and a `capcom know rm` of a top-level Note. It never adds a block to
  a file without one. A write below the top level changes nothing, since the block lists only the
  top level. A hand edit of a Note, or of `config.yaml` renaming the doing status, refreshes
  nothing: `capcom check` reports the stale block, and `capcom agents` rewrites it.
- A marker that appears twice, an end marker before the begin marker, or a begin marker that is
  never closed is an error at that marker's line, for `agents` and `check` alike.
- Every other agent guide that `agents.guides` lists (see [Config](#config); `CLAUDE.md` and
  `GEMINI.md` by default) reaches the block in `AGENTS.md` and never holds a copy of it. A guide
  reaches it when it resolves to the same file as `AGENTS.md` (a link `CLAUDE.md -> AGENTS.md`, or
  the file an `AGENTS.md` link points to), or when one of its lines, white space trimmed, is
  `@<rel>` or `@./<rel>`, `<rel>` being the root `AGENTS.md` relative to the directory of the file
  the guide resolves to, in slash form: `@AGENTS.md` at the root, `@../AGENTS.md` one level down.
- `capcom agents` appends the import line `@<rel>` to each listed guide that exists but does not
  reach the block, after a blank line and in the file's line ending, keeping its permissions, and
  prints `added @AGENTS.md to CLAUDE.md`. It never creates a missing guide, and leaves a guide that
  links outside the working tree unread and unwritten (`skipped CLAUDE.md: links outside the
  working tree`). Guides that resolve to one file are read, and written, once.
- While `AGENTS.md` carries the block, `capcom check` reports each listed guide that exists but
  does not reach it as an `agents-guide` problem
  (`CLAUDE.md: does not reach the capcom block in AGENTS.md; run capcom agents to add @AGENTS.md`),
  and `capcom agents --check` fails on the first one once the block is current. A listed guide that
  is not a regular file, or is larger than 1 MiB, is an `agents-guide` problem too. A missing guide,
  one linking outside the working tree, an `AGENTS.md` without the block and `guides: []` report
  nothing.
- An `AGENTS.md` that is a symbolic link keeps its link: the block is written to the file it
  points to, but only when that file lies inside the working tree (`AGENTS.md -> CLAUDE.md`). An
  `AGENTS.md` link that resolves outside the working tree is neither read nor written.
  `capcom agents` fails on it; `capcom check`, `agents --check` and Note writes treat it as having
  no block. An `AGENTS.md` that is not a regular file (a named pipe or a directory, or a link to
  one), or is larger than 1 MiB, is never read either, but is an error for `agents`,
  `agents --check` and `check` alike (an `agents-block` problem in `check`); a Note write warns
  that the block was not refreshed.

Edits inside the markers are overwritten on the next refresh; write anything else outside them.
