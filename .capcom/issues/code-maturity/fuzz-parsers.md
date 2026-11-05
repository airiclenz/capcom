---
title: No fuzz tests cover the parsers of hostile input
status: done
type: task
priority: 1
labels: [maturity, testing]
plan: 2026-11-04 - 00 - p1-maturity-plan
created: 2026-11-04T18:27:23Z
closed: 2026-11-05T00:13:29Z
reason: "Resolved by plan p1-maturity-plan items 4-8 (archived at 1ac8d9d): seven fuzz targets, 10-minute soaks clean, every target fuzzed 30s per push in .github/workflows/fuzz.yml (kept off the ADR-0008 release gate)."
---

The repository has no `func Fuzz` anywhere. capcom parses files any branch, merge or agent
can write, and the v0.15.0 fixes (a forged link marker crashing `capcom ui`, a stray code fence
hiding conflict markers from `capcom check`, control bytes reaching the terminal) were all
hostile-input bugs a fuzzer finds in minutes.

Add native Go fuzz tests (`testing.F`), one file `fuzz_test.go` per package, for these entry
points:

| Target | Entry point | Invariants |
| --- | --- | --- |
| `FuzzDecode` | `frontmatter.Decode(path, data, kind, apply)` in `internal/frontmatter` | never panics; an error names `path` |
| `FuzzParseIssue` | `issue.Parse(path, data)` in `internal/issue/file.go` | never panics; when it succeeds, `Parse(path, i.Serialize())` succeeds and serializes to the same bytes (canonical form is a fixed point) |
| `FuzzParseNote` | `knowledge.Parse(path, data)` in `internal/knowledge/note.go` | never panics; same round-trip property with the Note serializer `capcom fmt` uses |
| `FuzzParseConfig` | `config.Parse(path, data)` in `internal/config/parse.go` | never panics; an error names `path` and a line |
| `FuzzParseApply` | `batch.Parse(name, data)` in `internal/batch` | never panics; an error names `name` |
| `FuzzConflictMarkers` | `rawfm.ConflictMarkers(data)` and `rawfm.Aliases(data)` in `internal/rawfm` | never panics; every returned marker's line exists in `data` |
| `FuzzMergeFile` | `issue.MergeFile(base, ours, theirs, textMerge)` in `internal/issue/merge.go`, with a pure-Go textMerge stub that concatenates or returns a conflict | never panics |

Seed each corpus with `f.Add` from the existing fixtures (`internal/issue/testdata/*.md`,
this repository's own `.capcom/` files read at test time, the default `config.yaml`) plus the
hostile cases already fixed (unclosed fence, CRLF, NUL and escape bytes, a key repeated,
a frontmatter fence inside a code block).

Run each target for a short budget in CI: add a job `fuzz` (ubuntu-latest) with a matrix over
`package:target` pairs running
`go test -run='^$' -fuzz='^<Target>$' -fuzztime=30s ./internal/<package>`. Go fuzzes one target
per invocation, hence the matrix. Commit any crasher the run finds under the package's
`testdata/fuzz/<Target>/` so it becomes a regression case, and fix the bug it shows in the
same change, with a CHANGELOG entry.

On the Raspberry Pi host fuzz with `GOMEMLIMIT=2GiB` and `-parallel=2`.

## Why

Every input capcom reads may come from an untrusted branch or a confused agent; fuzzing is
the systematic way to find the panics and mis-parses that so far have been found one at a
time after release.

## Acceptance

- The seven fuzz targets in the table exist and each holds its invariants.
- Each target runs 10 minutes locally with no new failure; any crasher found is committed under `testdata/fuzz/` and fixed.
- `.github/workflows/ci.yml` has a `fuzz` job that runs every target for 30s and fails on a crash.
- Plain `go test ./...` still runs the seed corpora as ordinary tests and passes.
