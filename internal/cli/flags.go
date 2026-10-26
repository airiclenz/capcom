package cli

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
)

// The flag names two or more commands share. A flag of a single command stays in that command's
// file.

// Flags of capcom list that other commands share.
const (
	flagStatus    = "status"
	flagLabel     = "label"
	flagNoLabel   = "no-label"
	flagType      = "type"
	flagPriority  = "priority"
	flagParent    = "parent"
	flagPlan      = "plan"
	flagPlanned   = "planned"
	flagUnplanned = "unplanned"
	flagAll       = "all"
	flagLimit     = "limit"
)

// Flags of capcom update that other commands share.
const (
	flagTitle          = "title"
	flagAddLabel       = "add-label"
	flagRemoveLabel    = "remove-label"
	flagBodyAppend     = "body-append"
	flagBodyAppendFile = "body-append-file"
	flagBodyReplaceOld = "body-replace-old"
	flagBodyReplaceNew = "body-replace-new"
	flagIfMatch        = "if-match"
)

// Flags of capcom create that other commands share.
const (
	flagID       = "id"
	flagBodyFile = "body-file"
	flagStanding = "standing"
)

// Flags of the know subcommands that other commands share.
const (
	flagDepth   = "depth"
	flagRetired = "retired"
)

// flagExcerpt makes capcom search and capcom know search print the body lines each hit matches on.
const flagExcerpt = "excerpt"

// flagNoElsewhere skips the Elsewhere scan of the commands that run one.
const flagNoElsewhere = "no-elsewhere"

// flagCheck makes capcom agents and capcom fmt --check report the state of what they would write
// instead of writing it.
const flagCheck = "check"

// unlimitedDepth is the --depth of know tree that prints every level.
const unlimitedDepth = 0

// valueChecker checks --status, --type, --category and --priority values against cfg. A value the
// flag does not accept is refused through refuse, carrying in Valid the values it does accept, so
// list can type the refusal as a usage error and the writing commands as invalid.
type valueChecker struct {
	cfg    config.Config
	refuse func(format string, args ...any) *output.Error
}

// usageValues returns the valueChecker of a filter flag, whose refusals are usage errors.
func usageValues(cfg config.Config) valueChecker {
	return valueChecker{cfg: cfg, refuse: output.UsageErrorf}
}

// invalidValues returns the valueChecker of a flag that sets a field, whose refusals are invalid.
func invalidValues(cfg config.Config) valueChecker {
	return valueChecker{cfg: cfg, refuse: func(format string, args ...any) *output.Error {
		return output.Errorf(output.CodeInvalid, format, args...)
	}}
}

// status refuses a --status that no status in the config declares; the declared statuses are
// listed as `name (category)` in file order.
func (c valueChecker) status(status string) error {
	if _, isDeclared := c.cfg.CategoryOf(status); isDeclared {
		return nil
	}
	valid := make([]string, 0, len(c.cfg.Statuses))
	for _, declared := range c.cfg.Statuses {
		valid = append(valid, fmt.Sprintf("%s (%s)", declared.Name, declared.Category))
	}
	return c.refused(valid, "--%s %q: not a status declared in config.yaml", flagStatus, status)
}

// issueType refuses a --type that the config does not declare.
func (c valueChecker) issueType(issueType string) error {
	if slices.Contains(c.cfg.Types, issueType) {
		return nil
	}
	return c.refused(slices.Clone(c.cfg.Types), "--%s %q: not a type declared in config.yaml", flagType, issueType)
}

// category refuses a --category that is not one of the status categories.
func (c valueChecker) category(category string) error {
	if slices.Contains(statusCategories, config.Category(category)) {
		return nil
	}
	valid := make([]string, 0, len(statusCategories))
	for _, known := range statusCategories {
		valid = append(valid, string(known))
	}
	return c.refused(valid, "--%s %q: not a status category", flagCategory, category)
}

// priority refuses a --priority outside issue.MinPriority to issue.MaxPriority.
func (c valueChecker) priority(priority int) error {
	if priority >= issue.MinPriority && priority <= issue.MaxPriority {
		return nil
	}
	valid := make([]string, 0, issue.MaxPriority-issue.MinPriority+1)
	for value := issue.MinPriority; value <= issue.MaxPriority; value++ {
		valid = append(valid, strconv.Itoa(value))
	}
	return c.refused(valid, "--%s %d: out of range", flagPriority, priority)
}

// refused builds the refusal with the formatted message and the values the flag accepts.
func (c valueChecker) refused(valid []string, format string, args ...any) error {
	failure := c.refuse(format, args...)
	failure.Valid = valid
	return failure
}
