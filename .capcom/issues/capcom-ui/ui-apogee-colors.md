---
title: Draw capcom ui in apogee's colours, from editable scheme files
status: open
type: feature
priority: 2
labels: [ui]
plan: 2026-10-16 - 02 - ui-apogee-colors-columns-version-plan
created: 2026-10-16T16:12:45Z
---

capcom ui's dark and light presets are Go maps inside `internal/ui/theme.go` with their own
palette. The UI should draw in apogee's colours, so the two tools look like one family, and the
colours should live in one easy-to-edit place per scheme, as a first step towards switchable
colour schemes.

Wanted (decided by the user, 2026-10-16):

- Each preset is an embedded YAML file, `internal/ui/schemes/dark.yaml` and `light.yaml`, in
  apogee's scheme file format: one `role: "#rrggbb"` line per colour role, commented.
- The colours are apogee's, by this mapping (apogee role → capcom role): gauge → accent,
  chrome → border, muted → muted, skill → link, selection → selection, tool-header → heading,
  code → code, muted-bright → status-todo, mode-allow-edits → status-doing,
  mode-auto → status-waiting, success → status-done, chrome → status-dropped.
- `ui.theme` and the `ui.colors` overrides keep working as they do.

## Acceptance

- `capcom ui` under `theme: dark` and `theme: light` draws apogee's colours by that mapping.
- Changing a colour means editing one line of a scheme file; no Go code names a hex colour.
