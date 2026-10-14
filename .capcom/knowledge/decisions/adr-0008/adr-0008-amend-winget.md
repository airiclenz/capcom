---
title: "ADR 0008 amendment of 2026-10-14: an open first winget submission leaves winget setup-pending"
refs: [cmd/release/winget.go]
---

Amended 2026-10-14: while `AiricLenz.Capcom` is not in winget-pkgs and an open pull request
already submits another version of it, the winget step is setup-pending, names that pull request,
and `publish` runs no `komac`. Only once the first submission merges does the next release run
`komac update`.

This adds to the sentence in `adr-0008-decision` that "the first release runs `komac submit`".
Before, every release while the first pull request awaited review ran `komac submit` again, because
the probe matched open pull requests by version only. Releases 0.1.0 to 0.6.0 so left seven rival
new-package pull requests open in `microsoft/winget-pkgs`, and the six older ones were closed by
hand. winget-pkgs needs only the latest version, so a release skipped while the first submission
waits loses nothing.
