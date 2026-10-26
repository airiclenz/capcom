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
   failing verify. The same example is in `capcom know add --help`, so the generated reference
   carries it too.

The generated CLI reference is otherwise current: `go run ./cmd/gendocs` diffs clean.

Not every `--ref` in the docs is meant to resolve here: `docs/site/concepts/knowledge.md:42` refs
`internal/auth/refresh.go#func RefreshToken` as a made-up example of another project.

## Acceptance

- No hand-written page pins a capcom version.
- The README's and `know add --help`'s `--ref` example names a file that exists in this repository.
- A test fails when a `--ref` in the README or in a command's help names a path missing from this
  repository; examples of other projects in docs/site are not checked.
