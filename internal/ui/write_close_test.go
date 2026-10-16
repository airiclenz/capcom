package ui

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/ops"
)

// The files of readyFiles' issues the close, reopen and log tests write.
const (
	betaPath     = "issues/beta.md"
	finishedPath = "issues/finished.md"
)

// closedLine matches the closed time in an issue's frontmatter, which a close stamps with the
// clock.
var closedLine = regexp.MustCompile(`(?m)^closed: .*$`)

// wantWrite returns the file at path of a twin of readyFiles' store after write changed it
// through ops, as the matching CLI command changes it.
func wantWrite(t *testing.T, path string, write func(writer *ops.Ops) error) []byte {
	t.Helper()
	twin := newTestStore(t, readyFiles)
	if err := write(ops.New(twin.store, twin.index)); err != nil {
		t.Fatalf("twin write: %v", err)
	}
	return readStoreFile(t, twin, path)
}

// withoutClosedTime returns content with its closed time blanked, so two closes stamped at
// different seconds compare equal.
func withoutClosedTime(content []byte) []byte {
	return closedLine.ReplaceAll(content, []byte("closed: <time>"))
}

// submitPrompt presses key, types text into the prompt it opens and submits it with Enter.
func submitPrompt(t *testing.T, m model, key rune, text string) model {
	t.Helper()
	m = typeKeys(t, press(t, m, runeKey(key)), text)
	return press(t, m, specialKey(tea.KeyEnter))
}

// newClosedSelectedModel returns a model over ts on the Issues tab, filtered to status:done so the
// closed issue finished is on screen, with finished selected.
func newClosedSelectedModel(t *testing.T, ts *testStore) model {
	t.Helper()
	m := applyFilterText(t, newIssuesModel(t, ts, "", 120, 30), "status:done")
	if got := selectedIssueID(m); got != "finished" {
		t.Fatalf("selected after status:done = %q, want finished", got)
	}
	return m
}

func TestWriteClose_C_ClosesAsCloseDoes(t *testing.T) {
	cases := []struct {
		name       string
		moves      []tea.KeyPressMsg
		id         string
		path       string
		wantNotice string
	}{
		{name: "no issue made ready", id: "alpha", path: alphaPath, wantNotice: "closed alpha · "},
		{
			name:       "names the newly ready issue",
			moves:      []tea.KeyPressMsg{runeKey('j')},
			id:         "beta",
			path:       betaPath,
			wantNotice: "closed beta; now ready: gamma · ",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := wantWrite(t, tc.path, func(writer *ops.Ops) error {
				_, err := writer.Close(tc.id, "", "done", "fixed in 3f2a1c9", time.Now())
				return err
			})
			ts := newTestStore(t, readyFiles)
			m := pressKeys(t, newTestModel(t, ts, 120, 30), tc.moves...)

			m = submitPrompt(t, m, 'c', "fixed in 3f2a1c9")

			if m.textPrompt.isOpen || m.prompt.isOpen {
				t.Errorf("a prompt is still open after the close: %q", statusRow(m))
			}
			if got := readStoreFile(t, ts, tc.path); !bytes.Equal(withoutClosedTime(got), withoutClosedTime(want)) {
				t.Errorf("%s after c:\n%s\nwant:\n%s", tc.id, got, want)
			}
			if got := statusRow(m); !strings.Contains(got, " · "+tc.wantNotice) {
				t.Errorf("status bar = %q, want it to report %q", got, strings.TrimSuffix(tc.wantNotice, " · "))
			}
		})
	}
}

func TestWriteClose_ReasonPrompt_MatchesGolden(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), 120, 30)

	m = typeKeys(t, press(t, m, runeKey('c')), "fixed in 3f2a1c9")

	golden(t, "close-reason-120x30", m.View())
}

func TestWriteClose_R_ReopensAClosedIssueAsReopenDoes(t *testing.T) {
	want := wantWrite(t, finishedPath, func(writer *ops.Ops) error {
		_, err := writer.Reopen("finished", "", "open", "came back on Safari", time.Now())
		return err
	})
	ts := newTestStore(t, readyFiles)
	m := newClosedSelectedModel(t, ts)

	m = submitPrompt(t, m, 'R', "came back on Safari")

	if got := readStoreFile(t, ts, finishedPath); !bytes.Equal(got, want) {
		t.Errorf("finished after R:\n%s\nwant:\n%s", got, want)
	}
	if got := statusRow(m); !strings.Contains(got, "reopened finished") {
		t.Errorf("status bar = %q, want it to report reopened finished", got)
	}
}

func TestWriteClose_L_AppendsAsLogDoes(t *testing.T) {
	want := wantWrite(t, alphaPath, func(writer *ops.Ops) error {
		_, err := writer.AppendLog("alpha", "", "tried the retry path", time.Now())
		return err
	})
	ts := newTestStore(t, readyFiles)
	m := newTestModel(t, ts, 120, 30)

	m = submitPrompt(t, m, 'L', "tried the retry path")

	if got := readStoreFile(t, ts, alphaPath); !bytes.Equal(got, want) {
		t.Errorf("alpha after L:\n%s\nwant:\n%s", got, want)
	}
	if got := statusRow(m); !strings.Contains(got, "alpha: log entry added") {
		t.Errorf("status bar = %q, want it to report the log entry", got)
	}
}

func TestWriteClose_EscOnEachPrompt_WritesNothing(t *testing.T) {
	for _, key := range []rune{'c', 'R', 'L'} {
		t.Run(string(key), func(t *testing.T) {
			ts := newTestStore(t, readyFiles)
			before := readStoreFile(t, ts, alphaPath)
			m := newTestModel(t, ts, 120, 30)

			m = typeKeys(t, press(t, m, runeKey(key)), "some text")
			m = press(t, m, specialKey(tea.KeyEscape))

			if m.textPrompt.isOpen {
				t.Errorf("prompt %q still open after Esc", statusRow(m))
			}
			if got := readStoreFile(t, ts, alphaPath); !bytes.Equal(got, before) {
				t.Errorf("alpha after %c and Esc:\n%s\nwant it unchanged", key, got)
			}
		})
	}
}

func TestWriteClose_Refusals_ShowTheMessageAndWriteNothing(t *testing.T) {
	cases := []struct {
		name       string
		isClosed   bool
		key        rune
		text       string
		path       string
		wantNotice string
	}{
		{name: "blank reason", key: 'c', text: "   ", path: alphaPath, wantNotice: "write: the reason is empty; say why"},
		{name: "blank reopen reason", isClosed: true, key: 'R', text: "", path: finishedPath, wantNotice: "write: the reason is empty; say why"},
		{name: "blank log line", key: 'L', text: " ", path: alphaPath, wantNotice: "write: the log text is empty"},
		{name: "close of a closed issue", isClosed: true, key: 'c', text: "again", path: finishedPath, wantNotice: "write: finished is already closed"},
		{
			name:       "reopen of an open issue",
			key:        'R',
			text:       "why not",
			path:       alphaPath,
			wantNotice: "write: alpha is not closed; only a closed issue can be reopened",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := newTestStore(t, readyFiles)
			before := readStoreFile(t, ts, tc.path)
			var m model
			if tc.isClosed {
				m = newClosedSelectedModel(t, ts)
			} else {
				m = newTestModel(t, ts, 120, 30)
			}

			m = submitPrompt(t, m, tc.key, tc.text)

			if m.writeNotice != tc.wantNotice {
				t.Errorf("notice = %q, want %q", m.writeNotice, tc.wantNotice)
			}
			if got := readStoreFile(t, ts, tc.path); !bytes.Equal(got, before) {
				t.Errorf("%s after the refusal:\n%s\nwant it unchanged", tc.path, got)
			}
		})
	}
}

func TestWriteClose_L_FileChangedOnDisk_AsksAndYRunsAgain(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	m := newTestModel(t, ts, 120, 30)
	editStoreFile(t, ts, alphaPath, "title: Alpha task\n", "title: Alpha task renamed\n")
	edited := readStoreFile(t, ts, alphaPath)

	m = submitPrompt(t, m, 'L', "tried the retry path")

	if want := "changed on disk: title — run log again? (y/n)"; m.prompt.text != want {
		t.Errorf("prompt = %q, want %q", m.prompt.text, want)
	}
	if got := readStoreFile(t, ts, alphaPath); !bytes.Equal(got, edited) {
		t.Errorf("alpha after a refused L:\n%s\nwant the edit unchanged", got)
	}
	m = press(t, m, runeKey('y'))
	got := string(readStoreFile(t, ts, alphaPath))
	if !strings.Contains(got, "Test Author: tried the retry path") || !strings.Contains(got, "title: Alpha task renamed") {
		t.Errorf("alpha after y:\n%s\nwant the edit with the log entry", got)
	}
}

func TestWriteClose_L_HeldLock_Asks(t *testing.T) {
	answers := []struct {
		name      string
		key       tea.KeyPressMsg
		isWritten bool
	}{
		{name: "y", key: runeKey('y'), isWritten: true},
		{name: "n", key: runeKey('n'), isWritten: false},
	}
	for _, answer := range answers {
		t.Run(answer.name, func(t *testing.T) {
			ts := newTestStore(t, readyFiles)
			acquireTestLock(t, ts)
			before := readStoreFile(t, ts, alphaPath)
			m := newTestModel(t, ts, 120, 30)

			m = submitPrompt(t, m, 'L', "tried the retry path")
			if got := statusRow(m); got != "locked by refactor the parser, write anyway? (y/n)" {
				t.Fatalf("status bar = %q, want the lock question", got)
			}
			m = press(t, m, answer.key)

			isWritten := !bytes.Equal(readStoreFile(t, ts, alphaPath), before)
			if isWritten != answer.isWritten {
				t.Errorf("alpha written after %s = %v, want %v", answer.name, isWritten, answer.isWritten)
			}
		})
	}
}
