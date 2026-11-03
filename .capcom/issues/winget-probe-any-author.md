---
title: The winget probe trusts any pull request with a matching title
status: open
type: bug
priority: 2
plan: 2026-11-03 - 00 - audit-urgent-fixes-plan
created: 2026-11-03T08:23:42Z
---

`probeWinget` (`cmd/release/winget.go:60`) counts any open winget-pkgs pull request whose title
names `AiricLenz.Capcom` and the version as submitted, and any title naming the package as a
first submission in flight. `OpenPulls` (`cmd/release/channelrepos.go`) merges the signed-in
account's own pull requests with `searchPulls`, a `gh pr list --search` that is not filtered by
author, and winget-pkgs accepts pull requests from anyone.

Wanted: accept only pull requests the signed-in account authored, either by dropping the
unfiltered search (the viewer query already finds the account's own) or by filtering it with
`--author @me` or the author login from `--json author`.

Source: docs/reviews/code-audit-2026-11-03.md.

## Why

Anyone can open a pull request titled "New version: AiricLenz.Capcom version 0.15.0" before the
maintainer publishes; publish then skips the winget step as done, or parks it as setup-pending,
and the real submission never happens.

## Acceptance

- A probe fed an open pull request with a matching title by another author reports the winget step pending, not done or setup-pending.
- The maintainer's own matching pull request still reports done.
- Fixed before the next release is cut.
