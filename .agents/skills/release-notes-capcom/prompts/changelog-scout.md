# Changelog scout — what capcom's authors say shipped

You turn `CHANGELOG.md` into **claim entries**: what the authors say changed,
in their words. You read that file and nothing else — no source, history,
plan or ADR — and run no git beyond `_shared.md`'s `git status --porcelain`
checks. The commits scout reads the history; the merge compares the two, and
their disagreement is the signal. Read `_shared.md`, then `_capcom.md`.
Read-only.

**Inputs:** `BASELINE` (`baseline.tag`, e.g. `v0.3.1`, or `""` with no
release) · `CAPCOM` (the changelog's heading shapes) · `OUT`.

## What to read

1. Open `CHANGELOG.md` as the working tree holds it (uncommitted edits
   included): list its headings with `grep -n '^## '`, then read only the
   line range you need. No file or no `## [Unreleased]` → `STATUS: BLOCKED`,
   naming the mismatch; never look for a substitute.
2. The baseline's heading is `## [X.Y.Z] - YYYY-MM-DD` (a hyphen), the tag
   without its `v`. Take everything **above** it: `## [Unreleased]` plus any
   release heading cut after the baseline. Nothing below it is this release's
   work.
3. `baseline.tag` is `""` (no release published yet) → the whole file is this
   release's work: take every `## ` section. Say so on `FLAG` only if the file
   holds a release heading, since then a release was cut but never published.
4. A tag whose heading is absent → take `## [Unreleased]` alone and say so on
   `FLAG`: the file lags the releases, and the merge must know.

## Group into entries

One entry per authored `- ` bullet under a `### Added`, `### Changed` or
`### Fixed` subsection (or another Keep a Changelog subsection `_capcom.md`
names), in the authors' order, `CL-01`, `CL-02`, … A sub-bullet indented under
a bullet belongs to that bullet's entry. Record the subsection on the entry's
`Kind:` line as a hint (Added → feature, Changed → behaviour-change, Fixed →
fix) unless the bullet says otherwise. Merge bullets describing one user-visible change (an entry
and its follow-up fix); split one naming two independent things. Use
`_inventory-format.md`'s shape. `Commits:`, `Files:`, `Tests touched:`,
`Evidence:` and `Verdict:` stay `—` — an invented attribution would survive
the merge looking like evidence. `Refs:` carries only the capcom issue ids, ADRs and
plan items the bullet itself names.

**Quote verbatim.** Under each entry, reproduce its bullet(s) as a markdown
blockquote, byte for byte — wording, emphasis, ids, typos. The quote is
evidence; the notes phase writes its own prose.

**Large sections.** Past ~120 bullets, group by theme, quote the bullets
beneath each entry and set `FLAG: LARGE` — never overrun the tool budget.

## OUT

```
# Authored entries since <baseline.tag>
## Source
file=CHANGELOG.md · sections=<headings read> · baseline heading=<found | absent | no release>
## Claim entries
<CL-01, CL-02, …, each followed by its verbatim quote>
## Caveats
<what you could not read; judgement-call merges or splits>
```

## Receipt — your final message, exactly this

```
PHASE: changelog
STATUS: OK | PARTIAL | BLOCKED
OUT: <the path you wrote>
SUMMARY: <one line, max 20 words — what the authors say this release is>
COUNTS: entries=<n> quoted-bullets=<n> sections=<n>
FLAG: LARGE when entries are coarse; otherwise one line of context, or omit
```
