---
title: internal/hooks fails on the current toolchain, so CI is red
status: open
type: bug
priority: 1
created: 2026-10-26T20:06:45Z
---

`internal/hooks` fails on the installed toolchain, so the suite is red.

    --- FAIL: TestMerge_UnusableContent_FailsWithPathAndLine/syntax_error
        Merge = "...: invalid character 'o' looking for beginning of value"; want
        "...: invalid character 'o' looking for beginning of object key string"

The test asserts the exact wording of a standard-library JSON decoder error, and that wording
differs between the v1 decoder and the v2 one that `go.mod`'s `go 1.25.0` enables. It passes with
`GOEXPERIMENT=nojsonv2` and fails without, which pins the cause to the decoder, not to capcom.

CI reads its Go version from `go.mod`, so CI is likely red for the same reason.

The fix is not to chase the wording: the contract worth asserting is capcom's own, namely that a
refusal names the path, the line and "invalid settings". Asserting the decoder's prose makes the
suite depend on the toolchain.

## Acceptance

- `go test ./internal/hooks` passes on the toolchain in `go.mod` and on a newer one.
- The test asserts the path, the line and capcom's own phrase, not the decoder's exact wording.
- CI is green again.
