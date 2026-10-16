package userconfig_test

import (
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/airiclenz/capcom/internal/userconfig"
)

// importPath is this package's import path, which only internal/ui may import (ADR 0011).
const importPath = "github.com/airiclenz/capcom/internal/userconfig"

// configPath returns a config file path inside a fresh temp dir whose capcom directory does not
// exist yet.
func configPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "capcom", "config.yaml")
}

// writeConfig writes content as the config file at a fresh path and returns that path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := configPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// readFile returns the content of path.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}

// skipWithoutPermissions skips a test that relies on file permissions being enforced.
func skipWithoutPermissions(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions are not enforced on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
}

// uncomment returns the created template with its YAML lines uncommented: "# ui:" and every line
// indented under it.
func uncomment(template string) string {
	lines := strings.Split(template, "\n")
	for index, line := range lines {
		if line == "# ui:" || strings.HasPrefix(line, "#   ") {
			lines[index] = strings.TrimPrefix(line, "# ")
		}
	}
	return strings.Join(lines, "\n")
}

func TestDefault_NoFile_ThemeAutoWithoutOverrides(t *testing.T) {
	config := userconfig.Default()

	want := userconfig.Config{UI: userconfig.UI{Theme: userconfig.ThemeAuto, Colors: map[string]string{}}}
	if !reflect.DeepEqual(config, want) {
		t.Errorf("Default() = %#v, want %#v", config, want)
	}
}

func TestPath_UserConfigDir_EndsInCapcomConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	path, err := userconfig.Path()

	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if want := filepath.Join("capcom", "config.yaml"); !strings.HasSuffix(path, want) {
		t.Errorf("Path() = %q, want a path ending in %q", path, want)
	}
}

func TestLoad_MissingFile_CreatesAllCommentedTemplate(t *testing.T) {
	path := configPath(t)

	config, warnings, err := userconfig.Load(path)

	if err != nil || len(warnings) != 0 {
		t.Fatalf("Load() warnings = %v, error = %v; want none", warnings, err)
	}
	if !reflect.DeepEqual(config, userconfig.Default()) {
		t.Errorf("Load() = %#v, want Default()", config)
	}
	for number, line := range strings.Split(strings.TrimSuffix(readFile(t, path), "\n"), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			t.Errorf("line %d = %q, want every line commented", number+1, line)
		}
	}
}

func TestLoad_SecondLoad_LeavesFileByteIdentical(t *testing.T) {
	path := configPath(t)
	if _, _, err := userconfig.Load(path); err != nil {
		t.Fatalf("first Load() error = %v", err)
	}
	created := readFile(t, path)

	config, warnings, err := userconfig.Load(path)

	if err != nil || len(warnings) != 0 {
		t.Fatalf("second Load() warnings = %v, error = %v; want none", warnings, err)
	}
	if !reflect.DeepEqual(config, userconfig.Default()) {
		t.Errorf("second Load() = %#v, want Default()", config)
	}
	if after := readFile(t, path); after != created {
		t.Errorf("file after second Load =\n%s\nwant unchanged\n%s", after, created)
	}
}

func TestLoad_CreatedTemplate_NamesEveryRoleAndUncommentsCleanly(t *testing.T) {
	created := configPath(t)
	if _, _, err := userconfig.Load(created); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	template := readFile(t, created)
	path := writeConfig(t, uncomment(template))

	config, warnings, err := userconfig.Load(path)

	for _, role := range userconfig.Roles {
		if !strings.Contains(template, role) {
			t.Errorf("template does not name the role %q", role)
		}
	}
	if err != nil || len(warnings) != 0 {
		t.Fatalf("Load(uncommented) warnings = %v, error = %v; want none\n%s", warnings, err, uncomment(template))
	}
	if config.UI.Theme != userconfig.ThemeAuto || len(config.UI.Colors) == 0 {
		t.Errorf("Load(uncommented) = %#v, want theme auto and the example overrides", config)
	}
}

func TestLoad_ValidFile_ReturnsSettings(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    userconfig.UI
	}{
		{"empty file", "", userconfig.UI{Theme: "auto", Colors: map[string]string{}}},
		{"null ui", "ui:\n", userconfig.UI{Theme: "auto", Colors: map[string]string{}}},
		{"null theme and colors", "ui:\n  theme:\n  colors:\n", userconfig.UI{Theme: "auto", Colors: map[string]string{}}},
		{"dark theme", "ui:\n  theme: dark\n", userconfig.UI{Theme: "dark", Colors: map[string]string{}}},
		{"light theme", "ui:\n  theme: light\n", userconfig.UI{Theme: "light", Colors: map[string]string{}}},
		{
			"every colour form",
			"ui:\n  colors:\n    accent: \"#7aa2f7\"\n    border: '#AbC'\n    muted: 0\n    link: 255\n    code: \"12\"\n",
			userconfig.UI{Theme: "auto", Colors: map[string]string{
				"accent": "#7aa2f7", "border": "#AbC", "muted": "0", "link": "255", "code": "12",
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfig(t, test.content)

			config, warnings, err := userconfig.Load(path)

			if err != nil || len(warnings) != 0 {
				t.Fatalf("Load() warnings = %v, error = %v; want none", warnings, err)
			}
			if !reflect.DeepEqual(config.UI, test.want) {
				t.Errorf("Load().UI = %#v, want %#v", config.UI, test.want)
			}
		})
	}
}

func TestLoad_UnknownKeyOrRole_WarnsAndKeepsTheRest(t *testing.T) {
	tests := []struct {
		name    string
		content string
		line    int
		message string
	}{
		{"unknown top-level key", "editor: vim\nui:\n  theme: dark\n", 1, `unknown key "editor", ignored`},
		{"unknown ui key", "ui:\n  theme: dark\n  mouse: off\n", 3, `unknown key "mouse" in ui, ignored`},
		{
			"unknown colour role",
			"ui:\n  theme: dark\n  colors:\n    status-active: \"#fff\"\n",
			4, `unknown colour role "status-active", ignored`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfig(t, test.content)

			config, warnings, err := userconfig.Load(path)

			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			want := []userconfig.Warning{{Path: path, Line: test.line, Message: test.message}}
			if !reflect.DeepEqual(warnings, want) {
				t.Errorf("Load() warnings = %#v, want %#v", warnings, want)
			}
			if config.UI.Theme != userconfig.ThemeDark || len(config.UI.Colors) != 0 {
				t.Errorf("Load().UI = %#v, want theme dark and no overrides", config.UI)
			}
		})
	}
}

func TestWarning_String_RendersPathAndLine(t *testing.T) {
	tests := []struct {
		warning userconfig.Warning
		want    string
	}{
		{userconfig.Warning{Path: "c.yaml", Line: 3, Message: "m"}, "c.yaml:3: m"},
		{userconfig.Warning{Path: "c.yaml", Message: "m"}, "c.yaml: m"},
	}
	for _, test := range tests {
		if got := test.warning.String(); got != test.want {
			t.Errorf("String() = %q, want %q", got, test.want)
		}
	}
}

func TestLoad_InvalidFile_ErrorsWithLineOnDefaults(t *testing.T) {
	tests := []struct {
		name    string
		content string
		line    int
		message string
	}{
		{"malformed YAML", "ui:\n  theme: dark\n  colors: [\n", 3, "invalid YAML"},
		{"theme outside the presets", "ui:\n  theme: blue\n", 2, "ui.theme: expected auto, dark or light"},
		{"theme of the wrong type", "ui:\n  theme: [dark]\n", 2, "ui.theme: expected auto, dark or light"},
		{"ui of the wrong type", "ui: dark\n", 1, "ui: expected a mapping"},
		{"colors of the wrong type", "ui:\n  colors: [accent]\n", 2, "ui.colors: expected a mapping"},
		{"short hex colour", "ui:\n  colors:\n    accent: \"#12\"\n", 3, "ui.colors.accent: expected"},
		{"non-hex digits", "ui:\n  colors:\n    accent: \"#gggggg\"\n", 3, "ui.colors.accent: expected"},
		{"number above 255", "ui:\n  colors:\n    link: 256\n", 3, "ui.colors.link: expected"},
		{"negative number", "ui:\n  colors:\n    link: -1\n", 3, "ui.colors.link: expected"},
		{"unquoted hex colour", "ui:\n  colors:\n    accent: #fff\n", 3, "an unquoted # starts a comment"},
		{"colour name", "ui:\n  colors:\n    accent: red\n", 3, "ui.colors.accent: expected"},
		{"second document", "ui:\n  theme: dark\n---\nui:\n  theme: light\n", 3, "only one YAML document"},
		{"not a mapping", "- ui\n", 1, "the config must be a mapping"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeConfig(t, test.content)

			config, warnings, err := userconfig.Load(path)

			var failure *userconfig.Error
			if !errors.As(err, &failure) {
				t.Fatalf("Load() error = %v, want *userconfig.Error", err)
			}
			prefix := path + ":" + strconv.Itoa(test.line) + ": "
			if !strings.HasPrefix(err.Error(), prefix) || !strings.Contains(err.Error(), test.message) {
				t.Errorf("Load() error = %q, want prefix %q containing %q", err, prefix, test.message)
			}
			if len(warnings) != 0 || !reflect.DeepEqual(config, userconfig.Default()) {
				t.Errorf("Load() = %#v, warnings %v; want Default() and no warnings", config, warnings)
			}
		})
	}
}

func TestLoad_UnwritableDirectory_WarnsOnDefaults(t *testing.T) {
	skipWithoutPermissions(t)
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
	path := filepath.Join(parent, "capcom", "config.yaml")

	config, warnings, err := userconfig.Load(path)

	if err != nil {
		t.Fatalf("Load() error = %v, want a warning", err)
	}
	if len(warnings) != 1 || warnings[0].Path != path || warnings[0].Line != 0 ||
		!strings.Contains(warnings[0].Message, "cannot create the config file") {
		t.Errorf("Load() warnings = %#v, want one creation warning at %s", warnings, path)
	}
	if !reflect.DeepEqual(config, userconfig.Default()) {
		t.Errorf("Load() = %#v, want Default()", config)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("Stat(%s) error = %v, want the file absent", path, statErr)
	}
}

func TestLoad_UnreadableFile_ErrorsOnDefaults(t *testing.T) {
	skipWithoutPermissions(t)
	path := writeConfig(t, "ui:\n  theme: dark\n")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	config, _, err := userconfig.Load(path)

	var failure *userconfig.Error
	if !errors.As(err, &failure) || failure.Path != path {
		t.Fatalf("Load() error = %v, want *userconfig.Error at %s", err, path)
	}
	if !reflect.DeepEqual(config, userconfig.Default()) {
		t.Errorf("Load() = %#v, want Default()", config)
	}
}

func TestPackage_ImportedOnlyByUI(t *testing.T) {
	root := filepath.Join("..", "..")
	allowed := map[string]bool{
		filepath.Join(root, "internal", "ui"):         true,
		filepath.Join(root, "internal", "userconfig"): true,
	}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (allowed[path] || strings.HasPrefix(entry.Name(), ".")) && path != root {
			return filepath.SkipDir
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			if spec.Path.Value == strconv.Quote(importPath) {
				t.Errorf("%s imports %s; only internal/ui may (ADR 0011)", path, importPath)
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
}
