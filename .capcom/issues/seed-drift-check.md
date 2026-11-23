---
title: Nothing tells a seeded repo that capcom ships a corrected base Note
status: done
type: feature
priority: 3
deps: [know-seed]
plan: 2026-11-23 - 03 - know-seed-plan
created: 2026-11-23T16:39:03Z
closed: 2026-11-23T17:41:20Z
reason: "Delivered by plan 2026-11-23 - 03 - know-seed-plan (archived in 2595679): capcom check warns of an outdated seeded Note via the seed-text hash Label, never of an edited one, and capcom know seed --update takes the new text."
---

`capcom know seed` never overwrites a Note. A base Note corrected in a newer capcom reaches a
seeded repository only when someone runs `capcom know seed --dry-run` and reads a
"skipped (differs)" line. Nobody runs that without a reason, so a wrong sentence stays.

Wanted: a seeded Note that is outdated, and not edited on purpose, is mentioned where agents and
users already look, such as `capcom check` or `capcom prime`.

Telling "outdated" from "edited on purpose" is the hard part: a repo that edited a Note must not
be warned forever. It needs the text as seeded to be known, for example a hash stored with the
seed Label, so that only a Note whose body is still the seeded text and differs from the binary's
text counts as outdated. Whether capcom then offers to update such a Note is part of this issue's
design.

## Why

Two review rounds of the first base set each found sentences that were false in some kind of
repository. More will be found after release, and a correction that reaches no seeded repository
is worth little.

## Acceptance

- A seeded Note whose body is unchanged since the seed, and differs from the text in the running
  binary, is mentioned by the chosen command with its id and the way to see the new text.
- A seeded Note the repo edited is never mentioned.
- A repo that was never seeded sees no new output.
- The mention is not a `capcom check` failure.


The check stays driven by the embedded markdown files of the base set (see `know-seed`): a Note
added to or changed in the set later is covered with no change to Go code.
