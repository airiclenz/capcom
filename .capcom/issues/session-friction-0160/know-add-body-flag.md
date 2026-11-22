---
title: know add has no --body, so every Note body goes through a temp file
status: open
type: feature
priority: 3
labels: [agent-ergonomics]
plan: 2026-11-22 - 01 - session-friction-followups-plan
created: 2026-11-22T17:47:25Z
---

`capcom create` takes `--body` and `--body-file`; `capcom know add` takes only
`--body-file` (`internal/cli/know_add.go`). `know update` has `--body-append` but no
`--body` either.

Seen: 93 `know add` calls, nearly all preceded by `cat > <scratch>/x.md <<'EOF' ... EOF` to
make the file, when the agent did not think of `--body-file -`.

Wanted: `--body <text>` on `know add`, taken literally as on `create`, and refused together
with `--body-file`. Decide whether `know update` gets it too, beside `--body-file`.

## Acceptance

- `capcom know add --id x --title T --body "text"` writes the Note with that body.
- `--body` with `--body-file` fails with a clear message; `--help` shows an example.
