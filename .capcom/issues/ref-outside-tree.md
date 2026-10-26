---
title: A Note ref to a path a clone lacks fails check only there
status: done
type: bug
priority: 2
created: 2026-10-26T07:51:30Z
closed: 2026-10-26T09:47:07Z
reason: "Resolved by field-eval-ergonomics plan (archived 39bac45), items 9-11: check and know verify report unshared-ref; know add/update refuse git-dir refs and warn on missing or ignored ones"
---

A Note's ref can name a path that will not exist in another clone, and capcom does not notice until
`capcom check` fails there. `capcom know add --ref <path>` accepts any path, even a missing one, and
`capcom check` passes as long as the path exists in this worktree. A ref into the git directory
(for example `.git/capcom/locks/_main.lck`), or to a file git ignores, passes in the worktree
that wrote it and fails `capcom check` in every fresh clone.

Why: in the field evaluation (capcom 0.9.1) the agent wrote a Note about the Tree lock and gave it a
ref to the lock file under `.git`. `capcom check` passed in its worktree. In a fresh clone it would
have failed, and the agent's runs stop on a failing `capcom check`, so a person had to remove the
ref by hand.

Wanted:
- `know add` and `know update` refuse a ref that resolves inside the git directory, and warn
  about a ref to a path that does not exist or that git ignores.
- `capcom check` reports a ref inside the git directory, or to an ignored path, as a problem even
  when the file exists here, naming the Note and the ref.

## Acceptance

- `capcom know add --id x --title X --ref .git/HEAD` fails with an error that names the ref.
- `--ref` naming a missing or ignored path succeeds with a warning that names the path.
- `capcom check` fails on an existing Note whose ref points into the git directory or to an ignored
  file, in the worktree where the file exists.
- Tests cover add, update and check.
