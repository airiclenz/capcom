package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mergeDriverPath is the path argument the merge-driver tests pass, as git's %P would.
const mergeDriverPath = ".capcom/issues/login.md"

// Log entries the merge-driver tests append, oldest first.
const (
	driverEntryBase   = "- 2026-10-01 Bob: seen"
	driverEntryOurs   = "- 2026-10-05 Alice: started"
	driverEntryTheirs = "- 2026-10-06 Carol: triaged"
)

// driverIssueFile returns an issue file titled title whose Log, its last section, holds entries.
func driverIssueFile(title string, entries ...string) string {
	return "---\ntitle: " + title + "\nstatus: open\n---\n\nBody.\n\n## Log\n\n" +
		strings.Join(entries, "\n") + "\n"
}

// writeDriverInputs writes base, ours and theirs to files in dir, as git does for %O %A %B, and
// returns their paths.
func writeDriverInputs(t *testing.T, dir, base, ours, theirs string) (basePath, oursPath, theirsPath string) {
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

// readDriverFile returns the content of path.
func readDriverFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(content)
}

func TestMergeDriver_BothAppendOutsideAnyStore_WritesBothEntriesToOurs(t *testing.T) {
	dir := outsideGitDir(t)
	basePath, oursPath, theirsPath := writeDriverInputs(
		t,
		dir,
		driverIssueFile("Login", driverEntryBase),
		driverIssueFile("Login", driverEntryBase, driverEntryOurs),
		driverIssueFile("Login", driverEntryBase, driverEntryTheirs),
	)

	stdout, stderr, exitCode := runCLI(t, dir, "merge-driver", basePath, oursPath, theirsPath, mergeDriverPath)

	if exitCode != ExitOK || stdout != "" || stderr != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q; want 0 and no output", exitCode, stdout, stderr)
	}
	want := driverIssueFile("Login", driverEntryBase, driverEntryOurs, driverEntryTheirs)
	if got := readDriverFile(t, oursPath); got != want {
		t.Errorf("ours = %q, want %q", got, want)
	}
}

func TestMergeDriver_ConflictingTitles_LabelsMarkersAndExitsOne(t *testing.T) {
	// git merge-file reads merge.conflictStyle only inside a repository, as when git runs the
	// driver; the diff3 style shows the base label too.
	dir := newGitRepo(t)
	runGit(t, dir, "config", "merge.conflictStyle", "diff3")
	basePath, oursPath, theirsPath := writeDriverInputs(
		t,
		dir,
		driverIssueFile("Login", driverEntryBase),
		driverIssueFile("Login form", driverEntryBase, driverEntryOurs),
		driverIssueFile("Login page", driverEntryBase, driverEntryTheirs),
	)

	_, stderr, exitCode := runCLI(
		t, dir, "merge-driver", basePath, oursPath, theirsPath, mergeDriverPath,
		"merged common ancestors", "HEAD", "feature",
	)

	if exitCode != ExitError || !strings.Contains(stderr, mergeDriverPath+": "+conflictMessage) {
		t.Fatalf("exit %d, stderr %q; want 1 and the conflict naming %s", exitCode, stderr, mergeDriverPath)
	}
	got := readDriverFile(t, oursPath)
	wantHunk := "<<<<<<< HEAD\ntitle: Login form\n" +
		"||||||| merged common ancestors\ntitle: Login\n" +
		"=======\ntitle: Login page\n>>>>>>> feature\n"
	wantLog := "## Log\n\n" + strings.Join([]string{driverEntryBase, driverEntryOurs, driverEntryTheirs}, "\n") + "\n"
	if !strings.Contains(got, wantHunk) || !strings.HasSuffix(got, wantLog) {
		t.Errorf("ours = %q, want the hunk %q and the merged Log %q", got, wantHunk, wantLog)
	}
}

func TestMergeDriver_LabelsAbsentOrUnexpanded_FallBackToPath(t *testing.T) {
	cases := map[string][]string{
		"absent":     nil,
		"unexpanded": {"%S", "%X", "%Y"},
	}
	for name, labels := range cases {
		t.Run(name, func(t *testing.T) {
			dir := outsideGitDir(t)
			basePath, oursPath, theirsPath := writeDriverInputs(
				t,
				dir,
				driverIssueFile("Login", driverEntryBase),
				driverIssueFile("Login form", driverEntryBase),
				driverIssueFile("Login page", driverEntryBase),
			)
			args := append([]string{"merge-driver", basePath, oursPath, theirsPath, mergeDriverPath}, labels...)

			_, _, exitCode := runCLI(t, dir, args...)

			got := readDriverFile(t, oursPath)
			wantOurs := "<<<<<<< " + mergeDriverPath + " (ours)\n"
			wantTheirs := ">>>>>>> " + mergeDriverPath + " (theirs)\n"
			if exitCode != ExitError || !strings.Contains(got, wantOurs) || !strings.Contains(got, wantTheirs) {
				t.Errorf("exit %d, ours = %q; want 1 and the markers %q and %q", exitCode, got, wantOurs, wantTheirs)
			}
		})
	}
}

func TestMergeDriver_CRLFInputs_KeepCRLF(t *testing.T) {
	dir := outsideGitDir(t)
	toCRLF := strings.NewReplacer("\n", "\r\n").Replace
	basePath, oursPath, theirsPath := writeDriverInputs(
		t,
		dir,
		toCRLF(driverIssueFile("Login", driverEntryBase)),
		toCRLF(driverIssueFile("Login", driverEntryBase, driverEntryOurs)),
		toCRLF(driverIssueFile("Login form", driverEntryBase, driverEntryTheirs)),
	)

	_, stderr, exitCode := runCLI(t, dir, "merge-driver", basePath, oursPath, theirsPath, mergeDriverPath)

	if exitCode != ExitOK {
		t.Fatalf("exit %d, stderr %q; want 0", exitCode, stderr)
	}
	want := toCRLF(driverIssueFile("Login form", driverEntryBase, driverEntryOurs, driverEntryTheirs))
	if got := readDriverFile(t, oursPath); got != want {
		t.Errorf("ours = %q, want %q", got, want)
	}
}

func TestMergeDriver_MissingInput_FailsNamingItAndKeepsOurs(t *testing.T) {
	dir := outsideGitDir(t)
	ours := driverIssueFile("Login", driverEntryBase, driverEntryOurs)
	basePath, oursPath, _ := writeDriverInputs(t, dir, driverIssueFile("Login", driverEntryBase), ours, "")
	missing := filepath.Join(dir, ".merge_file_missing")

	_, stderr, exitCode := runCLI(t, dir, "merge-driver", basePath, oursPath, missing, mergeDriverPath)

	if exitCode != ExitError || !strings.Contains(stderr, missing) {
		t.Errorf("exit %d, stderr %q; want 1 and an error naming %s", exitCode, stderr, missing)
	}
	if got := readDriverFile(t, oursPath); got != ours {
		t.Errorf("ours = %q, want it unchanged %q", got, ours)
	}
}
