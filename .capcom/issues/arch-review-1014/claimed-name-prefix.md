---
title: mv, rm and --parent prefix-match past a name a broken file claims
status: open
type: bug
priority: 1
plan: 2026-10-14 - 00 - architecture-review-fixes-plan
created: 2026-10-14T08:35:10Z
---

A name that a file capcom cannot use claims (its id or an alias) is not reserved when a write
verb resolves its argument through `tree.Resolve`: the file is not a node, so the name misses the
exact-id and alias lookups and falls through to prefix matching, which can land on a different
node. Reproduced: with `.capcom/issues/ghost/foo.md` (an orphan: no `ghost.md` beside its
directory) and a valid `foo-bar`, `capcom show foo` reports `id "foo" not found:
.capcom/issues/ghost/foo.md is invalid`, but `capcom rm foo` removes `foo-bar`. `mv` and
`create --parent` resolve the same way, so the wrong node is moved or adopted. The index-backed
reads (`index.Resolve`) already refuse such a name; the tree-backed writes do not, and a write
acting on the wrong node is silent data loss.

Wanted (ratified 2026-10-14): a name an unparsed, invalid, orphan or left-out file claims is
refused by every verb and never prefix-matched past, with the same error kind and wording the
index gives for that name. The one exception stays: `rm` / `know rm` by the exact id of an
unparsed live file still removes that file, so a broken file can be deleted. `mv` naming an
unparseable moved file keeps its existing parse-error refusal.

## Acceptance

- `capcom rm foo`, `capcom mv foo x` and `capcom create x --parent foo` exit 1 and write nothing
  when `foo` is claimed by an unparsed, invalid, orphan or left-out file, with the error kind and
  wording `index.Resolve` gives for `foo`; `know rm` / `know mv` / `know add --parent` alike.
- `capcom rm <id>` and `capcom know rm <id>` by the exact id of an unparsed live file still
  remove it.
- runCLI tests cover each kind of unusable claimant; `go vet ./...` is clean and the touched
  packages' tests pass.

Source: docs/reviews/architecture-review-2026-10-14-0848.html, 1 · One owner for Tree membership and resolution.
