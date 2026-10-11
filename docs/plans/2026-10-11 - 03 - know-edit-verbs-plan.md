# Note edit verbs: know update, know mv, know rm

**Goal:** Notes can be changed, renamed, moved and removed through capcom, mirroring the issue
verbs, so nobody edits `.capcom/knowledge/` by hand. Every Note write prints an etag and refreshes
an existing AGENTS.md Managed block when the top level changed.

**Date:** 2026-10-11
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 49d4b71
**Closes:** know-edit-verbs

**Sources:**
- `.capcom/issues/know-edit-verbs.md`
- `docs/adr/0001-the-directory-tree-is-the-hierarchy.md`, `docs/adr/0002-ids-are-chosen-slugs-and-old-names-live-on-as-aliases.md`
- `docs/format.md` — "Note files", "Aliases", "The Managed block in AGENTS.md"
- `CONTEXT.md` — Note, Alias, Managed block

**Ratified design calls** (user, 2026-10-11):
- **Removal:** `capcom know rm <id>` deletes the Note file and refuses a Note with children.
- **Body edits:** `know update` takes the issue body flags plus `--body-file` for a whole-body replace.
- **Etag:** `know show` and every Note write print `etag:`; `know update`, `know mv` and `know rm` take `--if-match`.
- **Rename:** the old id becomes an Alias of the Note (ADR 0002), as `capcom mv` does for issues.

**Standing requirements:**
- skills: coding-standards
- Every error names the file, and the line when known.
- CLI tests go through `runCLI`/`runCLIWith`, hermetic, never `t.Parallel()`.
- Pi host: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.

**Out of scope:**
- An issue remove verb; rewriting Note body text or Refs that mention a renamed id.
- Tree-lock checks on the new verbs (no write verb checks the advisory lock).
- Changes to the Managed block's intro text (`blockIntro`).

**Regression check (2026-10-11, 49d4b71):**
- 1: guard folded (decision: no local `edit` shadows the package; read_test.go and mutate_test.go in Files; log helpers)
- 2: guard folded (decision: etag from the file read from disk; `--kv` dropped; pinned show text; omitempty etag)
- 3: guard folded (decision: `TestKnowUpdate_IfMatchFromShow_Writes`; warning-only refresh helper; shared body step)
- 4: recast (decision: `tree.Move` takes the noun, `tree.RenameAliases`); guard folded (rename seam, rewrite order)
- 5: guard folded (decision: `tree.RenameAliases`, noun "Note"; own Note failure mapping; warning-only refresh)
- 6: guard folded (the Note's own children directory; warning-only refresh)
- 7: guard folded (Acceptance greps the stale bullet and etag; stale-site rule covers know.go help)
- 4 (round 2): guard folded (decision: noun phrase with its article, "an issue" / "a note"; destination checked before any file is read; rename-seam test moves to tree, `issue.SetMoveRename` deleted)
- 5 (round 2): guard folded (decision: "a note" in lowercase, matching the Note errors in internal/cli/know.go)

## 1. Shared label, body and etag helpers outside package issue — ✅ DONE (2026-10-11)

NOTES (2026-10-11): TestETag moved from internal/issue/file_test.go to internal/edit/edit_test.go as TestETag_HashesContent, using inline byte inputs instead of issue golden files; the issue-level WithLabels/AppendBody/ReplaceBody tests were replaced by edit-level tests, leaving TestIssue_ReplaceBody_NotExactlyOnce_FailsWithErrReplaceMatch to pin the issue.ErrReplaceMatch alias.
NOTES (2026-10-11): the editIssue callback parameter in internal/cli/update.go is renamed edit -> change (doc comment updated) so it does not shadow package edit.

**What:**
**Goal:** label merging, body append/replace and the file etag are free functions in a package
that both `internal/issue` and `internal/knowledge` may import; issue behaviour and output are
byte-identical.
**Approach (assumed at the header base):** new package `internal/edit` with
`Labels(labels, add, remove []string) []string`, `AppendBody(body, text string) string`,
`ReplaceBody(body, old, replacement string) (string, error)`, `ErrReplaceMatch`, and `ETag(data
[]byte) string`, moved from `internal/issue/mutate.go` (`WithLabels`, `AppendBody`, `ReplaceBody`,
`appendBlock`, `withFinalNewline`, `normalizeNewlines`) and `internal/issue/file.go` (`ETag`). The
`issue.Issue` methods stay as thin wrappers over them; `issue.ErrReplaceMatch` becomes
`= edit.ErrReplaceMatch` so `errors.Is` keeps working. Every caller of `issue.ETag` switches to
`edit.ETag` and `issue.ETag` is deleted. One deep module, no duplicated logic.
**Regression guard.** Keep the package name `edit`; rename every local identifier named `edit` in a file that imports the package (for example the `edit` callback parameter of `editIssue` in internal/cli/update.go) so nothing shadows it. Every caller of `issue.ETag`, tests included, switches (`grep -rn 'issue\.ETag' --include=*.go .`; internal/cli/read_test.go `fileETag`). `AppendLog` still needs `appendBlock`, `withFinalNewline` and `normalizeNewlines`: `edit` exports them and `issue` calls them, so no copy stays in `issue`.
**Files:** internal/edit/edit.go, internal/edit/edit_test.go, internal/issue/mutate.go, internal/issue/mutate_test.go, internal/issue/file.go, internal/issue/file_test.go, internal/cli/update.go, internal/cli/show.go, internal/cli/close.go, internal/cli/mv.go, internal/cli/read_test.go
**Read first:** internal/issue/mutate.go — WithLabels, AppendBody, ReplaceBody, AppendLog; internal/issue/file.go — ETag; internal/cli/update.go — editIssue; internal/cli/read_test.go — fileETag; internal/issue/file_test.go — TestETag
**Tests:** move the free-function cases of `internal/issue/mutate_test.go` that cover label, append
and replace semantics into `internal/edit/edit_test.go`, keeping one `errors.Is(err,
issue.ErrReplaceMatch)` case in `mutate_test.go` so the alias stays pinned; an `ETag` test; the
issue tests stay green.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/edit/ ./internal/issue/` and `GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/`
**Commit:** `refactor(edit): share label, body and etag helpers between issues and Notes`

## 2. Notes carry an etag in know show and know add — ✅ DONE (2026-10-11)

NOTES (2026-10-11): added TestKnowSearch_JSON_CarriesNoEtag beyond the named tests, pinning the regression guard that know search --json does not gain an etag; test helpers noteFileETag and textETag added to know_test.go.

**What:**
**Goal:** `capcom know show <id>` prints `etag: <hex>` (text) and an `etag` field (`--json`),
and `capcom know add` prints the new file's `etag:`; the value equals
`edit.ETag` of the Note file's bytes.
**Approach (assumed at the header base):** in `internal/cli/know.go`, `noteData` and
`noteText` gain the etag, computed from the Note file's bytes read from disk;
`addNote` returns the etag of the serialized bytes it wrote. Depends on item 1.
**Regression guard.** `know show` computes the etag from the Note file's bytes read from disk, the way `show` does for issues through `readIssueFile`, never from the index body (`index.Body`), so the etag always matches what `--if-match` checks. `noteData` gains `ETag string json:"etag,omitempty"`, set by `noteShowResult` only; `newNoteData` is unchanged, so `know search --json` does not change. The text `etag:` line goes after `path:`, as in `issueText`. `know show --kv` stays unsupported (no `Result.KV`); the Goal no longer promises `--kv`.
**Files:** internal/cli/know.go, internal/cli/know_test.go
**Read first:** internal/cli/know.go — noteShowResult, noteText, newNoteData, addNote; internal/cli/show.go — issue show etag via readIssueFile; internal/cli/issue_render.go — issueText, issueData.ETag; internal/cli/know_test.go — TestKnowShow_Text_PrintsFrontmatterThenBody
**Tests:** `TestKnowShow_PrintsEtag_MatchingTheFile`, `TestKnowAdd_PrintsEtag` via `runCLI`, text and
`--json`; the pinned want string of `TestKnowShow_Text_PrintsFrontmatterThenBody` gains the
`etag:` line after `path:`; `TestKnowAdd_*` that pin `refreshed: AGENTS.md` as the suffix keep it
last (the `etag:` line comes before it).
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run 'KnowShow|KnowAdd' ./internal/cli/`
**Commit:** `feat(know): print the Note etag from know show and know add`

## 3. capcom know update — ✅ DONE (2026-10-11)

**What:**
**Goal:** `capcom know update <id>` changes a Note's title (`--title`), labels (`--add-label`,
`--remove-label`), refs (`--add-ref`, `--remove-ref`) and body (`--body-file` replaces the whole
body, `-` reads stdin; `--body-append`, `--body-append-file`, `--body-replace-old` with
`--body-replace-new`), guarded by `--if-match`; it prints `id:`, `path:`, `etag:` and refreshes an
existing stale Managed block (`refreshed: AGENTS.md`).
**Approach (assumed at the header base):** new `internal/cli/know_update.go`, registered in
`newKnowCommand`. A read-modify-write cycle like `editIssue`: resolve with `resolveNote`, read the
file, check `--if-match` against `edit.ETag` (mismatch → `CodeConflict`, message citing the file),
`knowledge.Parse`, apply in the order title, labels, refs, whole body, replace, append, then
`knowledge.Validate` and `tree.WriteFileAtomic`. `--body-file` is exclusive with the append and
replace flags; no change flag is a usage error (`hasAnyFlag`). New refs pass
`knowledge.ValidateRef`. After the write refresh the block unconditionally through the
warning-only helper (see the guard) — it writes only when stale, so a title change on a nested
Note is a no-op. Depends on item 2.
**Regression guard.** Add the test `TestKnowUpdate_IfMatchFromShow_Writes`: an etag printed by `know show` is accepted by `know update --if-match` and the write succeeds. The refresh goes through `refreshAgentsAfterAdd`, renamed `refreshAgentsAfterWrite`, never a direct `refreshAgentsBlock` call: tangled markers are a warning, and `refreshed: AGENTS.md` and the json `agents` key come out as for `know add`. `newNote`'s body step (CRLF to LF, blank to no body, leading blank line, final newline) becomes one helper that `know add` and `know update --body-file` both call.
**Files:** internal/cli/know_update.go, internal/cli/know_update_test.go, internal/cli/know.go
**Read first:** internal/cli/update.go — editIssue, hasAnyFlag; internal/cli/know.go — newNote, refreshAgentsAfterAdd, resolveNote; internal/knowledge/note.go — Parse, Serialize; internal/knowledge/verify.go — ValidateRef
**Tests:** `TestKnowUpdate_*` via `runCLI`/`runCLIWith`: each flag group; stdin body; `--if-match`
stale → conflict and file untouched; `TestKnowUpdate_IfMatchFromShow_Writes`; bad ref refused with
the file named; no flags → usage error; top-level title change refreshes the block; nested title
change leaves AGENTS.md byte-identical; tangled AGENTS.md markers → warning, Note written, exit 0;
`know add --body-file b.md` and `know update --body-file b.md` give identical body bytes.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run KnowUpdate ./internal/cli/`
**Commit:** `feat(know): add know update to change a Note's title, labels, refs and body`

## 4. A type-agnostic mover in package tree

**What:** Recast at the regression check (2026-10-11).
**Goal:** the plan-and-apply machinery behind `capcom mv` (destination checks, file and directory
renames, empty-directory cleanup, ordered apply with roll-back) lives in package `tree` with no
knowledge of the issue format; `capcom mv` behaves and prints exactly as before.
**Approach (assumed at the header base):** move `TakenError`, `Destination`, `checkDestination`,
`moveStep`, `planRenames`, `renameStep`, `removeEmptyDirStep`, `Apply` and `rollBack` from
`internal/issue/move.go` into `internal/tree/move.go` as an exported `tree.Move` built from the
tree, the node and the destination, which accepts file rewrites (path, new bytes) from its caller.
`issue.PlanMove` keeps only the issue rewrites (`planRewrites`, `renameDeps`) and
feeds them in. Keep `ErrMoveIntoSubtree` reachable under `errors.Is` from the issue side.
Refactor only: no behaviour change. Depends on item 1.
**Regression guard.** The caller passes the item kind as a noun phrase with its article (issues "an issue", Notes "a note"), and every kind-specific error of `tree.Move` uses that phrase, so issue error text stays byte-identical and no "an Note" can occur; `renameAliases` moves into package tree as exported `tree.RenameAliases`, used by `issue.PlanMove`. Package tree checks the destination (ValidateID, taken, own subtree, unknown parent) and runs `PathFor` before the caller reads any file — plan in tree first, add the rewrites after — so an invalid or taken new id fails invalid/exists, never with an unparseable file's parse error. Each rewrite carries path, original and new bytes, and `tree.Move` applies every rewrite at its current path before any rename, as today. The rename seam moves with the mover: `internal/tree/export_test.go` exposes it, `issue.SetMoveRename` is deleted (a tree export_test.go is invisible to issue's tests), and `TestMoveApply_SecondRenameFails_LeavesTreeUnchanged` (internal/issue/move_test.go:269), with its rewrite-undo case, moves to `internal/tree/move_test.go`.
**Files:** internal/tree/move.go, internal/tree/move_test.go, internal/tree/export_test.go, internal/issue/move.go, internal/issue/move_test.go, internal/issue/export_test.go, internal/cli/mv.go
**Read first:** internal/issue/move.go — PlanMove, checkDestination, lookupExact, planRewrites, planRenames, renameStep, Apply, rollBack; internal/issue/export_test.go — SetMoveRename; internal/issue/move_test.go — TestMoveApply_SecondRenameFails_LeavesTreeUnchanged, TestPlanMove_UnderItselfOrDescendant_Refused, TestPlanMove_RenameOntoTakenIDOrAlias_Refused;
internal/cli/mv.go — move, moveFailure; internal/cli/mv_test.go — TestMv_UnderOwnChild_FailsInvalid; internal/tree/tree.go — PathFor, ChildrenOf, sentinel errors; internal/tree/atomic.go — WriteFileAtomic; internal/tree/tree_test.go — writeNodes
**Tests:** the type-agnostic cases (rename with children directory, reparent, empty parent dir
removed, roll-back on a failing step through the `internal/tree/export_test.go` rename seam,
`TestMoveApply_SecondRenameFails_LeavesTreeUnchanged` with its rewrite-undo case) move to
`internal/tree/move_test.go`; `internal/tree` tests for `tree.RenameAliases` and for the noun phrase
("an issue", "a note") in the taken-id and own-subtree messages; an `issue.PlanMove` test that an
invalid or taken new id with an unparseable issue file in the tree fails invalid/exists, not with
the parse error; `internal/issue/move_test.go` keeps the alias and dep rewrites; `TestMv_*` unchanged and green, and the full `capcom mv` own-subtree stderr is unchanged.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/ ./internal/issue/` and `GOMEMLIMIT=2GiB go test -count=1 -run Mv ./internal/cli/`
**Commit:** `refactor(tree): extract the type-agnostic mover from issue moves`

## 5. capcom know mv

**What:**
**Goal:** `capcom know mv <id> [<new-id>] [--parent <id|.>] [--if-match <etag>]` renames a Note
and/or moves it below another Note (`.` is the top level), carrying its children directory; on a
rename the old id is added to the Note's `aliases` and a new id that was a former alias leaves
them; it prints `id:`, `path:`, `etag:` and refreshes an existing stale Managed block.
**Approach (assumed at the header base):** new `knowledge.PlanMove` in `internal/knowledge/move.go`
building a `tree.Move` with one rewrite: the moved Note's aliases (`tree.RenameAliases`). Notes
have no deps, so no other file is rewritten. New
`internal/cli/know_mv.go` mirrors `move()` in `internal/cli/mv.go`: `tree.Load(KnowledgeDir,
readNoteAliases)`, resolve node and parent (`rootParentArg`), `--if-match` checked against the
moved file, errors through its own Note failure mapping (see the guard), then the warning-only
refresh helper from item 3. Depends on items 2, 3 and 4.
**Regression guard.** `knowledge.PlanMove` uses `tree.RenameAliases` (no duplicate of the alias rule). `knowledge.PlanMove` passes "a note" in lowercase, matching the existing Note error wording in internal/cli/know.go, so `know add` and `know mv` errors agree. `know mv` does not reuse `moveFailure`: a `TakenError` goes to `noteExistsFailure`, an unparseable moved Note to `noteFailure`, everything else is invalid at the Note's path. The refresh goes through `refreshAgentsAfterWrite` (item 3): tangled markers are a warning, never a failed command after the files moved.
**Files:** internal/knowledge/move.go, internal/knowledge/move_test.go, internal/cli/know_mv.go, internal/cli/know_mv_test.go, internal/cli/know.go
**Read first:** internal/cli/mv.go — move, rootParentArg, moveFailure; internal/cli/know.go — readNoteAliases, noteExistsFailure, noteFailure, refreshAgentsAfterAdd; internal/issue/move.go — renameAliases
**Tests:** `TestKnowMv_*` via `runCLI`: rename keeps an alias and `know show <old-id>` resolves;
reparent under a Note and back to `.`; children move along; taken id/alias → `exists` naming the
note, never "issue"; into own subtree refused with "a note" in the message; stale `--if-match` →
conflict, tree untouched; top-level change refreshes the block; tangled AGENTS.md markers →
warning, move kept. `knowledge` package test for the alias rewrite.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/knowledge/` and `GOMEMLIMIT=2GiB go test -count=1 -run KnowMv ./internal/cli/`
**Commit:** `feat(know): add know mv to rename a Note or move it in the tree`

## 6. capcom know rm

**What:**
**Goal:** `capcom know rm <id> [--if-match <etag>]` deletes a Note with no children, removes its
parent's children directory when left empty, prints `id:` and `path:` of the removed file, and
refreshes an existing stale Managed block; a Note with children is refused with an error naming
the Note file and its child count, and nothing is deleted.
**Approach (assumed at the header base):** new `internal/cli/know_rm.go`, registered in
`newKnowCommand`: `tree.Load`, resolve, refuse when the node has children, check `--if-match`,
`os.Remove` the file, then remove the empty children directory of the parent (reuse the tree
mover's empty-dir step from item 4 if exported, else `os.Remove` on an empty dir), then the
warning-only refresh helper from item 3. Its aliases go with the file, so the ids become free.
Depends on items 2, 3 and 4.
**Regression guard.** `tree.Load` skips dotfiles and non-`.md` files, so the node-children check misses them: after it, stat the Note's own children directory (its path minus `.md`); remove it with the file when empty, else refuse naming that directory and delete nothing, so no orphan directory is left. The refresh goes through `refreshAgentsAfterWrite` (item 3): tangled markers are a warning, never a failed command after the file is gone.
**Files:** internal/cli/know_rm.go, internal/cli/know_rm_test.go, internal/cli/know.go
**Read first:** internal/cli/know.go — refreshAgentsAfterAdd, resolveNote; internal/tree/tree.go — Load, walk, ChildrenOf; internal/issue/move.go — removeEmptyDirStep; internal/index/walk.go — walk; internal/cli/know_test.go — writeNoteFiles
**Tests:** `TestKnowRm_*` via `runCLI`: leaf removed and `know show` → not_found; last child removed
takes the empty directory with it; Note with children refused, files intact; Note whose own
children directory is empty → both removed; holding only a non-Note file (`diagram.png`) →
refused naming the directory, files intact; stale `--if-match` → conflict; top-level removal
refreshes the block; tangled AGENTS.md markers → warning; unknown id → not_found.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 -run KnowRm ./internal/cli/`
**Commit:** `feat(know): add know rm to remove a Note without children`

## 7. Document the Note edit verbs

**What:**
**Goal:** the user docs describe `know update`, `know mv`, `know rm`, Note etags and which Note
writes refresh the Managed block, with no sentence left saying only `know add` refreshes it or
that Notes cannot be changed through capcom.
**Approach (assumed at the header base):** `docs/format.md` — "Note files" (etag, edit verbs),
"Aliases" (Notes gain aliases through `know mv`), "The Managed block in AGENTS.md" (every top-level
change by add, update, mv or rm refreshes it); `README.md` — the command list and the `capcom mv`
sentence; `CONTEXT.md` — the Note entry if it states how Notes change; `internal/cli/know.go` — the
`know` command's Short and doc comment. Find every stale site with
`grep -rn "know add\|add and verify\|read and add" docs README.md CONTEXT.md internal/cli/know.go`.
Depends on items 3, 5 and 6.
**Regression guard.** The stale-site rule is every user-facing sentence or help string listing the know verbs or the block-refreshing writes, including the `know --help` Short "Read, search, add and verify Knowledge Notes" and its doc comment in internal/cli/know.go. Acceptance proves the stale `docs/format.md` bullet ("A top-level `capcom know add` refreshes ...") is gone and that Note etags are described.
**Files:** docs/format.md, README.md, CONTEXT.md, internal/cli/know.go
**Read first:** docs/format.md — "Note files", "Aliases", "The Managed block in AGENTS.md"; README.md — "Issues as files" capcom mv bullet, command list; CONTEXT.md — Note, Managed block; internal/cli/know.go — newKnowCommand
**Tests:** none (docs only).
**Acceptance:** `grep -n "know update" docs/format.md README.md && grep -n "know mv" docs/format.md README.md && grep -n "know rm" docs/format.md README.md && ! grep -n "A top-level \`capcom know add\` refreshes" docs/format.md && grep -n etag docs/format.md`
**Commit:** `docs: describe the Note edit verbs and Note etags`
