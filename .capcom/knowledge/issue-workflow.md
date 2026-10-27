---
title: "Before filing, starting or closing an issue: its body, claiming it, dependencies"
---

**The body.** An issue's body is self-contained: what is wrong or wanted, why, and how to tell it
is done under `## Acceptance`, readable without its plan. The plan says how and in what order, so
archiving a plan strands no issue. Never file a stub: `create` without a body warns.

**Starting work.** Start a session with `capcom next`: it prints the most urgent Ready issue in
full. `capcom ready` lists the rest.

**Claiming work.** `capcom start <id>` moves the issue to the first doing status and takes this
worktree's Tree lock with the issue id as its purpose, then prints the issue and the Lock token.
capcom keeps the token for the session in a token file, so running `capcom start <id>` again
resumes the lock after a restart or compaction and prints the token again.
`prime` lists the doing issues, so the next session sees who works on what. There is no assignee
key.

**Finishing work.** `capcom finish <id> --reason <text>` closes the issue, frees the Tree lock
through the token kept for the issue, deletes that kept token and names the Ready issue to work on
next; `--keep-lock` keeps the lock for more work under it, prints the token, and its next line
names `capcom start <next> --token <token>`, the start that resumes the lock for the next issue.

**Dependencies.**
- Record them with `capcom create --dep <id>` or `capcom dep add <id> <dep-id>`.
- Read them with `capcom dep list <id> --depth N` (deps and dependents, 0 prints every level),
  `capcom dep graph [id]` (a Mermaid flowchart, edges point dependency to dependent) and
  `capcom list --tree --depth N` (the parent and child outline).
- `capcom close <id>` ends its text with `now ready: a, b`, the issues this close made Ready.
- `capcom ready` ranks by priority, and a priority tie goes to the issue that unblocks more,
  shown as `-- unblocks N`.
