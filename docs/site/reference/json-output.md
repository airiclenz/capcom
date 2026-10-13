---
title: "JSON and KV output"
description: "The versioned --json envelope every command prints, its error codes and exit codes, and the KEY=VALUE --kv format some commands offer."
sidebar: { order: 3 }
---

Every capcom command prints plain text by default. For agents and scripts there are two
machine-readable formats: `--json`, which every command takes, and `--kv`, which some commands
take. Both are stable: a script written against them keeps working as capcom grows.

## The `--json` envelope

With `--json`, a command prints exactly one JSON object on one line, the **envelope**:

```console
$ capcom show session-refresh --json
{"schema":1,"kind":"issue","data":[{"id":"session-refresh","path":".capcom/issues/login-flow/session-refresh.md","parent":"login-flow","title":"Refresh the session token","status":"open","category":"todo","type":"task","priority":2,"labels":[],"deps":["db-schema"],"aliases":[],"created":"2026-10-13T18:54:28Z","etag":"e07aed565d368a66","body":"","related":[],"ready":true,"dependents":[]}],"elsewhere":[],"warnings":[]}
```

| Member | Always present | Value |
| --- | --- | --- |
| `schema` | yes | The envelope's version, currently `1`. |
| `kind` | yes | What `data` holds: `issue`, `issue-write`, `close`, `ready`, `prime`, `check`, `version`, ..., or `error`. |
| `data` | no | The command's result. Left out when a command fails before it has one. |
| `error` | no | Present only when the command failed; see Errors below. |
| `elsewhere` | yes | [Issue changes on other branches](../concepts/branches-and-merging.md#elsewhere), `[]` when there are none or the command does not scan. |
| `warnings` | yes | Warning messages, `[]` when there are none. |

`--json` is a global flag, so it may stand before or after the command name.

### Versioning

`schema` changes only when an existing field changes its meaning or disappears. New fields can
appear in `data` at any time without a change of `schema`, so a parser should ignore members it
does not know. Check `schema` once, and treat any value other than the one you were written for as
a reason to stop.

### Warnings

A warning never fails a command. In `--json` it goes into `warnings`, beside a normal result:

```console
$ capcom create --id audit-log2 --title "Log failed logins" --json
{"schema":1,"kind":"issue-write","data":{"etag":"58756f0fb416f617","id":"audit-log2","path":".capcom/issues/audit-log2.md"},"elsewhere":[],"warnings":["audit-log2 has no body: state what, why and a \"## Acceptance\" section with capcom update audit-log2 --body-append-file <file>"]}
```

A file capcom cannot read (a bad issue file, say) is reported as a warning by every command that
reads the store, until it is fixed; the rest of the store stays usable. How each problem with a
file is worded and located is described under
[Errors](file-format.md#errors) in the file format reference.

### Errors

A command that fails sets `error`, with a `code`, a `message` and, when a file is to blame, its
`path` and `line`:

```console
$ capcom dep add login-flow write-docs --json
{"schema":1,"kind":"error","error":{"code":"invalid","message":"login-flow cannot depend on write-docs: that would create the cycle login-flow -> write-docs -> login-flow","path":".capcom/issues/login-flow.md"},"elsewhere":[],"warnings":[]}
```

A command that fails on part of its work keeps its own `kind` and sets both `data` and `error`:

```console
$ capcom close nosuch --reason x --json
{"schema":1,"kind":"close","data":[{"id":"nosuch","outcome":"not_found","now_ready":[]}],"error":{"code":"not_found","message":"id \"nosuch\" not found"},"elsewhere":[],"warnings":[]}
```

`code` is one of a closed set:

| Code | Meaning | Exit code |
| --- | --- | --- |
| `not_found` | No Issue, Note or lock matches. | 3 |
| `locked` | Another session holds the [Tree lock](../concepts/tree-locks.md). | 3 |
| `exists` | The id or alias is already taken. | 1 |
| `invalid` | The input or a file is not valid. | 1 (2 for a usage error) |
| `conflict` | The file changed under you: its etag no longer matches `--if-match`, or it changed while the command ran. | 1 |
| `not_initialized` | There is no capcom store here; run `capcom init`. | 1 |

## Exit codes

| Exit code | Meaning |
| --- | --- |
| 0 | Success. Warnings may still have been reported. |
| 1 | An error. |
| 2 | A usage error: an unknown command or flag, or a wrong number of arguments. |
| 3 | Not found, or locked. |

`capcom check` and `capcom fmt --check` exit 1 when they find something to report, and
`capcom prime` always exits 0.

## The `--kv` format

`--kv` prints `KEY=VALUE` lines, one per field, and ends each record with a `STATUS=` line: `ok`,
an outcome such as `closed`, or an error code. It is meant for shell scripts, which can read a
value with a `sed` or a `grep` instead of a JSON parser:

```console
$ capcom create --id audit-log --title "Log failed logins" --kv
WARNING=audit-log has no body: state what, why and a "## Acceptance" section with capcom update audit-log --body-append-file <file>
ID=audit-log
PATH=.capcom/issues/audit-log.md
ETAG=58756f0fb416f617
STATUS=ok
```

A command that acts on several ids prints one record per id:

```console
$ capcom close audit-log audit-log2 --reason "not needed" --status dropped --kv
ID=audit-log
STATUS=closed
ID=audit-log2
STATUS=closed
```

A failure is a record too, with a `MESSAGE=` line and the error code as its status:

```console
$ capcom show nosuch --kv
ID=nosuch
MESSAGE=id "nosuch" not found
STATUS=not_found
```

Unlike `--json`, `--kv` is offered only by some commands, it goes after the command name, and it
leaves out the Elsewhere report. Which commands take it, and where its `WARNING=` lines go
relative to `STATUS=`, are listed in the paragraph on output in the README's
[What it does](../../../README.md#what-it-does) section.

## Next

- [CLI reference](cli/capcom.md): every command, with its flags.
- [File format](file-format.md): the files behind the data.
