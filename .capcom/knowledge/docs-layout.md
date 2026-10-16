---
title: "Before reading or changing a design: where the glossary, ADRs, plans and handoffs live"
labels: [docs]
refs: [CONTEXT.md, docs/adr, docs/design, docs/plans, docs/handoffs, docs/site, docs/site/reference/file-format.md, docs/site/reference/cli, cmd/gendocs]
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
- `docs/site/`: the user manual, rendered at https://airiclenz.com/capcom/ from a pinned capcom
  ref; GitHub renders the same files. Every page has YAML frontmatter with `title`, a one-sentence
  `description` and `sidebar: { order: <n> }`, and no body H1. Pages link each other with
  relative `.md` links, use the glossary's terms and never pin a capcom version number.
- `docs/site/reference/file-format.md`: the full reference for the files under `.capcom/`, and for
  the two files `capcom ui` keeps outside the repository: its per-user config `config.yaml` and
  its state file `state.yaml`.
- `docs/site/reference/cli/`: the CLI reference, one page per command, generated from the command
  tree's help text and flags. Never edit it by hand: after changing a command, a flag or its help
  text, run `go run ./cmd/gendocs` and commit the result. `TestReferenceUpToDate` in
  `internal/clidoc` fails while it is stale.
