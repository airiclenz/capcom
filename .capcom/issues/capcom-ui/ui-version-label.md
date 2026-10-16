---
title: Show the version and app name in the tab bar
status: open
type: feature
priority: 2
labels: [ui]
plan: 2026-10-16 - 02 - ui-apogee-colors-columns-version-plan
created: 2026-10-16T16:12:45Z
---

capcom ui does not say which capcom it is. The user wants `v<version> · capcom` in the top-right
corner (decided 2026-10-16): at the right end of the tab bar, its last character in the frame's
last column so it lines up with the right end of the rule under it. It is left out when the tabs
leave no room for it.

## Acceptance

- At 120 columns the tab bar ends in `v<the VERSION file's version> · capcom` in the last column.
- At a width the tabs fill, the label is not drawn and the tabs are not cut for it.
