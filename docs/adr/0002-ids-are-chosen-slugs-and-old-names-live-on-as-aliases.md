# Ids are chosen slugs, and old names live on as aliases

Every id is a short kebab-case slug of two to four words that says what the node is about
(`login-timeout-race`). The author always chooses it; capcom never generates one, and there is no
project prefix, because inside one repository a prefix only lengthens the id, and the index knows
the real id set, so telling an id from an ordinary word needs no prefix. An id may be renamed, and
the old name stays on the node as an **Alias** that every lookup resolves, so a citation in old
text keeps working without being rewritten. The bd import uses the same mechanism: each imported
issue gets a new slug and keeps its bd id (`apogee-qi3`) as an alias. A new id may not reuse an
existing id or alias.

Amended 2026-10-12: the floor is one word, not two, so a one-word glossary term is its own **Id**
([ADR 0010](0010-knowledge-holds-a-glossary-and-decision-records-as-note-trees.md)). The ceiling
of four words and 40 characters stays.

## Considered Options

- **Generated ids (hash or sequence), as bd, Beans and Backlog.md do.** Rejected: they say
  nothing in a plan, a commit or a `ready` listing, and agents then need a guard to force spoken
  ids anyway.
- **Keeping the legacy bd ids as they are.** Simpler to import, but it would keep 225 meaningless
  ids alive in the one place meant to be rid of them.
