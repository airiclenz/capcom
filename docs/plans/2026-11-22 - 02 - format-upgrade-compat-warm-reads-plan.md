# Plan: explicit store format upgrade, the compatibility promise, and fast warm reads

**Goal:** A write never raises the store format on its own: it fails and names `capcom upgrade`,
a new command that raises it on purpose. `docs/site/reference/compatibility.md` states what capcom
keeps stable, each promise tied to a test. `ready`, `next` and `list --all --json` run under 1 s and
`prime` under 1.5 s at N=10000 on the Raspberry Pi 5 host.
**Date:** 2026-11-22
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** aa30b57

**Sources:**
- Issues `explicit-format-upgrade`, `compat-policy-doc`, `warm-reads-large-store` (`capcom show <id>`)
- ADR 0009 (`capcom know show adr-0009-decision`); Notes `format-version`, `store-layout`, `docs-layout`, `cli-test-harness`, `coding-conventions`
- `docs/site/reference/json-output.md`, `docs/site/reference/file-format.md`, `docs/reviews/benchmarks-2026-11-05.md`
- Profile of 2026-11-22 (items 7–12): at N=10000, `elsewhere.Scan` is 71% of `ready` (per-file `gitx.ShowFile`, two git processes per modified file); `next` loads every issue twice (`briefing.ReadyList`, then `loadShowGraph`); `prime` also spends ~45% in `check.Run` via `readHealth`

**Ratified design calls** (Airic, 2026-11-22):
- **Format raise:** a write that needs a newer format than the store declares fails with code `invalid` and writes nothing; `capcom upgrade [--format <n>]` raises it explicitly. Recorded as ADR 0019, superseding ADR 0009's implicit raise.
- **Failure hint:** `ask the user before raising it for everyone: capcom upgrade --format <n>`. The Managed block does not gain `upgrade`.
- **Lower target:** `capcom upgrade --format <n>` on a store at format ≥ n exits 0, changes nothing, and says the store is already at its format; capcom never lowers a format.
- **Scope:** the three P2 issues only.

**Standing requirements:**
- skills: coding-standards
- Run single packages with `GOMEMLIMIT=2GiB`; never `./...` with `-race`; never two test commands at once (AGENTS.md).
- CLI tests go through `runCLI` (Note `cli-test-harness`); never `t.Parallel()`.
- After changing a command or its help: `go run ./cmd/gendocs` and commit `docs/site/reference/cli/`; never hand-edit it. Only JSON and KV goldens refresh with the package's `-update` flag; help goldens under `internal/cli/testdata/help/` are refreshed the way that test's harness says (read `golden_test.go` / `node_verbs_test.go`). Every golden is read before committing.
- Knowledge Notes are written with `capcom know add` / `capcom know update`, then `capcom check`.
- Any deviation from item text lands as a dated NOTES line under the item.

**Out of scope:**
- Config keys that need a newer format (`agents.guides`, `agents.file`): they already fail at parse with "set format: N"; unchanged.
- `html-export`, `scheduled-agent-eval`, `split-cli-package`.
- A persistent cache of the elsewhere scan.
- A JSON `schema` bump.

**Regression check (2026-11-22, aa30b57):**
- 1: guard folded
- 2: guard folded
- 3: guard folded (decision: the gate's final shape is settled here; supersedes ADR 0009's implicit raise; the `check` advice moves to item 5)
- 4: guard folded (decision: deletion only)
- 5: guard folded (decision: owns every user-facing raise site, `internal/check` included)
- 6: guard folded
- 7: recast (decision); yields to the hostile-branch rule of `gitx/branches.go` and review F-02(d)
- 8: recast (decision)
- 9: guard folded
- 10: guard folded (decision); yields to the Compose doc (`briefing.go:96-97`)
- 11: recast (decision)
- 12: guard folded
- 13: guard folded
- 7 (re-check): guard folded (decision: `readBlobs` / `changedFiles` test seams in elsewhere.go); `showFileLimit` survives for `blobs_test.go`, `ErrBlobTooLarge` doc reworded, warning wording noted
- 8 (re-check): guard folded (decision: pool calls the `changedFiles` seam); Acceptance checks `gitx.UnmergedRefs`
- 11 (re-check): guard folded; mutex scope over `c.problems`, deterministic overlap test, `internal/check/problem.go` added to Files

## 1. ADR 0019: a store format is raised only by `capcom upgrade` — ✅ DONE (2026-11-22)

NOTES (2026-11-22): the installed `capcom` (/home/linuxbrew/.linuxbrew/bin, 0.16.0) predates HEAD: its `know update` rewrote AGENTS.md's capcom block to an older shape (restored with git restore) and its `know show` takes one id only, so the Acceptance ran with a binary built from HEAD (`go build ./cmd/capcom`).
NOTES (2026-11-22): the rejected alternative (keep the implicit raise behind a config key) is recorded as a short "Not chosen" paragraph in `adr-0019-decision` rather than a separate `adr-0019-rejected` Note, since the item's Files name no rejected child.
NOTES (2026-11-22): `adr-0009`'s summary also says what still holds after ADR 0019 (ADR 0010: a record replaced in part names its successor and says what still holds).

**What:**
**Goal:** The `decisions` Knowledge branch holds `adr-0019` (summary) with an `adr-0019-decision` child recording the three format calls in the header (fail with `invalid` and the hint; `capcom upgrade [--format <n>]`, default the newest format, idempotent, never lowers), and `adr-0009-decision` says its implicit-raise clause is superseded by ADR 0019.
**Approach (assumed at the header base):** copy the shape of an existing record (`capcom know show adr-0018` and its decision child; `docs/adr/README.md` names ADR 0010's shape). Create with `capcom know add --parent decisions` and `--parent adr-0019`. In `adr-0009-decision`, the sentence "the store stays at format 1 until a write verb that needs a newer key raises it" gains a pointer: superseded by ADR 0019. Edit with `capcom know update`.
**Regression guard.** The `decisions` Note's list "The records:" gains a line for `adr-0019` and its title, in the shape of the others (`capcom know update decisions`). Per ADR 0010 (`adr-0010-decision`, Retired and Supersession: a record replaced in part stays current and names its successor; precedent `adr-0002`'s `superseded-by: [adr-0010]`), also run `capcom know update adr-0009 --add-superseded-by adr-0019` and amend `adr-0009`'s `Status:` line (e.g. "Accepted; amended in part by ADR 0019").
**Files:** .capcom/knowledge/decisions/adr-0019.md, .capcom/knowledge/decisions/adr-0019/adr-0019-decision.md, .capcom/knowledge/decisions/adr-0009/adr-0009-decision.md, .capcom/knowledge/decisions/adr-0009.md, .capcom/knowledge/decisions.md
**Read first:** .capcom/knowledge/decisions/adr-0009/adr-0009-decision.md — "init writes the key only as a comment" paragraph; .capcom/knowledge/decisions.md — records list; .capcom/knowledge/decisions/adr-0010/adr-0010-decision.md — Shape, Retired and Supersession; .capcom/knowledge/decisions/adr-0002.md — superseded-by and Status precedent; .capcom/knowledge/decisions/adr-0018.md — summary shape; docs/adr/README.md
**Tests:** none (knowledge only).
**Acceptance:** `capcom know show adr-0019 adr-0019-decision && grep -n "0019" ".capcom/knowledge/decisions/adr-0009/adr-0009-decision.md" && capcom check`
**Commit:** `docs(decisions): ADR 0019 raises the store format only on request`

## 2. `capcom upgrade` raises the store format on request — ✅ DONE (2026-11-22)

NOTES (2026-11-22): consequential edit — internal/cli/doc.go: made necessary by the new internal/cli/upgrade.go (TestDocFileMap_MatchesDirectory enumerates every file)
NOTES (2026-11-22): the format table (`config.FormatAddition`, `config.FormatAdditions(from, to)`) lives in internal/config/format.go as the item's Files line names, not config.go beside the constants; the --json `to` of an unchanged run is the store's own format (equal to `from`), not the requested lower target; no KV golden, since upgrade registers no --kv; json-output.md also gains a paragraph describing the upgrade data

**What:**
**Goal:** `capcom upgrade [--format <n>]` (default `config.FormatVersion`) raises `format:` in `.capcom/config.yaml` to n, creating the file when absent; prints `store format <from> → <n>`, one line per format added (`  <k>: <what it adds>`), and `capcom releases that read at most format <n-1> can no longer read this store`. A store at format ≥ n exits 0, writes nothing and prints `store is already at format <m>`. n outside 1..`config.FormatVersion` is a usage error (exit 2). `--json` emits kind `upgrade` with `from`, `to`, `changed` (bool) and `adds` (`[{"format": k, "adds": "<text>"}]`, empty when unchanged).
**Approach (assumed at the header base):** one table of what each format adds lives in `internal/config` beside the format constants (`FormatRelated` … `FormatBlockFile`): 2 issue `related` links; 3 Note `retired` and `superseded-by`; 4 config `agents.guides`; 5 standing issues (`standing`); 6 config `agents.file`. New `internal/cli/upgrade.go` modelled on `fmt.go` (`annotationIndex: storeOnly`, `runE`, `a.noteWrite()` only when changed), registered in `root.go` between `update` and `version`; the raise calls `store.Store.RaiseFormat` (→ `config.RaiseFormat`), unchanged. Add the JSON golden (and KV if `TestGolden_KVCovered` demands it).
**Regression guard.** Add `upgrade` to the `kind` list in `docs/site/reference/json-output.md` (the row that enumerates every kind). Acceptance also runs `TestReferenceUpToDate` (`internal/clidoc/reference_test.go`): `git diff --exit-code` ignores an untracked `capcom_upgrade.md`.
**Files:** internal/config/format.go, internal/config/format_test.go, internal/cli/upgrade.go, internal/cli/upgrade_test.go, internal/cli/root.go, internal/cli/golden_write_test.go, internal/cli/testdata/json/, internal/cli/testdata/kv/, docs/site/reference/cli/, docs/site/reference/json-output.md
**Read first:** internal/config/config.go — FormatVersion, FormatRelated…FormatBlockFile; internal/store/store.go — Store.RaiseFormat, loadConfig; internal/cli/fmt.go — newFmtCommand; internal/cli/root.go — prepareIndex, storeOnly, AddCommand list; internal/cli/format_fixture_test.go — copyFixtureStore, readStoreConfig;
  internal/cli/golden_kv_test.go — TestGolden_KindsCovered, TestGolden_KVCovered; internal/cli/golden_write_test.go — TestGolden_Fmt_PinsTheRewrittenPaths; docs/site/reference/json-output.md — kind list
**Tests:** on fixture stores (`copyFixtureStore`): format-4 → `upgrade --format 5` raises to 5 and lists format 5; format-3 → bare `upgrade` raises to `FormatVersion` listing 4..6; at-target and lower-target exit 0 with config bytes unchanged; `--format 0` and `--format <FormatVersion+1>` exit 2; missing config.yaml creates one at the target; `--json` golden.
**Acceptance:** `go vet ./internal/config/ ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/config/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Upgrade|Golden|Help' ./internal/cli/ && go run ./cmd/gendocs && git diff --exit-code docs/site/reference/cli/ && GOMEMLIMIT=2GiB go test -count=1 -run TestReferenceUpToDate ./internal/clidoc/`
**Commit:** `feat(upgrade): add capcom upgrade to raise the store format on request`

## 3. A write that needs a newer store format fails instead of raising it — ✅ DONE (2026-11-22)

NOTES (2026-11-22): the gate is `store.Store.RequireFormat(key, version) error` returning a `*store.FormatError` (Key, Needed, Declared, Path); `nodestore.FormatGate` / `Kind.WithFormatGate` replace `WithRaise`, `Codec.requiredFormat` now returns the key with the format (the highest-format key; ties name the table's first, so a retired Note names `retired`), and the Kind sets the error's Path to the node file, so the CLI error carries `path` (Note `error-messages`) beside the exact message and hint; `Error()` is `<path>: <message>; <hint>` for `capcom ui`
NOTES (2026-11-22): `Effects.RaisedFormat`, `warnRaisedFormat`/`formatRaisedWarning`, `stage.Overlay.RaiseFormat` and the batch aggregation are left in place (now always 0 / unused by apply) for item 4 to delete; the help texts that still promise a raise are item 5's
NOTES (2026-11-22): deleted `TestRaise_UnplaceableFormatKey_FailsAtTheKeyLineNamingTheConfigOnce` and `TestRelatedAdd_RaiseFails_FailsInvalidAtTheConfig` (a write no longer reads or writes config.yaml); `TestEdit_RaiseFails_*` became `TestEdit_UnreadableConfig_GatesOnTheOpenedFormatWithoutReadingIt`; the document-marker related test now raises through `capcom upgrade --format 2`; the apply all-or-none test sits in format_fixture_test.go (`TestFormatFixture_ApplyNeedingNewerFormat_WritesNothing`); `capcom ui` gets no new test: it reaches the gate through `ops.New` and shows `err.Error()`, which names `capcom upgrade --format <n>`
NOTES (2026-11-22): consequential edit — internal/nodestore/format.go: made necessary by the gate replacing Create's and Edit's raise (doc said "Unlike Create and Edit it raises no store format")
NOTES (2026-11-22): consequential edit — internal/stage/catalog.go: made necessary by apply passing `a.store.RequireFormat` to `ops.Staged` instead of the Overlay's RaiseFormat
NOTES (2026-11-22): consequential edit — internal/cli/know_update.go: made necessary by the gate (updateNote's doc comment said a format-3 line raises the store with a warning)
NOTES (2026-11-22): consequential edit — internal/cli/related.go: made necessary by the gate (newRelatedAddCommand's doc comment said the first link raises config.yaml to format 2)

**What:** Depends on items 1 and 2.
**Goal:** Every write of an issue or Note file carrying a key the store's declared format does not allow (`related` < 2, `retired`/`superseded-by` < 3, `standing` < 5) — `create`, `update`, `close`, `reopen`, `log`, `start`, `finish`, `dep add/rm`, `related add/rm`, `know update`, `apply` and `capcom ui` — fails with code `invalid`, message `<key> needs store format <n>, but the store declares format <m>`, hint `ask the user before raising it for everyone: capcom upgrade --format <n>`, and leaves the failing node's file and config.yaml unchanged (`apply`: all or none; a multi-id `close` still writes the others). No write changes `format:`.
**Approach (assumed at the header base):** rename the slot `Kind.raise` / `k.raiseFormat` / `WithRaise` (`internal/nodestore/kind.go`) to say what it does (e.g. `Kind.WithFormatGate`) and fill it, in `ops.go` (`ops.Ops.raise`, set from `s.RaiseFormat`) and `cli/apply.go` (`ops.Staged(..., overlay.RaiseFormat, ...)`), with a gate that compares the required format to the declared one and returns a typed error carrying key, needed and declared format; `nodestore.Codec.requiredFormat` must report which key needed it. `Kind.Create` (step 9) and `Kind.Edit` (step 8) run the gate before writing anything. Map the typed error to `output.Error{Code: invalid, Hint: ...}` in `app.kindFailure` (`internal/cli/errors.go`). Removing the now-dead raise plumbing is item 4.
**Regression guard.** Item 3 settles the gate's final shape — the slot is renamed in item 3 to say what it does (e.g. `Kind.WithFormatGate`, a func taking the key and the needed format and returning an error) and wired in ops.go and apply.go in its final form, so item 4 only deletes code (`Effects.RaisedFormat`, the raise warning, `stage.Overlay.RaiseFormat`, the batch aggregation) and never re-edits the gate's signature. The gate is also the default slot of `nodestore.Issues` and `nodestore.Notes` (`kind.go`), since `know update` reaches `nodestore.Notes` through `app.notes` (`know_add.go`), never ops; it compares to `s.Config.Format`, the opened format, never a re-read of config.yaml (ops tests open `config.Default()` with no config.yaml on disk). The typed error's `Error()` text names `capcom upgrade --format <n>` as well as the CLI Hint, since `capcom ui` shows only `err.Error()` (`ui/write.go`). The slot's func type changes, so `stage/staged_test.go` and `batch/run_test.go` (both pass `overlay.RaiseFormat` to `ops.Staged`) pass the gate here. The `check` advice "or run a capcom write verb that raises it" (`internal/check/problem.go`) is item 5's. Supersedes ADR 0009's decision (`adr-0009-decision`, "until a write verb that needs a newer key raises it"), as item 1 records.
**Files:** internal/nodestore/kind.go, internal/nodestore/codec.go, internal/store/store.go, internal/ops/ops.go, internal/cli/apply.go, internal/cli/errors.go, internal/cli/format_fixture_test.go, internal/cli/write_test.go, internal/cli/related_test.go, internal/cli/close_test.go, internal/cli/know_update_test.go, internal/nodestore/create_test.go, internal/nodestore/edit_test.go, internal/nodestore/kind_test.go, internal/stage/staged_test.go, internal/batch/run_test.go
**Read first:** internal/nodestore/kind.go — Kind.raise, Issues, Notes, WithRaise; internal/nodestore/codec.go — Codec.requiredFormat; internal/cli/know_add.go — app.notes; internal/ops/ops.go — New, Staged, Ops.issues; internal/cli/errors.go — app.kindFailure;
  internal/cli/apply.go — app.apply, operationCause; internal/cli/close.go — closeResult, closeOne
**Tests:** rewrite the raise tests to the failure: `TestFormatFixture_StandingRaisesFormat` → on the format-4 fixture `create --standing` fails `invalid`, names `capcom upgrade --format 5`, `git status --porcelain` empty; then `upgrade --format 5` and the same create succeeds. Same for `related add` on format-1, `know update --retired` on format-2, `close` of a file holding `related` in a format-1 store (one id, or asserting only that file and config.yaml), and an `apply` file whose second op creates a standing issue on format-4 (nothing written). Rewrite the nodestore raise tests (`TestCreate_StandingIssueBelowTheStandingFormat_RaisesTheFormat`, `TestEdit_GainsRelatedBelowFormatTwo_RaisesBeforeTheWrite`, `TestRaise_*`, `TestWithRaise_*`) to the gate; the failure's `Error()` names `capcom upgrade --format <n>`.
**Acceptance:** `go vet ./internal/nodestore/ ./internal/ops/ ./internal/store/ ./internal/stage/ ./internal/batch/ ./internal/cli/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/ ./internal/ops/ ./internal/store/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Format|Standing|Related|Close|KnowUpdate|Apply|Golden' ./internal/cli/`
**Commit:** `feat(store): fail a write that needs a newer store format`

## 4. Remove the implicit-raise plumbing — ✅ DONE (2026-11-22)

NOTES (2026-11-22): re-derived from the assumption that `Kind.configPath`, the nodestore raise tests and the raise wiring in `nodestore/create.go`, `nodestore/edit.go` and `stage/catalog.go` still existed: item 3 had already removed them, so those listed files needed no change; `Effects.RaisedFormat` (nodestore/kind.go) was the only nodestore leftover
NOTES (2026-11-22): `stage/plan.go` keeps only `Overlay.Plan` (`addVerbatimRewrite`, `checkUnchanged`, `rewriteOp` deleted); `stage/overlay.go` loses `RaiseFormat`, `readConfig`, `stagedConfig` and the `configPath`/`configName`/`config`/`format`/`raisedConfig` fields; the four `TestOverlay_StagedRaise_*` tests and the test fixture's `configPath` went with them
NOTES (2026-11-22): consequential edit — internal/cli/root.go: made necessary by removing warnRaisedFormat (the `writeWarnings` field comment named "the store format an edit raised" as its example)
NOTES (2026-11-22): consequential edit — internal/cli/doc.go: made necessary by removing the raise (the package map said related.go is "raising the store format")
NOTES (2026-11-22): `make lint`: staticcheck passes clean; govulncheck exits 3 on GO-2026-6859 in net/http of this host's go1.26.10 (fixed in go1.26.11), a standard-library finding independent of this change
NOTES (2026-11-22): help text still promising a raise (`create.go`, `update.go`, `related.go` Long texts) and the `check` advice in `internal/check/problem.go` are left for item 5

**What:** Depends on item 3.
**Goal:** No code path raises the store format except `capcom upgrade`: `Effects.RaisedFormat`, `formatRaisedWarning`, `app.warnRaisedFormat`, `stage.Overlay.RaiseFormat` and the batch aggregation of raised formats are gone, along with their tests; `config.RaiseFormat` / `store.Store.RaiseFormat` remain, used only by `upgrade`.
**Approach (assumed at the header base):** delete the warning calls in `create.go`, `node_verbs.go`, `node_edit.go`, `apply.go`; the const and method in `related.go`; `RaisedFormat` in `nodestore/kind.go` and `internal/batch/run.go`; `Overlay.RaiseFormat` in `internal/stage/overlay.go` and its plan write in `stage/plan.go`. Delete `TestOverlay_StagedRaise_*` and the nodestore raise tests; keep `TestRaiseFormat_*` in `store`/`config`.
**Regression guard.** Deletion only, per item 3's guard: item 4 removes `Effects.RaisedFormat`, the raise warning, `stage.Overlay.RaiseFormat` and the batch aggregation, and never re-edits the gate's signature (`stage/staged_test.go` and `batch/run_test.go` already pass item 3's gate). Delete every helper and field the raise alone used, which CI's staticcheck (U1000) otherwise flags: e.g. `addVerbatimRewrite`, `checkUnchanged`, `rewriteOp` (`stage/plan.go`), `stagedConfig`, `Overlay.config`/`format`/`raisedConfig`/`configName` (`stage/overlay.go`), `Kind.configPath` (`nodestore/kind.go`). Every code comment that says a write raises the format goes with them: `grep -rniE "rais(e|es|ed|ing)" internal --include=*.go | grep -v _test.go`, each hit outside `config.RaiseFormat`/`store.RaiseFormat` fixed (help text and user messages under `internal/cli` and `internal/check` are item 5's).
**Files:** internal/nodestore/kind.go, internal/nodestore/create.go, internal/nodestore/edit.go, internal/nodestore/kind_test.go, internal/nodestore/create_test.go, internal/nodestore/edit_test.go, internal/stage/overlay.go, internal/stage/plan.go, internal/stage/overlay_test.go, internal/stage/catalog.go, internal/batch/run.go, internal/cli/related.go, internal/cli/create.go, internal/cli/node_verbs.go, internal/cli/node_edit.go, internal/cli/apply.go
**Read first:** internal/stage/overlay.go — Overlay.RaiseFormat, stagedConfig, New; internal/stage/plan.go — Overlay.Plan, addVerbatimRewrite, checkUnchanged; internal/stage/staged_test.go — newStaged; internal/batch/run.go — Outcome.RaisedFormat;
  internal/cli/related.go — formatRaisedWarning, app.warnRaisedFormat; internal/nodestore/kind.go — Effects.RaisedFormat, Kind.configPath; internal/cli/node_edit.go — app.written; internal/batch/run_test.go — fixture setup
**Tests:** existing tests, minus the deleted raise tests; `grep -rn "RaisedFormat\|warnRaisedFormat\|formatRaisedWarning" internal` prints nothing.
**Acceptance:** `go vet ./internal/... && GOMEMLIMIT=2GiB go test -count=1 ./internal/nodestore/ ./internal/stage/ ./internal/batch/ ./internal/ops/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/cli/ && ! grep -rn "RaisedFormat\|warnRaisedFormat\|formatRaisedWarning\|Overlay) RaiseFormat" internal && make lint`
**Commit:** `refactor(store): remove the implicit store format raise`

## 5. Help texts and docs no longer promise an implicit raise — ✅ DONE (2026-11-22)

NOTES (2026-11-22): internal/cli/know_update.go needed no change: its help no longer mentioned a raise (item 4 had already reworded its comment), so it is not in FILES.
NOTES (2026-11-22): consequential edit — internal/cli/doc.go: comment reworded ("raises config.yaml's store format") so the Acceptance grep `raises (the|its) (store )?format` over internal/cli/*.go passes; it describes capcom upgrade, which still raises the format.
NOTES (2026-11-22): consequential edit — internal/cli/upgrade.go: newUpgradeCommand comment reworded for the same Acceptance grep; meaning unchanged.
NOTES (2026-11-22): consequential edit — internal/cli/golden_write_test.go: TestGolden_Upgrade_PinsTheAddedFormats comment reworded for the same Acceptance grep; meaning unchanged.
NOTES (2026-11-22): file-format.md Config "any new key ... raises the format" (ADR 0009's release rule, also caught by the Acceptance grep) now reads "each new key ... comes with a new format"; the Notes format-version and store-config got the same rewording plus "only capcom upgrade raises a store's" (ADR 0019), via capcom know update, capcom check clean.
NOTES (2026-11-22): file-format.md Related links now also lists `related rm` among the writes that fail on an issue already holding a related line (pinned by TestRelatedRm_FormatOneStoreHoldingRelated_FailsAndWritesNothing).

**What:** Depends on item 3.
**Goal:** No help text, generated reference page or doc says a write raises the store format; each says the write fails and names `capcom upgrade --format <n>`, and `file-format.md`'s config format rules say the format is raised only by `capcom upgrade`.
**Approach (assumed at the header base):** help strings in `internal/cli/create.go` and `update.go` ("The first standing Issue in a store below format 5 raises…") and `related.go` ("The first link in a store below format 2 raises…"); help golden `internal/cli/testdata/help/create.txt`; `docs/site/reference/file-format.md` (standing, related raise and warning text, Note raise to 3, Config format rules), `docs/site/concepts/knowledge.md`, `docs/site/concepts/dependencies.md`, `docs/site/concepts/issues.md`. Rule for every site: the Regression guard's rule and grep; fix each hit outside the generated `docs/site/reference/cli/` by hand, then regenerate it.
**Regression guard.** Item 5 owns every site that tells a user a write raises the format, including `internal/check/problem.go` `addFormatTooOld` (its advice "set format: N in <config>, or run a capcom write verb that raises it" becomes "ask the user before raising it for everyone: capcom upgrade --format N"; update its tests and any index/diagnose copy of that text) and `internal/cli/know_update.go` help ("raises a store below format 3"); the rule is any prose or message saying a write raises the store format, found with `grep -rniE "rais(e|es|ed|ing)" internal/cli/*.go internal/check docs/site .capcom/knowledge --include=*.go --include=*.md | grep -i format`, fixed by hand outside the generated docs/site/reference/cli/.
**Files:** internal/cli/create.go, internal/cli/update.go, internal/cli/related.go, internal/cli/know_update.go, internal/cli/testdata/help/, internal/check/problem.go, internal/check/check_test.go, internal/cli/check_test.go, internal/cli/init_test.go, .capcom/knowledge/, docs/site/reference/file-format.md, docs/site/concepts/knowledge.md, docs/site/concepts/dependencies.md, docs/site/concepts/issues.md, docs/site/reference/cli/
**Read first:** internal/cli/create.go — createLong; internal/cli/update.go — updateLong; internal/cli/related.go — relatedAddLong, formatRaisedWarning; internal/cli/testdata/help/create.txt; internal/cli/node_verbs_test.go — TestHelp_NodeVerbs_MatchGolden;
  internal/cli/help_text_test.go — TestHelpText_EveryCommand_PrintsExamplesBeforeShortRules, helpRuleMaxLines; docs/site/reference/file-format.md — Issue files (standing), Related links, Supersession, Config format; internal/check/problem.go — addFormatTooOld
**Tests:** help goldens under `internal/cli/testdata/help/` (e.g. `create.txt`) refreshed the way their harness says (`node_verbs_test.go` reads them as is; `-update` covers only json and kv) and read; the format-too-old advice pins updated in `internal/check/check_test.go`, `internal/cli/check_test.go` and `internal/cli/init_test.go`.
**Acceptance:** `GOMEMLIMIT=2GiB go test -count=1 ./internal/check/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Help|NodeVerbs|Check|Init' ./internal/cli/ && go run ./cmd/gendocs && git diff --exit-code docs/site/reference/cli/ && ! grep -rniE "raises (the|its) (store )?format" docs/site internal/cli/*.go`
**Closes:** explicit-format-upgrade
**Commit:** `docs: a write needing a newer format points to capcom upgrade`

## 6. Compatibility page: what capcom keeps stable — ✅ DONE (2026-11-22)

NOTES (2026-11-22): the page adds two test-backed store promises beyond the item's list: a write needing no newer format keeps config.yaml (`TestFormatFixture_LogKeepsFormat`) and an older capcom refuses a newer store (`TestFormatFixture_NewerFormatRefused`); the upgrade-only raise cites `TestFormatFixture_StandingNeedsUpgrade` and `TestFormatFixture_ApplyNeedingNewerFormat_WritesNothing` beside ADR 0019, and the KV promise also cites `TestGolden_KVCovered`.

**What:** Depends on item 5.
**Goal:** `docs/site/reference/compatibility.md` (`sidebar: { order: 5 }`) covers four surfaces plus a deprecation rule, each promise naming the test that enforces it: store files (every format ever written is read — `TestFormatFixture_*`, `internal/cli/testdata/stores/format-*`; a format is raised only by `capcom upgrade`, ADR 0019; the CHANGELOG names each format bump); JSON/KV (within `schema: 1` fields are only added — `TestGolden_ReadKinds`, `TestGolden_WriteKinds`, `TestGolden_KV`; a breaking change bumps `schema`); exit codes 0–4 and error codes `not_found exists invalid conflict locked not_initialized` fixed, new ones only added (`TestError_ExitCode_MapsCodes`, `TestDefaultHints_EveryCode_HasOne`, `TestExitCodes`, `TestGolden_Errors`); human text and `capcom ui` may change in any release. Deprecation: anything stable that must go is warned about for at least one minor release first, with a CHANGELOG entry. Linked from `README.md` ("Stable output"), `docs/site/getting-started/faq.md` (a new `##` question), `CONTRIBUTING.md` (a change breaking a promise needs a schema or format bump and a CHANGELOG entry), `llms.txt` and `docs/site/index.md` beside the other reference pages.
**Approach (assumed at the header base):** check each claim against the code before writing it: `output.SchemaVersion` (`internal/cli/output/output.go`), exit codes and error codes (`internal/cli/output/errors.go`), test names by grep. Promise only what a test enforces; drop any claim without one.
**Regression guard.** The six error codes and their exit codes cite `TestError_ExitCode_MapsCodes` and `TestDefaultHints_EveryCode_HasOne` (`internal/cli/output/errors_test.go`): `TestExitCodes` and `TestGolden_Errors` exercise only `not_found`, `invalid` and `locked`. "The CHANGELOG names each format bump" and the deprecation rule are stated as policy without a test. Acceptance fails on a missing test name or an unlinked file, and checks the sidebar order.
**Files:** docs/site/reference/compatibility.md, README.md, docs/site/getting-started/faq.md, CONTRIBUTING.md, llms.txt, docs/site/index.md
**Read first:** internal/cli/output/errors.go — Code consts, ExitOK..ExitNothingReady; internal/cli/output/errors_test.go — TestError_ExitCode_MapsCodes, TestDefaultHints_EveryCode_HasOne; internal/cli/golden_kv_test.go — TestExitCodes, TestGolden_KV; internal/cli/golden_write_test.go — TestGolden_WriteKinds, TestGolden_Errors;
  internal/cli/format_fixture_test.go — TestFormatFixture_Reads, TestFormatFixture_CurrentFormatHasFixture; internal/cli/output/output.go — SchemaVersion; docs/site/reference/json-output.md — schema paragraph; README.md — "Stable output" bullet
**Tests:** none (docs); every test name on the page must exist.
**Acceptance:** `` test -f docs/site/reference/compatibility.md && ! for t in $(grep -oE '`Test[A-Za-z_]+' docs/site/reference/compatibility.md | tr -d '`' | sort -u); do grep -rqn "func $t" internal || echo "missing $t"; done | grep -q missing && for f in README.md docs/site/getting-started/faq.md CONTRIBUTING.md llms.txt docs/site/index.md; do grep -q compatibility.md "$f" || exit 1; done && grep -q 'sidebar: { order: 5 }' docs/site/reference/compatibility.md ``
**Closes:** compat-policy-doc
**Commit:** `docs(reference): state the compatibility promise`

## 7. The elsewhere scan reads every changed copy through one git process — ✅ DONE (2026-11-22)

NOTES (2026-11-22): showFileLimit moved from internal/gitx/branches_test.go into internal/gitx/blobs_test.go, its only remaining user, with its comment naming ReadBlobs.
NOTES (2026-11-22): a copy whose path holds a line break gets the warning "branch <name>: <path>: path holds a line break; its branch copy is not read" (errLineBreak); its Modified entry is kept with no status.
NOTES (2026-11-22): the line-break test skips on Windows, whose file names cannot hold a line break.

**What:** Recast at the regression check (2026-11-22).
**Goal:** `elsewhere.Scan` reads the modified copies of all branches through `gitx.ReadBlobs`, one `git cat-file --batch` per 64 copies; its output (entries, their order, every warning text pinned by `internal/elsewhere/elsewhere_test.go`) is unchanged; `gitx.ShowFile` is gone. The 2026-11-22 profile is recorded on the issue.
**Approach (assumed at the header base):** in `internal/elsewhere/elsewhere.go`, split `scanBranch` in two passes: first `IsAncestor` and `ChangedFiles` per branch, then `gitx.ReadBlobs` (`internal/gitx/blobs.go`, already used by recap) over the `<branch.Commit>:<path>` copies in chunks of at most 64; `branchStatus` works from the returned blobs; warnings buffered per branch to keep branch order. Copies stay addressed by commit, not ref. Delete `gitx.ShowFile` (`internal/gitx/branches.go`) and its tests. Then `capcom log warm-reads-large-store` with the profile lines from the header's Sources (ready 71% in `elsewhere.Scan`, 58% `ShowFile`; next loads issues twice; prime ~45% `check.Run`).
**Regression guard.** `ReadBlobs` runs in chunks of at most 64 copies, each chunk's bytes dropped before the next (one `cat-file --batch` per 64 copies). On a `ReadBlobs` error every requested copy of that chunk gets `Blob{Err: err}` and keeps its Modified entry and its "branch <name>: …" warning. A path holding "\n" is skipped from the batch with a warning and keeps its Modified entry. Item 7 introduces the elsewhere test seams as package-level vars in internal/elsewhere/elsewhere.go (`readBlobs`, wrapping gitx.ReadBlobs, and `changedFiles`, wrapping gitx.ChangedFiles), so in-package tests can count calls and inject failures. `showFileLimit` stays for `blobs_test.go`: kept in `branches_test.go` with its comment naming ReadBlobs, or moved into `internal/gitx/blobs_test.go`; the `ErrBlobTooLarge` doc (`branches.go`) names ReadBlobs only. An unreadable copy's warning now carries ReadBlobs' wording (`branch <name>: <commit>:<path>: blob is missing`, or a `cat-file --batch` error); only the warnings pinned by `elsewhere_test.go` keep their text. This supersedes nothing: it yields to the hostile-branch rule of `gitx/branches.go` (a hostile branch must not make the scan buffer unbounded data) and review F-02(d) (`docs/reviews/archived/code-and-architecture-review-2026-10-13.md`), which still bind.
**Files:** internal/elsewhere/elsewhere.go, internal/elsewhere/elsewhere_test.go, internal/gitx/branches.go, internal/gitx/branches_test.go, internal/gitx/blobs_test.go, .capcom/issues/code-maturity/warm-reads-large-store.md
**Read first:** internal/elsewhere/elsewhere.go — scanBranch, branchStatus, Report.warn, Scan; internal/gitx/blobs.go — ReadBlobs, readBatch, readAnswer, Blob, ErrBlobMissing; internal/gitx/branches.go — ShowFile, ErrBlobTooLarge;
  internal/gitx/branches_test.go — showFileLimit, TestShowFile_ReadsTheBranchCopy; internal/gitx/blobs_test.go — TestReadBlobs_SeveralBlobs_ReadsEachInOneCall; internal/recap/recap.go — readCopies;
  internal/elsewhere/elsewhere_test.go — onBranch, scan, entryLines, TestScan_OversizedBranchCopy_WarnsOnceAndReportsNoEntry, TestScan_UnparsableCopy_ReportsModifiedWithAWarning
**Tests:** `internal/elsewhere` tests pass unchanged; add one asserting a scan over 3 branches with 2 modified copies each yields the same entries as before (order included); one where more than 64 copies span two chunks with the same entries; one where a chunk's `ReadBlobs` fails (the `readBlobs` seam swapped for a failing func, restored by `t.Cleanup`) and every copy keeps its Modified entry and warning; one where a path holding "\n" is skipped with a warning and keeps its Modified entry.
**Acceptance:** `go vet ./internal/elsewhere/ ./internal/gitx/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/elsewhere/ ./internal/gitx/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Elsewhere|Prime|Ready' ./internal/cli/ && ! grep -rn "ShowFile" internal && grep -q 'gitx.ReadBlobs' internal/elsewhere/elsewhere.go && capcom show warm-reads-large-store | grep -q '71%'`
**Commit:** `perf(elsewhere): read changed copies through one cat-file batch`

## 8. The elsewhere scan starts fewer git processes per branch — ✅ DONE (2026-11-22)

NOTES (2026-11-22): the unmerged filter runs on the candidates before withoutDuplicates (after HasCommit(HEAD)), so merged branches no longer cost the local/remote pair comparison; the reported entries are the same, since a branch dropped as merged could only have been dropped as redundant or yielded nothing.
NOTES (2026-11-22): a failing `git for-each-ref --no-merged` gives one new warning, "elsewhere: list unmerged branches: <git error>", and ends the scan, as a failing branch listing does; before, a merge-base failure warned per branch ("branch <name>: ...").
NOTES (2026-11-22): the six-branch order test gives each branch an issue file of its own, since branches with identical content committed in the same second share one commit and tree, and removing that tree would break two branches.

**What:** Recast at the regression check (2026-11-22). Depends on item 7.
**Goal:** `elsewhere.Scan` finds the branches not merged into HEAD with one `gitx.UnmergedRefs` call (`git for-each-ref --no-merged`), and runs the per-branch `ChangedFiles` diffs on a bounded worker pool, results kept in branch order; output and warnings unchanged.
**Approach (assumed at the header base):** keep `gitx.Branches` and add `gitx.UnmergedRefs` beside it, called once HEAD has a commit; keep `IsAncestor` only if another caller uses it. The pool collects into a slice indexed by branch position; a branch's failure warning stays in its slot.
**Regression guard.** `gitx.Branches` stays unchanged. A new `gitx.UnmergedRefs(dir, base)` (`for-each-ref --no-merged=<base> --format=%(refname)`) runs only after `HasCommit(HEAD)` is true and filters the candidates; `IsCurrent` and `Upstream` still come from every branch, the current one included. The worker bound is pinned in a test, not the Goal. The pool hands item 7's chunked blob pass its changes in branch order. Item 8's pool calls the `changedFiles` seam item 7 introduced and its tests use the same in-package style.
**Files:** internal/gitx/branches.go, internal/gitx/branches_test.go, internal/elsewhere/elsewhere.go, internal/elsewhere/elsewhere_test.go
**Read first:** internal/elsewhere/elsewhere.go — Scan, otherRecentBranches, withoutDuplicates, redundantRef, scanBranch; internal/gitx/branches.go — Branches, HasCommit, IsAncestor, ChangedFiles; internal/gitx/gitx.go — run, Error;
  internal/elsewhere/elsewhere_test.go — TestScan_BranchWithAMissingTree_WarnsForItAndReportsTheOthers, TestScan_UpstreamOfTheCurrentBranch_IsNotReported, TestScan_FreshRepository_ReportsNothing, newClone, gitOutput; internal/gitx/branches_test.go — TestIsAncestor_MergedAndDivergedBranches, commitFile
**Tests:** gitx: `UnmergedRefs` excludes a merged branch and includes an unmerged local and remote branch; `TestBranches_*` pass unchanged; elsewhere: 6 branches with one failing tree give the same entries and the one warning in branch order across 20 runs; a test swaps the `changedFiles` seam for a wrapper recording peak concurrent calls (restored by `t.Cleanup`) and pins the pool at most `runtime.NumCPU()` workers; `TestScan_FreshRepository_ReportsNothing` and `TestScan_UpstreamOfTheCurrentBranch_IsNotReported` pass unchanged.
**Acceptance:** `go vet ./internal/gitx/ ./internal/elsewhere/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/gitx/ ./internal/elsewhere/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Elsewhere|Next|Ready' ./internal/cli/ && ! grep -q 'IsAncestor(options.WorktreeRoot, branch.Ref, head)' internal/elsewhere/elsewhere.go && grep -q 'gitx.UnmergedRefs' internal/elsewhere/elsewhere.go`
**Commit:** `perf(elsewhere): list unmerged branches once and diff them in parallel`

## 9. `next` loads the issues once — ✅ DONE (2026-11-22)

NOTES (2026-11-22): the briefing test (`TestReadyListJudged_Judgement_IsTheReadyRulesOnEveryIssue`) is in internal/briefing/ready_test.go, beside the ReadyList tests and their fixtures, not in internal/briefing/briefing_test.go as **Files:** names; briefing_test.go is untouched.
NOTES (2026-11-22): `briefing.ReadyListJudged` returns a `briefing.Judgement` (the full `issue.Ready` verdict slice and the DepGraph) rather than a verdict map; `cli.newShowGraph` builds show's map from it, and `loadShowGraph` now calls `newShowGraph` too. `showResult` and `emitShown` take a `*showGraph` (nil loads it); show, start and context pass nil. The counting seam is the package variable `afterShowGraphLoad` in show.go, nil in production.

**What:**
**Goal:** When an issue is Ready, `capcom next` does not call `loadShowGraph`: the nodes and Ready verdicts its Ready list computed are reused to show the chosen issue; its text, JSON and KV output are unchanged.
**Approach (assumed at the header base):** add a sibling of `briefing.ReadyList` (via `judgeFiltered`) that also returns the loaded nodes and verdicts; `cli/next.go` builds the show graph from them and passes it to `emitShown` / `showResult` instead of `loadShowGraph` loading all nodes and rerunning `issue.Ready`. `show` keeps calling `loadShowGraph`.
**Regression guard.** `briefing.ReadyList` keeps its signature (callers in `internal/ui/ready.go`, `internal/cli/finish.go`, `internal/cli/ready.go` and the `readyList` seam): a sibling (e.g. `ReadyListJudged`, returning the items plus the verdict map and DepGraph `judgeFiltered` builds) serves only `next`. `start.go` and `context.go`, the other `showResult` callers, keep `showResult` falling back to `loadShowGraph`. The Goal is scoped to the Ready path: the nothing-Ready path still runs `ReadyHidden`, which judges again.
**Files:** internal/briefing/ready.go, internal/briefing/briefing_test.go, internal/cli/next.go, internal/cli/show.go, internal/cli/start.go, internal/cli/context.go, internal/cli/next_test.go
**Read first:** internal/briefing/ready.go — ReadyList, judgeFiltered, unblocksByCloser; internal/cli/show.go — emitShown, showResult, loadShowGraph, showGraph.facts; internal/cli/next.go — newNextCommand, emitNothingReady; internal/cli/ready.go — allHiddenWarnings;
  internal/ops/ops.go — Ops.IssueNodes; internal/ui/ready.go — ReadyList caller; internal/briefing/ready_test.go — indexIssueFiles, readyFixture; internal/cli/next_test.go — TestNext_OneReadyIssue_PrintsItAsShowDoes
**Tests:** existing `next` tests and goldens pass unchanged; a briefing test that the exposed verdicts match `issue.Ready` on a small tree; a seam counting `loadShowGraph` calls asserts zero for `next` with a Ready issue and one for `show`.
**Acceptance:** `go vet ./internal/briefing/ ./internal/cli/ ./internal/ui/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/briefing/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Next|Show|Golden' ./internal/cli/`
**Commit:** `perf(next): reuse the ready list's load to show the issue`

## 10. `prime` reads its sources concurrently — ✅ DONE (2026-11-22)

NOTES (2026-11-22): readLocks runs concurrently too, alongside readWork, readHealth and scanElsewhere, so all four sources share one readSources path; the join still appends failures in source order (store, index/ready/doing, locks, check, elsewhere).
NOTES (2026-11-22): on a recovered panic the Failures keep those the panicking source recorded before it panicked (then the SectionInternal one), exactly as the sequential Compose did; the regression guard named only the sources before it. Added TestCompose_SourcePanicsAfterAFailure_KeepsTheFailuresBeforeIt ([locks prime]) to pin the guard. The overlap test was confirmed to fail against a sequential readSources (then restored).

**What:**
**Goal:** `briefing.Compose` runs `readWork`, `readHealth` and `scanElsewhere` concurrently; its sections, their order and the order of `Failures` are the same as a sequential run (the order Compose reads its sources).
**Approach (assumed at the header base):** in `internal/briefing/briefing.go`, start each source in its own goroutine writing to its own result slot, wait for all, then assemble sections and append failures in the fixed source order. The sources share no mutable state; check that before changing (the index handle, the fake `runCheck` in `briefing_test`).
**Regression guard.** Each goroutine recovers its own panic into its slot; after the wait the first panic in source order yields `Briefing{Failures: failures of the sources before it + one SectionInternal "internal error: <value>"}` with every section nil, as today. This keeps the Compose doc's promise (`briefing.go:96-97`: "Even a defect becomes a Failure of SectionInternal, so a Briefing never costs a session its start"). An overlap test's `readyList` and `runCheck` fakes each wait (with a timeout) for the other to have started, so it fails against a sequential Compose.
**Files:** internal/briefing/briefing.go, internal/briefing/briefing_test.go
**Read first:** internal/briefing/briefing.go — Compose, Briefing.fail, readWork, readLocks, readHealth, scanElsewhere, the openIndex/readyList/listIssues/runCheck seams;
  internal/briefing/briefing_test.go — TestCompose_PanickingSource_IsOneInternalFailureAndNoSection, TestCompose_CheckFailsAndElsewhereWarns_FailInOrderAndKeepTheRest, TestCompose_BrokenIndexAndLocks_FailInOrderAndKeepTheRest, newBriefingRepo, failedSections;
  internal/check/check.go — Run; internal/store/store.go — Store; internal/cli/prime.go — Compose caller
**Tests:** a test where every source fails asserts `Failures` order; the overlap test above; `TestCompose_PanickingSource_IsOneInternalFailureAndNoSection` and the existing `briefing_test` and `prime` CLI tests pass unchanged.
**Acceptance:** `go vet ./internal/briefing/ && GOMEMLIMIT=2GiB go test -count=1 -race ./internal/briefing/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Prime' ./internal/cli/`
**Commit:** `perf(prime): read the briefing sources concurrently`

## 11. `tree.Load` parses files in parallel — ✅ DONE (2026-11-22)

NOTES (2026-11-22): the opt-in is a new `tree.LoadParallel` (only `check.loadTree` calls it); `load` gained a workers argument, 1 for Load and LoadContentFS. The pool is `preread` in tree.go: it reads the walked node files on min(NumCPU, files) goroutines into a slice by walk position, then hands `readContentFacts` (claims.go, unchanged and still sequential) a ContentFunc answering from that slice, so the facts, failures and problems are built in walk order. Orphan-directory files and Claimant still call the AliasFunc one at a time.
NOTES (2026-11-22): internal/tree/claims.go needed no change; internal/tree/export_test.go gained `ReadWorkers` so the overlap test computes the pool's bound; doc.go's file map names LoadParallel. The overlap probe releases every waiter once its timeout fires, so a sequential load fails in ~10 s instead of hanging; confirmed it fails against a sequential LoadParallel (then restored).

**What:** Recast at the regression check (2026-11-22).
**Goal:** The tree loader `check.Run` opts into parses issue and Note files on at most `runtime.NumCPU()` workers; the loaded tree, its order and every error and problem it reports are identical to a sequential load, so `check` and `prime`'s health section are unchanged.
**Approach (assumed at the header base):** in `internal/tree`, read the file list first, parse into a slice indexed by file position on a bounded pool, then build the tree sequentially from the slice, behind the opt-in only `check.Run` uses. Problems stay in path order.
**Regression guard.** The pool lives only behind an opt-in used by `check.Run` (a parallel variant of the tree loader, or an option), never in the shared `readContentFacts` path that `agents` (`block.go` topNotes/LoadContent) and `nodestore` (`load.go`) use, which stay sequential. check's callbacks (`readAliases` and its writes to the parsed/fields maps and `c.problems` in `internal/check/issues.go`, `notes.go`, `check.go`) are made goroutine-safe with one mutex and documented as called concurrently; problems stay in path order. The mutex guards the parsed/fields map writes and every read or append of `c.problems` (`checkConflictMarkers` through `addParseFailure` as one locked step, so `reader.Decode` runs before it), never `c.readFile`'s read or `reader.Decode`. The concurrency test makes overlap deterministic: the callback waits on a barrier (with a timeout) until a second call is in flight; the lower bound is skipped when the pool's bound is < 2, not on NumCPU alone.
**Files:** internal/tree/tree.go, internal/tree/claims.go, internal/tree/tree_test.go, internal/check/issues.go, internal/check/notes.go, internal/check/check.go, internal/check/problem.go, internal/check/check_test.go
**Read first:** internal/tree/tree.go — load, Load, newTree; internal/tree/claims.go — readContentFacts, claimOnlyFacts, firstAliasFailure; internal/check/problem.go — loadTree, readFile;
  internal/check/check.go — Run, checker.add, addParseFailure, checkConflictMarkers; internal/check/issues.go — checker.issues; internal/check/notes.go — checker.notes;
  internal/agents/block.go — topNotes; internal/nodestore/load.go — Kind.load
**Tests:** a tree of 200 files with 3 broken ones loads to the same result and problem list across 20 runs; the test's callback waits on a barrier until a second call is in flight, records peak concurrent calls and asserts 1 < peak <= NumCPU (lower bound skipped when the pool's bound is < 2); a check test runs `check.Run` through the parallel loader; existing `tree` and `check` tests pass.
**Acceptance:** `go vet ./internal/tree/ ./internal/check/ && GOMEMLIMIT=2GiB go test -count=1 -race ./internal/tree/ ./internal/check/ ./internal/briefing/`
**Commit:** `perf(tree): parse store files in parallel`

## 12. The index loads every usable issue without an id list — ✅ DONE (2026-11-22)

NOTES (2026-11-22): internal/index/index_test.go named in Files needed no change; the tests went to internal/index/query_test.go beside the existing List tests.
NOTES (2026-11-22): the all-issues path is taken when IsClosedIncluded is set and every other field is empty by length (an empty non-nil slice counts as empty), via Filter.isEveryUsableIssue; the test seam is beforeAllIssuesRead, and loadAllIssues runs beforeEntryRead between its row read and its list reads, covered by the new TestList_EveryUsableIssueRowsDeletedMidRead_ReturnsOneSnapshot.

**What:**
**Goal:** `index.List` with a Filter that selects every usable issue reads its rows and list columns with `excluded = 0` joins, no id list; results equal the id-list path.
**Approach (assumed at the header base):** in `internal/index/index.go` (`loadIssues`), branch on the all-issues case; the four `path IN json_each` list queries become joins on the same predicate. Other callers keep the id-list path.
**Regression guard.** `loadIssues` takes ids only; the branch lives in `List`/`issues` (`internal/index/query.go`), on a Filter with only `IsClosedIncluded` set (Limit 0, every other field empty). Keep `ORDER BY path` and the `beforeEntryRead` call so `TestList_RowsDeletedMidRead_ReturnsOneSnapshot` still holds. A test seam shows which path ran (`grep -q 'excluded = 0' internal/index/index.go` already matches at aa30b57, so it cannot).
**Files:** internal/index/index.go, internal/index/query.go, internal/index/index_test.go, internal/index/query_test.go
**Read first:** internal/index/query.go — List, issues, Filter.sql, beforeEntryRead; internal/index/index.go — loadIssues, issueLists, scanIssues, pathValues;
  internal/index/query_test.go — TestList_RowsDeletedMidRead_ReturnsOneSnapshot, deleteOnFirstEntryRead, TestList_Filters_SelectMatchingIssuesInTreeOrder; internal/briefing/ready.go — List(Filter{IsClosedIncluded: true})
**Tests:** a test that the all-issues path and the id-list path return equal results on a store with excluded and usable issues, asserting through a test seam that the all-issues path ran; `TestList_RowsDeletedMidRead_ReturnsOneSnapshot` passes unchanged.
**Acceptance:** `go vet ./internal/index/ && GOMEMLIMIT=2GiB go test -count=1 ./internal/index/ && GOMEMLIMIT=2GiB go test -count=1 -run 'Ready|List|Prime' ./internal/cli/`
**Commit:** `perf(index): load all usable issues without an id list`

## 13. Measure the warm reads at N=10000 and record them

**What:** Depends on items 7–12.
**Goal:** On the Raspberry Pi 5 host, with no other test process running, `BenchmarkReady`, `BenchmarkNext` and `BenchmarkListAllJSON` run under 1 s and `BenchmarkPrime` under 1.5 s at N=10000, and `docs/reviews/benchmarks-2026-11-05.md` gains a dated section with the new numbers beside the baseline.
**Approach (assumed at the header base):** run the issue's command; if a target is missed, profile it with the trace method (`-trace`, then `go tool trace -pprof=syscall`; filter with `-focus='benchCommandEach.func1' -ignore=runCLI`) and stop with the finding rather than tuning blind.
**Regression guard.** The new section's heading is `## 2026-11-22 — …` and it follows the earlier sections' fields (Command, Commit, Run, Values, table); the command carries `-timeout=0` as theirs do. Acceptance greps for that heading: `grep -n "2026-11"` already matches every existing heading.
**Files:** docs/reviews/benchmarks-2026-11-05.md
**Read first:** internal/cli/bench_test.go — BenchmarkPrime, BenchmarkReady, BenchmarkNext, BenchmarkListAllJSON, benchCommandEach, benchSizes; docs/reviews/benchmarks-2026-11-05.md — "## 2026-11-05 — recap after `recap-large-store`" section format
**Tests:** none (measurement).
**Acceptance:** `GOMEMLIMIT=2GiB go test -run='^$' -bench='Prime|Ready|Next|ListAllJSON' -benchtime=1x -timeout=0 ./internal/cli` shows the four N=10000 numbers under their targets; `grep -n '^## 2026-11-22' docs/reviews/benchmarks-2026-11-05.md`
**Closes:** warm-reads-large-store
**Commit:** `docs(reviews): record warm-read benchmarks after the fixes`
