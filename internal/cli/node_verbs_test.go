package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHelp_NodeVerbs_MatchGolden pins the whole --help of the mv, rm and create verbs of both Tree
// kinds byte for byte, so the issue and Note commands keep their wording when they share code.
// The golden files under testdata/help were written once from the tree before that sharing.
func TestHelp_NodeVerbs_MatchGolden(t *testing.T) {
	cases := map[string][]string{
		"mv":       {"mv"},
		"rm":       {"rm"},
		"create":   {"create"},
		"know-mv":  {"know", "mv"},
		"know-rm":  {"know", "rm"},
		"know-add": {"know", "add"},
	}
	for golden, verb := range cases {
		t.Run(golden, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", "help", golden+".txt"))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}

			stdout := mustRun(t, "", append(verb, "--help")...)

			if stdout != string(want) {
				t.Errorf("capcom %s --help differs from testdata/help/%s.txt\ngot:\n%s\nwant:\n%s",
					strings.Join(verb, " "), golden, stdout, want)
			}
		})
	}
}
