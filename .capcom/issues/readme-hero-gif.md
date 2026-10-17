---
title: Hero GIF of capcom ui in the README
status: done
type: task
priority: 3
created: 2026-10-17T08:51:03Z
closed: 2026-10-17T13:21:36Z
reason: graphics/demo rig filmed with photon; graphics/demo.gif shown under the README badges
---

The README describes the terminal UI in words only. A short looping clip of `capcom ui` at the
top shows what capcom is faster than any paragraph.

It is filmed with photon (github.com/airiclenz/photon), the demo recorder extracted from
apogee's demorig, so it can be re-recorded with one command whenever the UI changes. This
waits for photon v1 (its issue `photon-v1`).

Wanted: a `graphics/demo/` holding `photon.yaml` (launches `capcom ui` on a staged repo with a
few issues and Notes, no services), a `reset` script that restores the stage, and a
`storyboards/hero.yaml`; the GIF shipped at `graphics/demo.gif` and shown at the top of
`README.md`.

## Acceptance

- `photon record graphics/demo/storyboards/hero.yaml` passes its check, and
  `photon render` writes `graphics/demo.gif`.
- `README.md` shows the GIF under the badges.
