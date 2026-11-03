---
title: The busy-index branch of the briefing is untested
status: open
type: task
priority: 3
plan: 2026-11-03 - 02 - eval-harness-and-test-gaps-plan
created: 2026-11-03T08:23:42Z
---

The `errors.Is(err, index.ErrBusy)` branch of `readWork` (`internal/briefing/briefing.go:112`)
has no test; the only index-failure test asserts the "; run capcom reindex" suffix, never the
busy path that must not carry it. Several agents share a worktree index at session start, so
busy is the realistic failure, and a regression would tell agents to rebuild a healthy index.

## Acceptance

- A test holds the index busy (or stubs the open to return `ErrBusy`) and asserts one SectionIndex failure without the reindex hint, with locks and check still composed.
