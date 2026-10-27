---
title: Search help does not say a run of CJK characters counts as one word
status: done
type: chore
priority: 3
plan: 2026-10-27 - 00 - audit-p3-fixes-plan
created: 2026-10-26T20:06:46Z
closed: 2026-10-27T08:53:52Z
reason: Resolved by plan '2026-10-27 - 00 - audit-p3-fixes-plan' (items 2), archived in 0e1c89b
---

Search matches whole words, and a run of CJK characters counts as one word, so a word inside such a
run is never found. Nothing says so.

The full-text table uses the default `unicode61` tokenizer (`internal/index/schema.go:129`), and a
query term is matched as a whole word (`internal/index/index.go:291`). Reproduced with a title
`登录超时问题 login timeout bug`:

    capcom search timeout   -> matches
    capcom search 超时      -> no issue matches
    capcom search 登录      -> no issue matches
    capcom search 登录超时  -> no issue matches

Latin substrings miss too (`search time` does not find "timeout"), and `capcom search --help`
already says words are "matched as whole words". What a reader cannot guess is that Chinese and
Japanese text, which separates words with no spaces, makes a whole sentence one word, so search in
those languages finds only a query equal to an entire run. The same holds for `know search`.

Decided on 2026-10-26: document the limit rather than change the tokenizer. A trigram tokenizer
would still miss two-character words such as 登录, so a real fix needs a substring fallback or a CJK
tokenizer, which is not worth it until someone asks.

## Acceptance

- `capcom search --help` and `capcom know search --help` say that a run of CJK characters with no
  spaces counts as one word, with a short example.
- The CLI reference is regenerated with `go run ./cmd/gendocs`.
