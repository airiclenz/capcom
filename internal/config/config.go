// Package config parses and validates a store's config.yaml: the store's format version, the
// repository's statuses and the Status category each maps to, its issue types, the plan directories, the ready filter, the Tree
// lock TTL and the Elsewhere window. Every key is optional and falls back to its default; an
// unknown key or a bad value fails with the file and line it sits on.
package config

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
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

// categories lists every category in its canonical order.
var categories = []Category{CategoryTodo, CategoryDoing, CategoryWaiting, CategoryDone, CategoryDropped}

// FormatVersion is the highest store format this capcom reads, and the one `capcom init` writes
// (ADR 0009). Format 2 adds the related key to issue files; format 3 adds the retired and
// superseded-by keys to Notes.
const FormatVersion = 3

// keylessFormat is the format of a config.yaml without a format key: every store written before
// the key existed (ADR 0009).
const keylessFormat = 1

// formatComment is the template's comment line above its format key.
const formatComment = "# The store format version. capcom refuses a store whose format is newer than it reads."

// keylessFormatComment is the comment line the format-1 template carried above its commented-out
// format key; RaiseFormat replaces it together with that key.
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
)

// Defaults for the keys a config file leaves out.
const (
	defaultLockTTL       = 2 * time.Hour
	defaultElsewhereDays = 30
)

// maxElsewhereDays bounds elsewhere.days at about a century: a larger window overflows the
// Elsewhere scan's cutoff duration and silences the scan.
const maxElsewhereDays = 36500

// DefaultFile is the config.yaml `capcom init` writes. It parses to Default().
const DefaultFile = `# capcom configuration. Every key is optional; a missing key takes the value shown here.

` + formatComment + `
format: 3

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
  # A Tree lock whose heartbeat is older than this is stale.
  ttl: 2h

elsewhere:
  # Branches whose tip is older than this many days are not scanned.
  days: 30
`

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

// Parse decodes and validates config.yaml content. path names the file in every error. Keys the
// content leaves out take their Default value, except a missing format key, which means format 1;
// an empty document is the default configuration at format 1.
func Parse(path string, data []byte) (Config, error) {
	p := parser{path: path}
	config := Default()
	config.Format = keylessFormat

	root, err := p.document(data)
	if err != nil {
		return Config{}, err
	}
	if root == nil {
		return config, nil
	}
	if err := p.format(root, &config); err != nil {
		return Config{}, err
	}
	if err := p.topLevel(root, &config); err != nil {
		return Config{}, err
	}
	return config, nil
}

// realFormatLine matches a top-level format key and captures its value; commentedFormatLine
// matches a commented-out one such as the format-1 template's `# format: 1`.
var (
	realFormatLine      = regexp.MustCompile(`^format:[ \t]*([^#\s]*)`)
	commentedFormatLine = regexp.MustCompile(`^#[ \t]*format:[ \t]*[0-9]+[ \t]*$`)
)

// RaiseFormat returns config.yaml content that declares at least the given store format, every
// other byte kept, and whether it changed anything. A top-level format line is replaced by
// `format: <version>`; failing that, a commented-out one is, and the format-1 template's comment
// line directly above it becomes the current template's. Content with neither gains
// `format: <version>` as its first line, so the key is never written twice. Content already at
// version or above is returned unchanged.
func RaiseFormat(data []byte, version int) ([]byte, bool) {
	lines := bytes.SplitAfter(data, []byte("\n"))
	index, declared := findFormatLine(lines)
	if declared >= version {
		return data, false
	}

	formatKey := fmt.Sprintf("%s: %d", keyFormat, version)
	if index < 0 {
		raised := append([]byte(formatKey+lineEnding(lines[0], "\n")), data...)
		return raised, true
	}
	isCommented := commentedFormatLine.Match(trimLineEnding(lines[index]))
	lines[index] = []byte(formatKey + lineEnding(lines[index], ""))
	if isCommented && index > 0 && string(trimLineEnding(lines[index-1])) == keylessFormatComment {
		lines[index-1] = []byte(formatComment + lineEnding(lines[index-1], ""))
	}
	return bytes.Join(lines, nil), true
}

// findFormatLine returns the index of the line holding the top-level format key and the format it
// declares, or, without one, the first commented-out format line and keylessFormat. With neither
// the index is -1. A value that is not a whole number declares format 0, below every version.
func findFormatLine(lines [][]byte) (index int, declared int) {
	commented := -1
	for lineIndex, line := range lines {
		content := trimLineEnding(line)
		if match := realFormatLine.FindSubmatch(content); match != nil {
			version, err := strconv.Atoi(string(match[1]))
			if err != nil {
				version = 0
			}
			return lineIndex, version
		}
		if commented < 0 && commentedFormatLine.Match(content) {
			commented = lineIndex
		}
	}
	return commented, keylessFormat
}

// trimLineEnding returns line without its trailing LF or CRLF.
func trimLineEnding(line []byte) []byte {
	return bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r"))
}

// lineEnding returns the LF or CRLF line ends with, or fallback when it has none.
func lineEnding(line []byte, fallback string) string {
	switch {
	case bytes.HasSuffix(line, []byte("\r\n")):
		return "\r\n"
	case bytes.HasSuffix(line, []byte("\n")):
		return "\n"
	default:
		return fallback
	}
}

// parser decodes one config file, attributing every failure to its path and line.
type parser struct {
	path string
}

func (p parser) errorAt(node *yaml.Node, format string, args ...any) *Error {
	failure := &Error{Path: p.path, Message: fmt.Sprintf(format, args...)}
	if node != nil {
		failure.Line = node.Line
	}
	return failure
}

// document parses data into its top-level mapping node, or nil for an empty document.
func (p parser) document(data []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, &Error{Path: p.path, Message: fmt.Sprintf("invalid YAML: %v", err)}
	}
	if len(document.Content) == 0 {
		return nil, nil
	}
	root := document.Content[0]
	if root.Kind == yaml.ScalarNode && root.Tag == "!!null" {
		return nil, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, p.errorAt(root, "the config must be a mapping of keys")
	}
	return root, nil
}

// format reads the format key of root before any other key is looked at, so a store newer than
// this capcom fails as too new rather than on a key this capcom does not know (ADR 0009).
func (p parser) format(root *yaml.Node, config *Config) error {
	for index := 0; index+1 < len(root.Content); index += 2 {
		key, value := root.Content[index], root.Content[index+1]
		if key.Value != keyFormat {
			continue
		}
		version, err := strconv.Atoi(value.Value)
		if value.Kind != yaml.ScalarNode || err != nil || version < 1 {
			return p.errorAt(value, "format: expected a whole number, 1 or more")
		}
		if version > FormatVersion {
			return p.errorAt(key, "store format %d is newer than this capcom reads (up to %d); upgrade capcom",
				version, FormatVersion)
		}
		config.Format = version
		return nil
	}
	return nil
}

// topLevel applies every top-level key of root onto config. The format key was already read by
// format.
func (p parser) topLevel(root *yaml.Node, config *Config) error {
	return p.eachKey(root, func(key, value *yaml.Node) error {
		switch key.Value {
		case keyFormat:
			return nil
		case keyStatuses:
			return p.statuses(value, config)
		case keyTypes:
			return p.nameList(value, keyTypes, &config.Types, true)
		case keyPlans:
			return p.nameList(value, keyPlans, &config.Plans, false)
		case keyReady:
			return p.ready(value, config)
		case keyLock:
			return p.lock(value, config)
		case keyElsewhere:
			return p.elsewhere(value, config)
		default:
			return p.errorAt(key, "unknown key %q", key.Value)
		}
	})
}

// eachKey calls apply for every key/value pair of a mapping node, rejecting a non-mapping node
// and a key that appears twice.
func (p parser) eachKey(mapping *yaml.Node, apply func(key, value *yaml.Node) error) error {
	if mapping.Kind != yaml.MappingNode {
		return p.errorAt(mapping, "expected a mapping of keys")
	}
	seen := make(map[string]bool, len(mapping.Content)/2)
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		key, value := mapping.Content[index], mapping.Content[index+1]
		if seen[key.Value] {
			return p.errorAt(key, "duplicate key %q", key.Value)
		}
		seen[key.Value] = true
		if err := apply(key, value); err != nil {
			return err
		}
	}
	return nil
}

// statuses reads the ordered status mapping and checks every category and the required ones.
func (p parser) statuses(value *yaml.Node, config *Config) error {
	var statuses []Status
	err := p.eachKey(value, func(key, category *yaml.Node) error {
		if key.Value == "" {
			return p.errorAt(key, "a status name must not be empty")
		}
		if category.Kind != yaml.ScalarNode || !isCategory(Category(category.Value)) {
			return p.errorAt(category, "status %q: category must be one of %v", key.Value, categories)
		}
		statuses = append(statuses, Status{Name: key.Value, Category: Category(category.Value)})
		return nil
	})
	if err != nil {
		return err
	}

	config.Statuses = statuses
	for _, required := range []Category{CategoryTodo, CategoryDone} {
		if _, isPresent := config.DefaultStatus(required); !isPresent {
			return p.errorAt(value, "statuses: at least one status must be in the %s category", required)
		}
	}
	return nil
}

// nameList reads a list of non-empty, unique strings into target. A required list must not be
// empty.
func (p parser) nameList(value *yaml.Node, key string, target *[]string, isRequired bool) error {
	if value.Kind != yaml.SequenceNode {
		return p.errorAt(value, "%s: expected a list", key)
	}
	names := make([]string, 0, len(value.Content))
	seen := make(map[string]bool, len(value.Content))
	for _, item := range value.Content {
		if item.Kind != yaml.ScalarNode || item.Value == "" {
			return p.errorAt(item, "%s: every entry must be a non-empty string", key)
		}
		if seen[item.Value] {
			return p.errorAt(item, "%s: duplicate entry %q", key, item.Value)
		}
		seen[item.Value] = true
		names = append(names, item.Value)
	}
	if isRequired && len(names) == 0 {
		return p.errorAt(value, "%s: at least one entry is required", key)
	}
	*target = names
	return nil
}

func (p parser) ready(value *yaml.Node, config *Config) error {
	return p.eachKey(value, func(key, setting *yaml.Node) error {
		if key.Value != keyHide {
			return p.errorAt(key, "unknown key %q in %s", key.Value, keyReady)
		}
		return p.nameList(setting, keyReady+"."+keyHide, &config.Ready.Hide, false)
	})
}

func (p parser) lock(value *yaml.Node, config *Config) error {
	return p.eachKey(value, func(key, setting *yaml.Node) error {
		if key.Value != keyTTL {
			return p.errorAt(key, "unknown key %q in %s", key.Value, keyLock)
		}
		ttl, err := time.ParseDuration(setting.Value)
		if setting.Kind != yaml.ScalarNode || err != nil || ttl <= 0 {
			return p.errorAt(setting, "lock.ttl: expected a positive duration such as 2h or 90m")
		}
		config.Lock.TTL = ttl
		return nil
	})
}

func (p parser) elsewhere(value *yaml.Node, config *Config) error {
	return p.eachKey(value, func(key, setting *yaml.Node) error {
		if key.Value != keyDays {
			return p.errorAt(key, "unknown key %q in %s", key.Value, keyElsewhere)
		}
		days, err := strconv.Atoi(setting.Value)
		if setting.Kind != yaml.ScalarNode || err != nil || days < 0 || days > maxElsewhereDays {
			return p.errorAt(setting, "elsewhere.days: expected a whole number of days, 0 to %d", maxElsewhereDays)
		}
		config.Elsewhere.Days = days
		return nil
	})
}

func isCategory(candidate Category) bool {
	for _, category := range categories {
		if category == candidate {
			return true
		}
	}
	return false
}
