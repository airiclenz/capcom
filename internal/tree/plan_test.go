package tree_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/tree"
)

// errInjectedStep is the failure of a step a test adds after a create, to make Apply take it back.
var errInjectedStep = errors.New("injected step failure")

// createdContent is the content the create tests write, with a CRLF line ending AddCreate turns
// to LF.
const createdContent = "created\r\n"

func TestPlanAddCreate_MissingDirectories_CreatesFileAndDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parent", "child", "new-node.md")
	var plan tree.Plan
	plan.AddCreate(path, []byte(createdContent))

	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil || string(content) != "created\n" {
		t.Errorf("new-node.md = %q %v, want the content with LF line endings", content, err)
	}
}

func TestPlanAddCreate_LaterStepFails_RemovesFileAndTheDirectoriesItMade(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parent", "child", "new-node.md")
	var plan tree.Plan
	plan.AddCreate(path, []byte(createdContent))
	plan.AddStep(func() error { return errInjectedStep }, func() error { return nil })

	_, err := plan.Apply()

	if !errors.Is(err, errInjectedStep) {
		t.Fatalf("Apply err = %v, want the injected step failure", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "parent")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("parent/ after the rollback: Lstat err = %v, want it removed", err)
	}
	assertEntries(t, dir)
}

func TestPlanAddCreate_ExistingDirectory_RollbackKeepsIt(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "parent")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	var plan tree.Plan
	plan.AddCreate(filepath.Join(parent, "new-node.md"), []byte(createdContent))
	plan.AddStep(func() error { return errInjectedStep }, func() error { return nil })

	if _, err := plan.Apply(); !errors.Is(err, errInjectedStep) {
		t.Fatalf("Apply err = %v, want the injected step failure", err)
	}

	assertEntries(t, parent)
}

func TestPlanAddCreate_WriteFailsAfterItsMkdir_RemovesTheDirectoriesItMade(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parent", "child", "new-node.md")
	var plan tree.Plan
	tree.SetPlanCreate(&plan, func(path string, _ io.Reader) error {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return errInjectedStep
	})
	plan.AddCreate(path, []byte(createdContent))

	_, err := plan.Apply()

	if !errors.Is(err, errInjectedStep) {
		t.Fatalf("Apply err = %v, want the injected write failure", err)
	}
	assertEntries(t, dir)
}

func TestPlanAddCreate_FileAlreadyThere_FailsExistAndKeepsIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new-node.md")
	var plan tree.Plan
	plan.AddCreate(path, []byte(createdContent))
	concurrent := "written meanwhile\n"
	if err := os.WriteFile(path, []byte(concurrent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := plan.Apply()

	if !errors.Is(err, fs.ErrExist) || !strings.Contains(err.Error(), path) {
		t.Fatalf("Apply err = %v, want fs.ErrExist naming %s", err, path)
	}
	if content, err := os.ReadFile(path); err != nil || string(content) != concurrent {
		t.Errorf("new-node.md = %q %v, want the concurrent bytes kept", content, err)
	}
}

func TestPlanAddCreate_FileChangedBeforeTheRollback_LeavesItAndNamesIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "parent", "new-node.md")
	concurrent := "written meanwhile\n"
	var plan tree.Plan
	plan.AddCreate(path, []byte(createdContent))
	plan.AddStep(
		func() error {
			if err := os.WriteFile(path, []byte(concurrent), 0o644); err != nil {
				return err
			}
			return errInjectedStep
		},
		func() error { return nil },
	)

	_, err := plan.Apply()

	if !errors.Is(err, errInjectedStep) || errors.Is(err, tree.ErrChanged) ||
		!strings.Contains(err.Error(), path+": changed since it was created") {
		t.Fatalf("Apply err = %v, want the step failure and the create named as left", err)
	}
	if content, err := os.ReadFile(path); err != nil || string(content) != concurrent {
		t.Errorf("new-node.md = %q %v, want the concurrent bytes kept", content, err)
	}
}

// assertEntries fails the test unless the directory dir holds exactly the entries named.
func assertEntries(t *testing.T, dir string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	if strings.Join(got, ",") != strings.Join(names, ",") {
		t.Errorf("%s holds %v, want %v", dir, got, names)
	}
}
