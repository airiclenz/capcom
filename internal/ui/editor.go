package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/check"
	"github.com/airiclenz/capcom/internal/index"
	"github.com/airiclenz/capcom/internal/lock"
	"github.com/airiclenz/capcom/internal/store"
)

// keyEdit opens the file of the selected issue or Note in the user's editor.
const keyEdit = "e"

// The environment variables naming the editor e runs, in the order they are tried.
const (
	visualVariable = "VISUAL"
	editorVariable = "EDITOR"
)

// The texts of an edit.
const (
	// noEditorText refuses e while neither $VISUAL nor $EDITOR names an editor.
	noEditorText = "editor: set $VISUAL or $EDITOR to edit files"
	// editorFailurePrefix opens the notice of an editor that could not start or failed.
	editorFailurePrefix = "editor: "
	// checkFailurePrefix opens the notice of a check after an edit that could not read the store.
	checkFailurePrefix = "check: "
	// noProblemsFormat reports a check after an edit of the named file that found nothing wrong.
	noProblemsFormat = "%s: check found no problems"
	// problemsFormat reports a check after an edit that found problems: the one it shows first,
	// then how many it found in all.
	problemsFormat = "check: %s (problems: %d)"
)

// editRequest is one edit e asks for: the file of the selected issue or Note, relative to the
// working tree root with forward slashes as the index records it, and the words of the editor
// command. argv is the command to run, those words with the file's absolute path appended, once
// prepareEdit has resolved it.
type editRequest struct {
	path  string
	words []string
	argv  []string
}

// editPreparedMsg carries an edit ready to run, and the question the Tree lock asks before it;
// question is "" when the lock is free.
type editPreparedMsg struct {
	edit     editRequest
	question string
}

// editedMsg carries the editor's exit: the edit it made, and the error of an editor that could
// not start or exited with a failure.
type editedMsg struct {
	edit editRequest
	err  error
}

// checkedMsg carries the check after an edit: the status bar's report of it, and the error of the
// index refresh that followed it.
type checkedMsg struct {
	notice     string
	refreshErr error
}

// editorWords returns the words of the editor command the environment names through lookup:
// $VISUAL when it holds any, else $EDITOR, split on spaces. It returns nil when neither does.
func editorWords(lookup func(string) string) []string {
	for _, variable := range []string{visualVariable, editorVariable} {
		if words := strings.Fields(lookup(variable)); len(words) > 0 {
			return words
		}
	}
	return nil
}

// execEditor is the model's execFn outside tests: it runs argv through tea.ExecProcess, which
// suspends the UI and hands the terminal to the editor, and restores and redraws the UI once it
// exits, sending done's message.
func execEditor(argv []string, done tea.ExecCallback) tea.Cmd {
	return tea.ExecProcess(exec.Command(argv[0], argv[1:]...), done)
}

// startEdit opens the file of the issue or Note the open tab's sidebar selects in the editor
// $VISUAL, else $EDITOR, names. It does nothing with nothing selected, and shows noEditorText
// when neither names one.
func (m model) startEdit() (model, tea.Cmd) {
	path, isSelected := m.selectedPath()
	if !isSelected {
		return m, nil
	}
	words := editorWords(os.Getenv)
	if words == nil {
		m.writeNotice = noEditorText
		return m, nil
	}
	m.writeNotice = ""
	return m, prepareEdit(m.session, editRequest{path: path, words: words})
}

// selectedPath returns the file of the issue or Note the open tab's sidebar selects, as the index
// records it, and false when it selects none.
func (m model) selectedPath() (string, bool) {
	switch m.tab {
	case TabReady:
		entry, isSelected := m.readySelection()
		return entry.Path, isSelected
	case TabIssues:
		entry, isSelected := m.issuesSelection()
		return entry.Path, isSelected
	case TabKnowledge:
		note, isSelected := m.knowledgeSelection()
		return note.Path, isSelected
	}
	return "", false
}

// prepareEdit returns the command that resolves edit's file to its absolute path in the session's
// working tree and reads this worktree's Tree lock, through the session: while another session
// holds the lock, live or stale, or when its record cannot be read, the edit asks first, as a
// write does.
func prepareEdit(s *session, edit editRequest) tea.Cmd {
	return func() tea.Msg {
		var status lock.Status
		readErr := s.do(func(opened *store.Store, _ *index.Index) error {
			absolute := filepath.Join(opened.WorktreeRoot, filepath.FromSlash(edit.path))
			edit.argv = append(slices.Clone(edit.words), absolute)
			var err error
			status, err = lock.ForWorktree(opened).Read()
			return err
		})
		return editPreparedMsg{edit: edit, question: lockQuestion(status, readErr)}
	}
}

// applyEditPrepared runs a prepared edit's editor, or asks the Tree lock's question first, whose
// yes runs it.
func (m model) applyEditPrepared(msg editPreparedMsg) (model, tea.Cmd) {
	run := m.runEditor(msg.edit)
	if msg.question != "" {
		m.prompt = newConfirmPrompt(msg.question, run)
		return m, nil
	}
	return m, run
}

// runEditor returns the command that runs edit's editor through execFn, which reports its exit as
// an editedMsg.
func (m model) runEditor(edit editRequest) tea.Cmd {
	return m.execFn(edit.argv, func(err error) tea.Msg {
		return editedMsg{edit: edit, err: err}
	})
}

// applyEdited takes in the editor's exit: an editor that failed is reported, and otherwise the
// store is checked and the index refreshed.
func (m model) applyEdited(msg editedMsg) (model, tea.Cmd) {
	if msg.err != nil {
		m.writeNotice = editorFailurePrefix + oneLine(msg.err.Error())
		return m, nil
	}
	return m, checkEdit(m.session, msg.edit)
}

// checkEdit returns the command that runs the capcom check checks on the session's store after
// edit, then refreshes the index, through the session.
func checkEdit(s *session, edit editRequest) tea.Cmd {
	return func() tea.Msg {
		var msg checkedMsg
		var problems []check.Problem
		checkErr := s.do(func(opened *store.Store, issueIndex *index.Index) error {
			var err error
			problems, err = check.Run(opened)
			_, msg.refreshErr = issueIndex.Refresh()
			return err
		})
		if checkErr != nil {
			msg.notice = checkFailurePrefix + oneLine(checkErr.Error())
			return msg
		}
		msg.notice = checkNotice(edit.path, problems)
		return msg
	}
}

// checkNotice returns the status bar's report of problems, the check after an edit of the file at
// path: the first problem in that file, else the first of all, with how many there are.
func checkNotice(path string, problems []check.Problem) string {
	if len(problems) == 0 {
		return fmt.Sprintf(noProblemsFormat, path)
	}
	first := problems[0]
	inEdited := slices.IndexFunc(problems, func(problem check.Problem) bool { return problem.Path == path })
	if inEdited >= 0 {
		first = problems[inEdited]
	}
	return fmt.Sprintf(problemsFormat, oneLine(first.String()), len(problems))
}

// applyChecked takes in the check after an edit: it reports it, with any failed index refresh,
// and reads every tab again, so the edit shows.
func (m model) applyChecked(msg checkedMsg) (model, tea.Cmd) {
	m.writeNotice = withRefreshFailure(msg.notice, msg.refreshErr)
	return m, m.reloadTabs()
}
