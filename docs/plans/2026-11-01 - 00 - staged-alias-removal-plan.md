# Staged alias removal — implementation plan

**Goal:** inside one staged batch, a ref that names an Alias which an earlier staged edit removed
resolves exactly as it would after the commit: as an id prefix over the remaining ids, or not
found.
**Date:** 2026-11-01
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 09ef292
**Closes:** staged-alias-removal

**Regression check (2026-11-01, 09ef292):**
- 1: guard folded (`NotFoundError` doc names the prefix-only path).
- 2: guard folded (writer decision: the staged file's recovered Aliases gate the prefix fallback,
  with an unparsed-`x` case; the control is exempt from the pre-item bite check; the fixture
  writes `aliases: [old]` itself); yields to ADR 0002 unusable claims.

**Sources:**
- `.capcom/issues/agent-ergonomics/staged-alias-removal.md`
- Note `adr-0002-membership` (one resolver: `tree.ResolveIn`)

**Ratified design calls:**
- **Fallback after a dropped alias:** resolve the ref as an id prefix only, the post-commit
  answer (prefix match, ambiguous, or not found), not a blanket not-found. (user, 2026-11-01)

**Standing requirements:**
- skills: coding-standards
- Tests on the Pi: single packages with `GOMEMLIMIT=2GiB`, never `./...` with `-race`.

**Out of scope:**
- Alias clashes (two issues listing one alias) that a staged edit resolves; no apply op edits
  aliases.
- Notes (`ResolveNote`); the staged catalog covers issues only.
- New apply ops that edit aliases.

## 1. Prefix-only resolve in tree and index — ✅ DONE (2026-11-01)

**What:**
**Goal:** `tree.ResolvePrefixIn(names, arg)` returns what `tree.ResolveIn` returns for an arg
whose exact name matches nothing, and `(*index.Index).ResolvePrefix(arg)` returns the usable issue
entry that prefix resolution finds, with the same errors `Resolve` gives on that path.

**Regression guard.** `NotFoundError`'s doc in `internal/tree/resolve.go` says it reports an arg
matching "no id, Alias or id prefix"; `ResolvePrefixIn` returns it for an arg that IS an Alias, so
the doc-comment work includes rewording it to "no id, Alias or id prefix, or (ResolvePrefixIn) no
id prefix".

**Approach (assumed at the header base):** split the prefix half of `tree.ResolveIn`
(`names.PrefixCandidates`, then 0 → `*NotFoundError`, 1 → `prefixMatch`, more →
`*AmbiguousError`) into exported `ResolvePrefixIn(names NameReader, arg string) (string, error)`;
`ResolveIn` calls it after its exact `Match`, so one implementation serves both. In
`internal/index/query.go` add `ResolvePrefix(arg string) (Entry, error)` beside `Resolve`: one
`snapshot`, `tree.ResolvePrefixIn(snapshotNames{x, q, issueNodes}, arg)`, then `x.resolved`. Doc
comments name the difference from `Resolve`: an exact id or Alias is not matched as such.

**Files:** internal/tree/resolve.go, internal/tree/resolve_test.go, internal/index/query.go,
internal/index/query_test.go
**Read first:** internal/tree/resolve.go — ResolveIn, prefixMatch, NotFoundError;
internal/index/query.go — Resolve, snapshotNames.PrefixCandidates, resolved;
internal/tree/resolve_test.go — resolveFixture; internal/index/query_test.go — issueWith

**Tests:**
- tree: `ResolvePrefixIn` on a name that is an Alias of one node and a prefix of another id
  returns the other id; on a unique prefix returns it; no prefix → `*NotFoundError`; two →
  `*AmbiguousError`. Existing `ResolveIn` tests stay green.
- index: `ResolvePrefix("old")` with issue `x` aliasing `old` and issue `older-thing` returns
  `older-thing`; without `older-thing` → `*tree.NotFoundError`.

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/tree/ ./internal/index/`
- `go vet ./internal/tree/ ./internal/index/`

**Commit:** `feat(index): resolve an issue by id prefix only`

## 2. Staged catalog honours a dropped alias — ✅ DONE (2026-11-01)

NOTES (2026-11-01): no CHANGELOG entry: the bug is latent (no apply op edits aliases), so no user-visible behaviour changes.
NOTES (2026-11-01): the shared switch is factored as mergeStaged; the fallback check is isAliasDropped, which tests the staged file's recovered Aliases (new stagedIssue.aliases field, from nodestore.Reading.Aliases).

**What:** fixes the latent bug in issue `staged-alias-removal`: `catalog.Resolve` hands a ref to
the index, which still matches the pre-batch Alias, and `stagedByID` then returns the staged issue
that no longer carries it.
**Goal:** within a staged batch, a ref naming an Alias that an earlier staged edit removed from
issue `x` resolves as `ResolvePrefix` plus the staged creates would: to the one id it is a
prefix of (the staged version when staged), `*tree.AmbiguousError` over several, or
`*tree.NotFoundError`; never to `x` by that Alias.

**Regression guard.** Test `name` against the staged file's recovered Aliases (keep
`reading.Aliases`, `internal/nodestore/read.go`, on `stagedIssue`) rather than `entry.Aliases`,
which is empty for a staged file that no longer parses; fall back to `ResolvePrefix` only when
those Aliases lack `name`. So an unparsed staged `x` still listing `old` fails
`*tree.UnusableError`, as today and after the commit. The fixture writes `x`'s file with
`aliases: [old]` itself (`issueText` in `internal/stage/staged_test.go` carries no aliases), so
`newCatalogStage` is not reused as-is. The item yields to ADR 0002 unusable claims
(`.capcom/knowledge/decisions/adr-0002/adr-0002-membership.md`): a name only an unparsed file
claims is refused by every verb and never matched as a prefix.

**Approach (assumed at the header base):** in `catalog.Resolve`
(`internal/stage/catalog.go`), when the index returns an entry it matched by exact Alias
(`entry.ID != name` and `name` in `entry.Aliases`) and `stagedByID` finds that issue staged with
`name` neither its id nor among its Aliases, replace `entry, err` with
`c.index.ResolvePrefix(ref)` and run the same switch on it (prefix vs staged creates, staged
swap-in, not-found → created prefix, ambiguous merge). Factor the switch so both paths share it;
no duplicated branch logic. Update the `Resolve` doc comment. Depends on item 1.

**Files:** internal/stage/catalog.go, internal/stage/catalog_test.go
**Read first:** internal/stage/catalog.go — catalog.Resolve, staged, stagedByID,
createdPrefixMatches; internal/nodestore/read.go — Reading.Aliases; internal/stage/catalog_test.go
— newCatalogStage; internal/stage/staged_test.go — issueText; internal/stage/overlay.go — Overlay.ReplaceFile

**Tests:** write `x`'s issue file with `aliases: [old]` in the fixture (not `issueText` as-is),
stage an edit dropping alias `old` from `x` through `Overlay.ReplaceFile` (no op edits
aliases), then `Catalog.Resolve`:
- `old` with no other match → `*tree.NotFoundError`.
- `old` with index issue `older-thing` → `older-thing`.
- `old` with a staged create `old-new` → `old-new`.
- control: an alias the staged edit keeps still resolves to `x`.
- an unparsed staged `x` (its new content no longer parses) still listing `old` →
  `*tree.UnusableError`.
Each case but the control must fail on the pre-item tree; the unparsed-`x` case passes there too
and must keep passing (it guards the fallback).

**Acceptance:**
- `GOMEMLIMIT=2GiB go test -count=1 ./internal/stage/`
- `go vet ./internal/stage/`

**Commit:** `fix(stage): resolve a dropped alias as the committed tree would`
