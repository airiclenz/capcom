---
title: One finding type and code set for check and the index
status: done
type: task
priority: 2
deps: [membership-owner]
plan: 2026-10-14 - 00 - architecture-review-fixes-plan
created: 2026-10-14T08:35:11Z
closed: 2026-10-14T11:17:48Z
reason: Resolved by plan "2026-10-14 - 00 - architecture-review-fixes-plan" (b1ff6b8).
---

"Is this Tree valid" has two vocabularies. The index reports `Warning`s (re-exports of
`diagnose.Finding` and its kinds, with no non-test callers of the re-exports); check reports
`check.Problem` with its own `check.Code` set. They cover overlapping rules and are kept in step
by hand: a `kindCode` translation from diagnose kinds to check codes, check rebuilding an
`issue.Error` / `knowledge.Error` from a finding just to add a parse failure, and prime / briefing
deliberately dropping index warnings to avoid counting a problem twice. `diagnose` itself is
shallow: `ParseIssue`, `ParseNote`, `ValidateIssue`, `ValidateNote` are one-to-one wrappers. The
two sides already disagree in wording: check reports a repeated id once per pair, the index once
per claimant; their alias findings differ in line and noun.

Wanted: one validation module with one finding type and one code set. The index stores per-file
verdicts; check layers its whole-Tree rules (deps, cycles, plans, refs) on top and keeps reading
files, not the index (deliberate, unchanged). Ratified calls (2026-10-14):

- The finding type takes check's JSON shape (`path`, `line` always present, `code`, `message`);
  check and prime JSON stay byte-identical except for the two wording calls below.
- A repeated id uses the index wording everywhere: one finding per claimant ("... also the id of
  P; both are left out"), code `duplicate`, so check's count rises by one per pair.
- An alias finding gives the frontmatter `aliases` line and the kind noun ("issue" / "note") in
  check and the index alike.
- Kept as is: `repeated-alias` folds into check code `duplicate`; conflict markers stay
  check-only; prime ignores index warnings.

## Acceptance

- `internal/index` declares no `Warning`, `WarningKind` or `WarningX` names.
- Every check code is a `diagnose.Kind`; no `check.CodeX` constants remain; `check.Problem` and
  `diagnose.Finding` are one type.
- `diagnose` exposes one per-file verdict per kind; `ParseIssue`, `ParseNote`, `ValidateIssue`
  and `ValidateNote` are gone.
- Tests pin the closed set of code strings and the JSON element shape of check `data` and prime
  `broken_refs`; check and prime output is unchanged apart from the repeated-id and alias
  wording above.
- `go vet ./...` is clean and the touched packages' tests pass.

Source: docs/reviews/architecture-review-2026-10-14-0848.html, 2 · One answer to "is this Tree valid".
