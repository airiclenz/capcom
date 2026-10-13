---
title: "Before cutting a release: use release-capcom, never tag or bump by hand"
labels: [release]
refs: [docs/releasing.md, .agents/skills/release-capcom/SKILL.md, cmd/release]
---

Releases are cut locally with the `release-capcom` skill (`.agents/skills/release-capcom/`), which
drives `go run ./cmd/release`. `release-notes-capcom` drafts the notes without publishing. Never
tag, bump `VERSION` or edit a CHANGELOG release heading by hand: `prepare` and `publish` do that.

`docs/releasing.md` covers the prerequisites, the one-time setup, the verbs and resuming a failed
publish. ADR 0008 records the design.

`cmd/release` is a dev tool, never shipped. It imports nothing from `internal/`. Its tests are the
one exception to ADR 0007: apart from the version-bump script test (`versionbump_test.go`, see Note
`adr-0008-amend-bumptest`), they never run git, gh, go or komac. What the verbs ask of the capcom
checkout and its GitHub repository goes through the `repo` seam (`repo.go`), held as plain state in
`memRepo`; only `execRepo`'s own tests pin its argv. What they ask of the tap, the bucket and
winget-pkgs goes through the `channelRepos` seam (`channelrepos.go`) the same way, held in
`memChannelRepos`; only `execChannelRepos`' own tests pin its argv. The toolchain, komac,
`gh auth token` and brew are faked through the runner seam (`runner.go`).
