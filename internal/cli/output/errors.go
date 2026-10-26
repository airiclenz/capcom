package output

import (
	"errors"
	"fmt"
	"strings"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/store"
)

// Code is the machine-readable class of a failed command. It appears as error.code in the
// --json envelope and as the STATUS value in --kv output.
type Code string

// The closed set of error codes capcom reports.
const (
	CodeNotFound       Code = "not_found"
	CodeExists         Code = "exists"
	CodeInvalid        Code = "invalid"
	CodeConflict       Code = "conflict"
	CodeLocked         Code = "locked"
	CodeNotInitialized Code = "not_initialized"
)

// commandPlaceholder stands, in a hint, for the path of the command that failed, such as
// "capcom update"; a failure whose command is unknown fills it with rootCommandPath.
const (
	commandPlaceholder = "capcom <command>"
	rootCommandPath    = "capcom"
)

// defaultHints names, for every Code, the command that most often gets the caller past a failure of
// that class. A failure that sets its own Hint does not use it.
var defaultHints = map[Code]string{
	CodeNotFound:       "capcom search <text>",
	CodeExists:         "capcom show <id>",
	CodeInvalid:        commandPlaceholder + " --help",
	CodeConflict:       "capcom show <id>",
	CodeLocked:         "capcom lock status",
	CodeNotInitialized: "capcom init",
}

// Process exit codes.
const (
	ExitOK       = 0
	ExitError    = 1
	ExitUsage    = 2
	ExitNotFound = 3
	// ExitNothingReady is the exit code of capcom next when no Issue is Ready, and of nothing else.
	ExitNothingReady = 4
)

// Error is a typed command failure. Path and Line, when set, locate the file (and line within it)
// the failure is about; Line is meaningful only together with Path. Suggestion, when set, is the
// closest valid form of what the caller mistyped, such as the command path "know show" below
// capcom; every format reports it apart from Message. Suggestions, when set, are the closest
// known ids or aliases to one the caller named, closest first, and Valid the declared values a
// flag accepts; every format reports each apart from Message and leaves it out when empty. Hint,
// when set, is the command that gets the caller past this failure, and wins over its Code's
// default; known values in it are filled in and unknown ones stay <placeholders>, while
// "capcom <command>" is filled with the failing command's path (WithCommand). Every format reports
// the hint apart from Message.
type Error struct {
	Code        Code
	Message     string
	Path        string
	Line        int
	Suggestion  string
	Suggestions []string
	Valid       []string
	Hint        string
	// commandPath is the path of the command that failed, such as "capcom update"; WithCommand is
	// the one way to set it.
	commandPath string
	isUsage     bool
	// isNothingReady marks the failure of finding no Ready Issue, which exits with
	// ExitNothingReady. NothingReadyErrorf is the one way to set it.
	isNothingReady bool
	// isRendered marks a failure the output layer already wrote with its partial result; Exit
	// only maps it to the exit code. Rendered is the one way to set it.
	isRendered bool
}

// Errorf builds an Error with the given code and a formatted message.
func Errorf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// UsageErrorf builds an invalid-code Error for a malformed invocation (unknown command, bad flag,
// wrong argument count); it exits with ExitUsage instead of ExitError.
func UsageErrorf(format string, args ...any) *Error {
	return &Error{Code: CodeInvalid, Message: fmt.Sprintf(format, args...), isUsage: true}
}

// NothingReadyErrorf builds a not_found Error for a command that found no Ready Issue to act on;
// it exits with ExitNothingReady instead of ExitNotFound.
func NothingReadyErrorf(format string, args ...any) *Error {
	return &Error{Code: CodeNotFound, Message: fmt.Sprintf(format, args...), isNothingReady: true}
}

// Error renders the message prefixed by "path:" or "path:line:" when a location is known.
func (e *Error) Error() string {
	switch {
	case e.Path != "" && e.Line > 0:
		return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Message)
	case e.Path != "":
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	default:
		return e.Message
	}
}

// hintText is the hint every format reports: Hint, or the default of the failure's Code, with
// "capcom <command>" filled by the failing command's path, or by "capcom" when that is unknown. It
// is empty for a Code without a default.
func (e *Error) hintText() string {
	hint := e.Hint
	if hint == "" {
		hint = defaultHints[e.Code]
	}
	commandPath := e.commandPath
	if commandPath == "" {
		commandPath = rootCommandPath
	}
	return strings.ReplaceAll(hint, commandPlaceholder, commandPath)
}

// ExitCode maps the error to the process exit code: usage errors 2, not_found and locked 3, no
// Ready Issue (NothingReadyErrorf) 4, everything else 1.
func (e *Error) ExitCode() int {
	if e.isUsage {
		return ExitUsage
	}
	if e.isNothingReady {
		return ExitNothingReady
	}
	if e.Code == CodeNotFound || e.Code == CodeLocked {
		return ExitNotFound
	}
	return ExitError
}

// Rendered returns a copy of failure marked as already written with its partial result, so Exit
// maps it to its exit code without writing it again. failure itself is left unchanged.
func Rendered(failure *Error) *Error {
	marked := *failure
	marked.isRendered = true
	return &marked
}

// Exit turns a command's final error into the process exit code, writing the failure through
// renderer first. nil gives ExitOK. A failure marked by Rendered keeps its code and is not
// written again. An error that is not an *Error is a malformed invocation cobra rejected before
// any command ran, so it is written and exits with ExitUsage. Any other *Error is written and
// maps through ExitCode. A failure renderer cannot write gives ExitError.
func Exit(renderer Renderer, err error) int {
	if err == nil {
		return ExitOK
	}
	failure := typedFailure(err)
	if failure.isRendered {
		return failure.ExitCode()
	}
	if renderErr := renderer.RenderError(failure); renderErr != nil {
		return ExitError
	}
	return failure.ExitCode()
}

// InCommand returns err as an *Error naming commandPath as the command that failed, as WithCommand
// does; an error that is not an *Error becomes a usage error, as Exit treats it. nil stays nil.
func InCommand(err error, commandPath string) error {
	if err == nil {
		return nil
	}
	return typedFailure(err).WithCommand(commandPath)
}

// WithCommand returns a copy of the failure naming commandPath, such as "capcom update", as the
// command that failed, which fills "capcom <command>" in its hint. A failure that already names
// its command keeps it. The failure itself is left unchanged.
func (e *Error) WithCommand(commandPath string) *Error {
	named := *e
	if named.commandPath == "" {
		named.commandPath = commandPath
	}
	return &named
}

// typedFailure returns err as the *Error Exit writes: an *Error itself, and any other error, a
// malformed invocation cobra rejected before any command ran, as a usage error.
func typedFailure(err error) *Error {
	if failure, isTyped := err.(*Error); isTyped {
		return failure
	}
	return UsageErrorf("%s", err.Error())
}

// AsError returns err as an *Error, giving an untyped failure the invalid code so every failure
// reaches the output layer with a code from the closed set.
func AsError(err error) *Error {
	var typed *Error
	if errors.As(err, &typed) {
		return typed
	}
	return &Error{Code: CodeInvalid, Message: err.Error()}
}

// StoreFailure types a failure to open or create the store: a bad config.yaml is invalid at its
// file and line, a missing store is not_initialized, and a directory outside git is invalid.
func StoreFailure(err error) *Error {
	var configFailure *config.Error
	switch {
	case errors.As(err, &configFailure):
		return &Error{
			Code:    CodeInvalid,
			Message: configFailure.Message,
			Path:    configFailure.Path,
			Line:    configFailure.Line,
		}
	case errors.Is(err, store.ErrNotInitialized):
		return Errorf(CodeNotInitialized, "%s", err.Error())
	default:
		return Errorf(CodeInvalid, "%s", err.Error())
	}
}
