---
title: capcom depends on the archived gopkg.in/yaml.v3
status: done
type: chore
priority: 2
labels: [maturity, deps]
plan: 2026-11-05 - 00 - p2-maturity-plan
created: 2026-11-04T18:27:23Z
closed: 2026-11-05T10:56:22Z
reason: "Resolved by plan '2026-11-05 - 00 - p2-maturity-plan' (archived in 1a6b7b0) item 1: every import moved to go.yaml.in/yaml/v3, now a direct requirement; tests pass with no expected error text changed."
---

`go.mod` requires `gopkg.in/yaml.v3 v3.0.1`, whose repository is archived and gets no fixes.
Its maintained successor, `go.yaml.in/yaml/v3` (the YAML organisation's fork, same API), is
already in the module graph as an indirect dependency (`v3.0.4`, pulled in through
`github.com/spf13/cobra/doc` by `internal/clidoc`).

The direct importers of `gopkg.in/yaml.v3` today are `cmd/release`, `internal/batch`,
`internal/clidoc`, `internal/config`, `internal/frontmatter`, `internal/hooks`, `internal/ui`
and `internal/userconfig` (find them with
`grep -rl 'gopkg.in/yaml.v3' --include=*.go cmd internal`).

What to do: replace the import path in every file, `go mod tidy` so `go.yaml.in/yaml/v3`
becomes a direct requirement and `gopkg.in/yaml.v3` leaves `go.mod` (it may stay in `go.sum`
if a dependency still needs it), then run each importing package's tests one at a time. Watch
for behaviour differences in error message text: capcom's errors cite `path:line` from YAML
errors (`internal/frontmatter`, `internal/config/parse.go`), so any test that pins an error
string must still pass unchanged; if the new module words an error differently, adapt
capcom's wrapping so the user-visible message stays the same.

## Why

An archived parser sits on capcom's hottest input path: every issue, Note, config and apply
file goes through it, so an unfixed parser bug becomes a capcom bug.

## Acceptance

- No `.go` file under `cmd/` or `internal/` imports `gopkg.in/yaml.v3`.
- `go.mod` lists `go.yaml.in/yaml/v3` as a direct requirement and does not list `gopkg.in/yaml.v3` as a direct one.
- `go test -count=1` passes for every package that imported the old module, with no test's expected error text changed.
