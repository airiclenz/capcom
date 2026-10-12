// Package issue reads and writes issue files: markdown with a YAML frontmatter between two `---`
// lines. Parse turns a file into an Issue, Serialize writes one back in the canonical form (fixed
// key order, flow-style lists, RFC3339 UTC timestamps, body kept verbatim), and parsing a
// canonical file and serialising it again yields the same bytes. Validate checks an Issue against
// the repository's config. Every error names the file, and the line when one is to blame.
package issue

import (
	"time"

	"github.com/airiclenz/capcom/internal/frontmatter"
)

// Priority bounds and the priority an issue without one gets. 0 is the most urgent.
const (
	MinPriority     = 0
	MaxPriority     = 4
	DefaultPriority = 2
)

// Frontmatter keys, in the order Serialize writes them.
const (
	keyTitle    = "title"
	keyStatus   = "status"
	keyType     = "type"
	keyPriority = "priority"
	keyLabels   = "labels"
	keyDeps     = "deps"
	keyRelated  = "related"
	keyPlan     = "plan"
	keyAliases  = "aliases"
	keyCreated  = "created"
	keyClosed   = "closed"
	keyReason   = "reason"
)

// Issue is one parsed issue file. Its id is the file name and is never stored in the file. Empty
// strings, nil lists and zero times are fields the file leaves out.
type Issue struct {
	Title  string
	Status string
	Type   string
	// Priority runs from MinPriority (most urgent) to MaxPriority; Parse sets DefaultPriority
	// when the file has none.
	Priority int
	Labels   []string
	// Deps are the ids of the issues this one depends on.
	Deps []string
	// Related are the ids of the issues this one is linked to without depending on them. The
	// link is stored on one issue only; the reverse is derived.
	Related []string
	// Plan is the basename of the Plan document, resolved against the configured plan
	// directories.
	Plan string
	// Aliases are the issue's former ids (ADR 0002).
	Aliases []string
	// Created is when the issue was created, in UTC to the second.
	Created time.Time
	// Closed is when the issue was last closed, in UTC to the second; zero while it is open.
	Closed time.Time
	// Reason says why the issue was closed.
	Reason string
	// Body is the markdown after the frontmatter, byte for byte (LF line endings), with the
	// `## Acceptance` and `## Log` sections by convention.
	Body string

	// lines maps each frontmatter key Parse found to its 1-based line in the file, so Validate
	// can cite it. It is nil for an Issue built in memory.
	lines map[string]int
}

// Error is an issue file that cannot be used: the one frontmatter.Error both kinds report, with
// the file, the 1-based line of the offending key or value (0 when no single line is to blame) and
// the message.
type Error = frontmatter.Error

// DepsLine returns the 1-based line of the deps key in the file the Issue was parsed from, or 0
// when the file has none or the Issue was built in memory.
func (i Issue) DepsLine() int {
	return i.lines[keyDeps]
}

// RelatedLine returns the 1-based line of the related key in the file the Issue was parsed from,
// or 0 when the file has none or the Issue was built in memory.
func (i Issue) RelatedLine() int {
	return i.lines[keyRelated]
}

// PlanLine returns the 1-based line of the plan key in the file the Issue was parsed from, or 0
// when the file has none or the Issue was built in memory.
func (i Issue) PlanLine() int {
	return i.lines[keyPlan]
}

// AliasesLine returns the 1-based line of the aliases key in the file the Issue was parsed from,
// or 0 when the file has none or the Issue was built in memory.
func (i Issue) AliasesLine() int {
	return i.lines[keyAliases]
}
