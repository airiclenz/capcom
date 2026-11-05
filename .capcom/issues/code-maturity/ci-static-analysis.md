---
title: CI runs only go vet; add staticcheck and govulncheck
status: done
type: chore
priority: 1
labels: [maturity, ci]
created: 2026-11-04T18:27:23Z
closed: 2026-11-05T00:13:29Z
reason: "Resolved by plan p1-maturity-plan item 3 (archived at 1ac8d9d): CI lint job and make lint run staticcheck 2026.2.2 and govulncheck v1.9.0, tree clean, CONTRIBUTING names both."
---

`.github/workflows/ci.yml` runs `go vet ./...` and the tests, nothing else. Bugs staticcheck
catches (unused results, wrong `fmt` verbs, impossible conditions, deprecated API use) and
known vulnerabilities in the module graph (for example in `modernc.org/sqlite`,
`golang.org/x/sys`, `golang.org/x/text` or the charm.land TUI stack) reach a release unseen.

What to do:

- Add a job `lint` (ubuntu-latest only) to `.github/workflows/ci.yml`, after the `test` job's
  shape: `actions/checkout@v4`, `actions/setup-go@v5` with `go-version-file: go.mod`, then
  `go run honnef.co/go/tools/cmd/staticcheck@<pinned version> ./...` and
  `go run golang.org/x/vuln/cmd/govulncheck@<pinned version> ./...`. Pin exact versions, never
  `@latest`, so CI does not break on a tool release.
- Fix every finding staticcheck reports on the current tree in the same change, or, for a
  finding that is a deliberate choice, add a `staticcheck.conf` at the repository root that
  disables that one check with a comment saying why. No `//lint:ignore` without a reason.
- If govulncheck reports a reachable vulnerability, bump the affected module in `go.mod`
  (`go get <module>@<fixed version>`, `go mod tidy`) and run the affected package's tests.
- Add a `lint` target to the `Makefile` running the same two commands, so the check is
  reproducible locally.
- Mention both tools in `CONTRIBUTING.md` where it lists the checks CI runs.

## Why

Static analysis and a vulnerability scan are the cheapest maturity signal a Go project can
show, and govulncheck only reports vulnerabilities in code paths capcom actually calls.

## Acceptance

- `.github/workflows/ci.yml` has a `lint` job that runs staticcheck and govulncheck at pinned versions and fails the workflow on any finding.
- `staticcheck ./...` and `govulncheck ./...` exit 0 on the tree as committed.
- `make lint` runs the same two commands.
- `CONTRIBUTING.md` names both tools among the CI checks.
