---
title: "ADR 0001 decision: a node is a file named after its id, with its children in a sibling directory of the same name"
---

Issues and Knowledge are both trees, and capcom stores each tree as the directory tree itself: a
node is a markdown file named after its id, and its children live in a sibling directory of the
same name (`issues/login-flow.md` with `issues/login-flow/login-timeout-race.md`). We chose this
over flat files with a `parent:` field because the structure then reads straight off `ls` and
`tree` without capcom, and the one hierarchy principle covers both stores. Ids stay unique across
the whole tree and independent of the path, so references by id survive a move. capcom moves a node with a plain file-system rename and
never runs git; git records it as a rename in the diff. A node has exactly one parent; every other
relationship between Issues is a Dependency.
