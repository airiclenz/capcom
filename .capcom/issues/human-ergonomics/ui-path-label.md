---
title: The UI tab bar does not say which worktree it shows
status: open
type: feature
priority: 2
plan: 2026-10-19 - 02 - ui-path-label-plan
created: 2026-10-19T19:17:00Z
---

The tab bar of `capcom ui` ends in a version label, `v0.9.1 · capcom`, but never says which
working tree the UI is showing. A human with several terminals open, each running `capcom ui` in
a different repository or worktree, cannot tell them apart.

Wanted: the label names the worktree root first, using the same ` · ` separator the label
already uses: `~/repos/capcom · v0.9.1 · capcom`. A root under the user's home directory
shows `~` instead of the home path; any other root shows its full path. The path uses the OS's
own separators. The label is built in `tabBar` in `internal/ui/layout.go`; the root comes in
through `ui.Options`, set in `internal/cli/ui.go`.

When the terminal is too narrow for the full label, the tabs still win: the path goes first,
leaving `v0.9.1 · capcom`, and only then the whole label, as it does today.

## Acceptance

- With the worktree root under the home directory, the tab bar ends in `~/<rest> · v<version> · capcom`.
- With the worktree root outside it, the full path is shown, without `~`.
- A root equal to the home directory shows `~`; a sibling like `/home/userx` when home is
  `/home/user` is not shortened.
- Too narrow for the full label, the version-only label is drawn; narrower still, none.
- UI tests cover each case above; the `Options` and `tabBar` doc comments describe the new label.
