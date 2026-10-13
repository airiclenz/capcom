---
title: "cmd/release: seam for tap, bucket and winget repositories"
status: open
type: task
priority: 3
deps: [release-step-table]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:44Z
---

The capcom repository has a semantic seam in `cmd/release` (`repo`: `execRepo` plus `memRepo`),
but the tap, the bucket and winget-pkgs are reached through raw `gh`, `git` and `komac` argv, so
the tests parse command strings (`fakeRemote`, `smokeFake`), and the winget listing call is
written twice (in the probe and in the channel converge). Strength: worth exploring.

Wanted: a `channelRepos` interface with `Exists(repo)`, `ReadFile(repo, path)`,
`PublishFile(repo, path, content, subject) (changed bool, err)` and `OpenPulls(repo, query)`, and
two adapters, `execChannelRepos` and `memChannelRepos`; komac and brew stay on the `runner` seam.
Only `execChannelRepos`'s own tests may pin argv, mirroring the `execRepo` rule. It makes
failure injection for `channel-push-failure-test` trivial and is the home of
`winget-probe-search`'s non-search read. ADR 0008's test-seam amendment: this adds a seam beside
`repo` and `runner` and does not change the policy.

## Acceptance

- `channelRepos` has the two adapters, and probe, converge and smoke tests describe channel state
  through `memChannelRepos` instead of parsing argv.
- The winget open-pull-request listing is made in one place.
- `go vet ./...` is clean and `GOMEMLIMIT=2GiB go test ./cmd/release/` passes.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, A-12.
