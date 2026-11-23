---
title: "ADR 0020 decision: the base set is markdown files embedded in the binary, the only source; a seeded Note is the repository's own, marked by two Labels and replaced only by --update while unedited"
---

Some rules hold in any repository: keep secrets out of what git tracks, ask before a destructive
action, say what was not verified. A new store starts with an empty Knowledge tree, so every
repository had to write these rules itself or go without them. capcom now ships them as a base
set of Notes that `capcom know seed` adds to any store.

**The base set is markdown files embedded in the binary.** Each Note of the set is one markdown
file in `internal/seed/notes`, in the layout of a store's Knowledge tree: one root Note file, and
the Notes below it in the directory named after their parent. The directory is compiled in whole,
and the set is found by walking it. No Go file, test files included, names a Note's id, title or
text or the count of Notes, so wording is maintained by editing a markdown file, and a Note joins
or leaves the set when its file is added or deleted.

**The binary is the only source.** `capcom know seed` takes no path and no URL. A Note is an
instruction to an agent, so a set read from an outside source would be a prompt-injection channel.

**The ids are chosen slugs, so ADR 0002 holds.** The id of a Note of the set is its file name,
chosen by whoever wrote the file; capcom generates none. Every Note sits below one root Note and
carries its id as a prefix, so the capcom block in the agent guide grows by one line and a seeded
id does not collide with a repository's own Note.

**A seeded Note is the repository's own.** From the moment it is written it is edited, moved and
retired like any other Note. Plain `capcom know seed` adds the Notes the store lacks and never
changes a Note that exists: an id the store already holds is skipped, and the line printed for it
says why. Notes are written one at a time, parent first; a failure stops the run and a second run
finishes it.

**Two Labels carry the seed and the hash, so the store format does not change.**
`seed:<root id>` says the Note came from the base set. `seed-text:<version>:<hash>` holds the
capcom version that wrote it and a hash of the title and body it was written with. The hash is how
capcom tells a Note that is still as it was seeded from one that was edited since. Labels are an
existing key of a Note file, so there is no new frontmatter key and no new store format.

**`--update` replaces a Note only while its text is still the seeded text.** A later capcom may
ship a corrected text for a Note. A seeded Note whose title and body still match the hash in its
Label is then outdated: `capcom check` prints a warning that names it, which is no problem and
leaves the exit code as it is, and `capcom know seed --update` replaces its title and body with
the shipped text. A Note that was edited, that was the repository's own before, or that is retired
is never replaced. `capcom prime` says nothing about outdated Notes.

**The repository's own rule wins.** Each sentence of the set is written to hold in any kind of
repository, and the set holds no workflow opinions. Where a seeded Note still contradicts one of
the repository's own Notes, the repository's Note wins; the root Note of the set says so, so a
contradiction needs no edit of the seeded Notes.
