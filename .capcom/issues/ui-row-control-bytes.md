---
title: Control bytes in titles and lock records reach UI rows unsanitised
status: done
type: bug
priority: 3
plan: 2026-11-03 - 00 - audit-urgent-fixes-plan
created: 2026-11-03T08:23:42Z
closed: 2026-11-03T09:20:04Z
reason: "Resolved by plan 2026-11-03 - 00 - audit-urgent-fixes-plan item 3 (archived in 1dcb36a): oneLine and status-bar notices escape control bytes"
---

`oneLine` only joins `strings.Fields`, keeping ESC, C0 and C1 bytes. Issue titles, Note titles
and lock Holder and Purpose go through it and through `fit`/`ansi.Truncate`
(`internal/ui/layout.go:635`), which keeps escape sequences. Drawn at `internal/ui/closed.go:185`,
`knowledge.go:186`, `ready.go:213`, `detail.go:176`, `detail.go:294`, `statusbar.go:97-106`,
`write.go:212` and `:215`.

Unconfirmed: whether bubbletea v2 forwards or drops raw non-SGR sequences. First write a
throwaway test rendering a model whose title contains `"\x1b]0;x\x07"` and inspect
`m.View().Content`. If the sequence survives, make `oneLine` (or a shared display sanitiser)
drop control characters (`ansi.Strip` plus runes where `unicode.IsControl` holds) and apply it to
every repo-sourced or lock-record string drawn.

Do this after or with `ui-link-marker-panic`, which strips control bytes from bodies.

Source: docs/reviews/code-audit-2026-11-03.md.

## Why

A hostile repository or a prompt-injected agent in a parallel worktree could plant an OSC or CSI
sequence: title spoofing, an OSC 52 clipboard write, or forged status text or a y/n prompt.

## Acceptance

- The probe's result is logged on the issue.
- If exploitable, a test drawing every listed view with a title and a lock purpose holding OSC and CSI sequences finds no control bytes in the output; otherwise the issue is closed with the probe as its reason.

## Log

- 2026-11-03 Airic Lenz: Probe outcome: not run as a separate probe; plan 2026-11-03 - 00 - audit-urgent-fixes-plan item 3 escapes control bytes in oneLine (termsafe.Escape) unconditionally, covering every listed draw site, and statusBar escapes notices. ready_test.go asserts m.View().Content holds no raw OSC/CSI/BEL for a spoofed title and lock Holder/Purpose.
