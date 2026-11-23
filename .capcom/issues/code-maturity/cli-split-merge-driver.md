---
title: Move the Issue-file merge out of internal/cli into internal/mergedriver
status: done
type: task
priority: 3
labels: [maturity, refactor]
plan: 2026-11-23 - 01 - internal-cli-split-plan
created: 2026-11-23T08:01:16Z
closed: 2026-11-23T08:56:55Z
reason: Resolved by plan 2026-11-23 - 01 - internal-cli-split-plan, item 5 (archived in b56a611)
---

The git merge driver's work — read base, ours and theirs, merge the Log by entry and the rest
with git's text merge, write ours — sits in `internal/cli/merge_driver.go` (`mergeIssueFile`,
`gitTextMerge`). Move it to a new package `internal/mergedriver`, with direct unit tests;
cli keeps the argument parsing and the wording. Item 5 of the plan.

## Why

The merge is file and git logic its tests can drive directly instead of through runCLI.

## Acceptance

- `internal/mergedriver` merges with direct tests; `internal/cli` defines neither function
- merge-driver exit codes, failures and hints stay byte-identical
