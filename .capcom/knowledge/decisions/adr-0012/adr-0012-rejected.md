---
title: "ADR 0012 rejected options: driving cli.Run in process with --json, and copying the glue into the UI"
labels: [background]
---

- **Drive `cli.Run` in process with `--json`.** Identical behaviour for free, but the UI would
  marshal and parse JSON to talk to itself, receive errors as envelopes instead of typed values,
  and pay a fresh store and index open on every write.
- **Copy the glue into `internal/ui`.** Quick, and the first rule change to a write leaves the two
  copies disagreeing.
