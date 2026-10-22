---
title: "Decision records: why capcom works the way it does, one Note per record with its sections as children"
---

capcom's decision records: the choices that shape how it works, and the options weighed against
them. Each record is a Note `adr-NNNN` whose body opens with its Status line and a short summary
that gives the reading order of its children: the decision, any dated amendments and the rejected
options, each verbatim. A citation "ADR NNNN" resolves to its record.

The records:

- `adr-0001`: The directory tree is the hierarchy
- `adr-0002`: Ids are chosen slugs, and old names live on as aliases
- `adr-0003`: Statuses are configurable, and behaviour reads only their category
- `adr-0004`: Issues are versioned with the code, and a central store stays possible
- `adr-0005`: The Tree lock is a token with a heartbeat, not a held process
- `adr-0006`: Migration is agent-written files checked by capcom, not an importer
- `adr-0007`: Tests use real temp files, SQLite and git as local stand-ins
- `adr-0008`: The repo owns its release pipeline: two committed skills write the notes, a Go dev tool publishes
- `adr-0009`: The store declares its format version, and a binary refuses a store newer than it understands
- `adr-0010`: Knowledge holds a glossary and decision records as Note trees
- `adr-0011`: A per-user config holds personal settings, created only by capcom ui
- `adr-0012`: The CLI and the UI share one write path in an ops package
- `adr-0013`: capcom ui shows the Tree lock and asks before writing while another session holds it
- `adr-0014`: capcom ui remembers its layout in a machine-written state.yaml beside the hand-owned config.yaml
- `adr-0015`: capcom ui remembers its tab and selections per worktree in a ui-state.yaml under the git dir
- `adr-0016`: An agent session keeps its Lock token in a token file keyed by worktree and purpose under the git common dir
