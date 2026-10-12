# Glossary and ADR migration plan

**Goal:** move capcom's own glossary (`CONTEXT.md`) and decision records (`docs/adr/0001`–`0010`)
into Knowledge as Note trees in ADR 0010's shape, leave pointer stubs at the old paths, and rewire
every live reference.
**Date:** 2026-10-12
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 5bd0409

**Regression check (2026-10-12, 5bd0409):**
- 1: guard folded (check exits 0 with no output; double-quoted term titles; `CONTEXT.md` unchanged checked)
- 2: guard folded (paragraph-wise `--verify` for ADR 0004's split decision; `docs/adr/` unchanged checked)
- 3: guard folded (childless record body is Status line plus verbatim ADR text; `--verify` covers 0006 and 0007)
- 4: guard folded (one shared `git grep` reference check; bookkeeping clause carve-out for the glossary)
- 5: recast (reuses item 4's grep; owns AGENTS.md's `capcom know tree --depth 2` instruction)
- 1 (round 2): guard folded (the `glossary.md` body never names `CONTEXT.md` or `docs/adr`)
- 2 (round 2): guard folded (the `decisions.md` body never names `CONTEXT.md` or `docs/adr`)
- 4 (round 2): guard folded (shared grep excludes `internal/knowledge/knowledge_test.go`)
- 5 (round 2): guard folded (same exclusion; parent Note bodies rule; Acceptance reads the `adr-0010-amend-` child and the amended Status line)

**Sources:**
- docs/adr/0010-knowledge-holds-a-glossary-and-decision-records-as-note-trees.md (shape, ids, supersession, migration method)
- docs/adr/0002-ids-are-chosen-slugs-and-old-names-live-on-as-aliases.md (one-word ids, amended by 0010)
- docs/format.md §Knowledge (Note keys, `retired`, `superseded-by`)
- CONTEXT.md and docs/adr/*.md at the base commit (the text to migrate, verbatim)

**Ratified design calls (owner, 2026-10-12):**
- **Binary gate:** the installed `capcom` must be newer than 0.2.1 (one-word ids, format 3) before any item runs; the owner cuts that release with `release-capcom` first. No item builds or runs a tree-built binary.
- **Old files:** `CONTEXT.md` and `docs/adr/README.md` stay as short pointer stubs; the ten ADR files and the glossary text are deleted.
- **Short ADRs:** a record with no `##` section and no `Amended` paragraph keeps its Status line and full text in its own `adr-NNNN` body, with no children.
- **Cut points:** an ADR splits only at `##` headings and at paragraphs opening `Amended YYYY-MM-DD:`; bold lead-ins (`**Shape.**`) stay inside the decision text.
- **Supersession:** only ADR 0002 is amended by another record: `adr-0002` stays current with `superseded-by: [adr-0010]`. Self-amendments (0004, 0008) are `-amend-MMDD` children, not supersession.
- **Citations:** in-Note links `[ADR NNNN](NNNN-….md)` become plain `ADR NNNN`; human docs outside `.capcom/` link to the Note file path. Code comments citing `ADR NNNN` stay as they are (they resolve as ids).
- **Split script:** a throwaway `python3` script under `docs/skill-runs/migrate-knowledge/` (gitignored), never committed.

**Standing requirements:**
- skills: coding-standards
- Run every `capcom` command with the installed binary on PATH; `capcom check` must exit 0 with no output after each item.
- Note text cut from `CONTEXT.md` or an ADR is verbatim except the citation rewrite above; only titles, Status lines and summaries are newly written.
- A term Note's title is `<Term>: <gist>`; every other new title is one self-contained sentence.

**Out of scope:**
- `refs:` on the migrated Notes; plural aliases for terms (`notes`, `issues`); labels.
- Rewriting `ADR NNNN` citations in Go code, and anything under `docs/plans/`, `docs/handoffs/`, `docs/reviews/`, `docs/skill-runs/` or `CHANGELOG.md`.
- Any change to capcom's code, tests or release.

## 1. Glossary as a Note tree — ✅ DONE (2026-10-12)

NOTES (2026-10-12): hand-written titles and section summaries live in the throwaway script's tables (docs/skill-runs/migrate-knowledge/split_glossary.py, gitignored) rather than being edited into the files after a first pass; the script writes them double-quoted and capcom fmt normalised the five section titles that need no quoting

**What:**
**Goal:** `.capcom/knowledge/glossary.md` exists with six section children `glossary-structure`, `glossary-work`, `glossary-coordination`, `glossary-knowledge`, `glossary-agent-interface`, `glossary-ambiguities`; every `**Term**:` entry of `CONTEXT.md` is a grandchild Note under its section; the AGENTS.md managed block lists `glossary`; `CONTEXT.md` is unchanged.
**Approach (assumed at the header base):** first confirm the binary gate (Acceptance line 1); stop the item if it fails. Write `docs/skill-runs/migrate-knowledge/split_glossary.py`: parse `CONTEXT.md` into the intro paragraph, the `### <Section>` blocks under `## Language`, each `**Term**:` block (definition lines plus its `_Avoid_` line), and `## Flagged ambiguities`. Term id = the term lowercased, spaces to hyphens (`tree-lock`, `format-version`, `status-category`, `related-link`, `lock-token`, `managed-block`). Write `glossary.md` (body: the intro paragraph, then one line per section in the file's order, citing each section id), each `glossary/glossary-<section>.md` (body: one line saying what the section covers), each term file `glossary/glossary-<section>/<id>.md` with the definition and `_Avoid_` line verbatim. `glossary-ambiguities` holds the ambiguity bullet verbatim and has no children. Then write titles by hand: terms `<Term>: <gist>` read on their own; sections and `glossary` a one-sentence title. Run `capcom fmt`, then `capcom agents`. Binding rule: the `glossary.md` body never names `CONTEXT.md` or `docs/adr` (the shared reference grep's directory exclusions do not cover the parent Note).
**Regression guard.** `capcom check` prints nothing on success (`check: no problems` is `capcom prime`'s line), so it passes by exiting 0 with no output. Every term title holds ": ", which a plain YAML scalar rejects, so the script writes each `title:` double-quoted (as `.capcom/knowledge/docs-layout.md` does) and lets `capcom fmt` normalise the quoting. `CONTEXT.md` stays byte-identical and is checked.
**Files:** .capcom/knowledge/glossary.md, .capcom/knowledge/glossary/ (new tree), AGENTS.md
**Read first:** CONTEXT.md; internal/knowledge/note.go — Parse, Validate; internal/cli/check.go — newCheckCommand;
internal/tree/resolve.go — NormalizeArg; internal/tree/tree.go — ValidateID; internal/agents/block.go — TopNotes; internal/cli/know.go — outline
**Tests:** no Go change; the Acceptance commands are the check.
**Acceptance:**
- `capcom version` prints a version other than `capcom 0.2.1`, and `capcom know update --help | grep -q superseded-by`
- `capcom check` exits 0 with no output; `capcom agents --check` exits 0
- `git diff --quiet HEAD -- CONTEXT.md` exits 0
- `find .capcom/knowledge/glossary -name '*.md' | wc -l` prints 33 (6 sections, 27 terms), equal to 6 + `grep -c '^\*\*[^*]*\*\*:$' CONTEXT.md`
- `capcom know show "Tree lock"` shows id `tree-lock`; `capcom know show "Format version"` shows id `format-version`; `capcom know tree glossary` lists every section and term
- for every term, its Note body appears verbatim in `CONTEXT.md` (the script's own `--verify` mode, run after the hand-written titles)
**Commit:** `docs(knowledge): move the glossary into Knowledge`

## 2. Decision records 0001–0005 and the decisions parent — ✅ DONE (2026-10-12)

NOTES (2026-10-12): each adr-NNNN-rejected body is the Considered Options section's content verbatim without its "## Considered Options" heading line, which the child's title replaces; hand-written titles and summaries live in the throwaway script's tables (docs/skill-runs/migrate-knowledge/split_adr.py, gitignored; --parent writes decisions.md, --verify checks)
NOTES (2026-10-12): adr-0002 carries no superseded-by key yet: adr-0010 does not exist until item 3, which owns that supersession

**What:**
**Goal:** `.capcom/knowledge/decisions.md` exists, and `decisions/adr-0001` … `adr-0005` hold ADRs 0001–0005 in ADR 0010's shape; the AGENTS.md managed block lists `decisions`; `docs/adr/` is unchanged.
**Approach (assumed at the header base):** write `docs/skill-runs/migrate-knowledge/split_adr.py`, taking ADR file paths: cut each file at its H1 (the record title), the prose before the first `##` with any `Amended YYYY-MM-DD:` paragraph removed (`adr-NNNN-decision`), each `## Considered Options` section verbatim (`adr-NNNN-rejected`), and each `Amended YYYY-MM-DD:` paragraph verbatim (`adr-NNNN-amend-MMDD`). Rewrite `[ADR NNNN](….md)` links to `ADR NNNN`. A record with no `##` and no amendment keeps its full text in `adr-NNNN.md` with no children. Record body: `Status: Accepted.` (with `; amended YYYY-MM-DD` per amendment), a blank line, then a 3–5 line summary written by hand that names the reading order of its children. Record title = the H1 verbatim; child titles one sentence each. `decisions.md` title is one sentence; its body lists all ten records (ids `adr-0001` … `adr-0010`, titles from the ADR H1s) with one line each, written now so item 3 never touches it. Run the script on 0001–0005, write titles and summaries, then `capcom fmt` and `capcom agents`. Binding rule: the `decisions.md` body never names `CONTEXT.md` or `docs/adr` (the shared reference grep's directory exclusions do not cover the parent Note).
**Regression guard.** `--verify` checks a decision child paragraph by paragraph: it equals the pre-`##` prose with its `Amended` paragraphs removed, and each of its paragraphs appears verbatim (after the citation rewrite) in the ADR file — never one substring match, since ADR 0004's `Amended 2026-10-11:` paragraph sits between its decision paragraphs. `capcom check` passes by exiting 0 with no output. `docs/adr/` stays unchanged and is checked.
**Files:** .capcom/knowledge/decisions.md, .capcom/knowledge/decisions/ (adr-0001 … adr-0005 and children), AGENTS.md
**Read first:** docs/adr/0004-issues-are-versioned-with-the-code.md — Amended 2026-10-11 paragraph; internal/index/query.go — resolveID; internal/index/notes.go — ResolveNote;
internal/cli/know.go — noteText, resolveNote, outline; docs/adr/0010-knowledge-holds-a-glossary-and-decision-records-as-note-trees.md — Shape paragraph;
internal/cli/check.go — newCheckCommand
**Tests:** no Go change; the Acceptance commands are the check.
**Acceptance:**
- `capcom check` exits 0 with no output; `capcom agents --check` exits 0
- `git diff --quiet HEAD -- docs/adr` exits 0
- `capcom know show "ADR 0005"` shows id `adr-0005` and a body opening `Status:`
- `capcom know tree decisions` lists `adr-0001` … `adr-0005`; `adr-0002` has an `adr-0002-amend-1012` child and `adr-0004` an `adr-0004-amend-1011` child
- `grep -rn '\](0' .capcom/knowledge/decisions` prints nothing
- the script's `--verify` mode confirms every rejected and amend body appears verbatim (after the citation rewrite) in its ADR file, and every decision body paragraph by paragraph (Regression guard)
**Commit:** `docs(knowledge): move ADRs 0001 to 0005 into Knowledge`

## 3. Decision records 0006–0010 and ADR 0002's supersession — ✅ DONE (2026-10-12)

NOTES (2026-10-12): the format and superseded-by keys were written only by `capcom know update adr-0002 --add-superseded-by adr-0010` (capcom 0.3.0), which put `format: 3` as the first line of .capcom/config.yaml, above the header comment; neither key was hand-edited
NOTES (2026-10-12): the throwaway split_adr.py (gitignored) gained the 0008–0010 summaries and child titles, and its --verify now requires a childless record's body to be exactly "Status: Accepted." plus the ADR text below the H1, with no child directory; --verify --parent over all ten ADRs reports 10 records, 19 children, 0 failures

Depends on item 2.

**What:**
**Goal:** `decisions/adr-0006` … `adr-0010` hold ADRs 0006–0010 in the same shape as item 2; `adr-0006` and `adr-0007` have no children; `adr-0002` carries `superseded-by: [adr-0010]` and is not retired; `.capcom/config.yaml` declares `format: 3`.
**Approach (assumed at the header base):** run item 2's `split_adr.py` on 0006–0010 (rewrite it from item 2's Approach if it is gone), write titles and summaries by item 2's rules, then `capcom know update adr-0002 --add-superseded-by adr-0010`, which raises the store format itself. Never hand-edit `format:` or `superseded-by`. Run `capcom fmt`.
**Regression guard.** A childless record (`adr-0006`, `adr-0007`: no `##`, no amendment) has the body `Status: Accepted.`, a blank line, then the ADR text below the H1 verbatim (after the citation rewrite), with no summary; `--verify` checks that body against the ADR file for 0006 and 0007. `capcom check` passes by exiting 0 with no output.
**Files:** .capcom/knowledge/decisions/ (adr-0006 … adr-0010 and children, adr-0002.md), .capcom/config.yaml
**Read first:** internal/cli/know_update.go — updateNote, gainsSupersession; internal/cli/related.go — raiseStoreFormat; internal/config/config.go — RaiseFormat;
internal/knowledge/note.go — Validate; internal/cli/know.go — noteText; docs/adr/0006-migration-is-agent-written-files-checked-by-capcom.md;
docs/adr/0007-tests-use-real-temp-files-sqlite-and-git.md
**Tests:** no Go change; the Acceptance commands are the check.
**Acceptance:**
- `capcom check` exits 0 with no output; `capcom agents --check` exits 0
- `capcom know show adr-0002` shows `superseded-by: [adr-0010]` and no `retired`; `grep -n '^format: 3$' .capcom/config.yaml` matches
- `capcom know tree decisions` lists all ten records; `adr-0008` has an `adr-0008-amend-1012` child; `adr-0006` and `adr-0007` have none
- `grep -rn '\](0' .capcom/knowledge/decisions` prints nothing; `--verify` passes for 0006–0010, the bodies of `adr-0006` and `adr-0007` included
**Commit:** `docs(knowledge): move ADRs 0006 to 0010 into Knowledge`

## 4. Point live references at the Notes

Depends on item 3.

**What:**
**Goal:** no file outside the out-of-scope paths cites an ADR file under `docs/adr/` or reads the glossary from `CONTEXT.md`, except `.capcom/knowledge/docs-layout.md` (item 5's); human docs link to `.capcom/knowledge/decisions/adr-NNNN.md`; Notes cite records as `ADR NNNN`.
**Approach (assumed at the header base):** the rule: every live line matching `CONTEXT\.md\|docs/adr\|adr/00`. Notes `cli-test-harness` and `release-process` drop their `docs/adr/...` ref (their bodies already cite `ADR 0007` / `ADR 0008`). `docs/format.md`: the example Note's refs drop the ADR path; the `[ADR 0009](adr/0009-…)` link targets `../.capcom/knowledge/decisions/adr-0009.md`. `docs/releasing.md`: the ADR 0008 link likewise. `.agents/skills/release-capcom/SKILL.md`: cite `.capcom/knowledge/decisions/adr-0008.md` (`capcom know show "ADR 0008"`). `.agents/skills/release-notes-capcom/prompts/_capcom.md`: decision records live in `.capcom/knowledge/decisions/`; the Docs row lists `.capcom/knowledge/glossary/` instead of `CONTEXT.md`.
**Regression guard.** The reference grep is one shared command defined once in item 4 and immune to output-prefix differences — use `git grep --untracked -n -E 'CONTEXT\.md|docs/adr|adr/00' -- . ':!docs/plans' ':!docs/handoffs' ':!docs/reviews' ':!docs/skill-runs' ':!CHANGELOG.md' ':!CONTEXT.md' ':!docs/adr' ':!.capcom/knowledge/docs-layout.md' ':!.capcom/knowledge/decisions' ':!.capcom/knowledge/glossary' ':!internal/knowledge/knowledge_test.go'` (migrated verbatim Note text may name CONTEXT.md, e.g. ADR 0010's apogee sentence, and is never rewritten; `internal/knowledge/knowledge_test.go`'s `docs/adr` Ref fixtures are out of scope); it must print nothing. Verified by the plan writer on 2026-10-12 at 5bd0409: before this item the grep prints exactly the eight target lines (.agents/skills/release-capcom/SKILL.md:22, .agents/skills/release-notes-capcom/prompts/_capcom.md:52,63, .capcom/knowledge/cli-test-harness.md:4, .capcom/knowledge/release-process.md:4, docs/format.md:274,410, docs/releasing.md:6).
In `_capcom.md`, listing `.capcom/knowledge/glossary/` in the Docs surface row and the bookkeeping clause ("any commit touching only … `.capcom/knowledge/`") must not both hold: carve `.capcom/knowledge/glossary/` out of the bookkeeping clause in the same edit, or drop the glossary from the Docs row; never both rules.
**Files:** .capcom/knowledge/cli-test-harness.md, .capcom/knowledge/release-process.md, docs/format.md, docs/releasing.md, .agents/skills/release-capcom/SKILL.md, .agents/skills/release-notes-capcom/prompts/_capcom.md
**Read first:** .agents/skills/release-notes-capcom/prompts/_capcom.md — bookkeeping clause, User surfaces Docs row; docs/format.md — Note files example, Config `format` bullet;
docs/releasing.md — ADR 0008 link; .agents/skills/release-capcom/SKILL.md — Mission; .capcom/knowledge/cli-test-harness.md — refs;
.capcom/knowledge/release-process.md — refs
**Tests:** no Go change; the Acceptance commands are the check.
**Acceptance:**
- `capcom check` exits 0 with no output
- the reference grep in the Regression guard prints nothing
- every link target this item wrote exists: `ls .capcom/knowledge/decisions/adr-0008.md .capcom/knowledge/decisions/adr-0009.md`
**Commit:** `docs: cite decision records by their Notes`

## 5. Pointer stubs, docs-layout and ADR 0010's amendment

Depends on item 4.

**What:** Recast at the regression check (2026-10-12).
**Goal:** `CONTEXT.md` and `docs/adr/README.md` are pointer stubs of at most 10 lines; no other file is left in `docs/adr/`; Note `docs-layout` sends readers to the `glossary` and `decisions` Notes; `adr-0010` has an `adr-0010-amend-MMDD` child recording that capcom's own repository now keeps both as Notes.
**Approach (assumed at the header base):** `CONTEXT.md` stub: the glossary is the `glossary` Note tree; read a term with `capcom know show "<Term>"`; edit the files under `.capcom/knowledge/glossary/`. `docs/adr/README.md` stub: records are `adr-NNNN` Notes under `decisions` (`capcom know show "ADR 0005"`); a new record takes the next number and ADR 0010's shape. `git rm` the ten ADR files. `docs-layout` body: glossary → `glossary`, ADRs → `decisions`, both edited in place; its refs keep `CONTEXT.md` and `docs/adr` (the stubs) and the rest; its title is unchanged. Amendment child: id from the date the item runs, body `Amended YYYY-MM-DD: capcom's own repository now holds its glossary and decision records as Notes; CONTEXT.md and docs/adr/README.md only point to them.`; update `adr-0010`'s Status line. Run `capcom fmt`; `capcom agents` only if the block went stale.
**Regression guard.** Item 5 reuses item 4's grep with the `':!docs/adr'` and `':!CONTEXT.md'` exclusions dropped and accepts only the stub lines and nothing else; item 5 also owns AGENTS.md's hand-written instruction (outside the managed block) to run `capcom know tree` before work: it becomes `capcom know tree --depth N` with N chosen so the outline prints the top level and its direct children only (verify --depth semantics in internal/cli/know.go — outline), plus `capcom know tree <id>` to open one branch, so the session outline stays short once the 27 term Notes exist; add AGENTS.md to item 5's Files.
The reused grep keeps the `':!internal/knowledge/knowledge_test.go'` exclusion (its `docs/adr` Ref fixtures are out of scope). The parent Notes `glossary.md` and `decisions.md` are not covered by the directory exclusions, so their bodies never name `CONTEXT.md` or `docs/adr`. Acceptance also reads the amendment: `capcom know show adr-0010-amend` (unique prefix) prints "only point to them" and `capcom know show adr-0010` prints "amended".
**Files:** CONTEXT.md, docs/adr/ (README.md new, ten ADR files removed), .capcom/knowledge/docs-layout.md, .capcom/knowledge/decisions/adr-0010.md, .capcom/knowledge/decisions/adr-0010/ (amend child), AGENTS.md
**Read first:** AGENTS.md — hand-written "Run `capcom know tree` before starting work" paragraph (wrapped over two lines today), BEGIN CAPCOM marker;
internal/cli/know.go — newKnowTreeCommand, outline (top level is level 1, --depth 2 adds direct children); docs/adr/0010-knowledge-holds-a-glossary-and-decision-records-as-note-trees.md — opening paragraph "capcom's own repository does for now";
.capcom/knowledge/docs-layout.md — refs, body; internal/knowledge/verify.go — checkRef (directory ref passes)
**Tests:** no Go change; the Acceptance commands are the check.
**Acceptance:**
- `capcom check` exits 0 with no output; `capcom agents --check` exits 0
- `ls docs/adr` prints only `README.md`; `wc -l < CONTEXT.md` and `wc -l < docs/adr/README.md` are each at most 10
- `capcom know show docs-layout` cites `glossary` and `decisions`; `capcom know tree adr-0010` lists an `adr-0010-amend-` child
- `capcom know show adr-0010-amend` (unique prefix) prints "only point to them"; `capcom know show adr-0010` prints "amended"
- item 4's grep, with `':!docs/adr'` and `':!CONTEXT.md'` dropped, prints only the stubs' own lines and nothing else
- `grep -n 'capcom know tree --depth 2' AGENTS.md` matches outside the managed block (outline's top level is level 1, so `--depth 2` adds only its direct children), and the same line names `capcom know tree <id>`
**Commit:** `docs: leave pointers where the glossary and ADRs were`
