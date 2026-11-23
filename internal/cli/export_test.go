package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// goldenIssueIDs are the Issues newGoldenStore files.
var goldenIssueIDs = []string{"login-flow", "session-timeout", "refresh-retry", "login-copy", "old-login-page"}

// readSite returns every file below dir by its path relative to dir, with forward slashes.
func readSite(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(walked string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(walked)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(dir, walked)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = content
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir %s: %v", dir, err)
	}
	return files
}

func TestExport_WritesTheSiteAndReportsIt(t *testing.T) {
	root := newGoldenStore(t)
	dir := filepath.Join(t.TempDir(), "site")

	stdout := mustRun(t, root, "export", "--html", dir)

	site := readSite(t, dir)
	if _, isWritten := site["index.html"]; !isWritten {
		t.Fatalf("index.html not written; files = %d", len(site))
	}
	issues := string(site["issues.html"])
	for _, id := range goldenIssueIDs {
		page := "issues/" + id + ".html"
		if _, isWritten := site[page]; !isWritten {
			t.Errorf("%s not written", page)
		}
		if !strings.Contains(issues, `href="`+page+`"`) {
			t.Errorf("issues.html does not link %s", page)
		}
	}
	want := "exported " + strconv.Itoa(len(site)-1) + " pages to " + dir + "\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestExport_SecondRun_WritesTheSameBytes(t *testing.T) {
	root := newGoldenStore(t)
	dir := filepath.Join(t.TempDir(), "site")
	mustRun(t, root, "export", "--html", dir)
	first := readSite(t, dir)

	mustRun(t, root, "export", "--html", dir)

	second := readSite(t, dir)
	if len(second) != len(first) {
		t.Fatalf("second run wrote %d files, want %d", len(second), len(first))
	}
	for path, content := range first {
		if !bytes.Equal(second[path], content) {
			t.Errorf("%s differs between runs", path)
		}
	}
}

func TestExport_RefusedDir_FailsAndWritesNothing(t *testing.T) {
	tests := []struct {
		name    string
		dir     func(t *testing.T, root string) string
		message string
	}{
		{
			name:    "inside .capcom",
			dir:     func(_ *testing.T, _ string) string { return filepath.Join(".capcom", "out") },
			message: "lies inside the store directory .capcom",
		},
		{
			name: "foreign non-empty dir",
			dir: func(t *testing.T, _ string) string {
				foreign := t.TempDir()
				if err := os.WriteFile(filepath.Join(foreign, "notes.txt"), []byte("mine"), 0o644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				return foreign
			},
			message: "is not empty and holds no .capcom-export file",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newGoldenStore(t)
			dir := test.dir(t, root)
			target := dir
			if !filepath.IsAbs(target) {
				target = filepath.Join(root, target)
			}
			before := readSite(t, filepath.Dir(target))

			stdout, stderr, exitCode := runCLI(t, root, "export", "--html", dir)

			if exitCode != ExitError || stdout != "" || !strings.Contains(stderr, test.message) {
				t.Fatalf("exit = %d stdout = %q stderr = %q, want %d and %q", exitCode, stdout, stderr,
					ExitError, test.message)
			}
			after := readSite(t, filepath.Dir(target))
			if len(after) != len(before) {
				t.Errorf("the refused export changed %s: %d files, want %d", filepath.Dir(target),
					len(after), len(before))
			}
			if _, err := os.Stat(filepath.Join(target, ".capcom-export")); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("the refused export wrote its marker: %v", err)
			}
		})
	}
}

func TestExport_WithoutHTML_IsAUsageError(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "no --html", args: []string{"export"}},
		{name: "blank --html", args: []string{"export", "--html", " "}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newGoldenStore(t)

			stdout, stderr, exitCode := runCLI(t, root, test.args...)

			if exitCode != ExitUsage || stdout != "" || !strings.Contains(stderr, "html") {
				t.Errorf("exit = %d stdout = %q stderr = %q, want %d naming --html", exitCode, stdout,
					stderr, ExitUsage)
			}
		})
	}
}

func TestExport_JSON_GivesKindExport(t *testing.T) {
	root := newGoldenStore(t)
	dir := filepath.Join(t.TempDir(), "site")

	stdout := mustRun(t, root, "export", "--html", dir, "--json")

	envelope := decodeEnvelope(t, stdout)
	data, _ := envelope["data"].(map[string]any)
	pages := float64(len(readSite(t, dir)) - 1)
	if envelope["kind"] != "export" || data["dir"] != dir || data["pages"] != pages {
		t.Errorf("envelope = %v, want kind export with dir %s and %v pages", envelope, dir, pages)
	}
}
