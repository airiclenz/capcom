---
title: Session refresh races the login timeout
status: done
type: feature
priority: 1
labels: [owner-run, invariant]
deps: [token-store-rework, clock-skew-guard]
plan: 2026-10-11 - 00 - capcom-v1-issues-plan
aliases: [apogee-kk0.4, old-session-race]
created: 2026-10-11T09:30:00Z
closed: 2026-10-12T17:05:42Z
reason: "Fixed in 3f2a1c9: refresh now waits on the timeout lock."
---

The refresh path and the timeout path both clear the session.

## Acceptance

- Two concurrent refreshes never log the user out.

## Log

- 2026-10-11 airic: reproduced with two tabs.
