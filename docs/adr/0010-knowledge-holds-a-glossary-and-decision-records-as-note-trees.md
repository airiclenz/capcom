# Knowledge holds a glossary and decision records as Note trees

An agent that needs one term from a 233 KB glossary, or the current rule from a 34 KB decision
record, today reads the whole file or most of it: the cost of a lookup is the size of the file it
lives in, not the size of the answer. apogee measured this before migrating to capcom (101 glossary
terms in one `CONTEXT.md`, 98 ADRs averaging 15 KB, the largest 34 KB). The fix is to make the
unit an agent opens small, and **Knowledge** already stores small units in a browsable,
searchable, ref-checked **Tree**. So **Knowledge** may hold a repository's glossary and its
decision records, one **Note** per term and one **Note** per section, and the earlier rule that
"ADRs and the glossary are referenced, never copied" is withdrawn. It is allowed, not required: a
repository whose glossary and ADRs are small keeps them as files, and capcom's own repository does
for now.

**Shape.** A glossary is a top-level **Note** (`glossary`) whose body is the glossary's
introduction, with one child per section and one grandchild per term. A decision record is a
**Note** under a top-level `decisions` **Note**: its id is `adr-NNNN`, its **Title** is the
record's title, its body opens with the prose `Status:` line and a summary of three to five lines,
and its children are the record's sections verbatim (`adr-0076-decision`, `adr-0076-rejected`,
`adr-0076-amend-0908`). A term's **Title** is `<Term>: <gist>`, so an outline of a section reads as
a cheat sheet and a search hit often answers the question without a `show`. Children list in path
order, which is alphabetical; the parent's summary carries the reading order where it matters.

**Notes may be edited by hand.** The rule that **Notes** change only through capcom is withdrawn.
A glossary and decision records are long prose that people and skills edit in place, and the rule
guarded nothing a check cannot: a hand edit changes the file's bytes, so its etag changes and
`--if-match` still refuses a stale write, and `capcom check` reports bad frontmatter with
`path:line`. The write verbs stay as the scripted path.

**Ids are one to four words.** ADR 0002's floor of two words is lowered to one, for every **Id**,
so a one-word term (`turn`, `driver`) is its own handle; the 40-character ceiling stays. A term
longer than four words gets a shortened **Id** and keeps its full slug as an **Alias**, so the slug
of any term resolves.

**A citation is its own lookup key.** "ADR 0076" resolves as `adr-0076` and a bold `**Floor
guard**` as `floor-guard`, so the thousands of citations already in code and prose need no
rewrite. **Notes** cite each other by **Id**, which a move cannot break. Human-facing markdown
outside `.capcom/` keeps clickable path links, at the risk of a later move breaking them; capcom
does not check those links.

**Retired and Supersession.** A **Note** gains two optional keys: `retired: true` marks a **Note**
whose content no longer holds, and `superseded-by:` lists the **Ids** of the **Notes** that replace
it, checked to exist. A record replaced in whole is retired and names its successor; one replaced
in part stays current, names the successor, and says in its body what still holds; a retired
glossary term stays in its section. `know tree` and `know search` leave retired **Notes** out
unless asked for them. The two keys raise the **Format version** to 3 (ADR 0009).

**Migration is a script plus agent-written prose, checked by capcom.** Cutting a file at its terms
and headings is mechanical and runs once, so it is a throwaway script in the migrating
repository's plan, not a capcom verb; the gists and summaries no script can write are agent work;
`capcom check` validates the result, as ADR 0006 has it.

## Considered Options

- **Index the documents in place.** Files stay canonical; capcom parses headings and terms into
  its index and prints one section on request (`doc show CONTEXT.md#Driver`), with output budgets.
  Rejected: it keeps one 233 KB file that humans and agents without capcom still read whole, adds
  a second addressing scheme beside **Ids**, and teaches capcom each repository's heading
  conventions. Splitting into files serves every reader, and capcom's **Tree** already browses it.
- **Split into plain files outside `.capcom/`.** The same split under `docs/`. Rejected once
  hand-editing was allowed: the files are equally browsable in an editor, and **Notes** add
  ranked search, **Refs** checked against the code, **Aliases** across renames, and retired
  **Notes** hidden by default.
- **One summary Note per ADR, files canonical.** Cheaper to migrate, but the summary sits in a
  different store from the text it summarises and drifts silently: capcom notices a missing ref,
  not a changed file. A record whose summary is the parent of its sections keeps both together.
- **Status as labels.** Free-form and unvalidated, and nothing hides a labelled **Note** by
  default, so every agent must remember to filter.
- **A configurable lifecycle list with categories**, mirroring issue statuses (ADR 0003). capcom
  acts only on whether a **Note** is current, so the value names inform without driving
  behaviour; partial supersession is prose under any scheme. A boolean and a link carry the same
  behaviour with no config key.
- **An `order:` key for children.** Lookups go by **Id** and search, not by position, and an
  editor's file tree is alphabetical anyway.
- **A `know split` verb.** It would automate only the mechanical half of a once-only migration.
