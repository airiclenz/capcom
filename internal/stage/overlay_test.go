package stage_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/stage"
	"github.com/airiclenz/capcom/internal/store"
	"github.com/airiclenz/capcom/internal/tree"
)

// Contents the tests write and stage.
const (
	existingContent = "existing\n"
	editedContent   = "edited\n"
	createdContent  = "created\n"
	concurrentEdit  = "written meanwhile\n"
)

// fixture is a store directory holding one issue file, with an Overlay over it.
type fixture struct {
	overlay    *stage.Overlay
	store      *store.Store
	issuesDir  string
	existing   string
	newDir     string
	newFile    string
	configPath string
}

// newFixture creates a store directory holding the issue file existing-node.md and returns an
// Overlay over it, with the path of a child file a create would put in a new directory.
func newFixture(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	s := &store.Store{Root: filepath.Join(root, ".capcom"), WorktreeRoot: root}
	issuesDir := filepath.Join(s.Root, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(issuesDir, "existing-node.md")
	writeFile(t, existing, existingContent)
	newDir := filepath.Join(issuesDir, "existing-node")
	return fixture{
		overlay:    stage.New(s),
		store:      s,
		issuesDir:  issuesDir,
		existing:   existing,
		newDir:     newDir,
		newFile:    filepath.Join(newDir, "new-node.md"),
		configPath: s.ConfigPath(),
	}
}

// stageCreateAndEdit stages an edit of the existing file, read first as Edit reads it, and the
// create of the new child file.
func (f fixture) stageCreateAndEdit(t *testing.T) {
	t.Helper()
	if _, err := f.overlay.ReadRegularFile(f.existing); err != nil {
		t.Fatalf("ReadRegularFile: %v", err)
	}
	if err := f.overlay.ReplaceFile(f.existing, strings.NewReader(editedContent)); err != nil {
		t.Fatalf("ReplaceFile: %v", err)
	}
	if err := f.overlay.CreateFile(f.newFile, strings.NewReader(createdContent)); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
}

func TestOverlay_StagedCreateAndEdit_InvisibleOnDiskUntilApplyLandsBoth(t *testing.T) {
	f := newFixture(t)
	f.stageCreateAndEdit(t)

	assertFile(t, f.existing, existingContent)
	if _, err := os.Lstat(f.newDir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("new directory before Apply: Lstat err = %v, want it missing on disk", err)
	}

	if _, err := f.overlay.Plan().Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	assertFile(t, f.existing, editedContent)
	assertFile(t, f.newFile, createdContent)
}

func TestOverlay_StagedCreateAndEdit_ReadsSeeTheStagedFiles(t *testing.T) {
	f := newFixture(t)
	f.stageCreateAndEdit(t)

	if data, err := f.overlay.ReadRegularFile(f.existing); err != nil || string(data) != editedContent {
		t.Errorf("ReadRegularFile(existing) = %q %v, want the staged edit", data, err)
	}
	if data, err := f.overlay.ReadRegularFile(f.newFile); err != nil || string(data) != createdContent {
		t.Errorf("ReadRegularFile(new) = %q %v, want the staged create", data, err)
	}
	if info, err := f.overlay.Lstat(f.newDir); err != nil || !info.IsDir() {
		t.Errorf("Lstat(new directory) = %v %v, want a staged directory", info, err)
	}
	if info, err := f.overlay.Stat(f.existing); err != nil || info.Size() != int64(len(editedContent)) {
		t.Errorf("Stat(existing) = %v %v, want the staged size", info, err)
	}
	assertDirNames(t, f.overlay, f.issuesDir, "existing-node", "existing-node.md")
	assertDirNames(t, f.overlay, f.newDir, "new-node.md")
}

func TestOverlay_StagedCreateAndEdit_TreeLoadsTheStagedNodes(t *testing.T) {
	f := newFixture(t)
	f.stageCreateAndEdit(t)

	loaded, err := tree.LoadContentFS(f.overlay, f.issuesDir, "issue", nil)
	if err != nil {
		t.Fatalf("LoadContentFS: %v", err)
	}

	if node, isFound := loaded.Lookup("new-node"); !isFound || node.ParentID != "existing-node" {
		t.Errorf("Lookup(new-node) = %+v %v, want the staged create below existing-node", node, isFound)
	}
}

func TestOverlay_FileChangedAfterStaging_ApplyFailsChangedAndKeepsEveryFile(t *testing.T) {
	f := newFixture(t)
	f.stageCreateAndEdit(t)
	writeFile(t, f.existing, concurrentEdit)

	_, err := f.overlay.Plan().Apply()

	if !errors.Is(err, tree.ErrChanged) || !strings.Contains(err.Error(), f.existing) {
		t.Fatalf("Apply err = %v, want tree.ErrChanged naming %s", err, f.existing)
	}
	assertFile(t, f.existing, concurrentEdit)
	if _, err := os.Lstat(f.newDir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("new directory after the failed Apply: Lstat err = %v, want it missing", err)
	}
}

func TestOverlay_LaterCreateClashes_TakesBackEveryWrite(t *testing.T) {
	f := newFixture(t)
	f.stageCreateAndEdit(t)
	clash := filepath.Join(f.issuesDir, "clashing-node.md")
	if err := f.overlay.CreateFile(clash, strings.NewReader(createdContent)); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	writeFile(t, clash, concurrentEdit)

	_, err := f.overlay.Plan().Apply()

	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("Apply err = %v, want fs.ErrExist for the clashing create", err)
	}
	assertFile(t, f.existing, existingContent)
	assertFile(t, clash, concurrentEdit)
	if _, err := os.Lstat(f.newDir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("new directory after the failed Apply: Lstat err = %v, want it removed", err)
	}
}

func TestOverlay_WriteRefusals(t *testing.T) {
	f := newFixture(t)
	missing := filepath.Join(f.issuesDir, "missing-node.md")

	createErr := f.overlay.CreateFile(f.existing, strings.NewReader(createdContent))
	replaceErr := f.overlay.ReplaceFile(missing, strings.NewReader(editedContent))

	if !errors.Is(createErr, fs.ErrExist) || !strings.Contains(createErr.Error(), f.existing) {
		t.Errorf("CreateFile over a file on disk: err = %v, want fs.ErrExist naming it", createErr)
	}
	if !errors.Is(replaceErr, fs.ErrNotExist) || !strings.Contains(replaceErr.Error(), missing) {
		t.Errorf("ReplaceFile of a missing file: err = %v, want fs.ErrNotExist naming it", replaceErr)
	}
	if err := f.overlay.CreateFile(f.newFile, strings.NewReader(createdContent)); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}
	if err := f.overlay.CreateFile(f.newFile, strings.NewReader(createdContent)); !errors.Is(err, fs.ErrExist) {
		t.Errorf("CreateFile over a staged create: err = %v, want fs.ErrExist", err)
	}
}

func TestOverlay_ReadRegularFile_StagedFileOverOneMiB_FailsTooLarge(t *testing.T) {
	f := newFixture(t)
	atCap := strings.Repeat("x", nodefile.MaxNodeFileSize)
	if err := f.overlay.ReplaceFile(f.existing, strings.NewReader(atCap)); err != nil {
		t.Fatalf("ReplaceFile at the cap: %v", err)
	}
	if data, err := f.overlay.ReadRegularFile(f.existing); err != nil || len(data) != nodefile.MaxNodeFileSize {
		t.Fatalf("ReadRegularFile at the cap = %d bytes %v, want all %d bytes", len(data), err, nodefile.MaxNodeFileSize)
	}
	if err := f.overlay.ReplaceFile(f.existing, strings.NewReader(atCap+"x")); err != nil {
		t.Fatalf("ReplaceFile over the cap: %v", err)
	}

	data, err := f.overlay.ReadRegularFile(f.existing)

	if !errors.Is(err, nodefile.ErrTooLarge) || !strings.Contains(err.Error(), f.existing) {
		t.Errorf("ReadRegularFile over the cap: err = %v, want nodefile.ErrTooLarge naming it", err)
	}
	if data != nil {
		t.Errorf("ReadRegularFile over the cap returned %d bytes, want none", len(data))
	}
}

func TestOverlay_ReadRegularFile_StagedDirectory_FailsNotRegular(t *testing.T) {
	f := newFixture(t)
	if err := f.overlay.CreateFile(f.newFile, strings.NewReader(createdContent)); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	data, err := f.overlay.ReadRegularFile(f.newDir)

	if !errors.Is(err, nodefile.ErrNotRegular) || !strings.Contains(err.Error(), f.newDir) {
		t.Errorf("ReadRegularFile of a staged directory: err = %v, want nodefile.ErrNotRegular naming it", err)
	}
	if data != nil {
		t.Errorf("ReadRegularFile of a staged directory returned %d bytes, want none", len(data))
	}
}

func TestOverlay_OriginalETag_IsTheFileAsTheBatchFoundIt(t *testing.T) {
	f := newFixture(t)
	f.stageCreateAndEdit(t)

	etag, err := f.overlay.OriginalETag(f.existing)
	_, newErr := f.overlay.OriginalETag(f.newFile)

	if err != nil || etag != nodefile.ETag([]byte(existingContent)) {
		t.Errorf("OriginalETag(existing) = %q %v, want the ETag of the content before the edit", etag, err)
	}
	if !errors.Is(newErr, fs.ErrNotExist) {
		t.Errorf("OriginalETag(created) err = %v, want fs.ErrNotExist", newErr)
	}
}

func TestOverlay_StagedRaise_WritesNothingUntilApply(t *testing.T) {
	f := newFixture(t)
	original := "types: [bug]\r\nformat: 1\r\nplans: [epic]\r\n"
	writeFile(t, f.configPath, original)
	if err := os.Chmod(f.configPath, 0o600); err != nil {
		t.Fatal(err)
	}

	isRaised, err := f.overlay.RaiseFormat(2)

	if err != nil || !isRaised {
		t.Fatalf("RaiseFormat = %v %v, want true, nil", isRaised, err)
	}
	assertFile(t, f.configPath, original)
	if _, err := f.overlay.Plan().Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertFile(t, f.configPath, "types: [bug]\r\nformat: 2\r\nplans: [epic]\r\n")
	if info, err := os.Stat(f.configPath); runtime.GOOS != "windows" && (err != nil || info.Mode().Perm() != 0o600) {
		t.Errorf("config.yaml mode = %v (%v), want 0600 kept", info.Mode().Perm(), err)
	}
}

func TestOverlay_StagedRaise_MissingConfigIsCreated(t *testing.T) {
	f := newFixture(t)

	isRaised, err := f.overlay.RaiseFormat(2)

	if err != nil || !isRaised {
		t.Fatalf("RaiseFormat = %v %v, want true, nil", isRaised, err)
	}
	if _, err := os.Lstat(f.configPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("config.yaml before Apply: Lstat err = %v, want it missing", err)
	}
	if _, err := f.overlay.Plan().Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertFile(t, f.configPath, "format: 2\n")
}

func TestOverlay_StagedRaise_ReportsOnlyARaiseItStages(t *testing.T) {
	f := newFixture(t)
	writeFile(t, f.configPath, "format: 3\n")

	alreadyThere, err := f.overlay.RaiseFormat(2)
	if err != nil || alreadyThere {
		t.Fatalf("RaiseFormat(2) over format 3 = %v %v, want false, nil", alreadyThere, err)
	}
	raised, err := f.overlay.RaiseFormat(4)
	if err != nil || !raised {
		t.Fatalf("RaiseFormat(4) = %v %v, want true, nil", raised, err)
	}
	again, err := f.overlay.RaiseFormat(4)
	if err != nil || again {
		t.Fatalf("RaiseFormat(4) again = %v %v, want false, nil", again, err)
	}
}

func TestOverlay_StagedRaise_ConfigChangedBeforeApply_FailsChangedAndTakesBackTheBatch(t *testing.T) {
	f := newFixture(t)
	writeFile(t, f.configPath, "format: 1\n")
	f.stageCreateAndEdit(t)
	if _, err := f.overlay.RaiseFormat(2); err != nil {
		t.Fatalf("RaiseFormat: %v", err)
	}
	writeFile(t, f.configPath, "format: 1\n# changed\n")

	_, err := f.overlay.Plan().Apply()

	if !errors.Is(err, tree.ErrChanged) || !strings.Contains(err.Error(), f.configPath) {
		t.Fatalf("Apply err = %v, want tree.ErrChanged naming config.yaml", err)
	}
	assertFile(t, f.existing, existingContent)
	assertFile(t, f.configPath, "format: 1\n# changed\n")
}

// writeFile writes content to path, failing the test on any error.
func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// assertFile fails the test unless the file at path holds want.
func assertFile(t *testing.T, path string, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil || string(content) != want {
		t.Errorf("%s = %q %v, want %q", path, content, err, want)
	}
}

// assertDirNames fails the test unless the overlay lists dir as exactly the names given, in order.
func assertDirNames(t *testing.T, overlay *stage.Overlay, dir string, names ...string) {
	t.Helper()
	entries, err := overlay.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	if strings.Join(got, ",") != strings.Join(names, ",") {
		t.Errorf("ReadDir(%s) = %v, want %v", dir, got, names)
	}
}
