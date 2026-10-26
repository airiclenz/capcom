---
title: "JSON and KV output"
description: "The versioned --json envelope every command except capcom ui prints, its error codes and exit codes, and the KEY=VALUE --kv format some commands offer."
sidebar: { order: 3 }
---

Every capcom command prints plain text by default. For agents and scripts there are two
machine-readable formats: `--json`, which every command except `capcom ui` takes (the terminal UI
refuses it), and `--kv`, which some commands take. Both are stable: a script written against them
keeps working as capcom grows. Help (`capcom help` and any `--help`) and `capcom completion` are
not commands of the tracker: they print their text whatever `--json` says.

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
| `match` | no | Only on kinds `search` and `know-search`: `all` when every hit holds every word searched for, `any` when no item held them all and the hits hold some of them. |
| `data` | no | The command's result. Left out when a command fails before it has one. `null` when `capcom next` finds nothing Ready (kind `next`, exit code 4). |
| `error` | no | Present only when the command failed; see Errors below. |
| `elsewhere` | yes | [Issue changes on other branches](../concepts/branches-and-merging.md#elsewhere), `[]` when there are none or the command does not scan. |
| `warnings` | yes | Warning messages, `[]` when there are none. |

An Issue in `data` (`show`, `list`, `ready`, `prime`, `dep list`) carries `"standing": true` when
it is a [Standing Parent](../concepts/issues.md#standing-parent); the member is left out otherwise.
Such an Issue always has `"ready": false`, and its `standing` member is what holds it back.

With `--excerpt`, each hit of `search` and `know search` carries `excerpt`: the body lines that
hold a word searched for, as a list of strings. It holds the first 3 such lines with 1 line of
context either side, a `"…"` line between parts of the body kept apart, at most 8 lines, each cut
at 120 characters. The member is left out without `--excerpt`, and on a hit whose body holds no
word searched for, as when only its title matched.

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

A command that fails sets `error`, with a `code`, a `message`, a `hint` naming the command to run
next and, when a file is to blame, its `path` and `line`:

```console
$ capcom dep add login-flow write-docs --json
{"schema":1,"kind":"error","error":{"code":"invalid","message":"login-flow cannot depend on write-docs: that would create the cycle login-flow -> write-docs -> login-flow","path":".capcom/issues/login-flow.md","hint":"capcom dep add --help"},"elsewhere":[],"warnings":[]}
```

A command that fails on part of its work keeps its own `kind` and sets both `data` and `error`:

```console
$ capcom close nosuch --reason x --json
{"schema":1,"kind":"close","data":[{"id":"nosuch","outcome":"not_found","now_ready":[]}],"error":{"code":"not_found","message":"id \"nosuch\" not found","hint":"capcom search <text>"},"elsewhere":[],"warnings":[]}
```

A refusal of a status rule is `invalid`; closing or claiming a Standing Parent, say:

```console
$ capcom update feature-requests --status active --json
{"schema":1,"kind":"error","error":{"code":"invalid","message":"the issue is standing: it only groups its children and is never claimed or closed; run capcom update feature-requests --no-standing first","path":".capcom/issues/feature-requests.md","hint":"capcom update --help"},"elsewhere":[],"warnings":[]}
```

A mistyped command or subcommand also sets `suggestion`, the path below `capcom` of the closest
valid one; it is left out when none is close:

```console
$ capcom know shwo x --json
{"schema":1,"kind":"error","error":{"code":"invalid","message":"unknown command \"shwo\" for \"capcom know\"","suggestion":"know show","hint":"capcom know --help"},"elsewhere":[],"warnings":[]}
```

A mistyped long flag sets it to the closest flag of that command, its own or one it inherits such
as `--json`; an unknown shorthand such as `-z` gets none:

```console
$ capcom update x --statsu active --json
{"schema":1,"kind":"error","error":{"code":"invalid","message":"unknown flag: --statsu","suggestion":"--status","hint":"capcom update --help"},"elsewhere":[],"warnings":[]}
```

An unknown id or a value a flag does not accept can set two lists, each left out when empty:
`suggestions`, the closest known ids or aliases (at most three, closest first), and `valid`, the
values the flag accepts (a status as `name (category)`). Text output ends the message with
`; did you mean a, b or c?` and `; use one of: x, y, z`. An unknown id is matched against the
ids and aliases of its own kind, Issues or Notes: a name within two edits of it, or sharing a
hyphen-separated word of three or more letters with it, is close; when none is, `suggestions` is
left out. `list`, `create`, `update`, `close` and `reopen` set `valid` for an undeclared
`--status` or `--type`, an unknown `--category` (`list`) and a `--priority` outside 0 to 4, and
refuse it before anything is written:

```console
$ capcom update login-flow --type bugg --json
{"schema":1,"kind":"error","error":{"code":"invalid","message":"--type \"bugg\": not a type declared in config.yaml","valid":["bug","feature","task","chore"],"hint":"capcom update --help"},"elsewhere":[],"warnings":[]}
```

`code` is one of a closed set. Each code has a default `hint`, which a failure with a more specific
next step replaces. A hint names the values capcom knows, such as the command that failed in place
of `<command>`; any other `<placeholder>`, such as `<text>`, is yours to fill in. Text output prints
the hint as a last line, `hint: <command>`, after the `capcom:` line.

| Code | Meaning | Default hint | Exit code |
| --- | --- | --- | --- |
| `not_found` | No Issue, Note or lock matches. | `capcom search <text>` | 3 |
| `locked` | Another session holds the [Tree lock](../concepts/tree-locks.md). | `capcom lock status` | 3 |
| `exists` | The id or alias is already taken. | `capcom show <id>` | 1 |
| `invalid` | The input or a file is not valid. | `capcom <command> --help` | 1 (2 for a usage error) |
| `conflict` | The file changed under you: its etag no longer matches `--if-match`, or it changed while the command ran. | `capcom show <id>` | 1 |
| `not_initialized` | There is no capcom store here; run `capcom init`. | `capcom init` | 1 |

## Exit codes

| Exit code | Meaning |
| --- | --- |
| 0 | Success. Warnings may still have been reported. |
| 1 | An error. |
| 2 | A usage error: an unknown command or flag, a flag that holds one value given more than once, or a wrong number of arguments. |
| 3 | Not found, or locked. |
| 4 | Nothing is Ready: `capcom next` found no Issue to print. Used for nothing else. |

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

`capcom show <id> --kv` prints one record per issue, every key present in this order, with an
empty value when the file leaves the field out (except `PRIORITY=`, which then reads as the default
2) and list values comma-separated:

```console
$ capcom show audit-log --kv
ID=audit-log
ETAG=58756f0fb416f617
ISSUE_STATUS=open
CATEGORY=todo
TITLE=Log failed logins
TYPE=bug
PRIORITY=1
LABELS=auth,security
PARENT=
DEPS=session-store
PLAN=
STATUS=ok
```

`ISSUE_STATUS=` is the issue's own status, `STATUS=` the outcome of looking it up, and `ETAG=` the
value `--if-match` takes. `capcom next --kv` prints the same record; `capcom start --kv` adds
`LOCK=` and `TOKEN=` just before `STATUS=`.

A failure is a record too, with a `MESSAGE=` line, its hint as a `HINT=` line and the error code as
its status:

```console
$ capcom show nosuch --kv
ID=nosuch
MESSAGE=id "nosuch" not found
HINT=capcom search <text>
STATUS=not_found
```

A failure that carries a `suggestion` in `--json` adds it as a `SUGGESTION=` line before its
`STATUS=` line:

```console
$ capcom shwo nosuch --kv
MESSAGE=unknown command "shwo" for "capcom"
SUGGESTION=show
HINT=capcom --help
STATUS=invalid
```

Its `suggestions` and `valid` lists follow the `SUGGESTION=` line as `SUGGESTIONS=` and `VALID=`
lines, in that order, each comma-separated, then the `HINT=` line, all before `STATUS=`; a failure
record of a multi-id command such as `show` carries them too.

Unlike `--json`, `--kv` is offered only by some commands, it goes after the command name, and it
leaves out the Elsewhere report. Which commands take it is on each command's page in
[the CLI reference](cli/capcom.md). Its `WARNING=` lines come first, before every record and so
before every `STATUS=` line; a failure prints none.

## Next

- [CLI reference](cli/capcom.md): every command, with its flags.
- [File format](file-format.md): the files behind the data.
