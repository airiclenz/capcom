package ops

import (
	"errors"
	"fmt"
	"slices"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/nodestore"
)

// ErrNoChange refuses an update that names no change (IssueUpdate.IsEmpty).
var ErrNoChange = errors.New("the update changes nothing")

// Replacement replaces the one occurrence of Old in a body with New.
type Replacement struct {
	Old string
	New string
}

// ReplacementError refuses the replacement at Index, counted from 0 in the order given, whose Old
// text is empty or does not occur in the body exactly once; Err wraps edit.ErrReplaceMatch.
type ReplacementError struct {
	Index int
	Old   string
	Err   error
}

// Error names the replacement by its position from 1 and its old text.
func (e *ReplacementError) Error() string {
	return fmt.Sprintf("replacement #%d %q: %v", e.Index+1, e.Old, e.Err)
}

// Unwrap returns the reason, so errors.Is matches edit.ErrReplaceMatch.
func (e *ReplacementError) Unwrap() error {
	return e.Err
}

// IssueUpdate describes the changes UpdateIssue applies to an issue: a nil field, or an empty list,
// leaves its part of the issue as it is.
type IssueUpdate struct {
	// Title is the new title.
	Title *string
	// Status is the new status, outside the closed categories: close and reopen own those moves.
	Status *string
	// Type is the new type.
	Type *string
	// Priority is the new priority.
	Priority *int
	// Plan is the new plan basename; "" clears it.
	Plan *string
	// IsStanding sets the standing mark when true and clears it when false.
	IsStanding *bool
	// AddLabels are labels to add, kept once each; RemoveLabels are removed after.
	AddLabels    []string
	RemoveLabels []string
	// Replacements apply to the body in order, each against the body the ones before it left.
	Replacements []Replacement
	// Appends are added to the end of the body in order, after the replacements.
	Appends []string
	// AppendText returns a text added to the end of the body after Appends; nil adds none. It runs
	// once the value checks have passed and before the reference resolves, so a text read from a
	// file or stdin is read after them; its error returns as it is.
	AppendText func() (string, error)
}

// IsEmpty reports whether u names no change at all.
func (u IssueUpdate) IsEmpty() bool {
	return u.Title == nil && u.Status == nil && u.Type == nil && u.Priority == nil && u.Plan == nil &&
		u.IsStanding == nil && len(u.AddLabels) == 0 && len(u.RemoveLabels) == 0 &&
		len(u.Replacements) == 0 && len(u.Appends) == 0 && u.AppendText == nil
}

// UpdateIssue applies u to the issue ref names (EditIssue). Before anything is read it refuses an
// empty update with ErrNoChange and then a plan, status, type or priority value the issue does not
// accept, in that order, as a *ValueError; both return unwrapped. Then AppendText runs, then ref
// resolves and the changes apply in this order: the standing mark, so a status change is judged on
// the issue as marked or unmarked, the status, title, type, priority and plan, the labels, the
// replacements, the appends and the AppendText text. A status rule's refusal is a *ChangeError
// wrapping the *issue.StatusError, and a replacement's a *ChangeError wrapping a *ReplacementError;
// either way nothing is written.
func (o *Ops) UpdateIssue(ref string, ifMatch string, u IssueUpdate) (nodestore.Written, error) {
	if u.IsEmpty() {
		return nodestore.Written{}, ErrNoChange
	}
	cfg := o.store.Config
	if err := checkUpdateValues(cfg, u); err != nil {
		return nodestore.Written{}, err
	}
	if u.AppendText != nil {
		text, err := u.AppendText()
		if err != nil {
			return nodestore.Written{}, err
		}
		u.Appends = append(slices.Clone(u.Appends), text)
	}
	return o.EditIssue(ref, ifMatch, func(target issue.Issue) (issue.Issue, error) {
		return applyUpdate(cfg, u, target)
	})
}

// EditBody returns body with every replacement applied in order, each against the body the ones
// before it left, then every append in order. A replacement whose old text is empty or does not
// occur exactly once fails as a *ReplacementError, returning no body.
func EditBody(body string, replacements []Replacement, appends []string) (string, error) {
	for index, replacement := range replacements {
		replaced, err := edit.ReplaceBody(body, replacement.Old, replacement.New)
		if err != nil {
			return "", &ReplacementError{Index: index, Old: replacement.Old, Err: err}
		}
		body = replaced
	}
	for _, text := range appends {
		body = edit.AppendBody(body, text)
	}
	return body, nil
}

// checkUpdateValues refuses, as a *ValueError, a plan that is not a bare basename, an undeclared
// status or type and a priority out of range, in that order. A declared status is judged by the
// status rules when the update applies.
func checkUpdateValues(cfg config.Config, u IssueUpdate) error {
	if u.Plan != nil {
		if err := CheckPlan(cfg, *u.Plan); err != nil {
			return err
		}
	}
	if u.Status != nil {
		if err := CheckStatus(cfg, *u.Status); err != nil {
			return err
		}
	}
	if u.Type != nil {
		if err := CheckType(cfg, *u.Type); err != nil {
			return err
		}
	}
	if u.Priority != nil {
		return CheckPriority(*u.Priority)
	}
	return nil
}

// applyUpdate applies the changes of u to target in the order UpdateIssue gives, its Appends
// already holding the AppendText text.
func applyUpdate(cfg config.Config, u IssueUpdate, target issue.Issue) (issue.Issue, error) {
	if u.IsStanding != nil {
		changed, err := issue.SetStanding(target, cfg, *u.IsStanding)
		if err != nil {
			return issue.Issue{}, err
		}
		target = changed
	}
	if u.Status != nil {
		changed, err := issue.SetStatus(target, cfg, *u.Status)
		if err != nil {
			return issue.Issue{}, err
		}
		target = changed
	}
	if u.Title != nil {
		target.Title = *u.Title
	}
	if u.Type != nil {
		target.Type = *u.Type
	}
	if u.Priority != nil {
		target.Priority = *u.Priority
	}
	if u.Plan != nil {
		target.Plan = *u.Plan
	}
	target.Labels = edit.List(target.Labels, u.AddLabels, u.RemoveLabels)
	body, err := EditBody(target.Body, u.Replacements, u.Appends)
	if err != nil {
		return issue.Issue{}, err
	}
	target.Body = body
	return target, nil
}
