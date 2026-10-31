package cli

import (
	"errors"
	"slices"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/ops"
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

// invalidErrorf builds the invalid-code refusal of a flag that sets a field.
func invalidErrorf(format string, args ...any) *output.Error {
	return output.Errorf(output.CodeInvalid, format, args...)
}

// valueFailure words err, when it is an *ops.ValueError, as the refusal of the flag that sets its
// field, `--<field> <value>: <reason>`, carrying in Valid the values the flag accepts. refuse
// types it: output.UsageErrorf for a filter flag, invalidErrorf for a flag that sets a field. Any
// other err, nil included, is returned as it is.
func valueFailure(err error, refuse func(format string, args ...any) *output.Error) error {
	var value *ops.ValueError
	if !errors.As(err, &value) {
		return err
	}
	format := "--%s %q: %v"
	if value.Field == ops.FieldPriority {
		format = "--%s %s: %v" // a number, written bare
	}
	failure := refuse(format, value.Field, value.Value, value.Err)
	failure.Valid = value.Valid
	return failure
}

// checkCategory refuses, as a usage error, a --category that is not one of the status categories,
// listing them in Valid.
func checkCategory(category string) error {
	if slices.Contains(statusCategories, config.Category(category)) {
		return nil
	}
	valid := make([]string, 0, len(statusCategories))
	for _, known := range statusCategories {
		valid = append(valid, string(known))
	}
	failure := output.UsageErrorf("--%s %q: not a status category", flagCategory, category)
	failure.Valid = valid
	return failure
}
