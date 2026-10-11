package issue

import (
	"fmt"
	"slices"

	"github.com/airiclenz/capcom/internal/config"
)

// Validate checks issue against the repository's config and returns every problem it finds, in
// frontmatter order, or nil when there is none. path names the file in each problem, and a
// problem cites the line of the key it concerns when the issue came from Parse.
//
// An issue needs a title, a declared status, a declared type, a priority from MinPriority to
// MaxPriority and a created time, and every label must be non-empty. An issue whose status is in
// the done or dropped category (ADR 0003) also needs a closed time and a reason.
func Validate(path string, issue Issue, cfg config.Config) []*Error {
	v := validator{path: path, issue: issue}

	if issue.Title == "" {
		v.add(keyTitle, "%s is required", keyTitle)
	}
	category, isDeclared := cfg.CategoryOf(issue.Status)
	switch {
	case issue.Status == "":
		v.add(keyStatus, "%s is required; use one of %v", keyStatus, statusNames(cfg))
	case !isDeclared:
		v.add(keyStatus, "status %q is not declared in config; use one of %v", issue.Status, statusNames(cfg))
	}
	switch {
	case issue.Type == "":
		v.add(keyType, "%s is required; use one of %v", keyType, cfg.Types)
	case !slices.Contains(cfg.Types, issue.Type):
		v.add(keyType, "type %q is not declared in config; use one of %v", issue.Type, cfg.Types)
	}
	if issue.Priority < MinPriority || issue.Priority > MaxPriority {
		v.add(
			keyPriority,
			"priority %d is out of range; use %d to %d",
			issue.Priority,
			MinPriority,
			MaxPriority,
		)
	}
	if slices.Contains(issue.Labels, "") {
		v.add(keyLabels, "%s: every entry must be non-empty text", keyLabels)
	}
	if issue.Created.IsZero() {
		v.add(keyCreated, "%s is required", keyCreated)
	}
	if category == config.CategoryDone || category == config.CategoryDropped {
		v.requireClosing(issue.Status, category)
	}
	return v.problems
}

// validator collects the problems of one issue.
type validator struct {
	path     string
	issue    Issue
	problems []*Error
}

// add records a problem with key, citing the key's line when the file had it.
func (v *validator) add(key string, format string, args ...any) {
	v.problems = append(v.problems, &Error{
		Path:    v.path,
		Line:    v.issue.lines[key],
		Message: fmt.Sprintf(format, args...),
	})
}

// requireClosing checks that a closed issue records when and why it was closed. A missing field
// cites the status line, which is what requires it.
func (v *validator) requireClosing(status string, category config.Category) {
	if v.issue.Closed.IsZero() {
		v.add(keyStatus, "status %q is in the %s category, so %s is required", status, category, keyClosed)
	}
	if v.issue.Reason == "" {
		v.add(keyStatus, "status %q is in the %s category, so %s is required", status, category, keyReason)
	}
}

func statusNames(cfg config.Config) []string {
	names := make([]string, 0, len(cfg.Statuses))
	for _, status := range cfg.Statuses {
		names = append(names, status.Name)
	}
	return names
}
