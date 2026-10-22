---
title: Finishing an issue leaves the lock to be released by hand
status: open
type: feature
priority: 1
deps: [next-command]
plan: 2026-10-22 - 00 - start-finish-commands-plan
created: 2026-10-19T14:54:44Z
---

Finishing an Issue takes `capcom close <id> --reason <text>`, `capcom lock release --token
<token>` and then `capcom ready` to choose the next one. Agents often forget the release, which
leaves the worktree locked for the next session until the TTL runs out.

Wanted: `capcom finish <id> --reason <text> --token <token>` closes the Issue, releases the
Tree lock when the token matches, prints the `now ready:` line `close` prints, and ends with the
top Ready Issue's id and title as `capcom next` would choose it. `--keep-lock` closes without
releasing, for a session that goes straight on to the next Issue. A token that does not match
the lock closes the Issue and reports that the lock was left alone, with the command to check it.

## Acceptance

- One call closes the Issue, frees the lock and names the next Issue.
- `--keep-lock` leaves the lock held; a wrong token leaves it held and says so.
- A closed Issue repeats as `already_closed` and still releases a matching lock.
- CLI tests cover each case; the Managed block and `issue-workflow` Note mention `finish`.
