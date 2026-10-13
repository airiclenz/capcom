---
title: "ADR 0008 amendment of 2026-10-12: one hermetic test runs version-bump.sh against a temp git repository"
---

Amended 2026-10-12: one test of `cmd/release` runs real `git`. `versionbump_test.go` drives
`.github/scripts/version-bump.sh`, the script that decides which commits the tag-on-version-bump
workflow tags, against a throwaway local repository in `t.TempDir()`. It isolates git from the
user's and the system's configuration (`GIT_CONFIG_GLOBAL`, `GIT_CONFIG_NOSYSTEM`,
`GIT_CEILING_DIRECTORIES`, fixed author and committer), never reaches the network or `gh`, runs
the script by absolute path, and skips where `bash` or `git` is missing and on Windows, since the
workflow runs only on Linux.

This replaces the sentence in `adr-0008-decision` that "its tests never run `git`": the tests of
the release verbs still never run `git`, `gh`, `go` or `komac`, and go through the `repo`,
`channelRepos` and runner seams as that Note says; the script test is the one exception, and it
touches none of them.
