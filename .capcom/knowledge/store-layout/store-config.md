---
title: "What config.yaml controls: statuses, types, plans, ready.hide, lock.ttl, elsewhere.days and format"
labels: [architecture]
refs: [docs/site/reference/file-format.md, internal/config/config.go#FormatVersion]
---

`.capcom/config.yaml` is optional, and so is every key in it; a missing key takes the default
`capcom init` writes. What each key controls:

- `statuses`: the issue statuses in order, each mapped to a category (todo, doing, waiting, done,
  dropped). Every rule reads the category, never the name (ADR 0003); the first status listed in a
  category is the one capcom moves an issue into.
- `types`: the issue types an issue may carry.
- `plans`: the directories an issue's `plan:` basename is looked up in, in order.
- `ready.hide`: labels whose issues `ready` leaves out unless asked for.
- `lock.ttl`: how old a Tree lock's heartbeat may grow before the lock is stale.
- `elsewhere.days`: how recent a branch tip must be for the Elsewhere scan to read it.
- `format`: the store format version, a missing key meaning 1 (ADR 0009). A binary refuses a store
  whose format is newer than it reads (`prime` only warns), and any new key in an issue, a Note or
  `config.yaml` raises the format by one. The highest format this capcom reads is `FormatVersion`.

`docs/site/reference/file-format.md` `## Config` is the reference: the file `init` writes, the
allowed values and the format raises.
