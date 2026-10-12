---
title: "ADR 0002 rejected options: generated ids and keeping the legacy bd ids"
---

- **Generated ids (hash or sequence), as bd, Beans and Backlog.md do.** Rejected: they say
  nothing in a plan, a commit or a `ready` listing, and agents then need a guard to force spoken
  ids anyway.
- **Keeping the legacy bd ids as they are.** Simpler to import, but it would keep 225 meaningless
  ids alive in the one place meant to be rid of them.
