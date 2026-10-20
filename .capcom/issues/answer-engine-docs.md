---
title: Answer engines cannot tell capcom apart or quote its docs
status: done
type: task
priority: 2
labels: [docs]
created: 2026-10-20T06:01:34Z
closed: 2026-10-20T06:03:14Z
reason: README and manual disambiguate the name, the manual has a FAQ and comparison page, the README FAQ answers common questions, root llms.txt indexes the docs
---

Answer engines (ChatGPT, Perplexity, Google AI Overviews, Claude) and coding agents learn about
capcom from its README and manual. Today three things hold it back:

- **The name collides with Capcom, the video game company.** Nothing on the page says capcom is
  unrelated to it, so a question about "capcom" can resolve to the wrong thing.
- **No answer-shaped pages.** People ask "capcom vs beads", "an issue tracker for Claude Code" or
  "how do I stop two agents editing one repo". The README FAQ answers a few of these, but the
  manual has no FAQ and no comparison with the alternatives people weigh.
- **No `llms.txt`.** An agent has no one-fetch index of the docs.

## Acceptance

- README and `docs/site/index.md` name capcom as a CLI or an issue tracker near the top, say where
  the name comes from, and say it is not affiliated with Capcom Co., Ltd.
- `docs/site/` has a FAQ page and a comparison page (beads, GitHub Issues, Task Master, a markdown
  TODO or agent memory file), each answer opening with a sentence that stands on its own.
- The README FAQ asks its questions the way people ask them and links both pages.
- A root `llms.txt` follows the llmstxt.org format and links the manual's markdown sources.
