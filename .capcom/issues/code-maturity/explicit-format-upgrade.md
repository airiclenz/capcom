---
title: An ordinary write silently raises the store format and locks out older capcom
status: open
type: feature
priority: 2
labels: [maturity, compat, needs-decision]
deps: [format-fixture-tests]
created: 2026-11-04T18:27:23Z
---

The first write that needs a newer store format raises `format:` in `.capcom/config.yaml` on
its own and only warns: for example the first standing Issue raises a store below format 5 to
5, the first `related` link raises it to 2 (`internal/cli/related.go`, `formatRaisedWarning`).
The raise runs through `store.Store.RaiseFormat` (`internal/store/store.go:147`), passed to
the node writes as `raise` (`internal/ops/ops.go`, `internal/nodestore/kind.go` `WithRaise`,
`internal/nodestore/edit.go`). From then on every teammate or CI job on an older capcom fails
with `store format N is newer than this capcom reads`. In a team, one person's routine
`capcom create --standing` decides everyone's upgrade, and an agent does it without a human
noticing the warning.

**Decision needed before starting (ask the maintainer, then record it as an ADR under the
`decisions` Notes, superseding the part of ADR 0009 that allows the implicit raise):**

- Proposed: a write that needs a newer format than the store declares fails, writing nothing,
  with code `invalid`, naming the key and the format it needs, and the hint
  `capcom upgrade --format <n>`. A new command `capcom upgrade [--format <n>]` (default
  `config.FormatVersion`) raises `format:` explicitly, prints what the new format adds and
  that older releases can no longer read the store, and is idempotent.
- Alternative: keep the implicit raise as the default but add a config key
  (`format_raise: explicit`) that turns it into the failure above.

Implementation notes for the proposed option: remove the raise from the write path (or make
`raise` a function that returns the new failure), add `internal/cli/upgrade.go` registered in
`internal/cli/root.go`, regenerate the CLI reference with `go run ./cmd/gendocs`, update
`docs/site/concepts/issues.md` (standing), the `related` docs, `docs/site/agents/agent-setup.md`
and `managed-block.md` (format 6), the help texts in `internal/cli/create.go` and
`internal/cli/related.go` that promise the raise, and add the command to the Managed block if
agents should know it. CHANGELOG entry under Unreleased describing the behaviour change.

## Why

A store format change is a team-wide compatibility event; it should happen because someone
chose it, not as a side effect of filing an issue.

## Acceptance

- The chosen behaviour is recorded as an ADR Note.
- With the proposed option: `capcom create --standing` on a format-4 store fails with code `invalid`, writes nothing and names `capcom upgrade --format 5`; `capcom upgrade --format 5` then raises it and the create succeeds.
- `capcom upgrade` on a store already at the target format exits 0 and changes nothing.
- The fixture tests of `format-fixture-tests` cover the new behaviour; docs, help texts and the CLI reference no longer promise an implicit raise.
