---
title: AGENTS.md read has no regular-file check or size cap; prime can hang or OOM
status: open
type: bug
priority: 1
deps: [guarded-file-reader, briefing-panic-test]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:14Z
---

The `AGENTS.md` read (`readGuide`) has no regular-file check and no size cap. It is reached from
`check.Run`, from `briefing.readHealth` (so from `capcom prime` at every session start), from the
refresh after every Note write and from `capcom agents`. Severity: High (security).

Trigger: a committed `AGENTS.md -> /dev/zero`, `/dev/urandom`, a FIFO or `/dev/tty`.
`capcom prime` then grows memory until it is OOM-killed, or blocks forever; the `recover` in
`Compose` cannot catch either. This breaks ADR 0009 ("prime turns every source failure into a
warning and exits 0") and Compose's contract that a Briefing never costs a session its start. A
link to a directory makes `check.Run` fail as a whole, which drops the Health section.

Wanted: read `AGENTS.md` through the guarded reader with a 1 MiB cap, and turn a refusal into an
`agents.Error` problem, never a command failure.

## Acceptance

- An agents test with `AGENTS.md` as a symlink to a FIFO (the existing `makeFIFO` helper, unix
  only) and to an oversized file asserts an `agents.Error` and no hang.
- A CLI `prime` test with the same fixture asserts exit 0 with a warning.
- `GOMEMLIMIT=2GiB go test ./internal/agents/`, then `./internal/cli/ -run Prime`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-02(a).
