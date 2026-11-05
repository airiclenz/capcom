# Contributing to capcom

capcom keeps its own project knowledge as Notes under [`.capcom/knowledge/`](.capcom/knowledge/).
This guide points to them rather than repeating them. With capcom installed, `capcom know tree`
lists them and `capcom know show <id>` reads one; without it, read the markdown files directly.

## Build and test

capcom needs the Go version named in [`go.mod`](go.mod). Its `toolchain` line pins the patched Go
release that CI and releases build with; with the default `GOTOOLCHAIN=auto`, an older local Go
fetches that toolchain and switches to it.

```sh
make build    # static ./capcom: CGO_ENABLED=0 go build -trimpath ./cmd/capcom
make vet      # go vet ./...
make test     # go test -count=1 ./...
make lint     # staticcheck and govulncheck over ./..., at the pins CI uses
make clean    # remove ./capcom
```

Without `make`, as on stock Windows, run the `go` commands in the comments directly; every target
is one of them, and `make lint` is the two `go run` lines in the [`Makefile`](Makefile). `make vet`
and `make test` take `PKG=<package>` to narrow them, for example `make test PKG=./internal/ui/`.
`make build` takes `BUILDFLAGS` and `LDFLAGS` for extra build flags. Releases
never go through `make` (Note `adr-0008-amend-makefile`).

CI runs `go vet ./...` and `go test -count=1 ./...` on Linux, macOS and Windows, with `-race` on
Linux and macOS; the Linux job fails when total coverage drops below `COVERAGE_FLOOR` in
[`.github/workflows/ci.yml`](.github/workflows/ci.yml). Its lint job runs `staticcheck` and
`govulncheck` over `./...` at the versions pinned in its `env` block, the same pins as
`STATICCHECK_VERSION` and `GOVULNCHECK_VERSION` in the `Makefile`; any staticcheck finding or
reachable vulnerability fails it, and a release needs CI green. On a machine with little memory,
such as a Raspberry Pi, test one package at a time with `GOMEMLIMIT=2GiB`
(`GOMEMLIMIT=2GiB make test PKG=./internal/cli/`), and never run `./...` with `-race` (see
[`AGENTS.md`](AGENTS.md)).

The parsers of hostile input carry native fuzz targets, one `fuzz_test.go` per package. Plain
`go test` runs their seed corpora as ordinary tests;
[`.github/workflows/fuzz.yml`](.github/workflows/fuzz.yml) fuzzes each target for 30s on every
push to `main` and every pull request, in a workflow of its own so a fuzz failure never blocks a
release. Go fuzzes one target per invocation; run one locally with

```sh
go test -run='^$' -fuzz='^FuzzParseIssue$' -fuzztime=1m ./internal/issue
```

adding `-parallel=2` and `GOMEMLIMIT=2GiB` on a small machine. A crash writes its input under the
package's `testdata/fuzz/<Target>/` (CI uploads that directory as an artifact): commit that file
with the fix in the same change, so it stays a regression case every `go test` runs.

The stress tests in `internal/cli/stress_test.go` run the real binary as many concurrent
processes, in one worktree and across linked worktrees. They sit behind the `stress` build tag,
so plain `go test` skips them;
[`.github/workflows/stress.yml`](.github/workflows/stress.yml) runs them on Linux and macOS on
every push to `main` and every pull request, apart from `ci.yml` so a failure never blocks a
release. Run them locally with

```sh
CAPCOM_STRESS_WORKERS=4 GOMEMLIMIT=2GiB go test -tags stress -count=1 -run Stress ./internal/cli
```

A failed run logs its seed; `CAPCOM_STRESS_SEED=<seed>` replays its random choices.

The command benchmarks in `internal/cli/bench_test.go` time whole commands, cold and warm index
and one `log` write, on generated stores of 1000 and 10000 issues (`CAPCOM_BENCH_LARGE=1` adds
50000). [`.github/workflows/bench.yml`](.github/workflows/bench.yml) runs them on every push to
`main` for the pushed commit and its parent, and fails when one is more than 25% slower with
p < 0.05 by `benchstat`; like the stress tests, it lives apart from `ci.yml`. Run one locally with

```sh
GOMEMLIMIT=2GiB go test -run='^$' -bench='^BenchmarkLog$' -benchtime=1x ./internal/cli
```

## Conventions and tests

- Code conventions, formatting and error messages: Note
  [`coding-conventions`](.capcom/knowledge/coding-conventions.md) and the Notes below it.
- Tests: Note [`cli-test-harness`](.capcom/knowledge/cli-test-harness.md). CLI tests run every
  command in process against a real temp git repository and SQLite index, and never run in
  parallel.
- Where the glossary, design decisions, plans and handoffs live: Note
  [`docs-layout`](.capcom/knowledge/docs-layout.md). Use the glossary's terms in code, docs and
  messages.
- The user manual lives in [`docs/site/`](docs/site/). Its CLI reference under
  [`docs/site/reference/cli/`](docs/site/reference/cli/) is generated: after changing a command,
  a flag or its help text, run `go run ./cmd/gendocs` and commit the result.
  `TestReferenceUpToDate` fails while the committed reference is stale.
- The README's GIF, [`graphics/demo.gif`](graphics/demo.gif), is filmed with
  [photon](https://github.com/airiclenz/photon) from
  [`graphics/demo/`](graphics/demo/). After a visible change to `capcom ui`, record it again on
  Linux or macOS: `photon setup graphics/demo/storyboards/hero.yaml` once, then
  `until photon record graphics/demo/storyboards/hero.yaml; do :; done` and
  `photon render graphics/demo/storyboards/hero.yaml`.

## Issues

Work is tracked as capcom issues under [`.capcom/issues/`](.capcom/issues/); `capcom ready` lists
the ones ready to work on, and `capcom close <id> --reason <text>` closes one when the work is
done. To report a bug without a change, open a GitHub issue.

## Commits and the changelog

- Commit messages follow Conventional Commits: `feat(know): ...`, `fix(check): ...`, `docs: ...`.
- No AI attribution in commits, pull requests, code or docs: no co-author trailers, no
  "generated with" footers.
- A change users can notice adds a line under `## [Unreleased]` in [`CHANGELOG.md`](CHANGELOG.md)
  ([Keep a Changelog](https://keepachangelog.com/en/1.1.0/)). Write it for users: what now
  behaves differently, with the exact command, flag or message.

## Releases

Releases are cut only by the maintainer, with the `release-capcom` skill. Never tag, bump
`VERSION` or edit a CHANGELOG release heading by hand; the release tool does that. See Note
[`release-process`](.capcom/knowledge/release-process.md) and
[`docs/releasing.md`](docs/releasing.md).

## Security

Report vulnerabilities privately, as described in [`SECURITY.md`](SECURITY.md).
