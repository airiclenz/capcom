package cli

import (
	"io"
	"time"

	"github.com/airiclenz/capcom/internal/cli/output"
	"github.com/airiclenz/capcom/internal/elsewhere"
)

// SchemaVersion is the temporary alias of output.SchemaVersion.
const SchemaVersion = output.SchemaVersion

// The temporary aliases of the output kind and --kv key the cli package names.
const (
	kindError    = output.KindError
	kvKeyMessage = output.KVKeyMessage
)

// Result is the temporary alias of output.Result.
type Result = output.Result

// KVRecord is the temporary alias of output.KVRecord.
type KVRecord = output.KVRecord

// KVPair is the temporary alias of output.KVPair.
type KVPair = output.KVPair

// Envelope is the temporary alias of output.Envelope.
type Envelope = output.Envelope

// ErrorBody is the temporary alias of output.ErrorBody.
type ErrorBody = output.ErrorBody

// Renderer is the temporary alias of output.Renderer.
type Renderer = output.Renderer

// outputFormat is the temporary alias of output.OutputFormat.
type outputFormat = output.OutputFormat

// The temporary aliases of the output formats.
const (
	formatText = output.FormatText
	formatJSON = output.FormatJSON
	formatKV   = output.FormatKV
)

// newRenderer is the temporary wrapper of output.NewRenderer.
func newRenderer(format outputFormat, stdout, stderr io.Writer) Renderer {
	return output.NewRenderer(format, stdout, stderr)
}

// kvLineBreaks is the temporary alias of output.KVLineBreaks.
var kvLineBreaks = output.KVLineBreaks

// addElsewhere scans the other local and remote-tracking branches for changes to the issue files
// below pathspecs (relative to the worktree root, with forward slashes) and adds what it finds to
// result, its failures as warnings. It scans nothing when isSkipped (--no-elsewhere), when there
// is no pathspec, and in --kv mode, which leaves Elsewhere out.
func (a *app) addElsewhere(result Result, isSkipped bool, pathspecs []string) Result {
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

// issuesPathspec is the pathspec of the whole issue tree, or none when the store lies outside the
// worktree.
func (a *app) issuesPathspec() []string {
	relative := relativeSlashPath(a.store.WorktreeRoot, a.store.IssuesDir())
	if relative == "" {
		return nil
	}
	return []string{relative}
}
