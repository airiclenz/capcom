# The worktree path in the UI tab bar

**Goal:** the `capcom ui` tab bar names the worktree it shows, home shortened to `~`, ahead of the
version label.
**Date:** 2026-10-19
**Status:** unexecuted
**sized for:** ~200k-context host
**base:** 1cb592c
**Closes:** ui-path-label

**Regression check (2026-10-19, 1cb592c):**
- 1: guard folded (label path test seam, regenerated goldens, rewritten version-label test, full package run)

**Sources:**
- `.capcom/issues/human-ergonomics/ui-path-label.md`
- Note `coding-conventions`; `docs/site/getting-started/terminal-ui.md`

**Ratified design calls** (the user, 2026-10-19):
- **UI path:** the worktree root (`store.WorktreeRoot`), not the directory `capcom ui` ran in.

**Standing requirements:**
- skills: coding-standards
- Tests on this host: `GOMEMLIMIT=2GiB`, one package at a time, never `./...` with `-race`.

**Out of scope:**
- Any other change to the tab bar or status bar.
- CLI suggestions and `capcom next` (plan `2026-10-19 - 01 - suggest-next-plan`).

## 1. The UI tab bar names the worktree — ✅ DONE (2026-10-19)

NOTES (2026-10-19): the test seam is `Options.Home` (the home directory the path is shortened by; empty, os.UserHomeDir with symlinks resolved); newTestOptions pins it to the test store's worktree root, so every golden's tab bar reads `~ · v1.2.3 · capcom`.
NOTES (2026-10-19): TestModel_TabBar_EndsInTheVersionLabelWhenItFits is rewritten as TestModel_TabBar_EndsInTheWidestLabelThatFits (path label, version-only and none, each at its least-gap width and one column short); the terminal-ui.md screen sample is widened by 17 columns so its tab bar carries the full label.

**What:**
**Goal:** the tab bar ends in `<path> · v<version> · capcom`, where `<path>` is the worktree
root with the home directory shown as `~` (`~/repos/capcom`, or `~` for home itself), the full
path otherwise, and never shortened for a sibling such as `/home/userx` under home
`/home/user`; too narrow for the full label it draws `v<version> · capcom`, narrower still none.
**Approach (assumed at the header base):**
- A pure function in `internal/ui/layout.go`, `displayPath(root, home string) string`: `~` when
  equal, `~` + separator + rest when `root` lies under `home` (path-component match, OS
  separators kept), `root` otherwise; empty `home` never shortens.
- `newModel` in `internal/ui/model.go` computes the label path once from
  `options.Store.WorktreeRoot` (symlinks already resolved) and `os.UserHomeDir()` passed through
  `filepath.EvalSymlinks` (a failure of either leaves home empty), into a new model field.
- `tabBar` tries the path label, then the version-only label, then none, each against
  `versionLabelMinGap`. The path is part of the version label, so it is drawn only when the
  model has a version. Update the `Options.Version`, `tabBar` and constant doc comments.
- Update `docs/site/getting-started/terminal-ui.md` (the tab bar sample and its description).

**Regression guard.**
- The goldens draw the tab bar with Version set, so the test store's random `t.TempDir` root would change every
  120-wide golden on every run: add a test seam, an `Options` field (e.g. `Home`, or the label path itself) that
  `newTestOptions` pins to a fixed value, and regenerate the goldens with `-update`.
- Rewrite TestModel_TabBar_EndsInTheVersionLabelWhenItFits into the three-width test the item describes, with the
  label path pinned through that seam, never the real temp root or `$HOME`.
- Add a full single-package run `GOMEMLIMIT=2GiB go test ./internal/ui/` to Acceptance so the UI goldens run.

**Files:** `internal/ui/layout.go`, `internal/ui/model.go`, `internal/ui/ui.go`,
`internal/ui/layout_test.go`, `internal/ui/harness_test.go`, `internal/ui/testdata/golden/`,
`docs/site/getting-started/terminal-ui.md`
**Read first:** `internal/ui/layout.go` — tabBar, versionLabelPrefix/Suffix/MinGap; `internal/ui/model.go` — newModel,
model.version; `internal/ui/ui.go` — Options; `internal/ui/harness_test.go` — newTestOptions, compareGolden;
`internal/ui/layout_test.go` — TestModel_TabBar_EndsInTheVersionLabelWhenItFits; `internal/cli/ui.go` — newUICommand
**Tests:** table test of `displayPath`: under home, equal to home, sibling prefix, outside home,
empty home. `tabBar` tests at three widths: full label, version-only, none; the label's last
character stays in the last column; the label path pinned through the test seam. Goldens
regenerated with `-update`.
**Acceptance:**
- `GOMEMLIMIT=2GiB go test ./internal/ui/ -run 'TabBar|DisplayPath|Version'`
- `GOMEMLIMIT=2GiB go test ./internal/ui/`
- `go vet ./internal/ui/`
**Commit:** `feat(ui): name the worktree in the tab bar`
