---
title: "Knowledge"
description: "How capcom keeps project Knowledge as a Tree of small Notes: Labels, Refs and suspect Notes, Retired Notes and Supersession, Background Notes, know tree and know verify."
sidebar: { order: 4 }
---

**Knowledge** is the committed, shared body of what is true about a repository that agents consult
on demand: conventions, gotchas, the domain glossary, decision records and the reasons behind
them. capcom keeps it as a [Tree](issues.md#the-tree) of small markdown files under
`.capcom/knowledge/`, versioned with the code like the Issues are.

The point of the Tree is that an agent reads only what it needs. Instead of loading one long
document into its context, it browses the titles, opens the branch that matches its task, and
reads the few Notes that apply.

## Note

A **Note** is one self-contained piece of Knowledge: a rule, a gotcha, a glossary term, or a
decision record or one section of it. A long document becomes a Note with its sections as
children, so a reader opens only the part it needs.

A Note is a file like an Issue: a short YAML header, then a free markdown body. Its **Title** is
the gist, written to be read on its own, so that browsing titles is already useful:

```markdown
---
title: Token refresh runs on a timer that logout must stop
labels: [gotcha]
refs: [internal/auth/refresh.go#func RenewToken]
---

The refresh timer keeps running after logout unless the logout handler stops it.
```

Add Notes with `capcom know add`, at the top of the Tree or below a Parent:

```console
$ capcom know add --id auth --title "How sessions and tokens work"
id: auth
path: .capcom/knowledge/auth.md
etag: bba1bca658a5fe48
$ capcom know add --id token-refresh --parent auth --title "Token refresh runs on a timer that logout must stop" --ref "internal/auth/refresh.go#func RefreshToken" --label gotcha
id: token-refresh
path: .capcom/knowledge/auth/token-refresh.md
etag: 78cca05e38ddee8c
```

Notes are meant to be edited by hand as well as through capcom. After a hand edit, run
`capcom check` to validate the files and `capcom fmt` to put them back in canonical form. The
scripted path is `capcom know update`, `know mv` and `know rm`, which take `--if-match <etag>` so
an agent never overwrites a Note that changed since it last read it. Every key of a Note file is
listed under [Note files](../reference/file-format.md#note-files).

## Browsing: `know tree`, `know show`, `know search`

`capcom know tree` prints the outline of Note ids and titles. On a large Tree, start shallow and
open one branch at a time:

```sh
capcom know tree --depth 2      # the top level and its direct children
capcom know tree auth           # one branch, from the Note auth down
capcom know show token-refresh  # one Note in full
capcom know search refresh      # Notes whose title or body contains every word
```

```console
$ capcom know tree
auth  How sessions and tokens work
  token-refresh  Token refresh runs on a timer that logout must stop
```

`capcom know show --with-children <id>` prints a Note together with everything below it, for a
decision record whose sections are child Notes.

## Label

A **Label** is an optional cross-cutting marker on an Issue or a Note, for groupings the Tree and
the Status cannot express (`gotcha`, `owner-run`, `invariant`). Labels are free-form and change
nothing capcom does, with one exception: `background`.

## Background Note

A **Background** Note is one labelled `background`. It still holds but is rarely the answer, like
the options a decision record rejected. `capcom know tree` leaves it out, together with every Note
below it, unless `--background` is given:

```console
$ capcom know add --id auth-history --parent auth --title "Why sessions moved from cookies to tokens" --label background
id: auth-history
path: .capcom/knowledge/auth/auth-history.md
etag: a412aa82e4bacdcc
$ capcom know tree
auth  How sessions and tokens work
  token-refresh  Token refresh runs on a timer that logout must stop
$ capcom know tree --background
auth  How sessions and tokens work
  auth-history  Why sessions moved from cookies to tokens
  token-refresh  Token refresh runs on a timer that logout must stop
```

`capcom know show` and `capcom know search` find a Background Note like any other, and a
`background` top-level Note is left out of the [Managed block](../agents/managed-block.md) in
`AGENTS.md`.

## Ref and suspect Notes

A **Ref** is a file, directory or symbol in the repository that a Note depends on. It is a path
relative to the working tree root (`internal/auth`), optionally followed by `#` and a symbol: text
the file must contain (`internal/auth/refresh.go#func RenewToken`). The symbol is found by a plain
text search, so any text works, not only code identifiers.

Refs make a Note checkable. When the code it describes is renamed or deleted, the Ref stops
resolving and the Note becomes **suspect**: it is still readable, but someone should check whether
it still holds. `capcom know verify` lists every Ref that no longer resolves and fails when it
finds one:

```console
$ capcom know verify
.capcom/knowledge/auth/token-refresh.md:4: ref "internal/auth/refresh.go#func RefreshToken": symbol "func RefreshToken" is not in the file
capcom: 1 note refs do not resolve
```

`capcom check` reports the same problems, and `capcom prime` lists them in the session
[briefing](../agents/briefing.md) under `broken note refs:`, so a stale Note surfaces the next time
an agent starts work. Fix it by updating the Note, its Refs or both:

```sh
capcom know update token-refresh \
  --remove-ref "internal/auth/refresh.go#func RefreshToken" \
  --add-ref "internal/auth/refresh.go#func RenewToken"
```

The exact rules, including what makes a Ref invalid, are under
[Refs](../reference/file-format.md#refs).

## Retired Notes and Supersession

A **Retired** Note is one whose content no longer holds, kept so that old citations still resolve.
**Supersession** is the link from a Note to the Notes that replace it, in its `superseded-by`
list. A Note replaced in whole is retired and names what replaces it; a Note replaced in part
stays current, names the Notes that take over, and says in its body what still holds.

```console
$ capcom know add --id token-renewal --parent auth --title "Token renewal is cancelled by logout through a context"
id: token-renewal
path: .capcom/knowledge/auth/token-renewal.md
etag: 4c6b34c9ab9815cb
$ capcom know update token-refresh --retire --add-superseded-by token-renewal
id: token-refresh
path: .capcom/knowledge/auth/token-refresh.md
etag: dc9d22ef3366c025
$ capcom know show token-refresh
id: token-refresh
path: .capcom/knowledge/auth/token-refresh.md
etag: dc9d22ef3366c025
parent: auth
title: Token refresh runs on a timer that logout must stop
labels: [gotcha]
refs: [internal/auth/refresh.go#func RenewToken]
retired: true
superseded-by: [token-renewal]
```

Browsing and search leave Retired Notes out, together with every Note below them, unless asked
with `--retired`:

```console
$ capcom know tree
auth  How sessions and tokens work
  token-renewal  Token renewal is cancelled by logout through a context
$ capcom know tree --retired
auth  How sessions and tokens work
  token-refresh  Token refresh runs on a timer that logout must stop
  token-renewal  Token renewal is cancelled by logout through a context
```

`capcom know show <id>` always prints the Note it names, retired or not, so a citation of the old
id leads straight to its replacement. `know verify`, `check` and `prime` skip the Refs of Retired
Notes. Retiring a Note in an older store raises its format version; see
[Supersession](../reference/file-format.md#supersession).

## Next

- [Tree locks](tree-locks.md): telling other agent sessions you are editing this working tree.
- [The Managed block](../agents/managed-block.md): how the top level of the Knowledge Tree reaches
  `AGENTS.md`.
- [File format](../reference/file-format.md#note-files): every key of a Note file.
