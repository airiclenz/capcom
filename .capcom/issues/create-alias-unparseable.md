---
title: create reuses an alias held by an unparseable issue file
status: done
type: bug
priority: 2
created: 2026-10-12T05:39:56Z
closed: 2026-10-12T06:30:58Z
reason: Resolved by plan 2026-10-12 - 00 - open-issues-plan (items 2-3, archived in 70b91be)
---

`capcom create` refuses an id that is already a healthy issue's alias, but accepts it when the
file holding that alias does not parse. The tree then holds a duplicate that `capcom check` only
reports once the broken file is fixed.

## Repro

```sh
git init -q && capcom init
capcom create --id first-issue --title First
capcom mv first-issue renamed-issue          # first-issue is now an alias
capcom create --id first-issue --title dup   # refused: already an alias of renamed-issue (correct)
# break renamed-issue.md's frontmatter, e.g. a git conflict marker before `title:`
capcom create --id first-issue --title dup2  # exit 0, only warns about the bad YAML
```

After the conflict is resolved, `check` reports
`renamed-issue.md:<line>: duplicate alias "first-issue": the id of .capcom/issues/first-issue.md`.
Found in the apogee beads-migration trial (2026-10-12), where it reused `apogee-rxj`.

The warning `create` prints names only the YAML error (`invalid YAML frontmatter: mapping values
are not allowed in this context`), not that the line is a conflict marker, though `check` reports
`conflict` for the same file.

`docs/format.md`'s refusal rule covers a problem concerning the issue being written, its path or
its parent; a file that cannot be parsed cannot say which ids it claims, so the rule is silent here.

## Acceptance

- Decide (and document in `docs/format.md`) what `create` / `mv` do when an unparseable file may
  claim the requested id: either recover the claimed ids from the raw `aliases:` / file name, or
  refuse the write while any unparseable issue file exists that could hold it.
- A CLI test covers the repro above.
- The `create` warning for a frontmatter holding a conflict marker says so.
