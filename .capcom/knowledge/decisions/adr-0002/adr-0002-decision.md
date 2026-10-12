---
title: "ADR 0002 decision: ids are author-chosen kebab-case slugs, and a renamed id lives on as an Alias"
---

Every id is a short kebab-case slug of two to four words that says what the node is about
(`login-timeout-race`). The author always chooses it; capcom never generates one, and there is no
project prefix, because inside one repository a prefix only lengthens the id, and the index knows
the real id set, so telling an id from an ordinary word needs no prefix. An id may be renamed, and
the old name stays on the node as an **Alias** that every lookup resolves, so a citation in old
text keeps working without being rewritten. The bd import uses the same mechanism: each imported
issue gets a new slug and keeps its bd id (`apogee-qi3`) as an alias. A new id may not reuse an
existing id or alias.
