---
title: Parse the tokens into a tree
status: open
type: task
priority: 2
labels: [parser]
deps: [tokenizer]
created: 2026-01-10T09:10:00Z
---

The grammar builds the config tree from the tokenizer's output.

## Acceptance

- A missing colon is reported at its line.
