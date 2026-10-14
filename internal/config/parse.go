package config

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// categories lists every category in its canonical order.
var categories = []Category{CategoryTodo, CategoryDoing, CategoryWaiting, CategoryDone, CategoryDropped}

// Parse decodes and validates config.yaml content. path names the file in every error. Keys the
// content leaves out take their Default value, except a missing format key, which means format 1;
// an empty document is the default configuration at format 1.
func Parse(path string, data []byte) (Config, error) {
	config, _, err := parse(path, data)
	return config, err
}

// parse is Parse that also returns the content's top-level mapping node, nil for an empty
// document.
func parse(path string, data []byte) (Config, *yaml.Node, error) {
	p := parser{path: path}
	config := Default()
	config.Format = MissingFormat

	root, err := p.document(data)
	if err != nil {
		return Config{}, nil, err
	}
	if root == nil {
		return config, nil, nil
	}
	if err := p.format(root, &config); err != nil {
		return Config{}, nil, err
	}
	if err := p.topLevel(root, &config); err != nil {
		return Config{}, nil, err
	}
	return config, root, nil
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

// document parses data into its top-level mapping node, or nil for an empty document. It fails on
// a second YAML document, so no key escapes the checks behind a `---` or `...` line.
func (p parser) document(data []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, p.yamlError(err)
	}
	var second yaml.Node
	if err := decoder.Decode(&second); err != io.EOF {
		if err != nil {
			return nil, p.yamlError(err)
		}
		return nil, p.errorAt(&second, "only one YAML document is allowed")
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

// yamlError turns a yaml.v3 syntax error into an *Error; the line stays inside the message.
func (p parser) yamlError(err error) *Error {
	return &Error{Path: p.path, Message: fmt.Sprintf("invalid YAML: %v", err)}
}

// format reads the format key of root before any other key is looked at, so a store newer than
// this capcom fails as too new rather than on a key this capcom does not know (ADR 0009).
func (p parser) format(root *yaml.Node, config *Config) error {
	key, value := formatEntry(root)
	if key == nil {
		return nil
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

// formatEntry returns the key and value nodes of root's first format key, or nil, nil without one.
func formatEntry(root *yaml.Node) (key *yaml.Node, value *yaml.Node) {
	for index := 0; index+1 < len(root.Content); index += 2 {
		if root.Content[index].Value == keyFormat {
			return root.Content[index], root.Content[index+1]
		}
	}
	return nil, nil
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
		case keyAgents:
			return p.agents(key, value, config)
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

// agents reads the agents section. Its key needs store format FormatAgents: in a store declaring
// less it fails at the key's line, naming the format to set, whatever the section holds.
func (p parser) agents(sectionKey, value *yaml.Node, config *Config) error {
	if config.Format < FormatAgents {
		return p.errorAt(sectionKey, "%s needs store format %d, but the store declares format %d: set format: %d",
			keyAgents, FormatAgents, config.Format, FormatAgents)
	}
	return p.eachKey(value, func(key, setting *yaml.Node) error {
		if key.Value != keyGuides {
			return p.errorAt(key, "unknown key %q in %s", key.Value, keyAgents)
		}
		return p.guides(setting, config)
	})
}

// guides reads agents.guides: a possibly empty list of local slash-separated paths, none of them
// AGENTS.md (in any letter case) and no two the same once cleaned.
func (p parser) guides(value *yaml.Node, config *Config) error {
	const field = keyAgents + "." + keyGuides
	var guides []string
	if err := p.nameList(value, field, &guides, false); err != nil {
		return err
	}

	seen := make(map[string]bool, len(guides))
	for index, guide := range guides {
		item := value.Content[index]
		// A backslash is a separator on Windows only, so a guide spelled with one would name a
		// different file on each platform.
		if strings.Contains(guide, `\`) || !filepath.IsLocal(filepath.FromSlash(guide)) {
			return p.errorAt(item, "%s: %q must be a slash-separated path inside the repository", field, guide)
		}
		cleaned := path.Clean(guide)
		if strings.EqualFold(cleaned, agentsFile) {
			return p.errorAt(item, "%s: %q is %s itself, the file that holds the Managed block", field, guide, agentsFile)
		}
		if seen[cleaned] {
			return p.errorAt(item, "%s: duplicate entry %q", field, guide)
		}
		seen[cleaned] = true
	}
	config.Agents.Guides = guides
	return nil
}

func isCategory(candidate Category) bool {
	for _, category := range categories {
		if category == candidate {
			return true
		}
	}
	return false
}
