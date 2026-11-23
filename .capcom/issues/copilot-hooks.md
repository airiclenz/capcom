---
title: capcom init cannot wire capcom prime into GitHub Copilot
status: done
type: feature
priority: 2
labels: [agents]
plan: 2026-11-23 - 02 - copilot-hooks-plan
created: 2026-11-23T09:14:45Z
closed: 2026-11-23T09:51:36Z
reason: "Resolved by plan 2026-11-23 - 02 - copilot-hooks-plan (archived in de43abb): prime --additional-context, init --copilot-hooks, Copilot docs"
---

`capcom init --claude-hooks`, `--codex-hooks` and `--apogee-hooks` wire `capcom prime` into Claude
Code, Codex and apogee sessions. GitHub Copilot (Copilot CLI, VS Code agent mode, the cloud agent)
has no such flag, so a Copilot session starts without the Briefing.

Copilot reads repository hooks from `.github/hooks/*.json`:
`{"version": 1, "hooks": {"sessionStart": [{"type": "command", "command": "..."}]}}`. Its
`sessionStart` hook reaches the model only through one JSON object on stdout,
`{"additionalContext": "<text>"}`; output that does not parse as JSON counts as no output. That is
why plain `capcom prime` cannot brief a Copilot session even where Copilot runs it: Copilot also
reads the repository's `.claude/settings.json`, so a repository set up with `--claude-hooks`
already runs `capcom prime` under Copilot, and the text is dropped.

Copilot's `preCompact` output is notification only and it has no post-compaction event, so only
session start can brief the model. `sessionStart` injects `additionalContext` since Copilot CLI
1.0.11 and fires once per session (not once per prompt) since 1.0.22. VS Code hooks are in public
preview. The cloud agent reads only `.github/hooks/*.json` on the default branch, and runs the hook
only when `capcom` is installed in its sandbox (`.github/workflows/copilot-setup-steps.yml`).

Sources: https://docs.github.com/en/copilot/reference/hooks-reference,
https://code.visualstudio.com/docs/agent-customization/hooks,
https://github.com/github/copilot-cli/blob/main/changelog.md (checked 2026-11-23).

## Acceptance

- `capcom prime --additional-context` prints exactly one JSON object,
  `{"additionalContext": "<the text Briefing>"}`, and still exits 0 when a part of the Briefing
  fails.
- `capcom init --copilot-hooks` merges a `sessionStart` hook running
  `capcom prime --additional-context` into `.github/hooks/capcom.json`, creating it with
  `"version": 1` when missing and keeping every other entry; a second run adds nothing; `--json`
  reports `copilot_hooks` as `added` or `present`; an unusable file fails naming its path.
- `--copilot-hooks` writes only its own file and combines freely with `--claude-hooks`,
  `--codex-hooks`, `--apogee-hooks` and `--merge-driver`; none of those touch `.github/hooks/`.
- `docs/site/agents/agent-setup.md` has a Copilot section: the minimum Copilot CLI release, that
  `.claude/settings.json` hooks also run under Copilot (a harmless extra run), and how to install
  capcom for the cloud agent. The CLI reference is regenerated.
