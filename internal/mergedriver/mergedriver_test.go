package mergedriver

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/gitx"
)

// Log entries the tests append, oldest first.
const (
	entryBase   = "- 2026-10-01 Bob: seen"
	entryOurs   = "- 2026-10-05 Alice: started"
	entryTheirs = "- 2026-10-06 Carol: triaged"
)

// testLabels are the conflict marker labels the tests merge under.
var testLabels = gitx.MergeLabels{Base: "base-label", Ours: "ours-label", Theirs: "theirs-label"}

// issueFile returns an Issue file titled title whose Log, its last section, holds entries.
func issueFile(title string, entries ...string) string {
	return "---\ntitle: " + title + "\nstatus: open\n---\n\nBody.\n\n## Log\n\n" +
		strings.Join(entries, "\n") + "\n"
}

// writeInputs writes base, ours and theirs to files in dir, as git does for %O %A %B, and returns
// their paths.
func writeInputs(t *testing.T, dir, base, ours, theirs string) (basePath, oursPath, theirsPath string) {
	t.Helper()
	basePath = filepath.Join(dir, ".merge_file_base")
	oursPath = filepath.Join(dir, ".merge_file_ours")
	theirsPath = filepath.Join(dir, ".merge_file_theirs")
	for path, content := range map[string]string{basePath: base, oursPath: ours, theirsPath: theirs} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	return basePath, oursPath, theirsPath
}

// readFile returns the content of path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(content)
}

// setTempDir points the process temp directory at dir for the rest of the test.
func setTempDir(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, dir)
	}
}

func TestMerge_BothAppendToLog_WritesBothEntriesToOurs(t *testing.T) {
	basePath, oursPath, theirsPath := writeInputs(
		t,
		t.TempDir(),
		issueFile("Login", entryBase),
		issueFile("Login", entryBase, entryOurs),
		issueFile("Login", entryBase, entryTheirs),
	)

	hasConflict, failedPath, err := Merge(basePath, oursPath, theirsPath, testLabels)

	if err != nil || hasConflict || failedPath != "" {
		t.Fatalf("Merge = %v, %q, %v; want no conflict, no failed path and no error", hasConflict, failedPath, err)
	}
	want := issueFile("Login", entryBase, entryOurs, entryTheirs)
	if got := readFile(t, oursPath); got != want {
		t.Errorf("ours = %q, want %q", got, want)
	}
}

func TestMerge_CRLFInputs_KeepCRLF(t *testing.T) {
	toCRLF := strings.NewReplacer("\n", "\r\n").Replace
	basePath, oursPath, theirsPath := writeInputs(
		t,
		t.TempDir(),
		toCRLF(issueFile("Login", entryBase)),
		toCRLF(issueFile("Login", entryBase, entryOurs)),
		toCRLF(issueFile("Login form", entryBase, entryTheirs)),
	)

	hasConflict, failedPath, err := Merge(basePath, oursPath, theirsPath, testLabels)

	if err != nil || hasConflict || failedPath != "" {
		t.Fatalf("Merge = %v, %q, %v; want no conflict, no failed path and no error", hasConflict, failedPath, err)
	}
	want := toCRLF(issueFile("Login form", entryBase, entryOurs, entryTheirs))
	if got := readFile(t, oursPath); got != want {
		t.Errorf("ours = %q, want %q", got, want)
	}
}

func TestMerge_ConflictingTitles_WritesLabelledMarkersAndReportsConflict(t *testing.T) {
	basePath, oursPath, theirsPath := writeInputs(
		t,
		t.TempDir(),
		issueFile("Login", entryBase),
		issueFile("Login form", entryBase),
		issueFile("Login page", entryBase),
	)

	hasConflict, failedPath, err := Merge(basePath, oursPath, theirsPath, testLabels)

	if err != nil || !hasConflict || failedPath != "" {
		t.Fatalf("Merge = %v, %q, %v; want a conflict, no failed path and no error", hasConflict, failedPath, err)
	}
	got := readFile(t, oursPath)
	wantOurs := "<<<<<<< " + testLabels.Ours + "\ntitle: Login form\n"
	wantTheirs := "title: Login page\n>>>>>>> " + testLabels.Theirs + "\n"
	if !strings.Contains(got, wantOurs) || !strings.Contains(got, wantTheirs) {
		t.Errorf("ours = %q, want the markers %q and %q", got, wantOurs, wantTheirs)
	}
}

func TestMerge_TextMerge_RemovesItsTempInputs(t *testing.T) {
	tempDir := t.TempDir()
	setTempDir(t, tempDir)
	basePath, oursPath, theirsPath := writeInputs(
		t,
		t.TempDir(),
		issueFile("Login", entryBase),
		issueFile("Login form", entryBase),
		issueFile("Login page", entryBase),
	)

	if _, _, err := Merge(basePath, oursPath, theirsPath, testLabels); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("temp directory holds %v after the merge, want it empty", entries)
	}
}

func TestMerge_MissingInput_ReturnsItsPathAndKeepsOurs(t *testing.T) {
	dir := t.TempDir()
	ours := issueFile("Login", entryBase, entryOurs)
	basePath, oursPath, _ := writeInputs(t, dir, issueFile("Login", entryBase), ours, "")
	missing := filepath.Join(dir, ".merge_file_missing")

	hasConflict, failedPath, err := Merge(basePath, oursPath, missing, testLabels)

	if !errors.Is(err, fs.ErrNotExist) || hasConflict || failedPath != missing {
		t.Errorf("Merge = %v, %q, %v; want no conflict, the failed path %s and a not-exist error", hasConflict, failedPath, err, missing)
	}
	if got := readFile(t, oursPath); got != ours {
		t.Errorf("ours = %q, want it unchanged %q", got, ours)
	}
}

func TestMerge_OursInReadOnlyDirectory_ReturnsOursPathAndKeepsOurs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a read-only directory does not stop file creation on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes into a read-only directory")
	}
	dir := t.TempDir()
	ours := issueFile("Login", entryBase, entryOurs)
	basePath, oursPath, theirsPath := writeInputs(
		t,
		dir,
		issueFile("Login", entryBase),
		ours,
		issueFile("Login", entryBase, entryTheirs),
	)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	hasConflict, failedPath, err := Merge(basePath, oursPath, theirsPath, testLabels)

	if err == nil || hasConflict || failedPath != oursPath {
		t.Errorf("Merge = %v, %q, %v; want no conflict, the failed path %s and an error", hasConflict, failedPath, err, oursPath)
	}
	if got := readFile(t, oursPath); got != ours {
		t.Errorf("ours = %q, want it unchanged %q", got, ours)
	}
}

func TestMerge_TextMergeFails_ReturnsNoPathAndKeepsOurs(t *testing.T) {
	setTempDir(t, filepath.Join(t.TempDir(), "missing"))
	ours := issueFile("Login form", entryBase)
	basePath, oursPath, theirsPath := writeInputs(
		t,
		t.TempDir(),
		issueFile("Login", entryBase),
		ours,
		issueFile("Login page", entryBase),
	)

	hasConflict, failedPath, err := Merge(basePath, oursPath, theirsPath, testLabels)

	if err == nil || hasConflict || failedPath != "" {
		t.Errorf("Merge = %v, %q, %v; want no conflict, no failed path and an error", hasConflict, failedPath, err)
	}
	if got := readFile(t, oursPath); got != ours {
		t.Errorf("ours = %q, want it unchanged %q", got, ours)
	}
}
