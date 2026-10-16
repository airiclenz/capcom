package clipboard

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// copiedText is the text every write in these tests hands over.
const copiedText = "copied text"

// run is one program start a recorder saw.
type run struct {
	program     string
	args        []string
	stdin       string
	hasDeadline bool
}

// recorder is a fake Run that records every start and answers with err.
type recorder struct {
	runs []run
	err  error
}

// Run records the start.
func (r *recorder) Run(ctx context.Context, stdin, program string, args ...string) error {
	_, hasDeadline := ctx.Deadline()
	r.runs = append(r.runs, run{program: program, args: args, stdin: stdin, hasDeadline: hasDeadline})
	return r.err
}

// makeProgram creates an empty file name in dir and returns its path.
func makeProgram(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, nil, 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// installed makes each name a real file under a fresh directory outside every Root, and returns a
// fake LookPath answering those names and exec.ErrNotFound for any other.
func installed(t *testing.T, names ...string) func(string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	paths := map[string]string{}
	for _, name := range names {
		paths[name] = makeProgram(t, dir, name)
	}
	return lookPathOf(paths)
}

// lookPathOf is a fake LookPath answering from paths, and exec.ErrNotFound for any other name.
func lookPathOf(paths map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if path, ok := paths[name]; ok {
			return path, nil
		}
		return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
	}
}

// environment is a fake Getenv answering from values.
func environment(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

// skipSymlinkTest skips a test that makes symlinks where that needs privileges (Windows) or where
// permission checks do not hold (root), as the store package's tests do.
func skipSymlinkTest(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests do not run on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("symlink tests do not run as root")
	}
}

func TestWriteNative_CandidateOrder(t *testing.T) {
	t.Parallel()
	every := []string{"wl-copy", "xclip", "xsel", "termux-clipboard-set", "clip.exe", "pbcopy"}
	wayland := map[string]string{"WAYLAND_DISPLAY": "wayland-0"}
	tests := []struct {
		name      string
		goos      string
		env       map[string]string
		installed []string
		program   string
		args      []string
	}{
		{"linux wayland runs wl-copy first", "linux", wayland, every, "wl-copy", nil},
		{"linux without wayland skips wl-copy", "linux", nil, every, "xclip", []string{"-in", "-selection", "clipboard"}},
		{"linux wayland without wl-copy runs xclip", "linux", wayland, []string{"xclip", "xsel"}, "xclip", []string{"-in", "-selection", "clipboard"}},
		{"linux falls to xsel", "linux", nil, []string{"xsel", "clip.exe"}, "xsel", []string{"--input", "--clipboard"}},
		{"android falls to termux", "android", nil, []string{"termux-clipboard-set", "clip.exe"}, "termux-clipboard-set", nil},
		{"wsl falls to clip.exe", "linux", nil, []string{"clip.exe", "pbcopy"}, "clip.exe", nil},
		{"freebsd uses the unix list", "freebsd", nil, every, "xclip", []string{"-in", "-selection", "clipboard"}},
		{"darwin runs pbcopy", "darwin", wayland, every, "pbcopy", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			runs := &recorder{}
			lookPath := installed(t, test.installed...)
			want, _ := lookPath(test.program)
			writer := Writer{
				GOOS:     test.goos,
				Getenv:   environment(test.env),
				Root:     t.TempDir(),
				LookPath: lookPath,
				Run:      runs.Run,
			}

			err := writer.WriteNative(copiedText)

			if err != nil {
				t.Fatalf("WriteNative: %v", err)
			}
			wantRuns := []run{{program: want, args: test.args, stdin: copiedText, hasDeadline: true}}
			assertRuns(t, runs.runs, wantRuns)
		})
	}
}

func TestWriteNative_NoProgram_ReturnsErrNoProgram(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		goos      string
		installed []string
	}{
		{"linux with nothing installed", "linux", nil},
		{"linux with only pbcopy", "linux", []string{"pbcopy"}},
		{"darwin without pbcopy", "darwin", []string{"xclip"}},
		{"an OS with no program route", "plan9", []string{"xclip", "pbcopy"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			runs := &recorder{}
			writer := Writer{
				GOOS:     test.goos,
				Getenv:   environment(nil),
				LookPath: installed(t, test.installed...),
				Run:      runs.Run,
			}

			err := writer.WriteNative(copiedText)

			if !errors.Is(err, ErrNoProgram) {
				t.Fatalf("WriteNative = %v, want ErrNoProgram", err)
			}
			assertRuns(t, runs.runs, nil)
		})
	}
}

func TestWriteNative_ProgramFailure_IsReturned(t *testing.T) {
	t.Parallel()
	failure := errors.New("exit status 1")
	runs := &recorder{err: failure}
	writer := Writer{
		GOOS:     "linux",
		Getenv:   environment(nil),
		LookPath: installed(t, "xclip", "xsel"),
		Run:      runs.Run,
	}

	err := writer.WriteNative(copiedText)

	if !errors.Is(err, failure) || len(runs.runs) != 1 {
		t.Fatalf("WriteNative = %v after %d runs, want the xclip failure after 1", err, len(runs.runs))
	}
}

func TestWriteNative_Fence(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		// lookPath answers xclip for the working tree root and an outside directory, both real.
		lookPath func(t *testing.T, root, outside string) (string, error)
	}{
		{"a program inside the working tree", func(t *testing.T, root, _ string) (string, error) {
			return makeProgram(t, root, "xclip"), nil
		}},
		{"a program in a subdirectory of the working tree", func(t *testing.T, root, _ string) (string, error) {
			if err := os.Mkdir(filepath.Join(root, "bin"), 0o755); err != nil {
				t.Fatalf("Mkdir: %v", err)
			}
			return makeProgram(t, filepath.Join(root, "bin"), "xclip"), nil
		}},
		{"a relative PATH entry", func(*testing.T, string, string) (string, error) {
			return "xclip", &exec.Error{Name: "xclip", Err: exec.ErrDot}
		}},
		{"a relative lookup result", func(*testing.T, string, string) (string, error) {
			return filepath.Join("bin", "xclip"), nil
		}},
		{"a missing path that cleans into the working tree", func(_ *testing.T, root, _ string) (string, error) {
			return strings.Join([]string{root, "missing", "..", "xclip"}, string(filepath.Separator)), nil
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root, outside := t.TempDir(), t.TempDir()
			path, lookErr := test.lookPath(t, root, outside)
			paths := map[string]string{"xsel": makeProgram(t, outside, "xsel")}
			runs := &recorder{}
			writer := Writer{
				GOOS:   "linux",
				Getenv: environment(nil),
				Root:   root,
				LookPath: func(name string) (string, error) {
					if name == "xclip" {
						return path, lookErr
					}
					return lookPathOf(paths)(name)
				},
				Run: runs.Run,
			}

			err := writer.WriteNative(copiedText)

			if !errors.Is(err, ErrRefused) {
				t.Fatalf("WriteNative = %v, want ErrRefused", err)
			}
			assertRuns(t, runs.runs, nil)
		})
	}
}

func TestWriteNative_FenceThroughSymlinks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		// arrange returns the Root to fence and the path LookPath answers for xclip.
		arrange func(t *testing.T, root, outside string) (string, string)
	}{
		{"a symlink outside pointing into the working tree", func(t *testing.T, root, outside string) (string, string) {
			link := filepath.Join(outside, "xclip")
			if err := os.Symlink(makeProgram(t, root, "xclip"), link); err != nil {
				t.Fatalf("Symlink: %v", err)
			}
			return root, link
		}},
		{"a working tree reached through a symlink", func(t *testing.T, root, outside string) (string, string) {
			link := filepath.Join(outside, "tree")
			if err := os.Symlink(root, link); err != nil {
				t.Fatalf("Symlink: %v", err)
			}
			return link, makeProgram(t, root, "xclip")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			skipSymlinkTest(t)
			fence, path := test.arrange(t, t.TempDir(), t.TempDir())
			runs := &recorder{}
			writer := Writer{
				GOOS:     "linux",
				Getenv:   environment(nil),
				Root:     fence,
				LookPath: lookPathOf(map[string]string{"xclip": path}),
				Run:      runs.Run,
			}

			err := writer.WriteNative(copiedText)

			if !errors.Is(err, ErrRefused) {
				t.Fatalf("WriteNative = %v, want ErrRefused", err)
			}
			assertRuns(t, runs.runs, nil)
		})
	}
}

func TestWriteNative_OutsideFence_Runs(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	tests := []struct {
		name string
		root string
		path string
	}{
		{"an empty Root fences nothing", "", makeProgram(t, outside, "xclip")},
		{"a program outside the working tree", t.TempDir(), makeProgram(t, t.TempDir(), "xclip")},
		{
			"a missing path that cleans outside the working tree",
			t.TempDir(),
			strings.Join([]string{outside, "missing", "..", "gone"}, string(filepath.Separator)),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			runs := &recorder{}
			writer := Writer{
				GOOS:     "linux",
				Getenv:   environment(nil),
				Root:     test.root,
				LookPath: lookPathOf(map[string]string{"xclip": test.path}),
				Run:      runs.Run,
			}

			err := writer.WriteNative(copiedText)

			if err != nil {
				t.Fatalf("WriteNative: %v", err)
			}
			wantArgs := []string{"-in", "-selection", "clipboard"}
			assertRuns(t, runs.runs, []run{{program: test.path, args: wantArgs, stdin: copiedText, hasDeadline: true}})
		})
	}
}

func TestWriteNative_Windows_CallsWriteWindowsOnly(t *testing.T) {
	t.Parallel()
	runs := &recorder{}
	var written []string
	writer := Writer{
		GOOS:   "windows",
		Getenv: environment(map[string]string{"TMUX": "/tmp/tmux-1000/default"}),
		LookPath: func(name string) (string, error) {
			t.Errorf("LookPath(%q) called on windows", name)
			return "", exec.ErrNotFound
		},
		Run: runs.Run,
		WriteWindows: func(text string) error {
			written = append(written, text)
			return nil
		},
	}

	err := writer.WriteNative(copiedText)

	if err != nil {
		t.Fatalf("WriteNative: %v", err)
	}
	if !slices.Equal(written, []string{copiedText}) {
		t.Errorf("WriteWindows got %q, want one write of %q", written, copiedText)
	}
	assertRuns(t, runs.runs, nil)
}

func TestWriteNative_WindowsFailure_IsReturned(t *testing.T) {
	t.Parallel()
	failure := errors.New("OpenClipboard failed")
	writer := Writer{
		GOOS:         "windows",
		Getenv:       environment(nil),
		WriteWindows: func(string) error { return failure },
	}

	err := writer.WriteNative(copiedText)

	if !errors.Is(err, failure) {
		t.Fatalf("WriteNative = %v, want the WriteWindows failure", err)
	}
}

func TestWriteTmux_WithoutTMUX_DoesNothing(t *testing.T) {
	t.Parallel()
	runs := &recorder{}
	writer := Writer{
		GOOS:   "linux",
		Getenv: environment(nil),
		LookPath: func(name string) (string, error) {
			t.Errorf("LookPath(%q) called outside tmux", name)
			return "", exec.ErrNotFound
		},
		Run: runs.Run,
	}

	err := writer.WriteTmux(copiedText)

	if err != nil {
		t.Fatalf("WriteTmux: %v", err)
	}
	assertRuns(t, runs.runs, nil)
}

func TestWriteTmux_WithTMUX_LoadsTheBuffer(t *testing.T) {
	t.Parallel()
	runs := &recorder{}
	lookPath := installed(t, "tmux")
	tmux, _ := lookPath("tmux")
	writer := Writer{
		GOOS:     "linux",
		Getenv:   environment(map[string]string{"TMUX": "/tmp/tmux-1000/default,1,0"}),
		Root:     t.TempDir(),
		LookPath: lookPath,
		Run:      runs.Run,
	}

	err := writer.WriteTmux(copiedText)

	if err != nil {
		t.Fatalf("WriteTmux: %v", err)
	}
	wantArgs := []string{"load-buffer", "-w", "-"}
	assertRuns(t, runs.runs, []run{{program: tmux, args: wantArgs, stdin: copiedText, hasDeadline: true}})
}

func TestWriteTmux_Failures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		// lookPath answers tmux, given the working tree root.
		lookPath func(t *testing.T, root string) func(string) (string, error)
		want     error
	}{
		{"tmux not on PATH", func(t *testing.T, _ string) func(string) (string, error) {
			return installed(t)
		}, ErrNoProgram},
		{"tmux inside the working tree", func(t *testing.T, root string) func(string) (string, error) {
			return lookPathOf(map[string]string{"tmux": makeProgram(t, root, "tmux")})
		}, ErrRefused},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			runs := &recorder{}
			writer := Writer{
				GOOS:     "linux",
				Getenv:   environment(map[string]string{"TMUX": "/tmp/tmux-1000/default,1,0"}),
				Root:     root,
				LookPath: test.lookPath(t, root),
				Run:      runs.Run,
			}

			err := writer.WriteTmux(copiedText)

			if !errors.Is(err, test.want) {
				t.Fatalf("WriteTmux = %v, want %v", err, test.want)
			}
			assertRuns(t, runs.runs, nil)
		})
	}
}

// assertRuns fails the test unless got holds exactly the starts in want, in order.
func assertRuns(t *testing.T, got, want []run) {
	t.Helper()
	equal := slices.EqualFunc(got, want, func(a, b run) bool {
		return a.program == b.program && slices.Equal(a.args, b.args) &&
			a.stdin == b.stdin && a.hasDeadline == b.hasDeadline
	})
	if !equal {
		t.Errorf("runs = %+v, want %+v", got, want)
	}
}
