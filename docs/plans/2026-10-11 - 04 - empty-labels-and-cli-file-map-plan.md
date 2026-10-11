# Empty labels and the internal/cli file map

**Goal:** No write command can store an empty label: empty entries in a label list are dropped,
and validation refuses an empty label before any file is written. `internal/cli` gets a `doc.go`
file map that a structural test keeps up to date.

**Date:** 2026-10-11
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 40691d7

**Regression check (2026-10-11, 40691d7):**
- 1: guard folded
- 2: guard folded (writer decision)
- 3: What corrected (writer decision)

**Sources:**
- `.capcom/issues/know-update-empty-label.md`
- `.capcom/issues/cli-package-file-map.md`
- `~/.claude/skills/coding-standards/SKILL.md` — Go rule: a `doc.go` file map and a structural test once a package passes ~10 files
- Knowledge Notes `cli-test-harness`, `coding-conventions`, `error-messages`

**Ratified design calls** (user, 2026-10-11):
- **Empty label:** dropped silently by every write command (`create --label`, `update --add-label`, `know add --label`, `know update --add-label`), as `know add` already does.
- **Scope:** the plan covers the bug `know-update-empty-label` and the chore `cli-package-file-map`.

**Standing requirements:**
- skills: coding-standards
- Every error names the file, and the line when known (Note `error-messages`).
- CLI tests go through `runCLI`/`runCLIWith`, hermetic, never `t.Parallel()`.
- Pi host: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.

**Out of scope:**
- Trimming whitespace in labels or refusing labels that contain only whitespace.
- Repairing existing files that already hold an empty label: `capcom check` already reports them.
- Adding a file map to any package other than `internal/cli`.

## 1. Drop empty entries when labels are merged — ✅ DONE (2026-10-11)

**What:** This fixes the bug `know-update-empty-label`. `capcom know update --add-label "x,,y"`
writes `labels: [x, "", y]`, which the parser rejects, so the Note stops resolving. The same
happens with `capcom create --label "x,,y"` and `capcom update --add-label "x,,y"` on issues.

**Regression guard.** Every doc comment that restates the label merge rule
(`grep -rn "unless it is already" internal/`: `edit.Labels` in `internal/edit/edit.go` and
`Issue.WithLabels` in `internal/issue/mutate.go`) must also say that empty entries of `add` are
dropped; `internal/issue/mutate.go` is part of this item.

**Goal:** Running `create --label`, `update --add-label` or `know update --add-label` with `"x,,y"`
writes `labels: [x, y]`. The written file parses, `show` resolves it, and `capcom check` passes.

**Approach (assumed at the header base):** `edit.Labels` (`internal/edit/edit.go`) skips an empty
entry of `add` the same way it skips one that is already there, and its doc comment says so. That
one change covers `issue.Issue.WithLabels` (used by `create` and `update`) and `applyNoteUpdate`
in `internal/cli/know_update.go`. Refs also merge through `edit.Labels` and are dropped the same
way; this is safe because `knowledge.ValidateRef` already refuses an empty ref. `newNote` in
`internal/cli/know.go` has its own loop that filters empty and repeated labels; replace it with
`edit.Labels(nil, options.labels, nil)` so there is only one rule.

**Files:** internal/edit/edit.go, internal/edit/edit_test.go, internal/issue/mutate.go, internal/cli/know.go, internal/cli/know_update_test.go, internal/cli/write_test.go
**Read first:** internal/edit/edit.go — Labels; internal/cli/know_update.go — applyNoteUpdate (ValidateRef runs on addRefs before the merge), writeNoteFile; internal/issue/mutate.go — Issue.WithLabels; internal/cli/create.go — newIssue;
internal/cli/update.go — applyUpdate, writeIssueFile; internal/cli/know.go — newNote (slices stays imported for the SortFunc at the tree listing); internal/cli/write_test.go — newWriteStore, readStoreFile, mustRun;
internal/cli/know_update_test.go — readNoteFile, newKnowStore (know_test.go)

**Tests:**
- `internal/edit/edit_test.go`: `TestLabels_EmptyAdd_Dropped`, which merges `["x", "", "y"]` into `nil` and expects `["x", "y"]`.
- `internal/cli/know_update_test.go`: `TestKnowUpdate_AddLabelWithEmptyEntry_DropsIt`, which runs `know update <id> --add-label "x,,y"` and then `know show <id>` and `check`. Both must exit 0, and the Note must carry `labels: [x, y]`.
- `internal/cli/write_test.go`: the same check for `create --label "x,,y"` and for `update <id> --add-label "x,,y"`.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/edit/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run 'KnowUpdate|KnowAdd|Create|Update'`
- `go vet ./internal/edit/ ./internal/cli/`

**Closes:** know-update-empty-label

**Commit:** `fix(edit): drop empty labels when merging so no write stores one`

## 2. Validation refuses an empty label — ✅ DONE (2026-10-11)

NOTES (2026-10-11): both Validate functions report a list with empty entries once, not once per empty entry, citing the labels key line (the parser also stops at the first one); issue.Validate tests it with slices.Contains(issue.Labels, ""), which is the exact `== ""` comparison the guard requires.

**What:** This fixes the gap the bug exposed. `issue.Validate` and `knowledge.Validate` accept an
empty label even though the parser rejects one, so a write can produce a file that cannot be read.

**Regression guard.** the empty-label check in both Validate functions tests `label == ""` exactly as the parsers do (internal/issue/file.go list reader, internal/knowledge/note.go list reader), never strings.TrimSpace, so check reports nothing for whitespace labels that parse today. Place the labels check between the priority and created checks in `issue.Validate`, and the Note labels loop between the title check and the refs loop in `knowledge.Validate`; extend both Validate doc comments with "every label must be non-empty".

**Goal:** `issue.Validate` and `knowledge.Validate` each report an empty label as a problem. The
problem names the file and reads `labels: every entry must be non-empty text`, the parser's wording.

**Approach (assumed at the header base):** In `internal/issue/validate.go`, the `validator` gains
a labels check that goes through `v.add(keyLabels, …)`. In `internal/knowledge/note.go`,
`Validate` gains a loop over `note.Labels` that is built like its existing loop over `note.Refs`.
Parsed files never hold an empty label, so `capcom check` reports nothing new.

**Files:** internal/issue/validate.go, internal/issue/validate_test.go, internal/knowledge/note.go, internal/knowledge/knowledge_test.go
**Read first:** internal/issue/validate.go — Validate, validator.add; internal/knowledge/note.go — Validate, parser.list (wording source), keyLabels; internal/issue/file.go — parser.list, Serialize key order;
internal/issue/validate_test.go — validIssue, problemStrings, TestValidate_ReturnsEveryProblem; internal/knowledge/knowledge_test.go — TestValidate_NoTitleOrPathlessRef_ReportsBoth, notePath;
internal/check/check.go — issue/Note Validate calls (parsed files only)

**Tests:**
- `internal/issue/validate_test.go`: `TestValidate_EmptyLabel_Reported`.
- `internal/knowledge/knowledge_test.go`: `TestValidate_EmptyLabel_Reported`.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/issue/`
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/knowledge/`
- `go vet ./internal/issue/ ./internal/knowledge/`

**Commit:** `fix(validate): report an empty label the parser would reject`

## 3. Add the internal/cli file map and structural test — ✅ DONE (2026-10-11)

**What:** `internal/cli` has 24 non-test files, well past the ~10-file threshold, but no file map. This item resolves the
chore `cli-package-file-map`.

**Goal:** `internal/cli/doc.go` holds the package comment and maps every non-test `.go` file in
`internal/cli` to its responsibility in one line each. A test in `internal/cli` fails when a
non-test `.go` file is missing from the map, or when the map names a file that does not exist.

**Approach (assumed at the header base):** Move the `// Package cli` comment from `root.go` into
`doc.go`, so the package keeps exactly one package comment. Below it, add one line per file in the
form `` `name.go` — responsibility ``. `doc_test.go` reads the directory with `os.ReadDir(".")`,
collects the `.go` files that do not end in `_test.go` (`doc.go` included), and reads `doc.go`.
It reports every file that is missing from the map and every mapped name that has no file. The
test depends on the working directory, which other `internal/cli` tests change, so it never calls
`t.Parallel()`.

**Files:** internal/cli/doc.go, internal/cli/doc_test.go, internal/cli/root.go
**Read first:** internal/cli/root.go — package comment (the only `// Package cli` in the tree), Run; internal/cli/testharness_test.go — runCLI (t.Chdir, restored on cleanup); internal/cli/write_test.go — t.Chdir setup;
.capcom/issues/cli-package-file-map.md — Acceptance; ~/.claude/skills/coding-standards/references/coding-standards.go.md — doc.go file-map rule

**Tests:** `internal/cli/doc_test.go`: `TestDocFileMap_MatchesDirectory`.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ -run TestDocFileMap`
- `go vet ./internal/cli/`
- `go doc ./internal/cli | head -5` prints the package comment.

**Closes:** cli-package-file-map

**Commit:** `chore(cli): add the internal/cli file map and its structural test`
