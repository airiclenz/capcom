---
title: The real clipboard runner and its timeout are never exercised
status: open
type: task
priority: 3
created: 2026-11-03T08:23:42Z
---

`runProgram` (`internal/clipboard/clipboard.go:216`), the only production runner, and its 2 s
`runTimeout` kill (`:23`, `:178`) are untested; every test injects `Writer.Run`. A regression in
stdin delivery, or in killing a wedged xclip or tmux, would freeze every copy and pass the suite.

## Acceptance

- A test runs the real default runner against a helper (the test binary re-executed through `os.Args[0]` with an env switch) and asserts stdin delivery and that a non-zero exit is returned.
- A helper that sleeps past a shortened timeout is killed with a context error; the timeout is a field or var so the test does not wait 2 s.
