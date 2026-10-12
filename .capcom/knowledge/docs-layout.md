---
title: "Before reading or changing a design: where the glossary, ADRs, plans and handoffs live"
labels: [docs]
refs: [CONTEXT.md, docs/adr, docs/design, docs/plans, docs/handoffs, docs/format.md]
---

Read the relevant source before changing what it covers:

- Note `glossary`: the domain glossary, one Note per term (Issue, Tree, Alias, Ready, Lock, ...).
  Use its terms in code, docs and messages; edit a term's file in place.
- Note `decisions`: the architectural decision records, one `adr-NNNN` Note each. Read the
  relevant record before changing what it covers; edit records in place.
- `CONTEXT.md` and `docs/adr/README.md` only point to the `glossary` and `decisions` Notes; the
  terms and records themselves live in Knowledge.
- `docs/design/`: research and design notes. `docs/design/research-notes.md` is the pre-v1
  research record, not current design.
- `docs/plans/`: implementation plans. Finished ones move to `docs/plans/archived/`.
- `docs/handoffs/`: handoff documents between sessions. Local to each clone:
  `docs/handoffs/.gitignore` ignores everything, so handoffs are never committed and other clones
  do not see them.
- `docs/format.md`: the full reference for the files under `.capcom/`.
