# Migrating from beads

capcom has no importer (ADR 0006). An agent moves a repository off beads (`bd`) by writing the
issue files itself, following this guide, and `capcom check` proves the result valid with
`path:line` problems the agent fixes in a loop. The one step that needs judgment, choosing a
readable slug for every beads id, stays with the agent.

The file formats are in [format.md](format.md). Read it first.

## Steps

1. **Freeze beads.** Tell every agent and person working in the repository to stop writing to
   beads. Run a final `bd sync`, so the database and its JSONL agree, then `bd export` to write
   every issue to one JSONL file, one issue per line. Keep the export outside the repository, or at
   least outside `.capcom/`, and do not commit it.
2. **Create the store.** Run `capcom init`. If the export uses statuses or types the default
   `.capcom/config.yaml` lacks (see the mapping below), declare them there now.
3. **Choose the ids.** Build the table of beads id to capcom id first, for every issue in the
   export, closed ones included. A capcom id is a kebab slug of two to four words of `a-z` and
   `0-9`, at most 40 characters, unique in the tree, chosen from what the issue is about
   (`login-timeout-race`), never derived mechanically from the hash. The beads id becomes the
   issue's alias.
4. **Write the files.** One file per issue, following the field mapping below, each placed in its
   parent's directory. Use the id table for every dependency and every parent.
5. **Check until clean.** Run `capcom check` and fix every `path:line` problem it lists; repeat
   until it lists none and exits 0.
6. **Rebuild the index.** Run `capcom reindex`, then compare `capcom list --all` against the
   export: the same number of issues, and the same ones open.
7. **Take back the git hooks.** beads may have set `core.hooksPath` to hooks of its own. Check with
   `git config --get core.hooksPath`. If it points at a beads directory, first move any hook you
   still need out of it (a `commit-msg` or `prepare-commit-msg` hook of your own, for example),
   then point `core.hooksPath` back at your hooks or unset it with
   `git config --unset core.hooksPath`. capcom installs no git hooks.
8. **Remove beads.** Delete the beads sections from `AGENTS.md` and run `capcom agents` to add the
   capcom block; replace any `bd prime` or `bd sync` session hook with the ones
   `capcom init --claude-hooks` writes; delete `.beads/`. Commit `.capcom/` and the removal of
   `.beads/` together, so no commit holds both trackers or neither.

## Field mapping

The left column is the key in the beads export.

| beads                     | capcom                                                                                     |
|---------------------------|--------------------------------------------------------------------------------------------|
| `id`                      | Becomes the issue's alias, kept exactly, dots included (`apogee-kk0.4`). The id is a new slug. |
| `title`                   | `title`, verbatim.                                                                         |
| `description`             | The start of the body, verbatim.                                                           |
| `acceptance_criteria`     | An `## Acceptance` section of the body, verbatim.                                          |
| `design`                  | A `## Design` section of the body, verbatim.                                               |
| `notes`                   | One entry in the `## Log` section (see Log entries).                                       |
| `comments`                | One `## Log` entry per comment, oldest first, with the comment's author and date.          |
| `status`                  | `status`, see Statuses.                                                                    |
| `priority`                | `priority`; both use 0 (most urgent) to 4.                                                 |
| `issue_type`              | `type`, see Types.                                                                         |
| `labels`                  | `labels`, except `parked` and `planned`, see Labels.                                       |
| `spec_id`                 | `plan`, as the plan's basename, see Plans.                                                 |
| `dependencies`            | By dependency type, see Dependencies.                                                      |
| `created_at`              | `created`, RFC3339 in UTC to the second (`2026-10-11T09:30:00Z`).                          |
| `closed_at`               | `closed`, in the same form; a Closed issue needs it.                                       |
| `close_reason`            | `reason`; a Closed issue needs it, so write `Closed in beads.` when the export has none.   |
| `updated_at`              | Dropped: git and the index know when a file changed.                                       |
| `assignee`, `owner`, `external_ref`, anything else | Dropped. Copy a value that still matters into a Log entry.        |

### Statuses

| beads status  | capcom status                                                                              |
|---------------|--------------------------------------------------------------------------------------------|
| `open`        | `open`, or `parked` when the issue carries the `parked` label.                             |
| `in_progress` | A status in the `doing` category. The default config has none: add one such as `active: doing` to `statuses`, or write `open`. |
| `blocked`     | `open` when a dependency blocks it, since capcom derives blocking from `deps`; `parked` when it waits on something outside the tracker. |
| `deferred`    | `parked`.                                                                                  |
| `closed`      | `done`; `dropped` when the close reason says it was abandoned, rejected or a duplicate.    |
| custom        | Declare it in `statuses`, mapped to the category that matches what it means.              |

### Types

`bug`, `feature`, `task` and `chore` carry over as they are. For any other type (`epic`,
`decision`, ...) either declare it in the config's `types`, or write the nearest default type.
An epic's children nest under it through their parent-child dependencies either way.

### Labels

- `parked` is a status in capcom, not a label: the issue gets status `parked` and loses the label.
- `planned` is dropped: an issue is Planned when its `plan:` field is set.
- Every other label carries over unchanged.

### Plans

beads' `spec_id` holds a path to a plan document, which dangles once the plan is archived. capcom's
`plan:` holds only the basename, the file name without its directory and without `.md`
(`2026-10-11 - 00 - capcom-v1-issues-plan`), and resolves it against every directory in the
config's `plans` list, the archive included. When `capcom check` reports `missing-plan`, find the
plan's current name, or drop the field and record the old value in a Log entry.

### Dependencies

Each entry in a beads issue's `dependencies` says that the issue (`issue_id`) depends on another
(`depends_on_id`), with a type. Mind the direction: the dependency belongs to the issue that holds
it, not to the one it names.

| beads type        | capcom                                                                                  |
|-------------------|-----------------------------------------------------------------------------------------|
| `blocks`          | The capcom id of `depends_on_id` goes into the issue's `deps`.                          |
| `parent-child`    | The issue's file moves into the directory of its parent (`depends_on_id`), see below.   |
| `related`         | A Log entry: `related to <capcom id>`.                                                  |
| `discovered-from` | A Log entry: `discovered while working on <capcom id>`.                                 |

Nesting comes from the parent-child dependencies alone. Dotted beads ids (`apogee-kk0.4`) are
usually children of the id before the dot, but not every child has a dotted id, so do not rely on
the dots. Write each `deps` entry as the capcom id, not the alias, although an alias resolves
too.

### Log entries

The `## Log` section is the dated, append-only record of the issue. Write each entry as one list
item, oldest first:

```markdown
## Log

- 2026-09-14 airic: reproduced with two tabs.
- 2026-09-15 beads: related to session-refresh-bug
```

A comment becomes `- <date of created_at> <author>: <text>`, the text verbatim, with any further
lines indented two spaces under the item. The `notes` field becomes one entry with the issue's
`updated_at` date and the author `beads`, since beads keeps no author for it. Related and
discovered-from links use the date of the dependency and the author `beads`.

### Mentions of other issues

Text that names another issue by its beads id keeps working, since the old id is that issue's
alias and every command resolves it. If you rewrite such mentions to the new ids, use the id table
from step 3 and touch only strings that are ids in the export: text such as `apogee-sim` that looks
like an id but names no issue stays as it is.
