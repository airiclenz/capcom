---
title: "Stale: a Tree lock not renewed in time, still held until resumed, taken over or broken"
---

A **Tree lock** whose holder has not renewed it within the configured time, or whose heartbeat is
dated more than that time in the future by a clock that ran fast. A stale lock is still held until its holder resumes it, someone takes it over on purpose, or it is broken.
_Avoid_: expired, dead, abandoned
