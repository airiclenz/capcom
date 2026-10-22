#!/bin/sh
# Prints the capcom prime Briefing for the session's repository. It stays silent when capcom is
# not installed, when the repository has no capcom store, and when the project or user settings
# already hold a hook whose "command" is exactly "capcom prime" (capcom init --claude-hooks), so
# the Briefing never prints twice. Any other mention of capcom prime, such as a permission rule,
# does not silence it. It always exits 0, so it never blocks a session.

command -v capcom >/dev/null 2>&1 || exit 0

project_dir=${CLAUDE_PROJECT_DIR:-.}
root=$(git -C "$project_dir" rev-parse --show-toplevel 2>/dev/null) || exit 0
[ -d "$root/.capcom" ] || exit 0

for settings in \
  "$root/.claude/settings.json" \
  "$root/.claude/settings.local.json" \
  "$project_dir/.claude/settings.json" \
  "$project_dir/.claude/settings.local.json" \
  "$HOME/.claude/settings.json"; do
  grep -Eqs '"command"[[:space:]]*:[[:space:]]*"capcom prime"' "$settings" && exit 0
done

cd "$root" && capcom prime
exit 0
