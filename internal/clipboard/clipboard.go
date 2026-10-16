package clipboard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	atotto "github.com/atotto/clipboard"

	"github.com/airiclenz/capcom/internal/store"
)

// ErrNoProgram is what a write returns when this machine has none of the programs it would run:
// no candidate for its OS is on PATH, or the OS has no program route at all.
var ErrNoProgram = errors.New("clipboard: no clipboard program available")

// ErrRefused is what a write returns when the program PATH names for it resolves inside the
// working tree, or is found through a relative PATH entry. Nothing runs.
var ErrRefused = errors.New("clipboard: program refused")

// runTimeout bounds one clipboard program: a helper that wedges waiting on a clipboard owner, or a
// wedged tmux server, must not leave a copy waiting forever.
const runTimeout = 2 * time.Second

// windowsGOOS is the one OS whose clipboard is written through the Win32 API, not a program.
const windowsGOOS = "windows"

// darwinGOOS is the OS whose clipboard is written through pbcopy.
const darwinGOOS = "darwin"

// candidate is one clipboard program and the arguments that make it read the text to copy from
// its standard input (atotto/clipboard v0.1.4's copy argv).
type candidate struct {
	program string
	args    []string
}

var (
	wlCopyCandidate             = candidate{program: "wl-copy"}
	xclipCandidate              = candidate{program: "xclip", args: []string{"-in", "-selection", "clipboard"}}
	xselCandidate               = candidate{program: "xsel", args: []string{"--input", "--clipboard"}}
	termuxClipboardSetCandidate = candidate{program: "termux-clipboard-set"}
	clipExeCandidate            = candidate{program: "clip.exe"}
	pbcopyCandidate             = candidate{program: "pbcopy"}
	tmuxCandidate               = candidate{program: "tmux", args: []string{"load-buffer", "-w", "-"}}
)

// unixGOOS is the set of operating systems served by the unix program ladder: atotto/clipboard
// v0.1.4's clipboard_unix.go build tags, plus the GOOS values Go builds under them (android under
// linux, illumos under solaris).
var unixGOOS = map[string]bool{
	"linux": true, "android": true, "freebsd": true, "netbsd": true, "openbsd": true,
	"solaris": true, "illumos": true, "dragonfly": true,
}

// errUnavailable marks a program PATH does not have; the native walk passes to the next candidate.
var errUnavailable = errors.New("not available")

// Writer writes copied text to the native clipboard and to tmux. Every input is a field; a nil
// function field, an empty GOOS, means the production default named on it. Root is the working
// tree no program may resolve inside; an empty Root fences nothing. A Writer is safe to copy and
// to use from several goroutines, as long as its functions are.
type Writer struct {
	// GOOS picks the program list. Empty means runtime.GOOS.
	GOOS string
	// Getenv reads the environment: WAYLAND_DISPLAY puts wl-copy first on the unix list, and TMUX
	// decides whether WriteTmux runs at all. Nil means os.Getenv.
	Getenv func(string) string
	// Root is the working tree root, the fence no program may resolve inside.
	Root string
	// LookPath resolves a program name on PATH. Nil means exec.LookPath.
	LookPath func(string) (string, error)
	// Run starts program with args, stdin on its standard input, and waits for it or for ctx.
	// program is the absolute path the lookup answered, never a bare name. Nil means runProgram.
	Run func(ctx context.Context, stdin, program string, args ...string) error
	// WriteWindows writes the Windows clipboard. Nil means atotto/clipboard's WriteAll.
	WriteWindows func(string) error
}

// WriteNative writes text to the host's clipboard. On Windows it calls WriteWindows and runs no
// program. On darwin it runs pbcopy; on the unix family the first of wl-copy (only with
// WAYLAND_DISPLAY set), xclip, xsel, termux-clipboard-set and clip.exe that PATH has, under a 2 s
// timeout.
//
// It returns ErrNoProgram when no candidate is on PATH, an error wrapping ErrRefused when the
// first candidate PATH has is refused (the walk stops there: a poisoned PATH is worth reporting,
// not stepping around), and the program's own error otherwise.
func (w Writer) WriteNative(text string) error {
	if w.goos() == windowsGOOS {
		return w.writeWindows()(text)
	}

	for _, program := range w.nativeCandidates() {
		path, err := w.resolve(program.program)
		if errors.Is(err, errUnavailable) {
			continue
		}
		if err != nil {
			return err
		}
		return w.runBounded(text, path, program)
	}
	return ErrNoProgram
}

// WriteTmux runs `tmux load-buffer -w -` with text on its standard input, under a 2 s timeout,
// when TMUX is set: tmux's own clipboard write, which it forwards to the outer terminal where an
// application's OSC 52 may be dropped. Outside tmux it looks nothing up, runs nothing and returns
// nil. It returns an error wrapping ErrNoProgram when tmux is not on PATH, one wrapping ErrRefused
// when it is refused, and tmux's own error otherwise.
func (w Writer) WriteTmux(text string) error {
	if w.getenv("TMUX") == "" {
		return nil
	}

	path, err := w.resolve(tmuxCandidate.program)
	if errors.Is(err, errUnavailable) {
		return fmt.Errorf("%w: %w", ErrNoProgram, err)
	}
	if err != nil {
		return err
	}
	return w.runBounded(text, path, tmuxCandidate)
}

// nativeCandidates is the ordered program list for the Writer's OS; empty where the OS has no
// program route.
func (w Writer) nativeCandidates() []candidate {
	goos := w.goos()
	if goos == darwinGOOS {
		return []candidate{pbcopyCandidate}
	}
	if !unixGOOS[goos] {
		return nil
	}

	candidates := make([]candidate, 0, 5)
	if w.getenv("WAYLAND_DISPLAY") != "" {
		candidates = append(candidates, wlCopyCandidate)
	}
	return append(candidates, xclipCandidate, xselCandidate, termuxClipboardSetCandidate, clipExeCandidate)
}

// resolve looks name up on PATH and fences the answer. It returns the path the lookup answered,
// which is what runs (a multi-call binary reads its name from it); an error wrapping
// errUnavailable when PATH does not have name; and an error wrapping ErrRefused when name is
// found through a relative PATH entry, or its real path lies inside Root.
func (w Writer) resolve(name string) (string, error) {
	path, err := w.lookPath()(name)
	if errors.Is(err, exec.ErrDot) {
		return "", fmt.Errorf("%w: %s is found through a relative PATH entry", ErrRefused, name)
	}
	if err != nil {
		return "", fmt.Errorf("%s %w: %w", name, errUnavailable, err)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%w: %s resolves to the relative path %s", ErrRefused, name, path)
	}
	if w.Root == "" {
		return path, nil
	}

	real, root := realPath(path), realPath(w.Root)
	if _, isInside := store.RelativePath(root, real); isInside {
		return "", fmt.Errorf("%w: %s resolves to %s, inside the working tree %s", ErrRefused, name, real, root)
	}
	return path, nil
}

// runBounded runs the resolved program with its arguments and text on its standard input, under
// runTimeout.
func (w Writer) runBounded(text, path string, program candidate) error {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	if err := w.runner()(ctx, text, path, program.args...); err != nil {
		return fmt.Errorf("clipboard: %s: %w", path, err)
	}
	return nil
}

// realPath resolves path through symlinks. When path does not exist it resolves the nearest
// existing ancestor and joins the rest back on, as apogee's EvalRealPath does, so a path under a
// symlinked directory (macOS's /var) still compares with a resolved root; with no existing
// ancestor it is the cleaned path.
func realPath(path string) string {
	cleaned := filepath.Clean(path)
	remainder := ""
	for current := cleaned; ; {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(resolved, remainder)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return cleaned
		}
		remainder = filepath.Join(filepath.Base(current), remainder)
		current = parent
	}
}

// goos is w.GOOS, or runtime.GOOS when it is empty.
func (w Writer) goos() string {
	if w.GOOS == "" {
		return runtime.GOOS
	}
	return w.GOOS
}

// getenv reads key through w.Getenv, or os.Getenv when it is nil.
func (w Writer) getenv(key string) string {
	if w.Getenv == nil {
		return os.Getenv(key)
	}
	return w.Getenv(key)
}

// lookPath is w.LookPath, or exec.LookPath when it is nil.
func (w Writer) lookPath() func(string) (string, error) {
	if w.LookPath == nil {
		return exec.LookPath
	}
	return w.LookPath
}

// runner is w.Run, or runProgram when it is nil.
func (w Writer) runner() func(context.Context, string, string, ...string) error {
	if w.Run == nil {
		return runProgram
	}
	return w.Run
}

// writeWindows is w.WriteWindows, or atotto/clipboard's WriteAll when it is nil.
func (w Writer) writeWindows() func(string) error {
	if w.WriteWindows == nil {
		return atotto.WriteAll
	}
	return w.WriteWindows
}

// runProgram is the production runner: it starts program with args and stdin on its standard
// input, and waits for it or for ctx to kill it. Its standard output and error are the null device
// (a nil stream in exec.Cmd), so a helper that prints a warning never writes into the frame the
// UI is drawing.
func runProgram(ctx context.Context, stdin, program string, args ...string) error {
	command := exec.CommandContext(ctx, program, args...)
	command.Stdin = strings.NewReader(stdin)
	return command.Run()
}
