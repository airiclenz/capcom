package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/airiclenz/capcom"
)

// The golden harness pins the --json envelope of each kind, and the --kv records of each command
// that offers --kv, byte for byte: a test runs a command against a fixed store and hands its stdout
// to assertGoldenJSON, which compares it with testdata/json/<name>.golden.json, or to
// assertGoldenKV, which compares it with testdata/kv/<command>.golden, after replacing the values
// that change from run to run. Run go test ./internal/cli -run Golden -update to rewrite the files
// from the commands' output.
//
// The volatile values and the one regexp that replaces each, in this order; an RFC 3339 value of
// a KEY=VALUE line, such as SINCE= or HEARTBEAT=, is a <TIME> through timePattern too:
//   - <DIR>: the store's temp root, matched in its JSON-escaped form and then in its native form;
//     the rest of the path after the root, up to the end of the JSON string, is turned into
//     slashes, so a golden file reads the same on Windows (dirPattern).
//   - <TIME>: an RFC 3339 timestamp, such as an Issue's created time (timePattern).
//   - <DATE>: the date of a Log entry line, "- YYYY-MM-DD Author: text" (logDatePattern).
//   - <ETAG>: the value of an "etag" member (etagPattern).
//   - <VERSION>: the release version as a JSON string, so a VERSION bump leaves the files green
//     (versionPattern).
//   - <TOKEN>: the value of a lock's "token" member (tokenPattern).
//   - <USER>@<HOST>: the value of a lock's "holder" member, user@host (holderPattern).
//   - <HOST>: the value of a lock's "host" member (hostPattern).
//   - <USER>: the value of a lock's "user" member (userPattern).
//   - <ETAG>: the value of an ETAG= line (kvETagPattern).
//   - <VERSION>: the value of a VERSION= line (kvVersionPattern).
//   - <TOKEN>: the value of a TOKEN= line (kvTokenPattern).
//   - <USER>@<HOST>: the value of a HOLDER= line when it is user@host (kvHolderPattern).

// isGoldenUpdate rewrites the golden files under testdata/json and testdata/kv from the commands'
// output: go test ./internal/cli -run Golden -update.
var isGoldenUpdate = flag.Bool("update", false, "rewrite the golden files under testdata/json and testdata/kv")

// goldenJSONDir holds the expected --json envelopes, one <name>.golden.json file per name. It is
// made absolute when the test binary starts, in the package directory, since runCLI changes the
// working directory to the store.
var goldenJSONDir = absolutePath(filepath.Join("testdata", "json"))

// absolutePath returns path made absolute against the working directory.
func absolutePath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		panic(fmt.Sprintf("Abs %s: %v", path, err))
	}
	return absolute
}

// goldenKVDir holds the expected --kv output, one <command>.golden file per command that offers
// --kv; it is made absolute as goldenJSONDir is.
var goldenKVDir = absolutePath(filepath.Join("testdata", "kv"))

// goldenJSONSuffix ends every golden file name under goldenJSONDir.
const goldenJSONSuffix = ".golden.json"

// goldenKVSuffix ends every golden file name under goldenKVDir.
const goldenKVSuffix = ".golden"

// The placeholders that stand for the volatile values in a golden file.
const (
	placeholderDir     = "<DIR>"
	placeholderTime    = "<TIME>"
	placeholderDate    = "<DATE>"
	placeholderETag    = "<ETAG>"
	placeholderVersion = "<VERSION>"
	placeholderToken   = "<TOKEN>"
	placeholderHost    = "<HOST>"
	placeholderUser    = "<USER>"
)

var (
	// timePattern matches an RFC 3339 timestamp, with or without fractional seconds.
	timePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`)
	// logDatePattern matches the date that starts a Log entry line, keeping the "- " before it
	// and the space after it.
	logDatePattern = regexp.MustCompile(`(- )\d{4}-\d{2}-\d{2}( )`)
	// etagPattern matches the value of an "etag" member.
	etagPattern = regexp.MustCompile(`("etag":")[0-9a-f]+(")`)
	// versionPattern matches the release version as a JSON string.
	versionPattern = regexp.MustCompile(`"` + regexp.QuoteMeta(capcom.Version()) + `"`)
	// tokenPattern matches the value of a lock's "token" member.
	tokenPattern = regexp.MustCompile(`("token":")[0-9a-f]+(")`)
	// holderPattern matches the value of a lock's "holder" member when it is user@host.
	holderPattern = regexp.MustCompile(`("holder":")[^"@]+@[^"]+(")`)
	// hostPattern matches the value of a lock's "host" member.
	hostPattern = regexp.MustCompile(`("host":")[^"]+(")`)
	// userPattern matches the value of a lock's "user" member.
	userPattern = regexp.MustCompile(`("user":")[^"]+(")`)
	// kvETagPattern matches the value of an ETAG= line.
	kvETagPattern = regexp.MustCompile(`(?m)^(ETAG=)[0-9a-f]+$`)
	// kvVersionPattern matches the release version as the value of a VERSION= line.
	kvVersionPattern = regexp.MustCompile(`(?m)^(VERSION=)` + regexp.QuoteMeta(capcom.Version()) + `$`)
	// kvTokenPattern matches the value of a TOKEN= line.
	kvTokenPattern = regexp.MustCompile(`(?m)^(TOKEN=)[0-9a-f]+$`)
	// kvHolderPattern matches the value of a HOLDER= line when it is user@host.
	kvHolderPattern = regexp.MustCompile(`(?m)^(HOLDER=)[^@\n]+@.+$`)
)

// dirPattern matches root followed by the rest of its JSON string value (everything up to the
// closing quote), with root in the form given.
func dirPattern(root string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(root) + `((?:\\.|[^"\\])*)`)
}

// normaliseGolden replaces the volatile values of got, a command's stdout run in the store at
// root, with their placeholders; an empty root replaces no paths.
func normaliseGolden(got, root string) string {
	if root != "" {
		got = replaceDir(got, jsonEscape(root), func(rest string) string {
			return strings.ReplaceAll(rest, `\\`, "/")
		})
		got = replaceDir(got, root, filepath.ToSlash)
	}
	got = timePattern.ReplaceAllString(got, placeholderTime)
	got = logDatePattern.ReplaceAllString(got, "${1}"+placeholderDate+"${2}")
	got = etagPattern.ReplaceAllString(got, "${1}"+placeholderETag+"${2}")
	got = versionPattern.ReplaceAllString(got, `"`+placeholderVersion+`"`)
	got = tokenPattern.ReplaceAllString(got, "${1}"+placeholderToken+"${2}")
	got = holderPattern.ReplaceAllString(got, "${1}"+placeholderUser+"@"+placeholderHost+"${2}")
	got = hostPattern.ReplaceAllString(got, "${1}"+placeholderHost+"${2}")
	got = userPattern.ReplaceAllString(got, "${1}"+placeholderUser+"${2}")
	got = kvETagPattern.ReplaceAllString(got, "${1}"+placeholderETag)
	got = kvVersionPattern.ReplaceAllString(got, "${1}"+placeholderVersion)
	got = kvTokenPattern.ReplaceAllString(got, "${1}"+placeholderToken)
	return kvHolderPattern.ReplaceAllString(got, "${1}"+placeholderUser+"@"+placeholderHost)
}

// replaceDir replaces every occurrence of root in got with <DIR>, followed by the rest of the
// path passed through toSlash.
func replaceDir(got, root string, toSlash func(string) string) string {
	pattern := dirPattern(root)
	return pattern.ReplaceAllStringFunc(got, func(match string) string {
		return placeholderDir + toSlash(strings.TrimPrefix(match, root))
	})
}

// jsonEscape returns text as encoding/json writes it inside a string, without the quotes.
func jsonEscape(text string) string {
	encoded, err := json.Marshal(text)
	if err != nil {
		panic(fmt.Sprintf("marshal %q: %v", text, err))
	}
	return string(encoded[1 : len(encoded)-1])
}

// compareGolden normalises got, a command's stdout run in the store at root, and compares it byte
// for byte with want, a golden file's content; it returns an error showing both when they differ.
func compareGolden(want, got, root string) error {
	normalised := normaliseGolden(got, root)
	if normalised == want {
		return nil
	}
	return fmt.Errorf("output differs from the golden file\n--- got ---\n%s\n--- want ---\n%s", normalised, want)
}

// assertGoldenJSON compares stdout, run in the store at root, with testdata/json/<name>.golden.json;
// with -update it writes the normalised stdout to that file instead. name is the kind, with a
// variant suffix such as next-empty where one kind needs two files.
func assertGoldenJSON(t *testing.T, name, root, stdout string) {
	t.Helper()
	assertGolden(t, filepath.Join(goldenJSONDir, name+goldenJSONSuffix), root, stdout)
}

// assertGoldenKV compares stdout, a --kv run in the store at root, with
// testdata/kv/<name>.golden; with -update it writes the normalised stdout to that file instead.
// name is the command's path below capcom, spaces as dashes, such as lock-acquire.
func assertGoldenKV(t *testing.T, name, root, stdout string) {
	t.Helper()
	assertGolden(t, filepath.Join(goldenKVDir, name+goldenKVSuffix), root, stdout)
}

// assertGolden compares stdout, run in the store at root, with the golden file at path; with
// -update it writes the normalised stdout to that file instead.
func assertGolden(t *testing.T, path, root, stdout string) {
	t.Helper()
	if *isGoldenUpdate {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
		if err := os.WriteFile(path, []byte(normaliseGolden(stdout, root)), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s (run go test ./internal/cli -run Golden -update to create it): %v", path, err)
	}
	if err := compareGolden(string(want), stdout, root); err != nil {
		t.Errorf("%s: %v", path, err)
	}
}

// goldenPlan is the basename of the plan the golden store's session-timeout belongs to.
const goldenPlan = "golden-plan"

// newGoldenStore returns the root of a committed git repository holding a fixed capcom store:
// a standing parent with two children, one waiting on the other, a Log entry, a Related link, a
// closed Issue, a plan, and a Note with a child whose ref names a committed source file. Ids,
// titles and bodies are fixed, so only the values normaliseGolden replaces vary between runs.
func newGoldenStore(t *testing.T) string {
	t.Helper()
	root := newGitRepo(t)
	mustRun(t, root, "init")
	writeGoldenFile(t, root, "internal/auth/refresh.go", "package auth\n")
	writeGoldenFile(t, root, "docs/plans/"+goldenPlan+".md", "# Golden plan\n")
	noteBody := filepath.Join(t.TempDir(), "note-body.md")
	if err := os.WriteFile(noteBody, []byte("The refresh timer starts at login.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", noteBody, err)
	}

	mustRun(t, root, "create", "--id", "login-flow", "--title", "Login flow", "--standing",
		"--body", "Groups the login work.")
	mustRun(t, root, "create", "--id", "session-timeout", "--parent", "login-flow",
		"--title", "Session timeout races the refresh", "--type", "bug", "--priority", "1",
		"--label", "auth", "--plan", goldenPlan,
		"--body", "The refresh in internal/auth/refresh.go fires after logout.",
		"--acceptance", "No refresh after logout")
	mustRun(t, root, "create", "--id", "refresh-retry", "--parent", "login-flow",
		"--title", "Retry a failed refresh", "--dep", "session-timeout", "--body", "Retry once on failure.")
	mustRun(t, root, "create", "--id", "login-copy", "--title", "Reword the login errors",
		"--priority", "3", "--body", "The errors say too little.")
	mustRun(t, root, "create", "--id", "old-login-page", "--title", "Remove the old login page",
		"--body", "The old page is unused.")
	mustRun(t, root, "close", "old-login-page", "--reason", "removed")
	mustRun(t, root, "log", "session-timeout", "reproduced with two tabs")
	mustRun(t, root, "related", "add", "login-copy", "session-timeout")
	mustRun(t, root, "know", "add", "--id", "auth-refresh", "--title", "How the auth refresh works",
		"--label", "auth", "--ref", "internal/auth", "--body-file", noteBody)
	mustRun(t, root, "know", "add", "--id", "token-lifetimes", "--parent", "auth-refresh",
		"--title", "Token lifetimes")

	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-q", "-m", "golden store")
	return root
}

// writeGoldenFile writes content to the slash path name below root, creating its directories.
func writeGoldenFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func TestGoldenHarness_CompareGolden_ReportsOnlyRealChanges(t *testing.T) {
	root := filepath.Join(string(filepath.Separator)+"tmp", "golden root")
	want := `{"kind":"issue","data":{"path":"<DIR>/.capcom","created":"<TIME>","etag":"<ETAG>",` +
		`"body":"- <DATE> Test Author: done","version":"<VERSION>",` +
		`"lock":{"holder":"<USER>@<HOST>","host":"<HOST>","user":"<USER>","token":"<TOKEN>"}}}`
	got := `{"kind":"issue","data":{"path":"` + jsonEscape(filepath.Join(root, ".capcom")) + `",` +
		`"created":"2026-11-05T07:58:12Z","etag":"803b58eea3012260",` +
		`"body":"- 2026-11-05 Test Author: done","version":"` + capcom.Version() + `",` +
		`"lock":{"holder":"ada@build-host","host":"build-host","user":"ada","token":"64fe804b1b93ad6677b7974a424b3822"}}}`
	tests := []struct {
		name      string
		got       string
		isChanged bool
	}{
		{name: "volatile values only", got: got, isChanged: false},
		{name: "renamed field", got: strings.Replace(got, `"created"`, `"created_at"`, 1), isChanged: true},
		{name: "added member", got: strings.Replace(got, `"data":{`, `"elsewhere":[],"data":{`, 1), isChanged: true},
		{name: "null for an object", got: `{"kind":"issue","data":null}`, isChanged: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := compareGolden(want, test.got, root)

			if (err != nil) != test.isChanged {
				t.Errorf("compareGolden error = %v, want a change reported: %t", err, test.isChanged)
			}
		})
	}
}
