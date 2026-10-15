---
title: "ADR 0012 decision: the write glue leaves internal/cli for a shared package that both front ends call"
---

The domain packages already hold the rules: `issue` mutates an Issue in memory (`SetStatus`,
`Close`, `Reopen`, `AppendLog`), and `nodestore` writes it atomically behind an ETag check. What
joins them sits in `internal/cli` and is private there: `editIssue` and `editNode` resolve a
reference and run the edit, `closeResult` closes and computes the issues the close made ready, and
the dep and log commands carry their own glue. A second front end, `capcom ui`, needs exactly
those operations.

**The glue moves to a shared package.** A package below `cli` (working name `internal/ops`) takes
an open store and index and offers one function per write: set status, claim, close, reopen, add a
log line, and the dep and node edits the CLI needs. Each takes an optional if-match ETag and
returns a typed result (the new ETag, the newly ready issues for a close), never rendered text.
`internal/cli` calls it and keeps only flag parsing and output; `internal/ui` calls it and keeps
only its prompts and redraw.

**One code path, so the two front ends cannot drift.** A rule added to closing (a reason check, a
new side effect) lands in one place and applies to both. The CLI's tests keep covering the
operations through `runCLI`; the package gains direct tests only where the CLI cannot reach a
case.

**Reads do not need this.** The UI reads straight from `index`, `briefing`, `tree` and `issue`,
which are already public within the module.
