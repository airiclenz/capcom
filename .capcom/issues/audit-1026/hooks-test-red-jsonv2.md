---
title: internal/hooks asserts the JSON decoder's wording and fails under encoding/json v2
status: open
type: bug
priority: 2
plan: 2026-10-26 - 03 - audit-p1-p2-fixes-plan
created: 2026-10-26T20:06:45Z
---

`internal/hooks` fails wherever the v2 JSON decoder is active, because a test asserts the exact
wording of a standard-library decoder error.

    --- FAIL: TestMerge_UnusableContent_FailsWithPathAndLine/syntax_error
        Merge = "...: invalid character 'o' looking for beginning of value"; want
        "...: invalid character 'o' looking for beginning of object key string"

The assertion is at `internal/hooks/settings_test.go:138`. The audit hit it on macOS; it passes
there with `GOEXPERIMENT=nojsonv2` and fails without. On go1.26.3 with an empty `GOEXPERIMENT` it
passes, and it fails there with `GOEXPERIMENT=jsonv2`. So the decoder the toolchain picks decides
the result, not capcom and not the `go 1.25.0` line in `go.mod`.

CI is green today: it installs Go from `go.mod` (`go-version-file`), which gives 1.25.x and the v1
decoder. It turns red the day `go.mod` moves to a toolchain whose default is v2, and any developer
on such a toolchain sees a red suite now.

The contract worth asserting is capcom's own: a refusal names the path, the line and "invalid
settings". Asserting the decoder's prose makes the suite depend on the toolchain.

## Acceptance

- `go test ./internal/hooks` passes with `GOEXPERIMENT=jsonv2` and with `GOEXPERIMENT=nojsonv2`.
- The test asserts the path, the line and capcom's own phrase, not the decoder's wording.
