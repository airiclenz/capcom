package ops

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/issue"
)

// The fields whose values the checks of this file refuse: each is the issue's frontmatter key,
// which the CLI flag that sets it shares.
const (
	FieldStatus   = "status"
	FieldType     = "type"
	FieldPriority = "priority"
	FieldPlan     = "plan"
)

// The reasons a ValueError gives for a value its field does not accept; a plan's reason is the
// error of config.Config.ValidatePlanName.
var (
	// ErrUndeclaredStatus refuses a status no status of config.yaml declares.
	ErrUndeclaredStatus = errors.New("not a status declared in config.yaml")
	// ErrUndeclaredType refuses a type config.yaml does not declare.
	ErrUndeclaredType = errors.New("not a type declared in config.yaml")
	// ErrPriorityRange refuses a priority outside issue.MinPriority to issue.MaxPriority.
	ErrPriorityRange = errors.New("out of range")
)

// ValueError refuses a value its field does not accept, before anything is read or written.
// Field is one of the Field constants, Value the value as given (a priority in decimal), Valid the
// values the field accepts in the order config.yaml or the range gives them (nil for a plan, whose
// values are not a closed set) and Err the reason.
type ValueError struct {
	Field string
	Value string
	Valid []string
	Err   error
}

// Error names the field, the value and the reason.
func (e *ValueError) Error() string {
	return fmt.Sprintf("%s %q: %v", e.Field, e.Value, e.Err)
}

// Unwrap returns the reason, so errors.Is matches its sentinel.
func (e *ValueError) Unwrap() error {
	return e.Err
}

// CheckStatus refuses a status that no status of cfg declares, a *ValueError listing the declared
// statuses as `name (category)` in file order.
func CheckStatus(cfg config.Config, status string) error {
	if _, isDeclared := cfg.CategoryOf(status); isDeclared {
		return nil
	}
	valid := make([]string, 0, len(cfg.Statuses))
	for _, declared := range cfg.Statuses {
		valid = append(valid, fmt.Sprintf("%s (%s)", declared.Name, declared.Category))
	}
	return &ValueError{Field: FieldStatus, Value: status, Valid: valid, Err: ErrUndeclaredStatus}
}

// CheckType refuses a type that cfg does not declare, a *ValueError listing the declared types.
func CheckType(cfg config.Config, issueType string) error {
	if slices.Contains(cfg.Types, issueType) {
		return nil
	}
	return &ValueError{Field: FieldType, Value: issueType, Valid: slices.Clone(cfg.Types), Err: ErrUndeclaredType}
}

// CheckPriority refuses a priority outside issue.MinPriority to issue.MaxPriority, a *ValueError
// listing every priority in the range.
func CheckPriority(priority int) error {
	if priority >= issue.MinPriority && priority <= issue.MaxPriority {
		return nil
	}
	valid := make([]string, 0, issue.MaxPriority-issue.MinPriority+1)
	for value := issue.MinPriority; value <= issue.MaxPriority; value++ {
		valid = append(valid, strconv.Itoa(value))
	}
	return &ValueError{Field: FieldPriority, Value: strconv.Itoa(priority), Valid: valid, Err: ErrPriorityRange}
}

// CheckPlan refuses a plan that is not a bare basename (config.Config.ValidatePlanName), a
// *ValueError carrying that refusal as its reason. An empty plan sets no plan, or clears one, so
// it is not checked.
func CheckPlan(cfg config.Config, plan string) error {
	if plan == "" {
		return nil
	}
	if err := cfg.ValidatePlanName(plan); err != nil {
		return &ValueError{Field: FieldPlan, Value: plan, Err: err}
	}
	return nil
}
