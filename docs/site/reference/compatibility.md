---
title: "Compatibility"
description: "What capcom keeps stable from one release to the next: the store files, the --json and --kv output, the exit codes and error codes, and how something stable is retired. Each promise names the test that holds it."
sidebar: { order: 5 }
---

An agent, script or CI job written against capcom keeps working on a newer capcom as long as it
relies only on what this page promises. Each promise names the test that holds it: a change that
breaks the promise fails that test, so it cannot reach a release unnoticed. The two promises with
no test, the CHANGELOG entry for a format bump and the deprecation rule, are marked as policy.

## Store files

The files under `.capcom/` are described in the [File format](file-format.md) reference.

- **Every store format capcom ever wrote is still read.** Each format has a fixture store under
  `internal/cli/testdata/stores/format-<n>`, written with only the keys its format allows.
  `TestFormatFixture_Reads` checks, reindexes and lists every one of them, and
  `TestFormatFixture_CurrentFormatHasFixture` fails when a new format arrives without its fixture.
- **A store's format is raised only by `capcom upgrade`**
  ([ADR 0019](../../../.capcom/knowledge/decisions/adr-0019.md)), on request, and never lowered. A
  write that needs a newer format than the store declares fails with code `invalid` and writes
  nothing; its hint names [`capcom upgrade --format <n>`](cli/capcom_upgrade.md). A write that
  needs no newer format leaves `config.yaml` as it is. Tests: `TestFormatFixture_StandingNeedsUpgrade`,
  `TestFormatFixture_ApplyNeedingNewerFormat_WritesNothing`, `TestFormatFixture_LogKeepsFormat`.
- **An older capcom refuses a newer store instead of misreading it.** A store whose format is newer
  than the binary reads fails every command with `store format <n> is newer than this capcom reads`,
  except `capcom prime`, which reports it as a warning and exits 0. Test:
  `TestFormatFixture_NewerFormatRefused`.
- **The CHANGELOG names each format bump** (policy): the new format, what it adds, and that earlier
  releases cannot read a store raised to it.

## JSON and KV output

The formats are described in [JSON and KV output](json-output.md).

- **Within `schema: 1`, fields are only added.** Every `--json` envelope carries `schema`,
  currently `1`. An existing field never changes its meaning or disappears while `schema` stays the
  same; new fields can appear at any time, so a parser should ignore members it does not know. The
  same holds for the keys of a `--kv` record. `TestGolden_ReadKinds` and `TestGolden_WriteKinds` pin
  the envelope of the main read and write kinds against golden files, and `TestGolden_KV` pins the
  `--kv` records of every command that offers `--kv` (`TestGolden_KVCovered` fails when one has no
  golden file), so any change to an existing field shows up as a failed test and a golden diff.
- **A breaking change bumps `schema`.** A change that removes a field or changes its meaning, in
  either format, comes with a new `schema` value. Check `schema` once, and treat any value other
  than the one you were written for as a reason to stop.

## Exit codes and error codes

The meanings are listed under [Errors](json-output.md#errors) and
[Exit codes](json-output.md#exit-codes).

| Exit code | Meaning |
| --- | --- |
| 0 | Success. |
| 1 | An error. |
| 2 | A usage error. |
| 3 | Not found, or locked. |
| 4 | Nothing is Ready (`capcom next` only). |

The error codes are `not_found`, `exists`, `invalid`, `conflict`, `locked` and `not_initialized`.

- **These exit codes and error codes are fixed; new ones are only added.** An existing code never
  changes its meaning or its exit code. `TestError_ExitCode_MapsCodes` pins the exit code of each
  of the six error codes, of a usage error and of nothing Ready, and
  `TestDefaultHints_EveryCode_HasOne` fails when an error code has no default hint, so a new code
  cannot arrive half-documented. `TestExitCodes` produces exit codes 0 to 4 from real commands, and
  `TestGolden_Errors` pins the error envelope in each of its documented forms; between them they
  run the `not_found`, `invalid` and `locked` codes end to end.

## What may change in any release

- **Human-readable text.** The plain-text output of every command, the wording of messages,
  warnings and hints, and help text. A script that needs a value reads it from `--json` or `--kv`,
  and tells failures apart by their `code`, never by their `message`.
- **`capcom ui`.** The terminal UI's screens, keys and layout. It takes no `--json`.

## Deprecation

Anything this page promises that has to go is first warned about for at least one minor release,
with a CHANGELOG entry that says what goes and what to use instead (policy). Only after that
release can it be removed, and a removal from `--json` or `--kv` output also bumps `schema`.
