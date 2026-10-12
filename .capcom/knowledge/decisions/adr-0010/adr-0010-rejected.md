---
title: "ADR 0010 rejected options: indexing in place, plain split files, summary Notes, status labels, a lifecycle list, an order key and a split verb"
labels: [background]
---

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
- **Status as labels.** Free-form and unvalidated, and capcom hides a labelled **Note** by
  default only for the one label it knows, `background`, so every agent must remember to filter
  any other status label.
- **A configurable lifecycle list with categories**, mirroring issue statuses (ADR 0003). capcom
  acts only on whether a **Note** is current, so the value names inform without driving
  behaviour; partial supersession is prose under any scheme. A boolean and a link carry the same
  behaviour with no config key.
- **An `order:` key for children.** Lookups go by **Id** and search, not by position, and an
  editor's file tree is alphabetical anyway.
- **A `know split` verb.** It would automate only the mechanical half of a once-only migration.
