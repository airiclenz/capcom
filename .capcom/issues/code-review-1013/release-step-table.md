---
title: "cmd/release: one step table and one file per channel"
status: open
type: task
priority: 3
deps: [release-no-downgrade, release-draft-probe, release-gate-binding, channel-push-failure-test, release-sums-check]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
---

In `cmd/release`, `publishSteps` and `publishActions` are parallel tables joined by name, with one
positional assumption left (`steps[0]` as the tag step). They cannot silently mis-pair, but two
tables per step is one too many. Channel knowledge is spread by verb across four files: the Scoop
manifest is rendered in `render.go`, parsed in `probes.go` and parsed again in `smoke.go`, so a
new channel touches four or five files. Strength: Strong.

Wanted: one `step` record, `{name, probe, describe, converge, smoke, doneDetail}`. Each channel
lives in its own file (`homebrew.go`, `scoop.go`, `winget.go`), `publish.go` keeps only the
orchestration, and `smoke.go` loops over the channels' smoke checks. This replaces the pure-move
split of `smoke.go` and `publish.go` (`split-release-files`, not filed). It waits for the release
fixes (`release-no-downgrade`, `release-draft-probe`, `release-gate-binding`,
`release-sums-check`) and `channel-push-failure-test`, so it moves fixed, tested code.

ADR 0008's rules hold: the gate has no bypass, every `gh release` names `--repo airiclenz/capcom`,
verb tests never run git, gh, go or komac, and a channel with pending setup is skipped. Release
tooling is hard to exercise end to end, so keep
`TestPublishConvergesFromRemoteStateOnceTheTagExists` and the argv-pinning `execRepo` tests green
throughout.

## Acceptance

- One step table, with no positional `steps[0]` lookup.
- `homebrew.go`, `scoop.go` and `winget.go` each hold their channel's render, probe, converge and
  smoke code.
- `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./cmd/release/` passes, including
  `TestPublishConvergesFromRemoteStateOnceTheTagExists` and the `execRepo` tests.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-11.
