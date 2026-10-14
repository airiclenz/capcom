package cli

import (
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// formatRaisedText is the warning line of the related add that raises a store to format 2.
const formatRaisedText = "warning: .capcom/config.yaml: store format raised to 2; " +
	"capcom releases that read only format 1 can no longer read this store\n"

// newRelatedStore creates a store of three free issues, db-schema with an alias.
func newRelatedStore(t *testing.T) string {
	t.Helper()
	return newIssueStore(t, map[string]string{
		"build-pipeline.md": readIssue("Build pipeline", "open", "", ""),
		"db-schema.md":      readIssue("Schema", "open", "aliases: [apogee-kk0.4]\n", ""),
		"login-flow.md":     readIssue("Login flow", "open", "", ""),
	})
}

// showRelated runs capcom show --json on id and returns its related list.
func showRelated(t *testing.T, root string, id string) []string {
	t.Helper()
	stdout := mustRun(t, root, "show", "--json", id)
	shown := decodeEnvelope(t, stdout)["data"].([]any)[0].(map[string]any)
	values, isList := shown["related"].([]any)
	if !isList {
		t.Fatalf("show %s data has no related list: %v", id, shown)
	}
	related := make([]string, 0, len(values))
	for _, value := range values {
		related = append(related, value.(string))
	}
	return related
}

// writeStoreConfig replaces the store's config.yaml with content.
func writeStoreConfig(t *testing.T, root string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".capcom", "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// readStoreConfig returns the store's config.yaml.
func readStoreConfig(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".capcom", "config.yaml"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}

func TestRelatedAdd_ThroughAlias_StoresTheCanonicalIDOnTheFirstIssueOnly(t *testing.T) {
	root := newRelatedStore(t)

	mustRun(t, root, "related", "add", "login-flow", "apogee-kk0.4")

	if content := readStoreFile(t, root, "login-flow.md"); !strings.Contains(content, "related: [db-schema]\n") {
		t.Errorf("login-flow.md = %q", content)
	}
	if content := readStoreFile(t, root, "db-schema.md"); strings.Contains(content, "related:") {
		t.Errorf("db-schema.md = %q, want no related key", content)
	}
}

func TestShow_RelatedLink_ListedOnBothIssues(t *testing.T) {
	root := newRelatedStore(t)
	mustRun(t, root, "related", "add", "login-flow", "db-schema")

	holder := showRelated(t, root, "login-flow")
	other := showRelated(t, root, "db-schema")
	text := mustRun(t, root, "show", "db-schema")

	if !slices.Equal(holder, []string{"db-schema"}) || !slices.Equal(other, []string{"login-flow"}) {
		t.Errorf("related: login-flow = %v, db-schema = %v", holder, other)
	}
	if !strings.Contains(text, "\nrelated: [login-flow]\n") {
		t.Errorf("show db-schema text = %q, want a related line", text)
	}
}

func TestShow_OwnAndReverseLinks_OwnFirstThenSortedOnce(t *testing.T) {
	root := newRelatedStore(t)
	writeIssueFiles(t, root, map[string]string{
		"login-flow.md": readIssue("Login flow", "open", "related: [db-schema]\n", ""),
		"db-schema.md":  readIssue("Schema", "open", "aliases: [apogee-kk0.4]\nrelated: [login-flow]\n", ""),
	})
	mustRun(t, root, "related", "add", "build-pipeline", "login-flow")

	related := showRelated(t, root, "login-flow")

	if want := []string{"db-schema", "build-pipeline"}; !slices.Equal(related, want) {
		t.Errorf("related = %v, want %v", related, want)
	}
}

func TestShow_NoLinks_RelatedIsAnEmptyList(t *testing.T) {
	root := newRelatedStore(t)

	related := showRelated(t, root, "login-flow")
	text := mustRun(t, root, "show", "login-flow")

	if len(related) != 0 || strings.Contains(text, "related:") {
		t.Errorf("related = %v text = %q, want none", related, text)
	}
}

func TestRelatedAdd_Self_FailsInvalid(t *testing.T) {
	root := newRelatedStore(t)

	_, stderr, exitCode := runCLI(t, root, "related", "add", "db-schema", "apogee-kk0.4")

	want := "capcom: .capcom/issues/db-schema.md: db-schema cannot be related to itself\n"
	if exitCode != ExitError || stderr != want {
		t.Errorf("exit = %d stderr = %q, want %q", exitCode, stderr, want)
	}
}

func TestRelatedAdd_AlreadyLinkedEitherWay_LeavesTheLinkAsItIs(t *testing.T) {
	root := newRelatedStore(t)
	mustRun(t, root, "related", "add", "login-flow", "db-schema")

	mustRun(t, root, "related", "add", "login-flow", "apogee-kk0.4")
	mustRun(t, root, "related", "add", "db-schema", "login-flow")

	if content := readStoreFile(t, root, "login-flow.md"); !strings.Contains(content, "related: [db-schema]\n") {
		t.Errorf("login-flow.md = %q", content)
	}
	if content := readStoreFile(t, root, "db-schema.md"); strings.Contains(content, "related:") {
		t.Errorf("db-schema.md = %q, want no related key", content)
	}
}

func TestRelatedAdd_JSON_ReportsTheIssueWrite(t *testing.T) {
	root := newRelatedStore(t)

	stdout, _, exitCode := runCLI(t, root, "related", "add", "--json", "login-flow", "db-schema")

	envelope := decodeEnvelope(t, stdout)
	data, _ := envelope["data"].(map[string]any)
	if exitCode != ExitOK || envelope["kind"] != kindIssueWrite || data["id"] != "login-flow" ||
		data["etag"] != fileETag(t, root, "login-flow.md") {
		t.Errorf("exit = %d envelope = %v", exitCode, envelope)
	}
}

func TestRelatedAdd_StaleIfMatch_FailsConflictAndWritesNothing(t *testing.T) {
	root := newRelatedStore(t)
	before := readStoreFile(t, root, "login-flow.md")

	_, stderr, exitCode := runCLI(t, root, "related", "add", "--if-match", "0000000000000000", "login-flow", "db-schema")

	if exitCode != ExitError || !strings.HasPrefix(stderr, "capcom: .capcom/issues/login-flow.md: etag 0000000000000000") {
		t.Errorf("exit = %d stderr = %q", exitCode, stderr)
	}
	if after := readStoreFile(t, root, "login-flow.md"); after != before {
		t.Errorf("login-flow.md changed:\n%s", after)
	}
}

func TestRelatedAdd_StaleIfMatchBelowFormatTwo_LeavesTheConfigAlone(t *testing.T) {
	root := newRelatedStore(t)
	const keylessConfig = "types: [bug, feature, task, chore]\n"
	writeStoreConfig(t, root, keylessConfig)

	stdout, stderr, exitCode := runCLI(t, root, "related", "add", "--if-match", "0000000000000000", "login-flow", "db-schema")

	if exitCode != ExitError || !strings.HasPrefix(stderr, "capcom: .capcom/issues/login-flow.md: etag 0000000000000000") {
		t.Errorf("exit = %d stderr = %q", exitCode, stderr)
	}
	if config := readStoreConfig(t, root); config != keylessConfig || strings.Contains(stdout, "warning:") {
		t.Errorf("config.yaml = %q stdout = %q, want the config unchanged and no warning", config, stdout)
	}
}

func TestRelatedRm_RoundTrip_RemovesTheKey(t *testing.T) {
	root := newRelatedStore(t)
	mustRun(t, root, "related", "add", "login-flow", "db-schema")

	mustRun(t, root, "related", "rm", "login-flow", "apogee-kk0.4")

	if content := readStoreFile(t, root, "login-flow.md"); strings.Contains(content, "related:") {
		t.Errorf("login-flow.md = %q, want no related key", content)
	}
	if related := showRelated(t, root, "db-schema"); len(related) != 0 {
		t.Errorf("db-schema related = %v, want none", related)
	}
}

func TestRelatedRm_FromTheOtherSide_RemovesItFromTheHolder(t *testing.T) {
	root := newRelatedStore(t)
	mustRun(t, root, "related", "add", "login-flow", "db-schema")

	stdout := mustRun(t, root, "related", "rm", "--json", "db-schema", "login-flow")

	if data := decodeEnvelope(t, stdout)["data"].(map[string]any); data["id"] != "login-flow" {
		t.Errorf("data = %v, want the write of login-flow", data)
	}
	if content := readStoreFile(t, root, "login-flow.md"); strings.Contains(content, "related:") {
		t.Errorf("login-flow.md = %q, want no related key", content)
	}
}

func TestRelatedRm_NotRelated_FailsNotFound(t *testing.T) {
	root := newRelatedStore(t)

	_, stderr, exitCode := runCLI(t, root, "related", "rm", "login-flow", "db-schema")

	want := "capcom: .capcom/issues/login-flow.md: login-flow and db-schema are not related\n"
	if exitCode != ExitNotFound || stderr != want {
		t.Errorf("exit = %d stderr = %q, want %q", exitCode, stderr, want)
	}
}

func TestRelatedRm_EntryTextOrIssue_RemovesTheMatchingEntries(t *testing.T) {
	tests := []struct {
		name    string
		related string
		arg     string
		extra   map[string]string
		want    string
	}{
		{name: "dangling entry by its text", related: "[gone-issue, db-schema]", arg: "gone-issue", want: "[db-schema]"},
		{name: "dangling entry prefixing one issue", related: "[db, build-pipeline]", arg: "db", want: "[build-pipeline]"},
		{
			name:    "dangling entry prefixing two issues",
			related: "[b, db-schema]",
			arg:     "b",
			extra:   map[string]string{"build-plan.md": readIssue("Build plan", "open", "", "")},
			want:    "[db-schema]",
		},
		{name: "dangling entry beside the issue it prefixes", related: "[db, db-schema]", arg: "db", want: "[db-schema]"},
		{name: "exact id beside a dangling prefix", related: "[apogee-kk0.4, db]", arg: "db-schema", want: "[db]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newRelatedStore(t)
			files := map[string]string{
				"login-flow.md": readIssue("Login flow", "open", "related: "+test.related+"\n", ""),
			}
			maps.Copy(files, test.extra)
			writeIssueFiles(t, root, files)

			mustRun(t, root, "related", "rm", "login-flow", test.arg)

			if content := readStoreFile(t, root, "login-flow.md"); !strings.Contains(content, "related: "+test.want+"\n") {
				t.Errorf("login-flow.md = %q, want related %s", content, test.want)
			}
		})
	}
}

func TestRelatedRm_StaleIfMatchOnTheHolder_FailsConflict(t *testing.T) {
	root := newRelatedStore(t)
	mustRun(t, root, "related", "add", "login-flow", "db-schema")
	otherETag := fileETag(t, root, "db-schema.md")

	_, stderr, exitCode := runCLI(t, root, "related", "rm", "--if-match", otherETag, "db-schema", "login-flow")

	if exitCode != ExitError || !strings.HasPrefix(stderr, "capcom: .capcom/issues/login-flow.md: etag "+otherETag) {
		t.Errorf("exit = %d stderr = %q", exitCode, stderr)
	}
	if content := readStoreFile(t, root, "login-flow.md"); !strings.Contains(content, "related: [db-schema]\n") {
		t.Errorf("login-flow.md = %q, want the link kept", content)
	}
}

func TestRelatedRm_NameMatchingNoIssueAndNoEntry_FailsNotFoundNamingTheArgument(t *testing.T) {
	root := newRelatedStore(t)
	mustRun(t, root, "related", "add", "login-flow", "db-schema")

	_, stderr, exitCode := runCLI(t, root, "related", "rm", "login-flow", "absent")

	want := "capcom: .capcom/issues/login-flow.md: login-flow is not related to absent\n"
	if exitCode != ExitNotFound || stderr != want {
		t.Errorf("exit = %d stderr = %q, want %q", exitCode, stderr, want)
	}
}

func TestRelatedRm_AmbiguousPrefixMatchingNoEntry_FailsNamingTheCandidates(t *testing.T) {
	root := newRelatedStore(t)
	files := map[string]string{
		"login-flow.md": readIssue("Login flow", "open", "related: [db-schema]\n", ""),
		"build-plan.md": readIssue("Build plan", "open", "", ""),
	}
	writeIssueFiles(t, root, files)

	_, stderr, exitCode := runCLI(t, root, "related", "rm", "login-flow", "b")

	want := "capcom: ambiguous id prefix \"b\": matches build-pipeline, build-plan\n"
	if exitCode != ExitError || stderr != want {
		t.Errorf("exit = %d stderr = %q, want %q", exitCode, stderr, want)
	}
	assertIssueFilesUnchanged(t, root, files)
}

func TestRelatedRm_DuplicatedName_FailsNamingBothFilesAndWritesNothing(t *testing.T) {
	root, files := newDuplicateIssueStore(t)
	files["holder.md"] = readIssue("Holder", "open", "related: [foo, foo-bar]\n", "")
	writeIssueFiles(t, root, files)

	_, stderr, exitCode := runCLI(t, root, "related", "rm", "holder", "foo")

	assertDuplicateRefusal(t, root, stderr, exitCode, ".capcom/issues/foo.md", ".capcom/issues/sub-issue/foo.md")
	assertIssueFilesUnchanged(t, root, files)
}

func TestRelatedRm_CitationSpellingOfADanglingEntry_RemovesItByItsText(t *testing.T) {
	root := newRelatedStore(t)
	writeIssueFiles(t, root, map[string]string{
		"login-flow.md": readIssue("Login flow", "open", "related: [gone-issue, db-schema]\n", ""),
	})

	mustRun(t, root, "related", "rm", "login-flow", "Gone Issue")

	if content := readStoreFile(t, root, "login-flow.md"); !strings.Contains(content, "related: [db-schema]\n") {
		t.Errorf("login-flow.md = %q", content)
	}
}

func TestRelatedAdd_KeylessConfig_RaisesTheFormatOnceWithAWarning(t *testing.T) {
	root := newRelatedStore(t)
	writeStoreConfig(t, root, "types: [bug, feature, task, chore]\n")

	first := mustRun(t, root, "related", "add", "login-flow", "db-schema")
	second := mustRun(t, root, "related", "add", "login-flow", "build-pipeline")

	if !strings.HasSuffix(first, formatRaisedText) || strings.Contains(second, "warning:") {
		t.Errorf("first = %q\nsecond = %q, want the warning on the first only", first, second)
	}
	if config := readStoreConfig(t, root); config != "format: 2\ntypes: [bug, feature, task, chore]\n" {
		t.Errorf("config.yaml = %q", config)
	}
}

func TestRelatedAdd_ConfigOpeningWithDocumentMarker_RaisesInsideIt(t *testing.T) {
	root := newRelatedStore(t)
	writeIssueFiles(t, root, map[string]string{"review-me.md": readIssue("Review me", "review", "", "")})
	statuses := "statuses:\n  open: todo\n  review: doing\n  done: done\n"
	writeStoreConfig(t, root, "# mine\n---\n"+statuses)

	mustRun(t, root, "related", "add", "login-flow", "db-schema")
	listed := mustRun(t, root, "list")

	if config := readStoreConfig(t, root); config != "# mine\n---\nformat: 2\n"+statuses {
		t.Errorf("config.yaml = %q", config)
	}
	if !strings.Contains(listed, "review-me") {
		t.Errorf("list = %q, want it to list review-me", listed)
	}
}

func TestRelatedAdd_MissingConfig_CreatesItAtFormatTwo(t *testing.T) {
	root := newRelatedStore(t)
	if err := os.Remove(filepath.Join(root, ".capcom", "config.yaml")); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	stdout := mustRun(t, root, "related", "add", "login-flow", "db-schema")

	if config := readStoreConfig(t, root); config != "format: 2\n" || !strings.HasSuffix(stdout, formatRaisedText) {
		t.Errorf("config.yaml = %q stdout = %q", config, stdout)
	}
}

func TestRelatedAdd_FormatTwoStore_LeavesTheConfigAlone(t *testing.T) {
	root := newRelatedStore(t)
	before := readStoreConfig(t, root)

	stdout := mustRun(t, root, "related", "add", "login-flow", "db-schema")

	if after := readStoreConfig(t, root); after != before || strings.Contains(stdout, "warning:") {
		t.Errorf("stdout = %q, config changed: %v", stdout, after != before)
	}
}

func TestRelatedAdd_ThenListJSON_LeavesRelatedOut(t *testing.T) {
	root := newRelatedStore(t)
	mustRun(t, root, "related", "add", "login-flow", "db-schema")

	stdout := mustRun(t, root, "list", "--json")

	for _, item := range decodeEnvelope(t, stdout)["data"].([]any) {
		if _, isPresent := item.(map[string]any)["related"]; isPresent {
			t.Errorf("list --json issue carries related: %v", item)
		}
	}
}

func TestRelatedAdd_RaiseFails_FailsInvalidAtTheConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a read-only directory does not stop file creation on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes into a read-only directory")
	}
	root := newRelatedStore(t)
	writeStoreConfig(t, root, "types: [bug, feature, task, chore]\n")
	storeDir := filepath.Join(root, ".capcom")
	if err := os.Chmod(storeDir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(storeDir, 0o755) })

	stdout, _, exitCode := runCLI(t, root, "--json", "related", "add", "login-flow", "db-schema")

	failure, _ := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if exitCode != ExitError || failure["code"] != string(CodeInvalid) || failure["path"] != ".capcom/config.yaml" {
		t.Errorf("exit = %d error = %v, want invalid at .capcom/config.yaml", exitCode, failure)
	}
	if related := showRelated(t, root, "login-flow"); len(related) != 0 {
		t.Errorf("login-flow related = %v, want no link written", related)
	}
}
