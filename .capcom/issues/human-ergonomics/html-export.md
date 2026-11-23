---
title: No way to share the issue and Note state as static pages
status: done
type: feature
priority: 3
plan: 2026-11-23 - 00 - agent-eval-ci-html-export-cli-split-plan
created: 2026-10-19T14:54:45Z
closed: 2026-11-23T08:04:33Z
reason: "Resolved by plan '2026-11-23 - 00 - agent-eval-ci-html-export-cli-split-plan' (items 4-6), archived in bb09cad: capcom export --html <dir> writes a deterministic, JavaScript-free static site and refuses a target inside .capcom/."
---

Sharing the state of a repository's work with someone who does not run `capcom ui` means
pointing them at raw markdown files. The files read fine one at a time but give no overview.

Wanted: `capcom export --html <dir>` writes a static, self-contained site: the Ready list, the
Issue tree with status, one page per Issue with its Log and dependencies, and the Knowledge
tree with one page per Note. No JavaScript is required to read it, and it never writes inside
`.capcom/`.

## Acceptance

- The export of this repository opens in a browser from the file system with working links.
- Running it twice produces identical files.
- Writing into a directory inside `.capcom/` is refused.
