---
title: The manual at airiclenz.com/capcom returns 404
status: open
type: task
priority: 1
labels: [docs]
created: 2026-10-20T06:03:14Z
---

The README, `docs/site/` and the `docs-layout` Note link the user manual at
https://airiclenz.com/capcom/, but on 2026-10-20 that URL returns 404: nothing renders
`docs/site/` yet. Answer engines and agents cite crawlable pages, so the manual's FAQ and
comparison pages do little until they are served, and the README's main docs link is dead.

The site build lives in the separate `airiclenz.com` repository (Astro), which is meant to copy
`docs/site/` from a pinned capcom ref.

## Acceptance

- https://airiclenz.com/capcom/ serves `docs/site/index.md` and every page under it.
- The site serves an `llms.txt` for the manual (https://airiclenz.com/capcom/llms.txt or the site
  root) and each page's markdown source.
- The manual's home page carries JSON-LD `SoftwareApplication` markup (name, operating systems,
  MIT license, download URL).
- The GitHub repository's homepage URL points at the manual.
