package store

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/config"
)

// setHermeticGitEnv isolates git from user and system configuration (ADR 0007) and stops git
// from discovering a repository above root, so tests do not call t.Parallel.
func setHermeticGitEnv(t *testing.T, root string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
}

// newRepo creates an empty git repository in a temp dir and returns its root.
func newRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	setHermeticGitEnv(t, root)
	command := exec.Command("git", "init", "-q", root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	return root
}

func resolved(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", path, err)
	}
	return real
}

func writeConfig(t *testing.T, root, content string) {
	t.Helper()
	path := filepath.Join(root, ".capcom", "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestInit_NewRepository_CreatesStoreLayout(t *testing.T) {
	root := newRepo(t)

	store, created, err := Init(root)

	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	want := []string{".capcom", ".capcom/issues", ".capcom/knowledge", ".capcom/config.yaml"}
	if !reflect.DeepEqual(created, want) {
		t.Errorf("created = %v, want %v", created, want)
	}
	content, err := os.ReadFile(filepath.Join(root, ".capcom", "config.yaml"))
	if err != nil || string(content) != config.DefaultFile {
		t.Errorf("config.yaml = %q (%v), want the default file", content, err)
	}
	if !reflect.DeepEqual(store.Config, config.Default()) {
		t.Errorf("Config = %+v, want the default", store.Config)
	}
}

func TestInit_ReRun_IsNoOpAndKeepsConfig(t *testing.T) {
	root := newRepo(t)
	if _, _, err := Init(root); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	custom := "statuses:\n  todo: todo\n  closed: done\n"
	writeConfig(t, root, custom)

	_, created, err := Init(root)

	if err != nil || len(created) != 0 {
		t.Fatalf("second Init: created = %v err = %v, want nothing created", created, err)
	}
	content, _ := os.ReadFile(filepath.Join(root, ".capcom", "config.yaml"))
	if string(content) != custom {
		t.Errorf("config.yaml = %q, want it untouched", content)
	}
}

func TestInit_FromSubdirectory_CreatesStoreAtWorktreeRoot(t *testing.T) {
	root := newRepo(t)
	subdirectory := filepath.Join(root, "src", "deep")
	if err := os.MkdirAll(subdirectory, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	store, _, err := Init(subdirectory)

	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if want := filepath.Join(resolved(t, root), ".capcom"); store.Root != want {
		t.Errorf("Root = %s, want %s", store.Root, want)
	}
}

func TestInit_OutsideGit_FailsNotGitRepository(t *testing.T) {
	outside := t.TempDir()
	setHermeticGitEnv(t, outside)

	_, _, err := Init(outside)

	if !errors.Is(err, ErrNotGitRepository) {
		t.Errorf("err = %v, want ErrNotGitRepository", err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, ".capcom")); !os.IsNotExist(statErr) {
		t.Errorf(".capcom was created outside git")
	}
}

func TestOpen_WithoutStore_FailsNotInitialized(t *testing.T) {
	root := newRepo(t)

	_, err := Open(root)

	if !errors.Is(err, ErrNotInitialized) {
		t.Errorf("err = %v, want ErrNotInitialized", err)
	}
}

func TestOpen_OutsideGit_FailsNotGitRepository(t *testing.T) {
	outside := t.TempDir()
	setHermeticGitEnv(t, outside)

	_, err := Open(outside)

	if !errors.Is(err, ErrNotGitRepository) {
		t.Errorf("err = %v, want ErrNotGitRepository", err)
	}
}

func TestOpen_FromSubdirectory_ResolvesAbsolutePaths(t *testing.T) {
	root := newRepo(t)
	if _, _, err := Init(root); err != nil {
		t.Fatalf("Init: %v", err)
	}
	subdirectory := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(subdirectory, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	store, err := Open(subdirectory)

	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	realRoot := resolved(t, root)
	if store.WorktreeRoot != realRoot || store.Root != filepath.Join(realRoot, ".capcom") {
		t.Errorf("WorktreeRoot = %s Root = %s, want under %s", store.WorktreeRoot, store.Root, realRoot)
	}
	gitDir := filepath.Join(realRoot, ".git")
	for name, path := range map[string]string{"GitDir": store.GitDir, "GitCommonDir": store.GitCommonDir} {
		if !filepath.IsAbs(path) || resolved(t, path) != gitDir {
			t.Errorf("%s = %s, want absolute %s", name, path, gitDir)
		}
	}
}

func TestOpen_InvalidConfig_FailsWithPathAndLine(t *testing.T) {
	root := newRepo(t)
	if _, _, err := Init(root); err != nil {
		t.Fatalf("Init: %v", err)
	}
	writeConfig(t, root, "types: [bug]\nlock:\n  ttl: 1h\n  colour: blue\n")

	_, err := Open(root)

	var failure *config.Error
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want *config.Error", err)
	}
	if failure.Path != ".capcom/config.yaml" || failure.Line != 4 || !strings.Contains(failure.Message, "colour") {
		t.Errorf("error = %v, want .capcom/config.yaml:4 naming colour", err)
	}
}
