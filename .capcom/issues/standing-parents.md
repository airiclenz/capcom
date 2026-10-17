---
title: A standing Parent that only groups children is offered as ready work
status: open
type: feature
priority: 2
created: 2026-10-17T18:55:29Z
---

`capcom ready` and `capcom prime` count a Parent whose children are all closed as Ready. That is
right for an ordinary Parent: once its children are done, what is left is to verify it and close
it. It is wrong for a standing Parent, an Issue that only groups children and never closes, such
as "feature requests" or "ops problems". With no open child it is listed as Ready work, and an
agent following "take one ready issue" claims it. capcom has no way to say that an Issue is such
a Parent.

## Why it matters

An agent trusts what `ready` tells it over a rule written in its guide. When the two disagree,
the agent claims the grouping Issue, plans its next session around it, or spends the session
orienting because the only Ready work on offer is not work. The fix belongs in the tool, so the
tool's answer and the written rule agree.

## Evidence

seqcom, an autonomous agent on a Raspberry Pi that uses capcom 0.8.0 as its only memory, keeps
five such Parents: `apogee-feature-requests`, `capcom-feature-requests`, `seqcom-ops`,
`seqcom-goals` and `dogfood-apogee-capcom`. Its AGENTS.md says "never claim or close these".
Even so, the agent:

- claimed `dogfood-apogee-capcom` in one run;
- named it as the next run's Issue in two later run summaries;
- spent an hour orienting in one run because `capcom ready` offered only these Parents.

The workaround today is a `container` label on those Parents, listed under `ready.hide` in
`.capcom/config.yaml`. It keeps them out of `ready`, but every repository has to invent it, and
nothing stops an agent from closing or claiming one.

## Proposal

To refine in a plan, not prescribed here:

- A way to mark an Issue as standing: a frontmatter field, a type, or a config list.
- A standing Issue is never listed by `ready` or `prime`. `ready --all` shows it with a reason
  such as `standing: holds children`.
- `close`, and `update --status` to a doing status, on a standing Issue are refused, or need a
  force flag.
- `check` reports a standing Issue that is closed or in a doing status.
- `create --parent <standing issue>` keeps working; its children are Ready by the usual rules.

## Acceptance

- An Issue can be marked standing, and the mark can be removed, through a capcom command; the
  mark survives a round trip through the file and `capcom check` accepts it. Tested.
- A standing Issue with no open children is absent from `ready` and from the Ready lists in
  `prime`; `ready --all` lists it with a standing reason. Its open children are listed as usual.
  Tested.
- `close` on a standing Issue, and `update --status` to a doing status, are refused with a
  message that names the standing mark and how to override or remove it. Tested, including the
  override if the plan keeps one.
- `check` reports a standing Issue that is closed or in a doing status. Tested.
- `create --parent` below a standing Issue works unchanged. Tested.
- The glossary gains a standing Parent term and the Ready term names the exclusion; the manual
  and the `--help` text of every changed command describe the mark; the managed block tells
  agents that standing Issues are never claimed.
- A CHANGELOG line under Unreleased.
