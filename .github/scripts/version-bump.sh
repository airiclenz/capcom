#!/usr/bin/env bash
# Decide whether a pushed range changed VERSION, and which commit(s) to tag for it.
# Usage: version-bump.sh <before-sha> <after-sha>
#
# Writes one output, `bumps`, to $GITHUB_OUTPUT (stdout when unset, so the decision
# is runnable by hand): a space-separated list of `<sha>=<tag>` pairs, oldest first,
# one per commit in the range that changed VERSION to a bare X.Y.Z value. The tag is
# `v` + VERSION. Empty when there is nothing to tag. Exits 1, after writing `bumps`,
# when it refused to tag any commit whose VERSION is not a bare X.Y.Z.
set -euo pipefail

before="${1:?before sha}"
after="${2:?after sha}"

zero='0000000000000000000000000000000000000000'
# Branch creation, or a history rewrite the clone no longer holds: fall back to the
# newest release tag reachable from the pushed head, so a bump buried under later
# commits is still in the range; with no such tag, to the parent of the pushed head,
# so a single commit is still comparable.
if [ "$before" = "$zero" ] || ! git cat-file -e "${before}^{commit}" 2>/dev/null; then
  before="$(git describe --tags --abbrev=0 --match 'v[0-9]*' "$after" 2>/dev/null \
    || git rev-parse --verify --quiet "${after}^" || true)"
fi
if [ -n "$before" ]; then range="${before}..${after}"; else range="$after"; fi

read_version() {  # missing file or missing commit yields empty, never a failed step
  git show "$1:VERSION" 2>/dev/null | tr -d ' \t\r\n' || true
}

# EVERY commit in the pushed range that changed VERSION, oldest first — not the push
# head, and not only the last one. A push here is routinely a burst of commits; the
# bump is a standalone commit that later commits sit on top of, and a burst can carry
# two bumps.
bumps=''
refused=''
for target in $(git log --first-parent --reverse --format=%H "$range" -- VERSION || true); do
  new="$(read_version "$target")"
  old="$(read_version "${target}^")"
  [ -n "$new" ] && [ "$new" != "$old" ] || continue
  if printf '%s\n' "$new" | grep -Eqx '[0-9]+\.[0-9]+\.[0-9]+'; then
    bumps="${bumps:+$bumps }${target}=v${new}"
  else
    echo "VERSION:1: at ${target} reads '${new}', not a bare X.Y.Z value — refusing to tag" >&2
    refused=yes
  fi
done

echo "bumps=$bumps" >> "${GITHUB_OUTPUT:-/dev/stdout}"
# A refused bump fails the step, after `bumps` is written: the valid bumps are still
# tagged, and the job goes red instead of passing over the refusal.
if [ -n "$refused" ]; then exit 1; fi
