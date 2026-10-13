package cli

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
)

// Flags of the know subcommands that other commands share.
const (
	flagDepth = "depth"
)

// flagNoElsewhere skips the Elsewhere scan of the commands that run one.
const flagNoElsewhere = "no-elsewhere"

// flagCheck makes capcom agents and capcom fmt --check report the state of what they would write
// instead of writing it.
const flagCheck = "check"

// unlimitedDepth is the --depth of know tree that prints every level.
const unlimitedDepth = 0
