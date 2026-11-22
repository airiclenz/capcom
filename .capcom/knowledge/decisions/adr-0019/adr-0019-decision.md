---
title: "ADR 0019 decision: a write that needs a newer store format fails and names capcom upgrade, which raises the format on request and never lowers it"
---

The store declares its format in `config.yaml` (ADR 0009), and a binary that reads only an older
format fails on it. Until now the first write that needed a newer format raised `format:` on its
own and only warned: the first standing Issue raised a store below format 5 to 5, the first
`related` link raised it to 2. From then on every teammate and CI job on an older release failed
with `store format N is newer than this capcom reads`. A format change is a compatibility event
for everyone who shares the store, so it should happen because someone chose it, not as a side
effect of filing an issue, and least of all because an agent did so without anyone reading the
warning.

**A write that needs a newer format fails.** When a write would put a key in a file that the
store's declared format does not allow, it fails with code `invalid` and writes nothing: no issue
or Note file, no `config.yaml`. The message names the key and the format it needs, and the hint
reads `ask the user before raising it for everyone: capcom upgrade --format <n>`, with `<n>` the
format the write needs. The hint addresses the agent as much as the person: raising the format is
the user's call.

**`capcom upgrade [--format <n>]` raises the format on request.** It is the one command that
raises `format:` in `config.yaml`. Without `--format` it raises the store to the newest format the
binary reads; with it, to `<n>`. It is idempotent: on a store already at or above the target it
exits 0, changes nothing, and says the store is already at its format. capcom never lowers a
format, since files written under the newer format may already use its keys.

**The Managed block does not gain `upgrade`.** The block teaches the commands an agent's work loop
uses; raising the format is a rare, deliberate step that the failure's hint names when it is
needed.

**What stays.** Everything else in ADR 0009 holds: the `format:` key, a binary failing on a store
newer than it reads, `prime` only warning, any new key raising the format in the same release, and
`init` never raising an existing store's format. Config keys that need a newer format already fail
when `config.yaml` is parsed, telling the user to set the format; that is unchanged.

**Not chosen: keep the implicit raise and add a config key that turns it into the failure.** It
leaves the default as the hazard, so the store that most needs the failure, one whose team never
read about the key, is the one that does not get it.
