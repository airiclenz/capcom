---
title: "Holder process: the process a Tree lock records as its holder, whose capcom runs act as the holder"
---

The process a **Tree lock** records as its holder: the nearest ancestor of the capcom that acquired
or last resumed the lock that is not a shell, a wrapper command or a capcom shim (a process named
`capcom` that runs capcom once per call), usually the agent harness the session runs in, named by
PID and start stamp. A capcom run whose process ancestry holds it is the holder: its writes stay
silent and renew the heartbeat, and its `lock touch`, `lock release` and `lock check` need no Lock
token.
_Avoid_: holder pid, owner process, session process
