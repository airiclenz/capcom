// Package clidoc renders capcom's command tree as the Markdown CLI reference under
// docs/site/reference/cli: one page per available command, each opening with the YAML frontmatter
// the docs page contract asks for. It is linked only by cmd/gendocs, so the capcom binary never
// carries the cobra/doc generator or its dependencies.
package clidoc
