---
title: Code and architecture review 2026-10-13
status: open
type: task
priority: 2
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:13Z
---

Parent of the issues filed from the code and architecture review of 2026-10-13
(`docs/reviews/code-and-architecture-review-2026-10-13.md`, section 7). The review verified
defects (two of them High security findings on repository-controlled reads), test gaps on
critical paths, large files to split and deepening refactors. The work runs in phases:

- Phase 0: the hostile-repository reads (`AGENTS.md`, `config.yaml`, Ref symbol files,
  other-branch blobs) and terminal control sequences in text output.
- Phase 1: correctness and concurrency fixes that need no refactor.
- Phase 2: pure file splits, behaviour-preserving, one commit each.
- Phase 3: the deepening refactors, with the fixes they absorb.
- Phase 4: the remaining test gaps.

Each child issue is readable on its own and carries its own acceptance.

## Acceptance

- every child issue is closed

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, section 7.
