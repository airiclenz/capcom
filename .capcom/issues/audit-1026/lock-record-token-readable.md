---
title: The lock record is world-readable and holds the token in cleartext
status: open
type: bug
priority: 2
created: 2026-10-26T20:06:45Z
---

The lock record is world-readable and contains the token in cleartext, while the token file
beside it is carefully owner-only.

    -rw-r--r--  _main.json          -> "token": "f950957d0acefa08ad1ec0a312bb28da"
    -rw-------  _main.<hash>.token

The record is written through the generic atomic writer, which uses 0644; `writeTokenFile` uses
0600 and its doc says "owner-only". The 0600 file protects nothing while the same secret sits
next to it at 0644.

The record also carries host, user, branch and purpose, which is mild but unnecessary exposure on
a shared build host or a multi-user checkout.

ADR 0016 accepts that the token guards against accident rather than a same-user process, so this
is defence in depth rather than a broken promise — but the two files should not disagree.

## Acceptance

- The lock record is written 0600 on Unix, matching the token file.
- No user-facing output echoes the token except the command that just created it.
- A test asserts the record's mode.
