package cli

import (
	"time"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/elsewhere"
)

// addElsewhere scans the other local and remote-tracking branches for changes to the issue files
// below pathspecs (relative to the worktree root, with forward slashes) and adds what it finds to
// result, its failures as warnings. It scans nothing when isSkipped (--no-elsewhere), when there
// is no pathspec, and in --kv mode, which leaves Elsewhere out.
func (a *app) addElsewhere(result output.Result, isSkipped bool, pathspecs []string) output.Result {
	if isSkipped || a.isKV || len(pathspecs) == 0 {
		return result
	}
	report := elsewhere.Scan(elsewhere.Options{
		WorktreeRoot: a.store.WorktreeRoot,
		Pathspecs:    pathspecs,
		Days:         a.store.Config.Elsewhere.Days,
		Now:          time.Now(),
	})
	result.Elsewhere = report.Entries
	result.Warnings = append(result.Warnings, report.Warnings...)
	return result
}
