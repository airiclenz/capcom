---
title: "Before cutting a release: use release-capcom, never tag or bump by hand"
labels: [release]
refs: [docs/releasing.md, docs/adr/0008-the-repo-owns-its-release-pipeline.md, .agents/skills/release-capcom/SKILL.md, cmd/release]
---

Releases are cut locally with the `release-capcom` skill (`.agents/skills/release-capcom/`), which
drives `go run ./cmd/release`. `release-notes-capcom` drafts the notes without publishing. Never
tag, bump `VERSION` or edit a CHANGELOG release heading by hand: `prepare` and `publish` do that.

`docs/releasing.md` covers the prerequisites, the one-time setup, the verbs and resuming a failed
publish. ADR 0008 records the design.

`cmd/release` is a dev tool, never shipped. It imports nothing from `internal/`. Its tests are the
one exception to ADR 0007: they fake git, gh, go and komac through its runner seam instead of
running them.
