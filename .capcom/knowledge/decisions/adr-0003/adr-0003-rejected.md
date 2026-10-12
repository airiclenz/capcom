---
title: "ADR 0003 rejected options: a fixed status set and free-form statuses with no category"
labels: [background]
---

- **A fixed status set.** Simple, but it blocks review queues, triage, in-progress tracking and
  any workflow capcom's authors did not foresee.
- **Free-form statuses with no category.** The bd failure: capcom could not know what a new
  status means, so `ready` would be wrong.
