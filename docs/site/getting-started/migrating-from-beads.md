---
title: "Migrating from beads"
description: "How an agent moves a repository off beads by writing capcom issue files and checking them until clean."
sidebar: { order: 3 }
---

capcom has no importer (ADR 0006). An agent moves a repository off beads (`bd`) by writing the
issue files itself, following this guide, and `capcom check` proves the result valid with
`path:line` problems the agent fixes in a loop. The one step that needs judgment, choosing a
readable slug for every beads id, stays with the agent.

The file formats are in the [File format](../reference/file-format.md) reference. Read it first.

## Steps

1. **Freeze beads.** Tell every agent and person working in the repository to stop writing to
   beads. Run a final `bd sync`, so the database and its JSONL agree, then `bd export` to write
   every issue to one JSONL file, one issue per line. Keep the export outside the repository and do
   not commit it; keep it after the cut-over too, since it is the only record of what does not
   migrate (see What does not migrate). Copy out every memory saved with `bd remember` as well:
   each becomes a Knowledge Note (see Memories).
2. **Create the store.** Run `capcom init`. If the export uses statuses or types the default
   `.capcom/config.yaml` lacks (see the mapping below), declare them there now.
3. **Choose the ids.** Build the table of beads id to capcom id first, for every issue in the
   export, closed ones included. A capcom id is a kebab slug of one to four words of `a-z` and
   `0-9`, at most 40 characters, unique in the tree, chosen from what the issue is about
   (`login-timeout-race`), never derived mechanically from the hash. The beads id becomes the
   issue's alias. A dotted beads id (`apogee-kk0.4`) is refused as an id, since a slug has no dots,
   so it survives only as the alias.
4. **Write the files.** One file per issue, following the field mapping below, each placed in its
   parent's directory. Use the id table for every dependency and every parent.
5. **Check until clean.** Run `capcom check` and fix every `path:line` problem it lists; repeat
   until it lists none and exits 0. A `conflict` problem is a git conflict marker left in the file
   by a merge; resolve it as [Merge conflicts in the Log](../reference/file-format.md#merge-conflicts-in-the-log)
   describes. Then run `capcom fmt` to rewrite every file in canonical form, so the first command
   that edits a hand-written file changes only what it means to change; `capcom fmt --check`
   exits 0 once none is left.
6. **Rebuild the index.** Run `capcom reindex`, then compare `capcom list --all` against the
   export: the same number of issues, and the same ones open. `capcom ready` may list fewer
   issues than `bd ready` did, see Labels.
7. **Take back the git hooks.** beads may have set `core.hooksPath` to hooks of its own in the
   repository's config. Check with `git config --local --get core.hooksPath`; `--local` reads only
   this repository's setting, so a `core.hooksPath` in your global config, which is not beads',
   does not show. If it points at a beads directory, first move any hook you still need out of it
   (a `commit-msg` or `prepare-commit-msg` hook of your own, for example), then point
   `core.hooksPath` back at your hooks or unset it with `git config --local --unset core.hooksPath`.
   Do this before the commit of step 8: beads' pre-commit hook re-exports its JSONL when `.beads/`
   is staged. capcom installs no git hooks.
8. **Remove beads.** Delete the beads sections from `AGENTS.md` and run `capcom agents` to add the
   capcom block. Replace beads' agent hooks: first delete by hand every hook group that runs a `bd`
   command (`bd prime`, `bd sync` or another) from `.claude/settings.json` and `.codex/hooks.json`,
   under `SessionStart`, `PreToolUse` or any other event, then run `capcom init --claude-hooks`,
   `capcom init --codex-hooks` or both. The flags only add capcom's `SessionStart` and `PreCompact`
   groups and keep every existing group, so they remove nothing; capcom has no `PreToolUse` hook,
   so a beads `PreToolUse` group goes without a replacement. Delete `.beads/`. Commit `.capcom/`
   and the removal of `.beads/` together, so no commit holds both trackers or neither.

## Field mapping

The left column is the key in the beads export.

| beads                     | capcom                                                                                     |
|---------------------------|--------------------------------------------------------------------------------------------|
| `id`                      | Becomes the issue's alias, kept exactly, dots included (`apogee-kk0.4`). The id is a new slug. |
| `title`                   | `title`, verbatim.                                                                         |
| `description`             | The start of the body, verbatim.                                                           |
| `acceptance_criteria`     | An `## Acceptance` section of the body, verbatim.                                          |
| `design`                  | A `## Design` section of the body, verbatim.                                               |
| `notes`                   | `## Log` entries, one per dated entry in the notes (see Log entries).                      |
| `comments`                | One `## Log` entry per comment, oldest first, with the comment's author and date.          |
| `status`                  | `status`, see Statuses.                                                                    |
| `priority`                | `priority`; both use 0 (most urgent) to 4.                                                 |
| `issue_type`              | `type`, see Types.                                                                         |
| `labels`                  | `labels`, except `parked` and `planned`, see Labels.                                       |
| `spec_id`                 | `plan`, as the plan's basename, when it names a plan; otherwise a Log entry, see Plans.    |
| `dependencies`            | By dependency type, see Dependencies.                                                      |
| `created_at`              | `created`, RFC3339 in UTC to the second (`2026-10-11T09:30:00Z`).                          |
| `closed_at`               | `closed`, in the same form; a Closed issue needs it.                                       |
| `close_reason`            | `reason`: a non-empty reason is kept verbatim (`Closed` included). A Closed issue needs one, so write `Closed in beads.` only when the export's is empty. |
| `updated_at`              | Dropped: git and the index know when a file changed. It dates no Log entry.                |
| `started_at`, `assignee`, `owner`, `external_ref`, `created_by`, `dependency_count`, `dependent_count`, `comment_count`, `_type`, anything else | Dropped, see What does not migrate. Copy a value that still matters into a Log entry. |

### Statuses

| beads status  | capcom status                                                                              |
|---------------|--------------------------------------------------------------------------------------------|
| `open`        | `open`, or `parked` when the issue carries the `parked` label.                             |
| `in_progress` | `active`.                                                                                  |
| `blocked`     | `open` when a dependency blocks it, since capcom derives blocking from `deps`; `parked` when it waits on something outside the tracker. |
| `deferred`    | `parked`.                                                                                  |
| `closed`      | `done`; `dropped` when the close reason says it was abandoned, rejected or a duplicate.    |
| custom        | Declare it in `statuses`, mapped to the category that matches what it means.              |

### Types

`bug`, `feature`, `task` and `chore` carry over as they are. For any other type (`epic`,
`decision`, ...) either declare it in the config's `types`, or write the nearest default type.
A beads `epic` becomes the Parent of its children, which nest under it through their parent-child
dependencies either way.

### Labels

- `parked` is a status in capcom, not a label: the issue gets status `parked` and loses the label.
  A closed issue keeps its closed status and just loses the label. This lowers the ready count:
  `bd ready` listed an open issue that carried only the `parked` label, but `parked` is in the
  waiting category, and a waiting issue is not ready, nor is any issue below it in the tree.
- `planned` is dropped: an issue is Planned when its `plan:` field is set. An issue whose `spec_id`
  names a plan gets `plan:` (see Plans), so it is Planned with or without the `planned` label,
  closed issues included.
- Every other label carries over unchanged.

### Plans

beads' `spec_id` holds a path to a plan document, which dangles once the plan is archived. capcom's
`plan:` holds only the basename, the file name without its directory and without `.md`
(`2026-10-11 - 00 - capcom-v1-issues-plan`), and resolves it against every directory in the
config's `plans` list, the archive included. When `capcom check` reports `missing-plan`, find the
plan's current name, or drop the field and record the old value in a Log entry.

A `spec_id` that points at a document that is not a plan (a review, a design note) gets no `plan:`
at all, since `plan:` makes the issue Planned: record the path in a Log entry instead, dated by
`created_at`, with the author `beads`: `- <created_at date> beads: spec: <path>`.

### Dependencies

Each entry in a beads issue's `dependencies` says that the issue (`issue_id`) depends on another
(`depends_on_id`), with a type. Mind the direction: the dependency belongs to the issue that holds
it, not to the one it names.

| beads type        | capcom                                                                                  |
|-------------------|-----------------------------------------------------------------------------------------|
| `blocks`          | The capcom id of `depends_on_id` goes into the issue's `deps`.                          |
| `parent-child`    | The issue's file moves into the directory of its parent (`depends_on_id`), see below.   |
| `related`         | `capcom related add <capcom id> <capcom id of depends_on_id>`, on one side only.        |
| `discovered-from` | A Log entry: `discovered while working on <capcom id>`.                                 |

A dependency's `created_by` and `metadata` are dropped; only its type, its two ids and, for a
discovered-from link, its date carry over.

Nesting comes from the parent-child dependencies alone. Dotted beads ids (`apogee-kk0.4`) are
usually children of the id before the dot, but not every child has a dotted id, so do not rely on
the dots. Write each `deps` entry as the capcom id, not the alias, although an alias resolves
too.

beads keeps one `related` link that both issues show, and so does capcom: `capcom related add`
stores it on the first issue only, and `capcom show` lists it on both. Run it once per beads
`related` dependency after the files are written, or write the other issue's capcom id into the
issue's `related` list by hand; never on both sides. Unlike a dependency, it blocks neither issue.
A store `capcom init` creates is at the current format, at least 2, which the `related` key needs (see Config in
[File format](../reference/file-format.md#config)).

### Log entries

The `## Log` section is the dated, append-only record of the issue. Write each entry as one list
item, oldest first:

```markdown
## Log

- 2026-09-14 airic: reproduced with two tabs.
- 2026-09-15 beads: discovered while working on session-refresh-bug
```

A comment becomes `- <date of created_at> <author>: <text>`, the text verbatim, with any further
lines indented two spaces under the item. The `notes` field is often a running record of several
dated entries; split it into one Log entry per date, each with that date and the author `beads`,
since beads keeps no author for it, and sort them in among the comments by date. Only a date at
the start of a line starts an entry (`2026-09-09 (owner): ...`, `2026-09-06 later: ...`); a date in
mid-sentence (`narrowed by the 2026-09-08 review`) is part of the text around it. Each entry runs
to the next line that starts with a date. The leading date and a `:` directly after it are
removed, and the rest of the text is kept verbatim, `(owner):` or `later:` included. An undated
lead before the first dated entry becomes an entry of its own, dated by `created_at`. A `notes`
value with no dated entries becomes one entry dated by `closed_at`, else by `created_at`. Never
date a note by `updated_at`: it moves on every edit, so it files old notes after later comments.
Discovered-from links use the date of the dependency and the author `beads`.

### Mentions of other issues

Text that names another issue by its beads id keeps working, since the old id is that issue's
alias and every command resolves it. If you rewrite such mentions to the new ids, use the id table
from step 3 and touch only strings that are ids in the export: text such as `apogee-sim` that looks
like an id but names no issue stays as it is.

### Memories

A memory saved with `bd remember` is a fact about the repository for agents, which in capcom is a
Knowledge Note. Write each one with
`capcom know add --id <slug> --title <title> --body-file <file>`: a slug chosen as for an issue, a
one-line title saying what the fact is about, and the memory as the body. See Note files in
[File format](../reference/file-format.md).

### What does not migrate

These have no home in capcom, so they go with `.beads/`:

- `.beads/interactions.jsonl`, beads' event log of every change to every issue;
- the Dolt history of the beads database and its remote;
- `started_at`, `updated_at`, the assignee and the other dropped fields of the mapping above,
  among them `created_by`, the counts `dependency_count`, `dependent_count` and `comment_count`,
  the record kind `_type`, and a dependency's `created_by` and `metadata`;
- the time of day, the ids and the exact whitespace of comments.

Keep the final `bd export` from step 1, and a copy of `.beads/interactions.jsonl`, outside the
repository after the cut-over, for as long as anyone may need to look one of them up.
