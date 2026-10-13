---
title: Extract internal/cli/output
status: done
type: task
priority: 3
deps: [split-know-go, cli-flags-file]
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:11:43Z
closed: 2026-10-13T18:09:22Z
reason: Resolved by plan 2026-10-13 - 00 - code-and-architecture-review-fixes-plan (archived in 12751c0)
---

The output module (`internal/cli/output.go`: `Result`, `KVRecord`, `Envelope`, `Renderer` and the
text, JSON and KV renderers; the top of `internal/cli/errors.go`: `Error`, `Code` and the exit
codes) is already deep, with a small interface (Result in, bytes out), but it shares a package with
28 command files, so nothing stops a command from reaching past that interface. Move it to a new
package `internal/cli/output`. The white-box tests that call `newRenderer` and `storeFailure` move
with it and become tests of the new package.

It gives every output rule one home: the `--kv` WARNING/STATUS order, the exit codes and the
sanitiser from `terminal-control-escape`. Decided 2026-10-13: extract `internal/cli/output` only;
`internal/cli` is not split into `issuecmd` and `knowcmd`.

Constraints: `output` imports only `elsewhere` (later `nodestore` and `tree` if the error mapping
moves) and must never import `internal/cli`. Every command stays in package `cli`, so `runCLI`
and the CLI test suite run unchanged. This is a package move, so it touches every command file's
references and exported names may need capitalising; it is still behaviour-preserving. It comes
after `cli-flags-file` and `split-know-go`.

## Acceptance

- `go list -f '{{join .Imports "\n"}}' ./internal/cli/output/` does not list `internal/cli`.
- `go vet ./...` is clean; `GOMEMLIMIT=2GiB go test ./internal/cli/output/`, then `./internal/cli/`,
  pass, with the CLI tests unchanged apart from the moved white-box tests.
- The diff is move-only apart from package qualifiers, capitalised names and import headers
  (`git diff --color-moved=zebra`).

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, P2-14 and A-14.
