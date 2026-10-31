---
title: "ADR 0018 rejected options: build the MCP server now, MCP for reads only, a generated tool list without a server"
labels: [background]
---

- **Build `capcom mcp` now.** It would remove flag syntax and quoting from every agent call, but
  the evals show those cost almost nothing: 3 failed calls in 74, none repeated, no task lost. It
  buys a dependency or a hand-written protocol loop and a third front end to keep in step, against
  a failure the numbers do not show.
- **MCP for reads only.** A server offering `next`, `ready`, `show`, `context`, `search` and the
  `know` reads, with writes left to the CLI. Reads are the commands agents already call without
  trouble, so it pays the whole cost of a server for the part that needs it least, and an agent
  still has to learn the CLI for every write.
- **A generated tool list file without a server.** Commit a JSON file of tool schemas, generated
  from the cobra flags as `cmd/gendocs` generates the CLI reference, for a harness to turn into
  shell calls. Nothing in capcom would read it, so it drifts unless a generator and a check keep
  it, and the harness would still quote and parse shell commands, which is the one thing a server
  was meant to remove.
