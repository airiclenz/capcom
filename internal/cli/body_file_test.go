package cli

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBodyFile_MissingFile_HintsAStdinRetry(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flag     string
		wantHint string
		takesKV  bool
	}{
		{
			name:     "create",
			args:     []string{"create", "--id", "from-file", "--title", "From file"},
			flag:     "--body-file",
			wantHint: "capcom create --body-file -",
			takesKV:  true,
		},
		{
			name:     "update",
			args:     []string{"update", "release-flow"},
			flag:     "--body-append-file",
			wantHint: "capcom update --body-append-file -",
			takesKV:  true,
		},
		{
			name:     "know add",
			args:     []string{"know", "add", "--id", "from-file", "--title", "From file"},
			flag:     "--body-file",
			wantHint: "capcom know add --body-file -",
		},
		{
			name:     "know update body",
			args:     []string{"know", "update", "cli-tests"},
			flag:     "--body-file",
			wantHint: "capcom know update --body-file -",
		},
		{
			name:     "know update append",
			args:     []string{"know", "update", "cli-tests"},
			flag:     "--body-append-file",
			wantHint: "capcom know update --body-append-file -",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newKnowStore(t)
			missing := filepath.Join(t.TempDir(), "missing.md")
			args := slices.Concat(test.args, []string{test.flag, missing})

			_, stderr, exitCode := runCLI(t, root, args...)
			jsonHint := jsonErrorHint(t, root, args...)

			if exitCode != ExitNotFound || lastLine(stderr) != "hint: "+test.wantHint {
				t.Errorf("exit = %d stderr = %q, want %d ending in hint: %s", exitCode, stderr, ExitNotFound, test.wantHint)
			}
			if jsonHint != test.wantHint {
				t.Errorf("--json hint = %q, want %q", jsonHint, test.wantHint)
			}
			if !test.takesKV {
				return
			}
			stdout, _, _ := runCLI(t, root, append(args, "--kv")...)
			if !strings.Contains(stdout, "\nHINT="+test.wantHint+"\nSTATUS=not_found\n") {
				t.Errorf("--kv stdout = %q, want HINT=%s", stdout, test.wantHint)
			}
		})
	}
}
