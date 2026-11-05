---
title: The merge driver is tested by examples, not by random concurrent edits
status: done
type: task
priority: 1
labels: [maturity, testing]
created: 2026-11-04T18:27:23Z
closed: 2026-11-05T00:13:29Z
reason: "Resolved by plan p1-maturity-plan items 9-10 (archived at 1ac8d9d): MergeFile property test (5000 iterations clean) and end-to-end git merge property test, CAPCOM_PROPERTY_ITERATIONS controls both."
---

"Issues branch and merge with the code" is capcom's main promise, and the `capcom merge-driver`
(`internal/cli/merge_driver.go`, built on `issue.MergeFile` in `internal/issue/merge.go`, which
merges the Log by entry and hands the rest to `git merge-file`) is what keeps it. Its tests
(`internal/issue/merge_test.go`, the CLI tests) use hand-picked cases, so an interleaving
nobody thought of can still lose a Log entry or leave a file `capcom check` rejects.

Add a property-based test, deterministic per seed (use `math/rand/v2` with a seed printed on
failure, no new dependency), in two layers:

1. **Unit, `internal/issue`:** generate a base issue file, then two independent sequences of
   random edits (append Log entries with distinct dates and texts, change title, priority,
   status, labels, deps, body paragraphs) giving `ours` and `theirs`. Merge with
   `MergeFile` and a real `git merge-file` textMerge (skip with `t.Skip` when `git` is not on
   PATH). Properties: when the merge reports no conflict, the result parses with
   `issue.Parse`, every Log entry of `ours` and of `theirs` appears exactly once and in date
   order, and a field changed on one side only takes that side's value. When it reports a
   conflict, the result holds conflict markers that `rawfm.ConflictMarkers` finds.
2. **End to end, `internal/cli`:** in a temp git repository set up through `runCLI` with
   `capcom init` (which registers the merge driver), create issues on `main`, branch twice,
   run random `capcom log`, `update`, `dep add`, `close` commands on each branch, commit, merge
   both into `main` with `git merge`, and assert that `capcom check` exits 0 or reports only
   `conflict` problems, and that `capcom show <id> --json` lists every Log entry both
   branches added. Follow the Note `cli-test-harness` (no `t.Parallel()`).

Run 200 iterations by default and 5000 under an environment variable
`CAPCOM_PROPERTY_ITERATIONS`, so CI stays fast and a local soak is one variable away. A
failure prints the seed and the three input files.

## Why

Random concurrent edits are exactly what several agents on several branches produce; a
lost Log entry is silent data loss.

## Acceptance

- The unit property test runs 5000 iterations with no failure, and fails with the seed printed when `MergeFile` is deliberately broken (checked once by hand, not committed).
- The end-to-end test merges two branches of random capcom writes and asserts `capcom check` passes and no Log entry is lost.
- `CAPCOM_PROPERTY_ITERATIONS` controls the iteration count; the default run of each test stays under 20s on CI.
- Any bug found is fixed with a CHANGELOG entry.
