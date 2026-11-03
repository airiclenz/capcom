# Recap oversized-warning text

**Goal:** Make the recap oversized-file test tell the `gitx.ErrBlobTooLarge` branch of `load` apart from its generic read-error branch, closing the two open recap issues.
**Date:** 2026-11-03
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 71a58d4

**Sources:**
- `.capcom/issues/audit-2026-11-03/test-recap-hostile-guards.md` (its Log names the gap left open)
- `.capcom/issues/recap-oversized-warning-text.md`
- Notes `cli-test-harness`, `coding-conventions`

**Ratified design calls:** none.

**Standing requirements:**
- skills: coding-standards
- Tests on this host: single package, `GOMEMLIMIT=2GiB`, no `-race` on `./...`.

**Out of scope:**
- Any change to `internal/recap/recap.go` behaviour or warning wording.
- The duplicate-id test (`TestCompute_DuplicateID_KeepsTheFirstFileAndWarnsOfTheSecond`): already meets its Acceptance.

**Regression check (2026-11-03, 71a58d4):**
- all items SAFE

## 1. Assert the oversized-file warning text in the recap test

**What:**
Test-only. `TestCompute_OversizedFile_IsAWarningAndLeftOut` in `internal/recap/recap_test.go` checks only the `recap: ` prefix and `huge.md`. Add an assertion that the warning contains `tree.ErrTooLarge.Error()` (`file is larger than 1 MiB`), the text the `ErrBlobTooLarge` branch of `load` writes. The generic branch would write `gitx.ErrBlobTooLarge`'s text (`blob is larger than the size limit`), which lacks it, so the assertion distinguishes the two. Reference the sentinel's `Error()` rather than a literal string; import `internal/tree` in the test if not already imported. Keep the existing prefix and file-name checks.
**Files:** `internal/recap/recap_test.go`
**Read first:** internal/recap/recap_test.go — TestCompute_OversizedFile_IsAWarningAndLeftOut, newStore, commit; internal/recap/recap.go — load, warn, maxFileSize;
internal/gitx/branches.go — ErrBlobTooLarge; internal/tree/errors.go — ErrTooLarge
**Tests:** the amended `TestCompute_OversizedFile_IsAWarningAndLeftOut`. Bite check: with the `errors.Is(err, gitx.ErrBlobTooLarge)` branch in `load` deleted, the test fails; restore it.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 -run 'TestCompute_OversizedFile' ./internal/recap/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/recap/`
- `go vet ./internal/recap/`
**Closes:** test-recap-hostile-guards; recap-oversized-warning-text
**Commit:** `test(recap): check the oversized-file warning text`
