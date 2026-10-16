---
title: Theme presets and colour overrides from the user config
status: done
type: feature
priority: 2
deps: [ui-skeleton, ui-user-config]
created: 2026-10-15T20:03:24Z
closed: 2026-10-16T10:28:25Z
reason: Resolved by the capcom-ui plan (docs/plans/archived/2026-10-16 - 00 - capcom-ui-plan.md, archived in 65127ff)
---

Colours for `capcom ui` from the per-user config (ADR 0011):

```yaml
ui:
  theme: auto        # auto | dark | light
  colors:            # optional overrides per role
    accent: "#7aa2f7"
```

`auto` (default) detects a light or dark background. Overrides name roles (accent, border, muted,
link, each status category, ...), not elements. `NO_COLOR` turns colour off whatever the file says.
Config warnings and errors show in the status bar.

## Acceptance

- `dark` and `light` presets exist; `auto` picks one from the detected background.
- An override changes that role everywhere it is used; an unknown role is a warning.
- `NO_COLOR` set produces output with no colour sequences.
- Golden tests cover both presets and `NO_COLOR`.
