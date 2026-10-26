---
title: The UI page pins v0.7.0 and the README refs a missing file
status: open
type: chore
priority: 3
created: 2026-10-26T20:06:45Z
---

Two documentation defects, both reproduced.

1. The terminal-UI page hard-codes `v0.7.0` in its sample screen, at
   `docs/site/getting-started/terminal-ui.md:49`, `:70` and `:72`, while `VERSION` is `0.11.0`.
   The Note `docs-layout` says docs/site pages never pin a capcom version number, so the page
   contradicts its own rule.

2. `README.md:155` shows

       capcom know add --id release-steps --title "How a release is cut" --ref docs/release.md

   but there is no `docs/release.md`; the file is `docs/releasing.md`. Run verbatim, capcom
   accepts the write, warns `ref "docs/release.md": no such file or directory`, and the next
   `capcom know verify` fails with exit 1. A reader copying the README's own example lands in a
   failing verify.

The generated CLI reference is current — `go run ./cmd/gendocs` diffs clean — so only the
hand-written pages are affected.

## Acceptance

- No hand-written page pins a capcom version.
- Every `--ref` in a doc example points at a file that exists, checked by the docs test if one
  can cover it.
- A test or check fails when a doc example references a missing path.
