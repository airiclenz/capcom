package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/airiclenz/capcom/internal/lock"
	"github.com/airiclenz/capcom/internal/ops"
)

// alphaPath is the file of the issue the Ready tab of readyFiles selects first.
const alphaPath = "issues/alpha.md"

// readStoreFile returns the content of the store file at name, relative to .capcom.
func readStoreFile(t *testing.T, ts *testStore, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(ts.store.Root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("ReadFile %s: %v", name, err)
	}
	return content
}

// editStoreFile replaces old with new in the store file at name, as another session editing it
// would, failing when the file does not hold old.
func editStoreFile(t *testing.T, ts *testStore, name, old, new string) {
	t.Helper()
	content := string(readStoreFile(t, ts, name))
	if !strings.Contains(content, old) {
		t.Fatalf("%s holds no %q", name, old)
	}
	path := filepath.Join(ts.store.Root, filepath.FromSlash(name))
	if err := os.WriteFile(path, []byte(strings.Replace(content, old, new, 1)), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", name, err)
	}
}

// wantStatusWrite returns alpha's file as capcom update alpha --status <status> writes it: a twin
// of readyFiles' store, changed through ops as the CLI changes it.
func wantStatusWrite(t *testing.T, status string) []byte {
	t.Helper()
	twin := newTestStore(t, readyFiles)
	if _, err := ops.New(twin.store, twin.index).SetStatus("alpha", "", status); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	return readStoreFile(t, twin, alphaPath)
}

func TestWrite_Claim_WritesTheFileAsUpdateDoes(t *testing.T) {
	want := wantStatusWrite(t, "active")
	ts := newTestStore(t, readyFiles)
	m := newTestModel(t, ts, 120, 30)

	m = press(t, m, runeKey('a'))

	if m.prompt.isOpen {
		t.Errorf("prompt %q opened over a free lock, want none", m.prompt.text)
	}
	if got := readStoreFile(t, ts, alphaPath); !bytes.Equal(got, want) {
		t.Errorf("alpha after a:\n%s\nwant:\n%s", got, want)
	}
	if got := statusRow(m); !strings.Contains(got, "claimed alpha") {
		t.Errorf("status bar = %q, want it to report claimed alpha", got)
	}
}

func TestWrite_StatusPicker_SetsThePickedStatus(t *testing.T) {
	want := wantStatusWrite(t, "parked")
	ts := newTestStore(t, readyFiles)
	m := newTestModel(t, ts, 120, 30)

	m = press(t, m, runeKey('s'))
	golden(t, "status-picker-120x30", m.View())
	m = press(t, m, runeKey('j'))
	m = press(t, m, runeKey('j'))
	m = press(t, m, specialKey(tea.KeyEnter))

	if m.picker.isOpen {
		t.Error("picker still open after Enter")
	}
	if got := readStoreFile(t, ts, alphaPath); !bytes.Equal(got, want) {
		t.Errorf("alpha after s j j Enter:\n%s\nwant:\n%s", got, want)
	}
}

func TestWrite_StatusPicker_OffersOnlyOpenStatuses(t *testing.T) {
	m := newTestModel(t, newTestStore(t, readyFiles), 120, 30)

	m = press(t, m, runeKey('s'))

	if got := strings.Join(m.picker.statuses, " "); got != "open active parked" {
		t.Errorf("picker statuses = %q, want open active parked", got)
	}
	if row := m.picker.cursor.highlight(len(m.picker.statuses)); row != 0 {
		t.Errorf("picker cursor = %d, want 0 on alpha's status open", row)
	}
}

func TestWrite_StatusPicker_EscWritesNothing(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	before := readStoreFile(t, ts, alphaPath)
	m := newTestModel(t, ts, 120, 30)

	m = press(t, m, runeKey('s'))
	m = press(t, m, runeKey('j'))
	m = press(t, m, specialKey(tea.KeyEscape))

	if m.picker.isOpen {
		t.Error("picker still open after Esc")
	}
	if got := readStoreFile(t, ts, alphaPath); !bytes.Equal(got, before) {
		t.Errorf("alpha after s j Esc:\n%s\nwant it unchanged", got)
	}
}

func TestWrite_FileChangedOnDisk_AsksAndYRunsAgain(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	m := newTestModel(t, ts, 120, 30)
	editStoreFile(t, ts, alphaPath, "title: Alpha task\nstatus: open\npriority: 1\n",
		"title: Alpha task renamed\nstatus: open\npriority: 0\n")
	edited := readStoreFile(t, ts, alphaPath)

	m = press(t, m, runeKey('a'))

	golden(t, "conflict-120x30", m.View())
	if got := readStoreFile(t, ts, alphaPath); !bytes.Equal(got, edited) {
		t.Errorf("alpha after a refused write:\n%s\nwant the edit unchanged", got)
	}
	m = press(t, m, runeKey('y'))
	if m.prompt.isOpen {
		t.Errorf("prompt %q still open after y", m.prompt.text)
	}
	got := string(readStoreFile(t, ts, alphaPath))
	if !strings.Contains(got, "status: active") || !strings.Contains(got, "title: Alpha task renamed") {
		t.Errorf("alpha after y:\n%s\nwant the edit with status active", got)
	}
}

func TestWrite_TypeOnlyChange_NamesType(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	m := newTestModel(t, ts, 120, 30)
	editStoreFile(t, ts, alphaPath, "type: task\n", "type: bug\n")

	m = press(t, m, runeKey('a'))

	golden(t, "conflict-type-120x30", m.View())
}

func TestWrite_ReformatOnly_NamesTheFile(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	m := newTestModel(t, ts, 120, 30)
	editStoreFile(t, ts, alphaPath, "title: Alpha task\nstatus: open\n", "status: open\ntitle: Alpha task\n")

	m = press(t, m, runeKey('a'))

	want := "changed on disk: file — run claim again? (y/n)"
	if m.prompt.text != want {
		t.Errorf("prompt = %q, want %q", m.prompt.text, want)
	}
}

func TestWrite_LogChanged_NamesLogNotBody(t *testing.T) {
	files := map[string]string{
		alphaPath: issueFile("title: Alpha task\nstatus: open\n", "Body.\n\n## Log\n\n- 2026-10-11 bob: one\n"),
	}
	ts := newTestStore(t, files)
	m := newTestModel(t, ts, 120, 30)
	editStoreFile(t, ts, alphaPath, "bob: one\n", "bob: one\n- 2026-10-12 eve: two\n")

	m = press(t, m, runeKey('s'))
	m = press(t, m, runeKey('j'))
	m = press(t, m, specialKey(tea.KeyEnter))

	want := "changed on disk: log — run status active again? (y/n)"
	if m.prompt.text != want {
		t.Errorf("prompt = %q, want %q", m.prompt.text, want)
	}
}

func TestWrite_HeldLock_AsksBeforeWriting(t *testing.T) {
	locks := []struct {
		name    string
		arrange func(t *testing.T, ts *testStore)
	}{
		{name: "held", arrange: acquireTestLock},
		{name: "stale", arrange: func(t *testing.T, ts *testStore) {
			acquireTestLock(t, ts)
			ageTestLock(t, ts, 3*time.Hour)
		}},
	}
	answers := []struct {
		name      string
		key       tea.KeyPressMsg
		isWritten bool
	}{
		{name: "y", key: runeKey('y'), isWritten: true},
		{name: "enter", key: specialKey(tea.KeyEnter), isWritten: true},
		{name: "n", key: runeKey('n'), isWritten: false},
		{name: "esc", key: specialKey(tea.KeyEscape), isWritten: false},
	}
	for _, held := range locks {
		for _, answer := range answers {
			t.Run(held.name+"/"+answer.name, func(t *testing.T) {
				ts := newTestStore(t, readyFiles)
				held.arrange(t, ts)
				before := readStoreFile(t, ts, alphaPath)
				m := newTestModel(t, ts, 120, 30)

				m = press(t, m, runeKey('a'))
				if got := statusRow(m); got != "locked by refactor the parser, write anyway? (y/n)" {
					t.Fatalf("status bar = %q, want the lock question", got)
				}
				m = press(t, m, answer.key)

				if m.prompt.isOpen {
					t.Errorf("prompt still open after %s", answer.name)
				}
				isWritten := !bytes.Equal(readStoreFile(t, ts, alphaPath), before)
				if isWritten != answer.isWritten {
					t.Errorf("alpha written after %s = %v, want %v", answer.name, isWritten, answer.isWritten)
				}
			})
		}
	}
}

func TestWrite_InvalidLockRecord_AsksWithTheReadError(t *testing.T) {
	ts := newTestStore(t, readyFiles)
	writeTestLockRecord(t, ts, []byte("not json"))
	_, readErr := lock.ForWorktree(ts.store).Read()
	if readErr == nil {
		t.Fatal("Read of an invalid lock record succeeded, want an error")
	}
	m := newTestModel(t, ts, 120, 30)

	m = press(t, m, runeKey('a'))

	want := "lock unreadable: " + oneLine(readErr.Error()) + ", write anyway? (y/n)"
	if m.prompt.text != want {
		t.Errorf("prompt = %q, want %q", m.prompt.text, want)
	}
	m = press(t, m, runeKey('y'))
	if got := string(readStoreFile(t, ts, alphaPath)); !strings.Contains(got, "status: active") {
		t.Errorf("alpha after y:\n%s\nwant status active", got)
	}
}

func TestWrite_KnowledgeTab_WritesNothing(t *testing.T) {
	files := map[string]string{"knowledge/note.md": "---\ntitle: A Note\n---\nBody.\n"}
	ts := newTestStore(t, files)
	options := newTestOptions(t, ts)
	options.Tab = TabKnowledge
	m := startModel(t, options, 120, 30)

	m = press(t, m, runeKey('a'))
	m = press(t, m, runeKey('s'))

	if m.prompt.isOpen || m.picker.isOpen || m.writeNotice != "" {
		t.Errorf("a and s on the Knowledge tab: prompt %v, picker %v, notice %q; want nothing",
			m.prompt.isOpen, m.picker.isOpen, m.writeNotice)
	}
}

// lockCallPattern matches a call that takes, renews or frees the Tree lock.
var lockCallPattern = regexp.MustCompile(`\.(Acquire|Touch|Release)\(`)

func TestUIPackage_NeverTakesTheTreeLock(t *testing.T) {
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		content, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		if match := lockCallPattern.Find(content); match != nil {
			t.Errorf("%s calls %s: the UI never acquires, touches or releases the lock", name, match)
		}
	}
}
