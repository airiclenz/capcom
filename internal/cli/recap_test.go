package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// recapRecent is the text recap prints for the second commit of newRecapRepo against the first.
const recapRecent = "closed: 1\n  old-task  Old task: shipped\n" +
	"filed: 1\n  new-task  New task\n"

// recapNotesAdded is the Notes added group of the second commit of newRecapRepo against the first.
const recapNotesAdded = "notes added: 1\n  new-note  New note\n"

// newRecapRepo returns a store with two commits: 30 days ago an Issue old-task and a Note old-note,
// one day ago a Log entry on old-task, old-task closed with reason shipped, an Issue new-task and a
// Note new-note.
func newRecapRepo(t *testing.T) string {
	t.Helper()
	root := newGitRepo(t)
	now := time.Now()
	mustRun(t, root, "init")
	mustRun(t, root, "create", "--id", "old-task", "--title", "Old task")
	mustRun(t, root, "know", "add", "--id", "old-note", "--title", "Old note")
	commitAt(t, root, now.AddDate(0, 0, -30), "base")

	mustRun(t, root, "log", "old-task", "made progress")
	mustRun(t, root, "close", "old-task", "--reason", "shipped")
	mustRun(t, root, "create", "--id", "new-task", "--title", "New task")
	mustRun(t, root, "know", "add", "--id", "new-note", "--title", "New note")
	commitAt(t, root, now.AddDate(0, 0, -1), "recent")
	return root
}

// commitAt commits every change in root with author and committer time at.
func commitAt(t *testing.T, root string, at time.Time, message string) {
	t.Helper()
	date := fmt.Sprintf("%d +0000", at.Unix())
	t.Setenv("GIT_AUTHOR_DATE", date)
	t.Setenv("GIT_COMMITTER_DATE", date)
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", message)
}

func TestRecap_EachSinceForm_ListsTheChangesAfterTheBoundary(t *testing.T) {
	root := newRecapRepo(t)
	tests := []struct {
		name string
		args []string
	}{
		{name: "default", args: []string{"recap"}},
		{name: "days", args: []string{"recap", "--since", "7d"}},
		{name: "date", args: []string{"recap", "--since", time.Now().AddDate(0, 0, -10).Format(time.DateOnly)}},
		{name: "ref", args: []string{"recap", "--since", "HEAD~1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout := mustRun(t, root, test.args...)

			if !strings.HasPrefix(stdout, recapRecent) || !strings.Contains(stdout, recapNotesAdded) {
				t.Errorf("stdout = %q, want it to start %q and hold %q", stdout, recapRecent, recapNotesAdded)
			}
			if !strings.Contains(stdout, "log: ") || !strings.Contains(stdout, "made progress\n") {
				t.Errorf("stdout = %q, want a log group holding the entry made progress", stdout)
			}
		})
	}
}

func TestRecap_SinceBeforeTheFirstCommit_ListsEverythingAsNew(t *testing.T) {
	root := newRecapRepo(t)

	stdout := mustRun(t, root, "recap", "--since", "40d")

	wantFiled := "filed: 2\n  new-task  New task\n  old-task  Old task\n"
	wantNotes := "notes added: 2\n  new-note  New note\n  old-note  Old note\n"
	if !strings.Contains(stdout, wantFiled) || !strings.Contains(stdout, wantNotes) {
		t.Errorf("stdout = %q, want it to hold %q and %q", stdout, wantFiled, wantNotes)
	}
}

func TestRecap_NothingChanged_SaysSinceWhen(t *testing.T) {
	root := newRecapRepo(t)

	stdout := mustRun(t, root, "recap", "--since", "HEAD")

	if want := "nothing changed since HEAD\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestRecap_UnbornHead_NothingChanged(t *testing.T) {
	root := newGitRepo(t)
	mustRun(t, root, "init")

	stdout := mustRun(t, root, "recap")

	if want := "nothing changed since 7d\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestRecap_UnknownSince_IsInvalidWithTheGitLogHint(t *testing.T) {
	root := newRecapRepo(t)
	for _, since := range []string{"no-such-ref", "-x"} {
		t.Run(since, func(t *testing.T) {
			_, stderr, exitCode := runCLI(t, root, "recap", "--since", since)

			if exitCode != ExitError || !strings.Contains(stderr, `--since "`+since+`": not a date`) {
				t.Errorf("exit = %d stderr = %q, want exit %d naming --since %q", exitCode, stderr, ExitError, since)
			}
			assertFailureHint(t, root, "git log --oneline -- .capcom", "recap", "--since", since)
		})
	}
}

func TestRecap_JSON_GivesKindRecapWithEveryGroup(t *testing.T) {
	root := newRecapRepo(t)

	envelope := decodeEnvelope(t, mustRun(t, root, "recap", "--since", "HEAD~1", "--json"))

	if envelope["kind"] != kindRecap {
		t.Fatalf("kind = %v, want %s", envelope["kind"], kindRecap)
	}
	data := envelope["data"].(map[string]any)
	if data["since"] != "HEAD~1" || data["boundary"] == "" {
		t.Errorf("since = %v boundary = %v, want HEAD~1 and a commit", data["since"], data["boundary"])
	}
	closed := data["closed"].([]any)
	if len(closed) != 1 {
		t.Fatalf("closed = %v, want old-task alone", closed)
	}
	if entry := closed[0].(map[string]any); entry["id"] != "old-task" || entry["reason"] != "shipped" {
		t.Errorf("closed[0] = %v, want old-task with reason shipped", entry)
	}
	for _, group := range []string{"status_changed", "notes_changed", "notes_retired"} {
		if entries, isList := data[group].([]any); !isList || len(entries) != 0 {
			t.Errorf("%s = %v, want an empty list", group, data[group])
		}
	}
}
