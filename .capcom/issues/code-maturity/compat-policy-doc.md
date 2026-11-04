---
title: No written compatibility promise for files, JSON output and exit codes
status: open
type: task
priority: 2
labels: [maturity, docs, compat]
deps: [json-golden-tests, format-fixture-tests]
created: 2026-11-04T18:27:23Z
---

capcom is at 0.x with frequent releases, and nothing tells an adopter what may change under
them. Three surfaces are depended on from outside: the file format under `.capcom/`
(`docs/site/reference/file-format.md`, store format versions in `internal/config/config.go`),
the `--json` envelope and `--kv` output (`docs/site/reference/json-output.md`, `schema: 1`), and
the exit codes and error `code` values.

Write `docs/site/reference/compatibility.md` (add it to the site sidebar next to the other
reference pages, and link it from `README.md` and `docs/site/getting-started/faq.md`) stating,
for each surface, what is stable and what may change before 1.0, for example:

- Store files: capcom reads every store format it ever wrote (enforced by the fixture tests
  of `format-fixture-tests`); a new format is only entered as the store-format rules say
  (see `explicit-format-upgrade`); the CHANGELOG names every format bump.
- JSON: within `schema: 1`, fields are only added, never renamed, removed or retyped; a
  breaking change bumps `schema` (enforced by `json-golden-tests`).
- Exit codes and error codes: fixed; new ones only added.
- Human text output and the terminal UI: may change in any release; agents should use
  `--json` or `--kv`.
- Deprecation: anything stable that must go is warned about for at least one minor release
  first, with a CHANGELOG entry.

Check every promise against the code before writing it (does `schema` exist and is it ever
bumped? which exit codes exist?), and promise only what the tests enforce. Add a short
pointer to the page in `CONTRIBUTING.md`: a change that breaks a promise needs a schema or
format bump and a CHANGELOG entry.

## Why

Adopters weigh a 0.x tool by what it promises not to break; capcom already behaves
carefully, and writing it down turns that care into something people can rely on.

## Acceptance

- `docs/site/reference/compatibility.md` exists, covers the four surfaces and the deprecation rule, and each promise names the test that enforces it.
- The page is linked from `README.md`, the FAQ, the site sidebar and `CONTRIBUTING.md`.
