---
title: Every error message names the file, and the line when known
labels: [conventions]
refs: [internal/cli/errors.go#type Error struct]
---

Every error message names the file, and the line when known. In `internal/cli` this is the
`Error` type: set `Path`, and `Line` when known, and the message prints as `path:line: message`.
