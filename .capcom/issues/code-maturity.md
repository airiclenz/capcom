---
title: Mature capcom's code for adoption beyond its own repository
status: open
type: task
priority: 2
labels: [maturity]
standing: true
created: 2026-11-04T18:27:23Z
---

Groups the code-side work that moves capcom from "well tested on itself" to "safe to adopt":
finding hostile-input bugs before users do, a written and test-enforced compatibility
contract, evidence at scale and under concurrency, and dependency hygiene.

Start with the cheap, independent items: `ci-static-analysis`, `yaml-module-swap` and
`fuzz-parsers`. The compatibility items build on each other: `json-golden-tests` and
`format-fixture-tests` first, then `compat-policy-doc`, which cites them;
`explicit-format-upgrade` needs a decision from the maintainer before it starts.

Every child follows the house rules: the Notes `coding-conventions` and `cli-test-harness`,
a CHANGELOG entry under `## [Unreleased]` for anything user-visible, and on the Raspberry Pi
host one package at a time with `GOMEMLIMIT=2GiB`, never `./...` with `-race`.

This is a standing Parent: it only groups its children and is never claimed or closed.
