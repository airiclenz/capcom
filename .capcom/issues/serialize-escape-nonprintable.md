---
title: Serialize writes non-printable runes raw and breaks the round-trip
status: open
type: bug
priority: 2
created: 2026-10-11T14:07:46Z
---

`issue.Serialize` writes YAML-non-printable runes such as U+FFFE and U+FFFF
raw: `needsEscape` covers only control characters, U+2028/U+2029 and the BOM.
So a frontmatter value containing U+FFFE (e.g. a `title`) parses, but its
serialisation fails to re-parse with "control characters are not allowed",
which breaks the byte-stable round-trip.

Found 2026-10-11, implementing plan `2026-10-11 - 00 - capcom-v1-issues-plan`, item 4 (Issue file
format).

## Acceptance

- `issue.Serialize` escapes every rune outside YAML's printable set.
- A round-trip test covers U+FFFE and U+FFFF.
