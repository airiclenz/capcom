---
title: A call log line over 64 KiB fails the agenteval read
status: open
type: bug
priority: 3
plan: 2026-10-29 - 01 - agenteval-cleanup-plan
created: 2026-10-29T18:08:41Z
---

In `cmd/agenteval/score.go:45`, `readCallLog` reads the call log with a `bufio.Scanner` at its default 64 KiB line limit. A logged capcom call longer than that, such as one with a huge `--body`, fails `scanner.Err()` (`cmd/agenteval/score.go:57`) with `bufio.Scanner: token too long`, so the task cannot be scored.

## Why

An agent that files an issue with a long body is a normal run, not a malformed log; the harness should score it.

## Acceptance

- A call log holding a line longer than 64 KiB reads back as that call, with its arguments intact.
- A test in `cmd/agenteval/score_test.go` covers such a line.
