---
title: Search cannot find a substring of a run of CJK characters
status: open
type: bug
priority: 3
created: 2026-10-26T20:06:46Z
---

Search cannot find a substring inside a run of CJK characters, and nothing says so.

The full-text table uses the default tokenizer, which treats a run of Han characters as one
token. Reproduced with a title `登录超时问题 login timeout bug`:

    capcom search timeout   -> matches
    capcom search 超时      -> no issue matches
    capcom search 登录      -> no issue matches
    capcom search 登录超时  -> no issue matches

The same holds for Notes in a mixed title. For anyone writing titles or bodies in Chinese,
Japanese or Korean, search is effectively broken for everything except an exact whole token,
while every other language behaves as expected.

## Acceptance

- A substring of a CJK run finds the issue, or the limitation is documented where search is
  documented and in `capcom search --help`.
- A test searches a CJK substring of a title and asserts a hit.
