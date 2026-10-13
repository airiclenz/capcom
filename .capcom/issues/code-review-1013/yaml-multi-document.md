---
title: A second YAML document in front matter or config.yaml is silently ignored
status: open
type: bug
priority: 2
plan: 2026-10-13 - 00 - code-and-architecture-review-fixes-plan
created: 2026-10-13T09:05:14Z
---

A second YAML document in front matter or in `config.yaml` is silently ignored. yaml.v3 stops at
a document end, so everything after it escapes the strict unknown-key, duplicate-key and syntax
checks, contradicting ADR 0009's "an unknown key is an error, never ignored". Severity: Medium
for `config.yaml`, Low for front matter.

Trigger: (a) a hand-written `config.yaml` with no format line that starts with `---` and custom
`statuses:`. A write that needs a newer format turns it, through `config.RaiseFormat`, into
`format: 2\n---\nstatuses: ...`. Every later command sees a one-key document: statuses, types,
plans, `lock.ttl` and `elsewhere.days` fall back to defaults, and Issues with custom statuses
drop out of the index. With a `%YAML` directive the result is invalid YAML and every command
fails. (b) Hand-edited front matter `title: a` / `...` / `junk: [` decodes cleanly, and the next
`fmt` or write drops the trailing lines silently.

Wanted: a second YAML document in front matter or in `config.yaml` is an error citing its line,
and `RaiseFormat` never produces one. Both document decoders use `yaml.NewDecoder` and require a
second `Decode` to return `io.EOF`; `config.RaiseFormat` inserts the format line after any
leading `%` directives and the `---` header line.

## Acceptance

- `decode_test.go`: `"---\ntitle: a\n...\nbogus: 1\n---\n"` fails with an error citing the
  `...` line.
- `config_test.go`: a config whose second document is `statuses:` fails parsing.
- `RaiseFormat` on `"---\nstatuses: {...}\n"` and on `"%YAML 1.2\n---\n..."` yields a file that
  re-parses to the same statuses with the new format.
- `GOMEMLIMIT=2GiB go test ./internal/frontmatter/`, then `./internal/config/`, pass.

Source: docs/reviews/code-and-architecture-review-2026-10-13.md, F-08.
