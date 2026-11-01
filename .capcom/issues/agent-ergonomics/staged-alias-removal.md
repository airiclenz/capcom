---
title: A staged edit that drops an alias still resolves by that alias
status: open
type: bug
priority: 4
created: 2026-11-01T01:23:25Z
---

Inside one staged batch (`internal/stage/catalog.go`, `catalog.Resolve`), a ref is first matched exactly against the staged issues' ids and aliases, then handed to the index. The index still holds the pre-batch aliases, so when a staged edit removes alias `old` from issue `x`, a later op in the same batch that names `old` resolves through the index to `x`, and `stagedByID` swaps in the staged `x`, which no longer carries `old`. After the commit, `old` no longer resolves at all.

Latent today: none of the five `capcom apply` ops (create, update, dep, close, log) can remove an alias, so no batch can reach this path. It becomes live as soon as an apply op, or another staged-ops caller, can edit aliases.

## Why

A batch must resolve every ref against the state its earlier ops produced. Otherwise the batch and the committed tree disagree on what a ref means: a dry run reports success, and the same ops run one by one would refuse.

## Acceptance

- Within a staged batch, a ref naming an alias that an earlier staged edit removed is refused as not found, the same as after the commit
- A test in `internal/stage` covers it: stage an edit dropping an alias, then `Resolve` that alias
