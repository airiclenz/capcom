package userconfig

// template is the file Load creates when none exists: every key present and commented out at its
// default, so the file documents itself without pinning any value. A colour override has no
// default, so the colors block shows examples instead. The roles listed must match Roles.
const template = `# capcom user config: personal settings for capcom ui (ADR 0011).
#
# This file belongs to you, not to any repository: only capcom ui reads it, and every repository
# you open shares it. Every key below is commented out at its default; remove the leading "# " of
# a line to set it. An unknown key is ignored with a warning.
#
# ui:
#   # The colour preset: auto picks dark or light from the terminal background.
#   theme: auto        # auto | dark | light
#
#   # Colour overrides by role; none by default, so each role takes the preset's colour. A value
#   # is a quoted "#rgb" or "#rrggbb" colour (an unquoted # starts a comment) or a number 0-255.
#   # Roles: accent, border, muted, link, selection, heading, code, and one per status category:
#   # status-todo, status-doing, status-waiting, status-done, status-dropped. The status bar's lock
#   # badge: lock-held while another session holds the Tree lock, lock-free while it is free.
#   colors:
#     accent: "#7aa2f7"
#     status-doing: 214
#
# NO_COLOR in the environment turns colour off, whatever this file says.
`
