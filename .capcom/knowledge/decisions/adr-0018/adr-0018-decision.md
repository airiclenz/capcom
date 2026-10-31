---
title: "ADR 0018 decision: no capcom mcp server now; the server it would be, its cost, the eval numbers and the trigger that reopens it"
---

Agents drive capcom through shell commands and flags. A Model Context Protocol server over stdio
would offer the same operations as typed tools with JSON schemas: statuses and types as enums,
required fields enforced, ids validated. For a small model that calls tools under constrained
decoding that removes flag-syntax errors; for any model it removes shell quoting and output
parsing. This record weighs that against the cost, and decides on the evidence.

**The server it would be.** `capcom mcp` speaks MCP over stdio and offers one tool per write that
`internal/ops` serves (create, update, start, close, reopen, log, dep add, dep remove), plus the
reads an agent's work loop uses: `next`, `ready`, `show`, `context`, `search`, `know show` and
`know tree`. Each tool calls the same `ops` function or read the CLI command calls, so the server is
a third front end on the one write path of ADR 0012, never a second implementation.

**Schemas that cannot drift.** Each tool's input schema is derived at run time from the cobra
command it mirrors, by walking its flags the way `cmd/gendocs` walks `cli.Command()` to write the
CLI reference: a flag's name, type, default and usage line become a property, its required mark a
required field. Enums come from the store, not from code: `status` from the statuses in
`.capcom/config.yaml`, `type` from its types. A new flag, status or type reaches the tool list
with no edit to the server.

**The Tree lock and ETags.** A tool takes the Lock token and the ETag as ordinary arguments:
`token` on the lock tools, as `--token` today, and `if_match` on every write that takes
`--if-match`. A write while another session holds the lock still succeeds, and ADR 0017's warning
travels in the tool result's warnings, in the wording `prime` uses, exactly as the envelope's
`warnings` carries it with `--json`.

**What it costs.** Either a dependency on an MCP Go SDK, with its own release cadence and
transitive modules in a binary that today depends on little, or a hand-written JSON-RPC stdio loop
that capcom then owns: framing, the initialize handshake, tool listing and error mapping. Either
way it is a third front end beside the CLI and `capcom ui`, with its own tests, its own docs and a
second way to do everything that the agent guide would have to teach.

**The numbers.** The agent eval of 2026-10-29 (Qwen3.8 27B, capcom 0.12.1;
`docs/reviews/agent-eval-2026-10-29-qwen3.8-27b-ud-q4-k-m.md`) passed 12 of 12 tasks in 74 calls;
3 calls failed and none was repeated, so every failure was recovered on the next call. The field
evaluation (`capcom-eval-report`, 2026-10-22, capcom 0.9.1) ranked its findings by the calls they
would have saved: search excerpts, the context command, all fields in `--kv`, a status filter on
the tree, suggested ids and values. None of them is a flag-syntax or quoting failure. Call count,
not flag syntax, was the cost, and `capcom apply` addresses it: one call applies a list of Issue
operations all or none.

**The decision.** Not now. capcom ships no `capcom mcp`, and no implementation Issues are filed
unless the owner accepts a revised record.

**When to revisit.** Reopen this record when an `agenteval` run of any model fails a task, or
repeats a failed call, because of flag syntax or shell quoting.
