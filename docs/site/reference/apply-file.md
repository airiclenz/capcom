---
title: "Apply file"
description: "The YAML file capcom apply reads: a list of Issue writes (create, update, dep add, close and log), run in order and written all or none, with its keys, values, if-match rule, dry run and errors."
sidebar: { order: 4 }
---

An **apply file** is a YAML list of Issue writes that [`capcom apply`](cli/capcom_apply.md) runs
in order and writes all or none. It saves an agent or a script one command per write, and with
that the shell quoting of every title and body: a plan that files three Issues and closes a fourth
is one file and one call. Give the file's path, or `-` to read it from stdin:

```console
$ capcom apply batch.yaml
$ capcom apply - < batch.yaml
```

## Operations

The file is one YAML list. Each entry is a mapping whose `op` key names the write, and whose other
keys are the flag names of that write's command, without the leading `--`, plus `id`, `dep` and
`text` for the command's arguments. There are five operations and no others:

| `op` | Does what | Required keys | Other keys |
| --- | --- | --- | --- |
| `create` | [`capcom create`](cli/capcom_create.md) | `id`, `title` | `body`, `why`, `acceptance`, `dep`, `label`, `parent`, `plan`, `priority`, `standing`, `status`, `type` |
| `update` | [`capcom update`](cli/capcom_update.md) | `id` and one key that changes the Issue | `if-match`, `title`, `status`, `type`, `priority`, `plan`, `standing`, `no-standing`, `add-label`, `remove-label`, `body-append`, `body-replace-old`, `body-replace-new` |
| `dep add` | [`capcom dep add`](cli/capcom_dep_add.md) | `id` (the dependent), `dep` (its new dependency) | `if-match` |
| `close` | [`capcom close`](cli/capcom_close.md) | `id`, `reason` | `status`, `if-match` |
| `log` | [`capcom log`](cli/capcom_log.md) | `id`, `text` | `if-match` |

`id` is the new Issue's id on a `create`, and on every other operation the Issue it writes, by id
or Alias as on the command line. Each key means what its flag means, and a value is checked as the
command checks it, with the same message: an undeclared `status` or `type`, a `priority` outside 0
to 4, an unknown `dep` or `parent`, a `dep add` that would make a cycle. A few rules follow from
the commands:

- An `update` must give at least one key that changes the Issue, as `capcom update` needs at least
  one such flag. `body-replace-old` and `body-replace-new` come in pairs, matched in order.
- `standing` and `no-standing` cannot be combined.
- A `close` without `status` moves the Issue to the first status of the `done` category; a `status`
  it gives must be in the `done` or `dropped` category. Closing an Issue that is already Closed
  fails the batch.
- A key given twice in one entry, or a key the operation does not take, is refused.

An operation sees the writes before it. A `create` can name an Issue an earlier `create` of the
same file made as its `parent` or `dep`, and a later `update`, `dep add`, `close` or `log` can write
it.

## Values

Each key takes one YAML shape, and a value of another shape is refused:

- **Text**: `id`, `title`, `body`, `why`, `parent`, `plan`, `status`, `type`, `reason`, `text`,
  `if-match`, and `dep` on a `dep add`. Any scalar is taken as its text, so `title: 2026` is the
  title `"2026"`. Use a YAML block scalar (`|`) for text that spans lines.
- **Integer**: `priority`.
- **true or false**: `standing` and `no-standing`.
- **A string or a list of strings, split on commas**: `dep` and `label` on a `create`, `add-label`
  and `remove-label`. `label: auth, db` and `label: [auth, db]` give the same two labels, as
  `--label auth,db` and `--label auth --label db` do.
- **A string or a list of strings, each kept whole**: `acceptance`, `body-append`,
  `body-replace-old` and `body-replace-new`. Each string is one value of the repeatable flag, commas
  and all.

Every value is inline. `body-file` and `body-append-file`, which read their text from a file, are
refused: put the text in the apply file itself.

## All or none

capcom parses the whole file first, and refuses it, writing nothing, when anything in it is
malformed. It then runs the operations in order against the store staged in memory: each write
goes through the same checks its command makes, and the next operation sees its result. Only once
every operation has passed does capcom write the staged files, together. The first operation that
fails stops the batch, and nothing is written: not its own write, and not the writes before it.

A file that changed on disk while the batch ran fails the batch the same way, with a `conflict`,
and a file that appeared where the batch creates one fails it with `exists`.

The warnings of the writes come once each, as their commands give them: an Issue created with no
body, say. A [Tree lock](../concepts/tree-locks.md) held for another purpose warns once for the
whole batch, and only after the batch is written.

## if-match

`update`, `dep add`, `close` and `log` take `if-match`, the etag `capcom show` prints. The batch
compares it with the etag of the Issue's file as it was on disk before the batch, not after the
operations before it, so a file that two operations write takes the same `if-match` in both. A
mismatch fails the batch with a `conflict`. On the command line only `capcom update` takes
`--if-match`; in an apply file `dep add`, `close` and `log` take it too.

A `create` takes no `if-match`. An `if-match` on an Issue that an earlier operation of the same
file created is refused: that Issue has no file before the batch, so it has no etag to match.

## Dry run

`--dry-run` runs the same batch, with the same checks, and reports the same outcome, but writes
nothing. Its text opens with the line `dry run: nothing written`, and its `--json` data holds
`"dry_run": true`. A dry run that passes is a batch that would land, unless the store changes
before the real run.

## Output

capcom prints one line per operation: its position in the file, its `op`, the Issue it wrote and
its outcome (`created`, `updated`, `dep_added`, `closed` or `logged`). A last line `now ready:`
names the Issues that existed before the batch, were not Ready then and are Ready after it; it is
left out when there are none. An Issue the batch created is never named there.

With `--json` the envelope's kind is `apply`; its data is described under
[JSON and KV output](json-output.md#apply). With `--kv` each operation is a record of `INDEX=`,
`LINE=`, `OP=`, `ID=` and `ETAG=`, ending in its outcome as `STATUS=`.

## Errors

A failure names the apply file (`stdin` for `-`) and the line of the problem, and a problem in an
operation starts with `operation <n> (<op>):`, its position in the file and its `op`:

```console
$ capcom apply batch.yaml
capcom: batch.yaml:24: operation 5 (close): .capcom/issues/db-schema.md: etag 0123456789abcdef does not match the file's etag 4db79d4cddcd67a1; show the issue again
hint: capcom show <id>
```

The line is the one the problem is on: the key with a value of the wrong shape, an `op` capcom
does not know, or the first line of an operation that fails when it runs. A failure while running
carries the code its command would report (`invalid`, `not_found`, `exists`, `conflict`), the file
it was about, and the command's own hint; a file that does not parse is `invalid`, hinting
`capcom apply --help`. In `--json` the file and line are the error's `path` and `line`:

```console
$ capcom apply - --json < bad.yaml
{"schema":1,"kind":"error","error":{"code":"invalid","message":"operation 1 (update): body-file is refused: apply files take text inline","path":"stdin","line":3,"hint":"capcom apply --help"},"elsewhere":[],"warnings":[]}
```

## Example

This file files three Issues under `login-flow`, the second and third depending on the first,
makes `login-flow` wait for the second, and closes `db-schema`, which `audit-log` was waiting for:

```yaml
- op: create
  id: session-store
  title: Store sessions in the database
  parent: login-flow
  body: Keep each session in the sessions table.
  acceptance: A session survives a restart
  label: auth, db
- op: create
  id: session-refresh
  title: Refresh the session token
  parent: login-flow
  priority: 1
  body: Refresh the token before it expires.
  dep: session-store
- op: create
  id: session-expiry
  title: Expire idle sessions
  parent: login-flow
  body: End a session after 30 idle minutes.
  dep: [session-store]
- op: dep add
  id: login-flow
  dep: session-refresh
- op: close
  id: db-schema
  reason: designed in the plan
  if-match: f2cbae1354f0f58e
```

```console
$ capcom apply batch.yaml --dry-run
dry run: nothing written
1  create  session-store  created
2  create  session-refresh  created
3  create  session-expiry  created
4  dep add  login-flow  dep_added
5  close  db-schema  closed
now ready: audit-log
$ capcom apply batch.yaml
1  create  session-store  created
2  create  session-refresh  created
3  create  session-expiry  created
4  dep add  login-flow  dep_added
5  close  db-schema  closed
now ready: audit-log
```

## Next

- [capcom apply](cli/capcom_apply.md): the command and its flags.
- [JSON and KV output](json-output.md): the envelope, its error codes and exit codes.
- [File format](file-format.md): the Issue files the batch writes.
