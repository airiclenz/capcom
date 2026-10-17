package ui

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/issue"
	"github.com/airiclenz/capcom/internal/lock"
	"github.com/airiclenz/capcom/internal/nodestore"
	"github.com/airiclenz/capcom/internal/ops"
	"github.com/airiclenz/capcom/internal/store"
)

// The keys of the writes on the selected issue besides the status picker's s.
const (
	// keyClaim claims the selected issue.
	keyClaim = "a"
	// keyClose asks for a reason and closes the selected issue.
	keyClose = "c"
	// keyReopen asks for a reason and reopens the selected issue.
	keyReopen = "R"
	// keyLog asks for a line and appends it to the selected issue's Log.
	keyLog = "L"
)

// The texts of a write.
const (
	// lockHeldFormat asks before a write while another session holds this worktree's lock, live
	// or stale.
	lockHeldFormat = "locked by %s, write anyway? (y/n)"
	// lockReadFormat asks before a write when this worktree's lock record could not be read.
	lockReadFormat = "lock unreadable: %s, write anyway? (y/n)"
	// conflictFormat asks, after a write found its issue changed on disk, whether to run it again
	// on the version just read.
	conflictFormat = "changed on disk: %s — run %s again? (y/n)"
	// fileField is what the conflict prompt names when the file changed on disk but no field, nor
	// the body or the Log, did, as a reformat leaves it.
	fileField = "file"
	// claimName names the claim in the conflict prompt, and claimedFormat reports one that landed.
	claimName     = "claim"
	claimedFormat = "claimed %s"
	// statusNameFormat names a status change in the conflict prompt, and statusSetFormat reports
	// one that landed.
	statusNameFormat = "status %s"
	statusSetFormat  = "%s: status %s"
	// closeName names a close in the conflict prompt, closedFormat reports one that landed, and
	// nowReadyFormat adds the issues it made Ready to that report when there are any.
	closeName      = "close"
	closedFormat   = "closed %s"
	nowReadyFormat = "%s; now ready: %s"
	// alreadyClosedFormat refuses the close of an issue that is already closed.
	alreadyClosedFormat = "%s is already closed"
	// standingFormat refuses the claim, the close and a move into the doing category of an issue
	// that carries the standing mark, naming the command that clears it.
	standingFormat = "%s is standing: it only groups its children and is never claimed or closed; " +
		"run capcom update %s --no-standing first"
	// reopenName names a reopen in the conflict prompt, and reopenedFormat reports one that landed.
	reopenName     = "reopen"
	reopenedFormat = "reopened %s"
	// notClosedFormat refuses the reopen of an issue that is not closed.
	notClosedFormat = "%s is not closed; only a closed issue can be reopened"
	// logName names a Log entry in the conflict prompt, and loggedFormat reports one that landed.
	logName      = "log"
	loggedFormat = "%s: log entry added"
	// writeFailurePrefix opens the notice of a write that failed.
	writeFailurePrefix = "write: "
	// unparsedWriteFormat refuses a write on an issue whose file does not parse.
	unparsedWriteFormat = "write: %s does not parse; fix its file first"
	// refreshAfterFormat adds, to the notice of a write, the index refresh after it that failed.
	refreshAfterFormat = "%s (refresh: %s)"
	// reloadAfterFormat adds, to the notice of a conflict, the read of the issue again that
	// failed.
	reloadAfterFormat = "%s (reload: %s)"
)

// writeAction is one write the UI makes on an issue: its name in the conflict prompt, and run,
// which makes it through writer, on the store whose config is cfg, on the issue id names with
// etag as if-match and returns the status bar's report of it.
type writeAction struct {
	name string
	run  func(writer *ops.Ops, cfg config.Config, id string, etag string) (string, error)
}

// claimAction is the claim a makes: the issue moves to the first status of the doing category.
// A standing issue is refused with standingFormat.
func claimAction() writeAction {
	return writeAction{
		name: claimName,
		run: func(writer *ops.Ops, _ config.Config, id string, etag string) (string, error) {
			written, err := writer.Claim(id, etag)
			return fmt.Sprintf(claimedFormat, written.ID), standingRefusal(id, err)
		},
	}
}

// statusAction is the status change the status picker makes: the issue moves to status. A
// standing issue moved into the doing category is refused with standingFormat.
func statusAction(status string) writeAction {
	return writeAction{
		name: fmt.Sprintf(statusNameFormat, status),
		run: func(writer *ops.Ops, _ config.Config, id string, etag string) (string, error) {
			written, err := writer.SetStatus(id, etag, status)
			return fmt.Sprintf(statusSetFormat, written.ID, status), standingRefusal(id, err)
		},
	}
}

// closeAction is the close c makes with reason: the issue moves to the status capcom close uses
// without --status, the first of the first closed category, with now, to the second, as its closed
// time as the CLI stamps it, and its report names the issues the close made Ready. An issue already closed is
// refused with alreadyClosedFormat, and a standing one with standingFormat.
func closeAction(reason string) writeAction {
	return writeAction{
		name: closeName,
		run: func(writer *ops.Ops, cfg config.Config, id string, etag string) (string, error) {
			// A config always declares a done status, so the first closed category has one.
			status, _ := cfg.DefaultStatus(config.ClosedCategories()[0])
			closedAt := time.Now().UTC().Truncate(time.Second)
			closed, err := writer.Close(id, etag, status, reason, closedAt)
			if errors.Is(err, issue.ErrAlreadyClosed) {
				return "", fmt.Errorf(alreadyClosedFormat, id)
			}
			if errors.Is(err, issue.ErrStanding) {
				return "", standingRefusal(id, err)
			}
			notice := fmt.Sprintf(closedFormat, closed.ID)
			if len(closed.NewlyReady) > 0 {
				notice = fmt.Sprintf(nowReadyFormat, notice, strings.Join(closed.NewlyReady, listSeparator))
			}
			return notice, err
		},
	}
}

// standingRefusal returns err, a write's failure on the issue id names, as the standingFormat
// refusal when it is issue.ErrStanding, and unchanged otherwise.
func standingRefusal(id string, err error) error {
	if errors.Is(err, issue.ErrStanding) {
		return fmt.Errorf(standingFormat, id, id)
	}
	return err
}

// reopenAction is the reopen R makes with reason: the issue moves to the status capcom reopen
// uses without --status, the first of the todo category, and the reason goes to its Log. An issue
// that is not closed is refused with notClosedFormat.
func reopenAction(reason string) writeAction {
	return writeAction{
		name: reopenName,
		run: func(writer *ops.Ops, cfg config.Config, id string, etag string) (string, error) {
			// A config always declares a todo status.
			status, _ := cfg.DefaultStatus(config.CategoryTodo)
			written, err := writer.Reopen(id, etag, status, reason, time.Now())
			if errors.Is(err, issue.ErrNotClosed) {
				return "", fmt.Errorf(notClosedFormat, id)
			}
			return fmt.Sprintf(reopenedFormat, written.ID), err
		},
	}
}

// logAction is the Log entry L appends with text, dated now as capcom log dates it.
func logAction(text string) writeAction {
	return writeAction{
		name: logName,
		run: func(writer *ops.Ops, _ config.Config, id string, etag string) (string, error) {
			written, err := writer.AppendLog(id, etag, text, time.Now())
			return fmt.Sprintf(loggedFormat, written.ID), err
		},
	}
}

// pendingWrite is one write on its way: the action, the issue's id, the ETag of the version on
// screen it sends as if-match, that version itself and its parent's id, which a conflict compares
// the file on disk with, and whether the user already said to write despite the Tree lock.
type pendingWrite struct {
	action          writeAction
	id              string
	etag            string
	shown           issue.Issue
	shownParentID   string
	isLockConfirmed bool
}

// writtenMsg carries what one run of a pending write did: the question the Tree lock asks before
// it, with nothing written; the report of a write that landed; the error of one that failed; or
// the issue read again after a conflict. refreshErr is the error of the index refresh that follows
// a write that landed or conflicted.
type writtenMsg struct {
	write      pendingWrite
	question   string
	notice     string
	err        error
	conflict   *conflictRead
	refreshErr error
}

// conflictRead is the issue read again after a write found it changed on disk: the conflict
// itself, the read for the detail pane, the fields that differ from the version on screen, and
// the error of a read that failed.
type conflictRead struct {
	err     error
	detail  detailMsg
	fields  []string
	readErr error
}

// runWrite returns the command that runs write through the session, as pendingWrite.land does.
func runWrite(s *session, write pendingWrite) tea.Cmd {
	return func() tea.Msg {
		msg := writtenMsg{write: write}
		msg.err = s.do(func(opened *store.Store, issueIndex *index.Index) error {
			msg = write.land(opened, issueIndex)
			return msg.err
		})
		return msg
	}
}

// land runs the write once, the caller holding the session's mutex. Unless the user already said
// to write anyway, it first reads this worktree's Tree lock: while another session holds it, live
// or stale, or when its record cannot be read, it returns the question to ask and writes nothing.
// Otherwise it makes the write through ops and, when it landed or conflicted, refreshes the index;
// a conflict then reads the issue again.
func (w pendingWrite) land(opened *store.Store, issueIndex *index.Index) writtenMsg {
	msg := writtenMsg{write: w}
	if !w.isLockConfirmed {
		msg.question = lockQuestion(lock.ForWorktree(opened).Read())
		if msg.question != "" {
			return msg
		}
	}
	notice, err := w.action.run(ops.New(opened, issueIndex), opened.Config, w.id, w.etag)
	var conflict *nodestore.ConflictError
	isConflict := errors.As(err, &conflict)
	if err != nil && !isConflict {
		msg.err = err
		return msg
	}
	_, msg.refreshErr = issueIndex.Refresh()
	if isConflict {
		msg.conflict = w.readAgain(opened, issueIndex, err)
		return msg
	}
	msg.notice = notice
	return msg
}

// lockQuestion returns the question a write asks before it goes ahead, given a read of this
// worktree's Tree lock, and "" when the lock is free. The UI holds no token, so a held lock, live
// or stale, is always another session's.
func lockQuestion(status lock.Status, err error) string {
	if err != nil {
		return fmt.Sprintf(lockReadFormat, oneLine(err.Error()))
	}
	if status.State() == lock.StateFree {
		return ""
	}
	return fmt.Sprintf(lockHeldFormat, oneLine(status.Record.Purpose))
}

// readAgain reads the issue of a write that conflictErr refused, as the detail pane reads it,
// and the fields that differ from the version the write was made on. The caller holds the
// session's mutex and has refreshed the index.
func (w pendingWrite) readAgain(opened *store.Store, issueIndex *index.Index, conflictErr error) *conflictRead {
	read := &conflictRead{err: conflictErr}
	entry, err := issueIndex.Resolve(w.id)
	if err != nil {
		read.readErr = err
		return read
	}
	read.detail, read.readErr = readIssueDetail(opened, issueIndex, entry)
	if read.readErr == nil && read.detail.etag != "" {
		read.fields = changedFields(w.shown, w.shownParentID, read.detail.node, read.detail.parentID)
	}
	return read
}

// changedFields returns what differs between shown, the version of an issue on screen with its
// parent's id, and onDisk, the file read again with its own: every frontmatter field, by its key,
// then the body outside the Log and the Log, in that order. When nothing differs, as after a
// reformat, it names the file.
func changedFields(shown issue.Issue, shownParentID string, onDisk issue.Issue, diskParentID string) []string {
	shownRest, shownLog := shown.SplitLog()
	diskRest, diskLog := onDisk.SplitLog()
	checks := []struct {
		name      string
		isChanged bool
	}{
		{"title", shown.Title != onDisk.Title},
		{"status", shown.Status != onDisk.Status},
		{"type", shown.Type != onDisk.Type},
		{"priority", shown.Priority != onDisk.Priority},
		{"labels", !slices.Equal(shown.Labels, onDisk.Labels)},
		{"parent", shownParentID != diskParentID},
		{"deps", !slices.Equal(shown.Deps, onDisk.Deps)},
		{"related", !slices.Equal(shown.Related, onDisk.Related)},
		{"plan", shown.Plan != onDisk.Plan},
		{"standing", shown.IsStanding != onDisk.IsStanding},
		{"aliases", !slices.Equal(shown.Aliases, onDisk.Aliases)},
		{"created", !shown.Created.Equal(onDisk.Created)},
		{"closed", !shown.Closed.Equal(onDisk.Closed)},
		{"reason", shown.Reason != onDisk.Reason},
		{"body", shownRest != diskRest},
		{"log", shownLog != diskLog},
	}
	fields := make([]string, 0, len(checks))
	for _, check := range checks {
		if check.isChanged {
			fields = append(fields, check.name)
		}
	}
	if len(fields) == 0 {
		return []string{fileField}
	}
	return fields
}

// startWrite runs action on the issue the detail pane shows, with the ETag it was read with. It
// does nothing on the Knowledge tab or before the selected issue has been read, and refuses an
// issue whose file does not parse with a notice.
func (m model) startWrite(action writeAction) (model, tea.Cmd) {
	write, notice, isReady := m.shownWrite(action)
	m.writeNotice = notice
	if !isReady {
		return m, nil
	}
	return m, runWrite(m.session, write)
}

// shownWrite returns the write of action on the issue the detail pane shows, and false when there
// is none to make: on the Knowledge tab, before the selected issue has been read, and, with the
// notice to show, for an issue whose file does not parse.
func (m model) shownWrite(action writeAction) (pendingWrite, string, bool) {
	if m.tab == TabKnowledge {
		return pendingWrite{}, "", false
	}
	id, isSelected := m.sidebarSelection()
	if !isSelected || m.issue.id != id {
		return pendingWrite{}, "", false
	}
	if m.issue.etag == "" {
		return pendingWrite{}, fmt.Sprintf(unparsedWriteFormat, id), false
	}
	write := pendingWrite{
		action:        action,
		id:            id,
		etag:          m.issue.etag,
		shown:         m.issue.node,
		shownParentID: m.issue.parentID,
	}
	return write, "", true
}

// applyWritten takes in one run of a write: it asks the Tree lock's question, asks to run the
// write again after a conflict, reports a failure, or reports a write that landed and reads every
// tab again.
func (m model) applyWritten(msg writtenMsg) (model, tea.Cmd) {
	switch {
	case msg.question != "":
		write := msg.write
		write.isLockConfirmed = true
		m.prompt = newConfirmPrompt(msg.question, runWrite(m.session, write))
		return m, nil
	case msg.conflict != nil:
		return m.applyConflict(msg)
	case msg.err != nil:
		m.writeNotice = writeFailurePrefix + oneLine(msg.err.Error())
		return m, nil
	}
	m.writeNotice = withRefreshFailure(msg.notice, msg.refreshErr)
	return m, m.reloadTabs()
}

// applyConflict takes in a write refused because its issue changed on disk: it shows the issue as
// read again and asks whether to run the write again on that version, naming what changed, and
// reads every tab again. An issue that could not be read again, or no longer parses, only reports
// the conflict.
func (m model) applyConflict(msg writtenMsg) (model, tea.Cmd) {
	conflict := msg.conflict
	failure := writeFailurePrefix + oneLine(conflict.err.Error())
	if conflict.readErr != nil {
		m.writeNotice = fmt.Sprintf(reloadAfterFormat, failure, oneLine(conflict.readErr.Error()))
		return m, m.reloadTabs()
	}
	m = m.applyDetail(conflict.detail).paintDeps()
	if conflict.detail.etag == "" {
		m.writeNotice = withRefreshFailure(failure, msg.refreshErr)
		return m, m.reloadTabs()
	}

	write := msg.write
	write.etag = conflict.detail.etag
	write.shown = conflict.detail.node
	write.shownParentID = conflict.detail.parentID
	question := fmt.Sprintf(conflictFormat, strings.Join(conflict.fields, listSeparator), write.action.name)
	m.prompt = newConfirmPrompt(question, runWrite(m.session, write))
	m.writeNotice = withRefreshFailure("", msg.refreshErr)
	return m, m.reloadTabs()
}

// withRefreshFailure adds the error of the index refresh after a write to its notice, or makes it
// the notice when there is none; a nil err leaves the notice as it is.
func withRefreshFailure(notice string, err error) string {
	switch {
	case err == nil:
		return notice
	case notice == "":
		return refreshFailurePrefix + oneLine(err.Error())
	}
	return fmt.Sprintf(refreshAfterFormat, notice, oneLine(err.Error()))
}
