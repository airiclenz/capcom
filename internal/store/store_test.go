package store

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/tree"
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

func TestInit_StoreDirIsAFile_FailsNamingItAndCreatesNothing(t *testing.T) {
	root := newRepo(t)
	storePath := filepath.Join(root, ".capcom")
	const content = "not a store\n"
	if err := os.WriteFile(storePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, _, err := Init(root)

	if err == nil || !strings.Contains(err.Error(), ".capcom: exists and is not a directory") {
		t.Fatalf("err = %v, want .capcom: exists and is not a directory", err)
	}
	if got, _ := os.ReadFile(storePath); string(got) != content {
		t.Errorf(".capcom = %q, want %q unchanged", got, content)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		t.Fatalf("ReadDir: %v", readErr)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if want := []string{".capcom", ".git"}; !reflect.DeepEqual(names, want) {
		t.Errorf("working tree holds %v, want %v", names, want)
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

// replaceConfigWithDir swaps the store's config.yaml for an empty directory of the same name.
func replaceConfigWithDir(t *testing.T, store *Store) {
	t.Helper()
	if err := os.Remove(store.ConfigPath()); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := os.Mkdir(store.ConfigPath(), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
}

func TestOpen_ConfigIsADirectory_FailsNamingTheConfig(t *testing.T) {
	root := newRepo(t)
	store, _, err := Init(root)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	replaceConfigWithDir(t, store)

	_, err = Open(root)

	if err == nil || !strings.Contains(err.Error(), ".capcom/config.yaml") {
		t.Errorf("err = %v, want an error naming .capcom/config.yaml", err)
	}
}

// initStore creates a store in a new repository, replaces its config.yaml with content and returns
// the opened store.
func initStore(t *testing.T, content string) *Store {
	t.Helper()
	root := newRepo(t)
	store, _, err := Init(root)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	writeConfig(t, root, content)
	return store
}

func TestRaiseFormat_CRLFConfig_KeepsCRLFAndMode(t *testing.T) {
	store := initStore(t, "types: [bug]\r\nformat: 1\r\nid_prefix: cc\r\n")
	if err := os.Chmod(store.ConfigPath(), 0o600); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	isRaised, err := store.RaiseFormat(2)

	if err != nil || !isRaised {
		t.Fatalf("RaiseFormat = %v, %v, want true, nil", isRaised, err)
	}
	content, _ := os.ReadFile(store.ConfigPath())
	if want := "types: [bug]\r\nformat: 2\r\nid_prefix: cc\r\n"; string(content) != want {
		t.Errorf("config.yaml = %q, want %q", content, want)
	}
	if info, err := os.Stat(store.ConfigPath()); runtime.GOOS != "windows" && (err != nil || info.Mode().Perm() != 0o600) {
		t.Errorf("config.yaml mode = %v (%v), want 0600 kept", info.Mode().Perm(), err)
	}
}

func TestRaiseFormat_FormatOneTemplate_WritesEveryByteConfigRaiseFormatReturns(t *testing.T) {
	const template = "# capcom configuration.\n\n" +
		"# The store format version; left as a comment, since a missing key means format 1.\n" +
		"# format: 1\n\ntypes: [bug, task]\n"
	store := initStore(t, template)
	want, _ := config.RaiseFormat([]byte(template), 2)

	isRaised, err := store.RaiseFormat(2)

	if err != nil || !isRaised {
		t.Fatalf("RaiseFormat = %v, %v, want true, nil", isRaised, err)
	}
	if content, _ := os.ReadFile(store.ConfigPath()); string(content) != string(want) {
		t.Errorf("config.yaml = %q, want %q", content, want)
	}
}

func TestRaiseFormat_AlreadyAtVersion_LeavesTheFileAlone(t *testing.T) {
	store := initStore(t, "format: 2\r\n")

	isRaised, err := store.RaiseFormat(2)

	if err != nil || isRaised {
		t.Fatalf("RaiseFormat = %v, %v, want false, nil", isRaised, err)
	}
	if content, _ := os.ReadFile(store.ConfigPath()); string(content) != "format: 2\r\n" {
		t.Errorf("config.yaml = %q, want it untouched", content)
	}
}

func TestRaiseFormat_MissingConfig_CreatesItHoldingTheFormatKey(t *testing.T) {
	store := initStore(t, "")
	if err := os.Remove(store.ConfigPath()); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	isRaised, err := store.RaiseFormat(2)

	if err != nil || !isRaised {
		t.Fatalf("RaiseFormat = %v, %v, want true, nil", isRaised, err)
	}
	if content, _ := os.ReadFile(store.ConfigPath()); string(content) != "format: 2\n" {
		t.Errorf("config.yaml = %q, want %q", content, "format: 2\n")
	}
}

func TestRaiseFormat_ConfigIsADirectory_FailsAndLeavesIt(t *testing.T) {
	root := newRepo(t)
	store, _, err := Init(root)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	replaceConfigWithDir(t, store)

	isRaised, err := store.RaiseFormat(3)

	if err == nil || isRaised {
		t.Fatalf("RaiseFormat = %v, %v, want false and an error", isRaised, err)
	}
	info, statErr := os.Stat(store.ConfigPath())
	if statErr != nil || !info.IsDir() {
		t.Fatalf("config.yaml = %v (%v), want the directory left in place", info, statErr)
	}
	if entries, _ := os.ReadDir(store.ConfigPath()); len(entries) != 0 {
		t.Errorf("config.yaml directory holds %d entries, want it empty", len(entries))
	}
}

// oversizedConfig is a config.yaml of 1 MiB + 1 byte made only of comment lines, so it parses as
// valid when read in full.
func oversizedConfig() string {
	const line = "# a comment line padding config.yaml past the guarded reader's limit\n"
	const size = 1<<20 + 1
	return strings.Repeat(line, size/len(line)+1)[:size]
}

// skipSymlinkTest skips a test that makes a symlink where symlink tests do not run: on Windows and
// as root.
func skipSymlinkTest(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("symlink tests do not run as root")
	}
}

// openWithin runs Open, failing the test when it has not returned after 5 s, as it does when it
// blocks opening a named pipe.
func openWithin(t *testing.T, root string) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := Open(root)
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Open did not return within 5 s")
		return nil
	}
}

func TestOpen_IrregularConfig_FailsNamingIt(t *testing.T) {
	cases := []struct {
		name     string
		setUp    func(t *testing.T, store *Store)
		wantText string
		wantErr  error
	}{
		{
			name: "named pipe",
			setUp: func(t *testing.T, store *Store) {
				removeConfig(t, store)
				makeFIFO(t, store.ConfigPath())
			},
			wantText: ".capcom/config.yaml: not a regular file",
			wantErr:  tree.ErrNotRegular,
		},
		{
			name: "symlink to a named pipe",
			setUp: func(t *testing.T, store *Store) {
				skipSymlinkTest(t)
				pipe := filepath.Join(store.Root, "pipe")
				makeFIFO(t, pipe)
				linkConfigTo(t, store, "pipe")
			},
			wantText: ".capcom/config.yaml: not a regular file",
			wantErr:  tree.ErrNotRegular,
		},
		{
			name: "symlink to a regular file",
			setUp: func(t *testing.T, store *Store) {
				skipSymlinkTest(t)
				target := filepath.Join(store.Root, "other.yaml")
				if err := os.WriteFile(target, []byte("types: [bug]\n"), 0o644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				linkConfigTo(t, store, "other.yaml")
			},
			wantText: ".capcom/config.yaml: not a regular file",
			wantErr:  tree.ErrNotRegular,
		},
		{
			name: "larger than 1 MiB",
			setUp: func(t *testing.T, store *Store) {
				writeConfig(t, store.WorktreeRoot, oversizedConfig())
			},
			wantText: ".capcom/config.yaml: file is larger than 1 MiB",
			wantErr:  tree.ErrTooLarge,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRepo(t)
			store, _, err := Init(root)
			if err != nil {
				t.Fatalf("Init: %v", err)
			}
			tc.setUp(t, store)

			err = openWithin(t, root)

			if err == nil || err.Error() != tc.wantText || !errors.Is(err, tc.wantErr) {
				t.Errorf("Open err = %v, want %q wrapping %v", err, tc.wantText, tc.wantErr)
			}
		})
	}
}

// removeConfig removes the store's config.yaml.
func removeConfig(t *testing.T, store *Store) {
	t.Helper()
	if err := os.Remove(store.ConfigPath()); err != nil {
		t.Fatalf("Remove: %v", err)
	}
}

// linkConfigTo replaces the store's config.yaml with a relative link to name in the store root.
func linkConfigTo(t *testing.T, store *Store, name string) {
	t.Helper()
	removeConfig(t, store)
	if err := os.Symlink(name, store.ConfigPath()); err != nil {
		t.Skipf("Symlink: %v", err)
	}
}

func TestInit_SymlinkedConfig_FailsNamingIt(t *testing.T) {
	skipSymlinkTest(t)
	root := newRepo(t)
	store, _, err := Init(root)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	linkConfigTo(t, store, "missing.yaml")

	_, _, err = Init(root)

	if err == nil || err.Error() != ".capcom/config.yaml: not a regular file" || !errors.Is(err, tree.ErrNotRegular) {
		t.Errorf("Init err = %v, want .capcom/config.yaml: not a regular file", err)
	}
	if target, linkErr := os.Readlink(store.ConfigPath()); linkErr != nil || target != "missing.yaml" {
		t.Errorf("config.yaml link = %q (%v), want it left pointing at missing.yaml", target, linkErr)
	}
}

func TestRaiseFormat_OversizedConfig_FailsAndLeavesIt(t *testing.T) {
	content := oversizedConfig()
	store := initStore(t, content)

	isRaised, err := store.RaiseFormat(3)

	if err == nil || isRaised || !strings.Contains(err.Error(), ".capcom/config.yaml") || !errors.Is(err, tree.ErrTooLarge) {
		t.Fatalf("RaiseFormat = %v, %v, want false and an error naming .capcom/config.yaml", isRaised, err)
	}
	if data, readErr := os.ReadFile(store.ConfigPath()); readErr != nil || string(data) != content {
		t.Errorf("config.yaml changed (%d bytes, %v), want its bytes unchanged", len(data), readErr)
	}
}
