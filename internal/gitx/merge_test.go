package gitx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMergeInputs writes base, ours and theirs to files in a fresh temp directory outside any
// repository and returns their paths.
func writeMergeInputs(t *testing.T, base, ours, theirs string) (basePath, oursPath, theirsPath string) {
	t.Helper()
	dir := t.TempDir()
	setHermeticGitEnv(t, dir)
	basePath = filepath.Join(dir, "base")
	oursPath = filepath.Join(dir, "ours")
	theirsPath = filepath.Join(dir, "theirs")
	for path, content := range map[string]string{basePath: base, oursPath: ours, theirsPath: theirs} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	return basePath, oursPath, theirsPath
}

func TestMergeFile_SeparateEdits_MergesCleanly(t *testing.T) {
	basePath, oursPath, theirsPath := writeMergeInputs(
		t,
		"one\ntwo\nthree\nfour\nfive\n",
		"ONE\ntwo\nthree\nfour\nfive\n",
		"one\ntwo\nthree\nfour\nFIVE\n",
	)

	merged, hasConflict, err := MergeFile(basePath, oursPath, theirsPath, MergeLabels{})

	if err != nil {
		t.Fatalf("MergeFile: %v", err)
	}
	if want := "ONE\ntwo\nthree\nfour\nFIVE\n"; string(merged) != want || hasConflict {
		t.Errorf("MergeFile = %q, conflict %v; want %q, no conflict", merged, hasConflict, want)
	}
}

func TestMergeFile_SameLineEdited_ReportsConflictWithLabels(t *testing.T) {
	basePath, oursPath, theirsPath := writeMergeInputs(t, "line\n", "ours line\n", "theirs line\n")
	labels := MergeLabels{Base: "the base", Ours: "the ours", Theirs: "the theirs"}

	merged, hasConflict, err := MergeFile(basePath, oursPath, theirsPath, labels)

	if err != nil {
		t.Fatalf("MergeFile: %v", err)
	}
	want := "<<<<<<< the ours\nours line\n=======\ntheirs line\n>>>>>>> the theirs\n"
	if string(merged) != want || !hasConflict {
		t.Errorf("MergeFile = %q, conflict %v; want %q, conflict", merged, hasConflict, want)
	}
}

func TestMergeFile_MissingInput_FailsWithoutBytes(t *testing.T) {
	basePath, oursPath, _ := writeMergeInputs(t, "line\n", "line\n", "line\n")
	missing := filepath.Join(filepath.Dir(basePath), "missing")

	merged, hasConflict, err := MergeFile(basePath, oursPath, missing, MergeLabels{})

	var gitFailure *Error
	if !errors.As(err, &gitFailure) || merged != nil || hasConflict {
		t.Fatalf("MergeFile = %q, conflict %v, err %v; want a git *Error and no bytes", merged, hasConflict, err)
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error %q does not name the missing file", err)
	}
}
