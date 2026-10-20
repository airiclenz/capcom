---
title: Agents cannot find capcom in a plugin or skill marketplace
status: active
type: task
priority: 3
labels: [agents]
created: 2026-10-20T06:03:14Z
---

Coding agents discover tools through plugin and skill marketplaces (the Claude Code plugin
marketplace, skill directories) and MCP registries, not through web search. capcom is listed in
none, so an agent can only learn of it from a person or a README it happens to read.

## Acceptance

- capcom is published as a Claude Code plugin (or skill) whose install runs or explains
  `capcom init` and `capcom agents`, listed in at least one public marketplace.
- The README's "Works with any coding agent" section links the listing.

## Log

- 2026-10-20 Airic Lenz: Plugin and marketplace committed (plugins/capcom, .claude-plugin/marketplace.json). claude plugin validate passes; only warning is the deliberately omitted version. Installs from a local marketplace. Remaining: push, then submit at claude.ai/directory/manage.
