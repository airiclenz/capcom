// Package config parses and validates a store's config.yaml: the store's format version, the
// repository's statuses and the Status category each maps to, its issue types, the plan directories, the ready filter, the Tree
// lock TTL, the Elsewhere window and the agent guides that must reach the Managed block. Every key is optional and falls back to its default; an
// unknown key or a bad value fails with the file and line it sits on.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Category is one of the five fixed meanings a Status can have (ADR 0003). capcom's behaviour
// reads only the category, never the status name.
type Category string

// The closed set of status categories.
const (
	CategoryTodo    Category = "todo"
	CategoryDoing   Category = "doing"
	CategoryWaiting Category = "waiting"
	CategoryDone    Category = "done"
	CategoryDropped Category = "dropped"
)

// IsClosed reports whether c is a Closed category: done or dropped. It is the one statement of
// which categories close an issue; ClosedCategories lists the same set.
func (c Category) IsClosed() bool {
	return c == CategoryDone || c == CategoryDropped
}

// ClosedCategories returns the categories IsClosed is true for, done then dropped, in a fresh
// slice the caller may change.
func ClosedCategories() []Category {
	return []Category{CategoryDone, CategoryDropped}
}

// FormatVersion is the highest store format this capcom reads, and the one `capcom init` writes
// (ADR 0009). Format 2 adds the related key to issue files; format 3 adds the retired and
// superseded-by keys to Notes; format 4 adds the agents.guides key to config.yaml; format 5 adds
// the standing key to issue files.
const FormatVersion = 5

// The store formats that capcom's format rules name (ADR 0009).
const (
	// MissingFormat is the format of a store with no config.yaml, or of a config.yaml with no
	// format key: every store written before the key existed.
	MissingFormat = 1
	// FormatRelated is the first store format whose issue files may carry the related key.
	FormatRelated = 2
	// FormatSupersession is the first store format whose Note files may carry the retired and
	// superseded-by keys.
	FormatSupersession = 3
	// FormatAgents is the first store format whose config.yaml may carry the agents key.
	FormatAgents = 4
	// FormatStanding is the first store format whose issue files may carry the standing key.
	FormatStanding = 5
)

// formatComment is the template's comment line above its format key.
const formatComment = "# The store format version. capcom refuses a store whose format is newer than it reads."

// keylessFormatComment is KeylessFile's comment line above its commented-out format key;
// RaiseFormat replaces it together with that key.
const keylessFormatComment = "# The store format version; left as a comment, since a missing key means format 1."

// Top-level and section keys of config.yaml.
const (
	keyFormat    = "format"
	keyStatuses  = "statuses"
	keyTypes     = "types"
	keyPlans     = "plans"
	keyReady     = "ready"
	keyHide      = "hide"
	keyLock      = "lock"
	keyTTL       = "ttl"
	keyElsewhere = "elsewhere"
	keyDays      = "days"
	keyAgents    = "agents"
	keyGuides    = "guides"
)

// agentsFile is the agent guide that holds the Managed block; no other guide may name it.
const agentsFile = "AGENTS.md"

// Defaults for the keys a config file leaves out.
const (
	defaultLockTTL       = 2 * time.Hour
	defaultElsewhereDays = 30
)

// maxElsewhereDays bounds elsewhere.days at about a century: a larger window overflows the
// Elsewhere scan's cutoff duration and silences the scan.
const maxElsewhereDays = 36500

// DefaultFile is the config.yaml `capcom init` writes into a store it creates. It parses to
// Default().
const DefaultFile = templateHead + formatComment + "\nformat: 5\n" + templateBody

// KeylessFile is the config.yaml `capcom init` writes into an existing store that has none: the
// same file as DefaultFile with the format key left as a comment under its own comment line, so
// the store keeps format 1. It parses to Default() at MissingFormat, and RaiseFormat turns it into
// DefaultFile.
const KeylessFile = templateHead + keylessFormatComment + "\n# format: 1\n" + templateBody

// templateHead is the part of the config.yaml template above its format comment.
const templateHead = `# capcom configuration. Every key is optional; a missing key takes the value shown here.

`

// templateBody is the part of the config.yaml template below its format key.
const templateBody = `
# Issue statuses in order, each mapped to its category: todo, doing, waiting, done or dropped.
# capcom's rules read only the category. The first status listed in a category is its default.
statuses:
  open: todo
  active: doing
  parked: waiting
  done: done
  dropped: dropped

# Issue types.
types: [bug, feature, task, chore]

# Directories, relative to the repository root, that an issue's plan: field is resolved against.
plans: [docs/plans, docs/plans/archived]

ready:
  # Issues carrying any of these labels are left out of ready unless asked for.
  hide: []

lock:
  # A Tree lock whose heartbeat is older than this, or dated more than this ahead, is stale.
  ttl: 2h

elsewhere:
  # Branches whose tip is older than this many days are not scanned.
  days: 30

# Agent guides that must reach AGENTS.md's Managed block by a link or an @AGENTS.md import (format 4).
# agents:
#   guides: [CLAUDE.md, GEMINI.md]
`

// planExtension is the extension a plan basename is looked up with, besides the bare name.
const planExtension = ".md"

// Status is one configured issue status and the category it belongs to.
type Status struct {
	Name     string
	Category Category
}

// Config is a parsed, validated config.yaml.
type Config struct {
	// Format is the store format version, from 1 up to FormatVersion; 1 when the file has no
	// format key.
	Format int
	// Statuses in file order. Every name is unique, and at least one status is in the todo
	// category and one in the done category.
	Statuses  []Status
	Types     []string
	Plans     []string
	Ready     Ready
	Lock      Lock
	Elsewhere Elsewhere
	Agents    Agents
}

// Ready configures the ready command.
type Ready struct {
	// Hide lists labels whose issues ready leaves out by default.
	Hide []string
}

// Lock configures the Tree lock.
type Lock struct {
	// TTL is how old a lock's heartbeat may be before the lock counts as stale. Always positive.
	TTL time.Duration
}

// Elsewhere configures the scan of other local and remote-tracking branches.
type Elsewhere struct {
	// Days is the age, in days, beyond which a branch tip is ignored. 0 to maxElsewhereDays.
	Days int
}

// Agents configures the agent guides kept beside AGENTS.md.
type Agents struct {
	// Guides lists the agent guides, as local slash-separated paths relative to the repository
	// root, that must reach AGENTS.md's Managed block. Unique, never AGENTS.md; empty turns the
	// rule off.
	Guides []string
}

// Error is a config file that cannot be used. Line is the 1-based line of the offending key or
// value, or 0 when no single line is to blame.
type Error struct {
	Path    string
	Line    int
	Message string
}

// Error renders "path:line: message", or "path: message" when the line is unknown.
func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// Default returns the configuration used for every key a config file leaves out, at the current
// format: the configuration DefaultFile parses to.
func Default() Config {
	return Config{
		Format: FormatVersion,
		Statuses: []Status{
			{Name: "open", Category: CategoryTodo},
			{Name: "active", Category: CategoryDoing},
			{Name: "parked", Category: CategoryWaiting},
			{Name: "done", Category: CategoryDone},
			{Name: "dropped", Category: CategoryDropped},
		},
		Types:     []string{"bug", "feature", "task", "chore"},
		Plans:     []string{"docs/plans", "docs/plans/archived"},
		Ready:     Ready{Hide: []string{}},
		Lock:      Lock{TTL: defaultLockTTL},
		Elsewhere: Elsewhere{Days: defaultElsewhereDays},
		Agents:    Agents{Guides: []string{"CLAUDE.md", "GEMINI.md"}},
	}
}

// CategoryOf returns the category of the named status, and false when the status is not
// configured.
func (c Config) CategoryOf(status string) (Category, bool) {
	for _, configured := range c.Statuses {
		if configured.Name == status {
			return configured.Category, true
		}
	}
	return "", false
}

// DefaultStatus returns the first status listed in category, the one capcom assigns when it moves
// an issue into that category (create, close, reopen), and false when no status maps to it.
func (c Config) DefaultStatus(category Category) (string, bool) {
	for _, configured := range c.Statuses {
		if configured.Category == category {
			return configured.Name, true
		}
	}
	return "", false
}

// ValidatePlanName returns an error when plan is not a bare basename, the only form a plan: value
// takes: it is refused when empty, absolute, holding a volume name, a / or a \, and when it is "."
// or "..". The error's text names the plans directories and leaves the plan itself out, for the
// caller to name it as it was given (a flag, a file).
func (c Config) ValidatePlanName(plan string) error {
	isPath := filepath.IsAbs(plan) || filepath.VolumeName(plan) != "" || strings.ContainsAny(plan, `/\`)
	if plan == "" || plan == "." || plan == ".." || isPath {
		return fmt.Errorf(
			"not a plan basename: name a file in one of the plans directories %v by its name alone, "+
				"with or without .md",
			c.Plans,
		)
	}
	return nil
}

// MissingPlanMessage returns the message of a plan that is in none of the plans directories, the
// text capcom check reports for it and a write that sets it warns with.
func (c Config) MissingPlanMessage(plan string) string {
	return fmt.Sprintf("plan %q is in none of the plans directories %v", plan, c.Plans)
}

// PlanPath returns the path of the file plan names, and whether one was found: plan is a basename,
// looked up as <dir>/<plan>.md and then <dir>/<plan> in each of the plans directories in turn,
// each relative to root, the repository's worktree root. Only a regular file counts: a directory
// is passed over. A plan ValidatePlanName refuses is never found.
func (c Config) PlanPath(root, plan string) (path string, isFound bool) {
	if c.ValidatePlanName(plan) != nil {
		return "", false
	}
	for _, dir := range c.Plans {
		base := filepath.Join(root, filepath.FromSlash(dir), plan)
		for _, candidate := range []string{base + planExtension, base} {
			info, err := os.Stat(candidate)
			if err == nil && info.Mode().IsRegular() {
				return candidate, true
			}
		}
	}
	return "", false
}
