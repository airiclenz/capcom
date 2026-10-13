---
title: "ADR 0009 decision: config.yaml declares the store's format version, and a binary fails on a store newer than it reads"
---

The files under `.capcom/` are versioned with the code
(ADR 0004), so a store outlives the binary that wrote
it, and two people on one repository can run two releases of capcom against the same files. capcom
is strict about what it reads: an unknown key in an issue, a Note or `config.yaml` is an error,
never ignored. Strictness alone is not enough once a newer release adds a key. An older binary then
sees a file it cannot parse and treats it as a bad file, so the issue disappears from `ready` and
`list`; worse, a write verb that rewrites the tree could drop what it did not understand. The store
therefore says which format it is written in, and a binary that does not know that format stops
before it reads anything else.

**The key.** `config.yaml` carries a top-level `format:` key, an integer. A missing key means
format 1, which is every store written so far. Each binary knows the highest format it can read.

**A store above the binary's highest format fails every command.** The error names
`.capcom/config.yaml` and the line of the `format:` key, says which format the store declares and
which the binary reads up to, and tells the user to upgrade capcom. It is raised when the store is
opened, before any issue or Note file is parsed, so no command reports a file of the newer format
as bad and no write verb touches one. There is no flag that reads a newer store anyway.

**The one exception is `capcom prime`.** A Briefing must never cost the session its start: `prime`
already turns every failure of its sources into a warning and exits 0. A too-new store is reported
the same way, as a single `warning: store:` line carrying the same message, and `prime` exits 0.
The agent still sees the instruction to upgrade capcom.

**Unknown keys stay errors, and any new key raises the format.** Adding a key to the frontmatter of
an issue or a Note, or to `config.yaml`, raises the format by one, in the same release. Without
that rule a binary that predates the key would accept the store's declared format and still
misread the file. Leaving unknown keys an error, rather than ignoring them, keeps a typo in a
hand-edited file loud and keeps a rewrite from silently discarding data.

**`capcom init` writes the key only as a comment.** While the format is 1, the config template
carries `format` as a comment line, never as `format: 1`: binaries released before this decision
(v0.1.0) do not know the key and would reject the config as an unknown key. A store that declares
nothing reads as format 1 to every binary. The first release that raises the format writes the key
for real, and from then on `init` writes the current format into a store it creates. Into an
existing store without `config.yaml`, `init` writes the template with the key still a comment, so
it never raises a store's format: the store stays at format 1 until a write verb that needs a newer
key raises it.
