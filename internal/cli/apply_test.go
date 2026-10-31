package cli

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// applyFile is the name the apply tests write their apply file under, in the store's root.
const applyFile = "batch.yaml"

// newApplyStore creates a store with blocker open, waiting open and depending on blocker, other
// open and shipped done.
func newApplyStore(t *testing.T) string {
	t.Helper()
	return newIssueStore(t, map[string]string{
		"blocker.md": readIssue("Blocker", "open", "", "\nThe blocker.\n"),
		"waiting.md": readIssue("Waiting", "open", "deps: [blocker]\n", "\nThe waiting one.\n"),
		"other.md":   readIssue("Other", "open", "", "\nThe other.\n"),
		"shipped.md": readIssue("Shipped", "done", "", "\nShipped.\n"),
	})
}

// writeApplyFile writes content as root's apply file.
func writeApplyFile(t *testing.T, root string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, applyFile), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// capcomFiles returns the content of every file below root's .capcom directory, keyed by its slash
// path relative to root.
func capcomFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(filepath.Join(root, ".capcom"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		files[filepath.ToSlash(relative)] = string(data)
		return err
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	return files
}

// threeCreates creates alpha, beta waiting for alpha, and gamma waiting for both.
const threeCreates = `- op: create
  id: alpha
  title: Alpha
  body: The alpha.
- op: create
  id: beta
  title: Beta
  body: The beta.
  dep: alpha
- op: create
  id: gamma
  title: Gamma
  body: The gamma.
  dep: [alpha, beta]
`

func TestApply_ThreeCreatesWithDeps_WritesThemInOneCall(t *testing.T) {
	root := newApplyStore(t)
	writeApplyFile(t, root, threeCreates)

	stdout := mustRun(t, root, "apply", applyFile)

	want := "1  create  alpha  created\n2  create  beta  created\n3  create  gamma  created\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if beta := readStoreFile(t, root, "beta.md"); !strings.Contains(beta, "deps: [alpha]") {
		t.Errorf("beta.md = %q, want deps: [alpha]", beta)
	}
	if gamma := readStoreFile(t, root, "gamma.md"); !strings.Contains(gamma, "deps: [alpha, beta]") {
		t.Errorf("gamma.md = %q, want deps: [alpha, beta]", gamma)
	}
}

func TestApply_FailingOperation_LeavesStoreUntouchedAndNamesIt(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "unknown op last",
			content: threeCreates + "- op: reopen\n  id: shipped\n",
			want:    applyFile + ":15: operation 4: ",
		},
		{
			name:    "unknown id in the middle",
			content: "- op: log\n  id: other\n  text: progress\n- op: update\n  id: no-such-issue\n  priority: 1\n- op: close\n  id: blocker\n  reason: done\n",
			want:    applyFile + ":4: operation 2 (update): ",
		},
		{
			name:    "a close of a Closed issue first",
			content: "- op: close\n  id: shipped\n  reason: again\n- op: log\n  id: other\n  text: progress\n",
			want:    applyFile + ":1: operation 1 (close): ",
		},
		{
			name:    "a stale if-match",
			content: "- op: log\n  id: other\n  text: progress\n- op: dep add\n  id: other\n  dep: blocker\n  if-match: 0000000000000000\n",
			want:    applyFile + ":4: operation 2 (dep add): .capcom/issues/other.md: etag 0000000000000000",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newApplyStore(t)
			writeApplyFile(t, root, test.content)
			before := capcomFiles(t, root)

			_, stderr, exitCode := runCLI(t, root, "apply", applyFile)

			if exitCode == ExitOK {
				t.Fatalf("exit = 0, want a failure")
			}
			if !strings.Contains(stderr, test.want) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, test.want)
			}
			if after := capcomFiles(t, root); !maps.Equal(after, before) {
				t.Errorf(".capcom/ changed:\nbefore = %v\nafter = %v", before, after)
			}
		})
	}
}

func TestApply_IssueFileFailure_KeepsItsPathAndLine(t *testing.T) {
	root := newApplyStore(t)
	writeApplyFile(t, root, "- op: log\n  id: other\n  text: progress\n- op: update\n  id: other\n  title: \"\"\n")

	stdout, _, exitCode := runCLI(t, root, "apply", applyFile, "--json")

	if exitCode != ExitError {
		t.Fatalf("exit = %d, want %d\nstdout = %s", exitCode, ExitError, stdout)
	}
	failure := decodeEnvelope(t, stdout)["error"].(map[string]any)
	if failure["path"] != applyFile || failure["line"] != float64(4) || failure["code"] != "invalid" {
		t.Errorf("error = %v, want invalid at %s line 4", failure, applyFile)
	}
	want := "operation 2 (update): .capcom/issues/other.md:2: title is required"
	if failure["message"] != want {
		t.Errorf("message = %q, want %q", failure["message"], want)
	}
}

// closeAndCreate closes blocker, which makes waiting Ready, and creates delta.
const closeAndCreate = `- op: close
  id: blocker
  reason: fixed
- op: create
  id: delta
  title: Delta
  body: The delta.
`

func TestApply_DryRunText_ReportsTheRealOutcomeAndWritesNothing(t *testing.T) {
	root := newApplyStore(t)
	writeApplyFile(t, root, closeAndCreate)
	before := capcomFiles(t, root)

	dryRun := mustRun(t, root, "apply", applyFile, "--dry-run")
	isUntouched := maps.Equal(capcomFiles(t, root), before)
	real := mustRun(t, root, "apply", applyFile)

	if !isUntouched {
		t.Errorf("the dry run changed .capcom/")
	}
	want := "1  close  blocker  closed\n2  create  delta  created\nnow ready: waiting\n"
	if real != want {
		t.Errorf("real run = %q, want %q", real, want)
	}
	if dryRun != dryRunLine+"\n"+real {
		t.Errorf("dry run = %q, want %q", dryRun, dryRunLine+"\n"+real)
	}
}

func TestApply_DryRunJSON_EqualsTheRealRunButForEtagAndDryRun(t *testing.T) {
	root := newApplyStore(t)
	writeApplyFile(t, root, closeAndCreate)

	dryRun := decodeEnvelope(t, mustRun(t, root, "apply", applyFile, "--dry-run", "--json"))
	real := decodeEnvelope(t, mustRun(t, root, "apply", applyFile, "--json"))

	if dryRun["kind"] != kindApply || real["kind"] != kindApply {
		t.Fatalf("kinds = %v and %v, want %s", dryRun["kind"], real["kind"], kindApply)
	}
	dryData, realData := dryRun["data"].(map[string]any), real["data"].(map[string]any)
	if dryData["dry_run"] != true || realData["dry_run"] != false {
		t.Errorf("dry_run = %v and %v, want true and false", dryData["dry_run"], realData["dry_run"])
	}
	for _, data := range []map[string]any{dryData, realData} {
		for _, operation := range data["ops"].([]any) {
			delete(operation.(map[string]any), "etag")
		}
		delete(data, "dry_run")
	}
	if !reflect.DeepEqual(dryData, realData) {
		t.Errorf("dry run data = %v, want the real run's %v", dryData, realData)
	}
	wantFirst := map[string]any{"index": float64(1), "line": float64(1), "op": "close", "id": "blocker", "outcome": "closed"}
	if first := realData["ops"].([]any)[0].(map[string]any); !maps.Equal(first, wantFirst) {
		t.Errorf("ops[0] = %v, want %v", first, wantFirst)
	}
}

func TestApply_KV_PrintsARecordPerOperation(t *testing.T) {
	root := newApplyStore(t)
	writeApplyFile(t, root, "- op: log\n  id: other\n  text: progress\n- op: dep add\n  id: other\n  dep: blocker\n")

	stdout := mustRun(t, root, "apply", applyFile, "--kv")

	for _, want := range []string{
		"INDEX=1\nLINE=1\nOP=log\nID=other\nETAG=",
		"STATUS=logged\nINDEX=2\nLINE=4\nOP=dep add\nID=other\nETAG=",
		"STATUS=dep_added\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want it to contain %q", stdout, want)
		}
	}
}

func TestApply_Stdin_ReadsTheApplyFile(t *testing.T) {
	root := newApplyStore(t)

	stdout, stderr, exitCode := runCLIWith(t, root, "- op: log\n  id: other\n  text: from stdin\n", nil, "apply", stdinPath)

	if exitCode != ExitOK {
		t.Fatalf("exit = %d\nstdout = %s\nstderr = %s", exitCode, stdout, stderr)
	}
	if other := readStoreFile(t, root, "other.md"); !strings.Contains(other, "from stdin") {
		t.Errorf("other.md = %q, want the log entry", other)
	}
}

func TestApply_StdinFailure_NamesStdin(t *testing.T) {
	root := newApplyStore(t)

	_, stderr, _ := runCLIWith(t, root, "- op: log\n  id: no-such-issue\n  text: lost\n", nil, "apply", stdinPath)

	if want := applyStdinName + ":1: operation 1 (log): "; !strings.Contains(stderr, want) {
		t.Errorf("stderr = %q, want it to contain %q", stderr, want)
	}
}

func TestApply_LockHeldForOneIssueOfTheBatch_WarnsOnce(t *testing.T) {
	root := newApplyStore(t)
	mustRun(t, root, "lock", "acquire", "--purpose", "other", "--holder", "other-session")
	writeApplyFile(t, root, "- op: log\n  id: other\n  text: progress\n- op: log\n  id: blocker\n  text: progress\n- op: log\n  id: other\n  text: more\n")

	dryRun := mustRun(t, root, "apply", applyFile, "--dry-run")
	real := mustRun(t, root, "apply", applyFile)

	if strings.Contains(dryRun, lockWarnAdvice) {
		t.Errorf("dry run = %q, want no lock warning", dryRun)
	}
	if strings.Count(real, lockWarnAdvice) != 1 {
		t.Errorf("stdout = %q, want exactly one lock warning", real)
	}
}

func TestApply_LockHeldForTheOnlyIssueOfTheBatch_IsSilent(t *testing.T) {
	root := newApplyStore(t)
	mustRun(t, root, "lock", "acquire", "--purpose", "other", "--holder", "other-session")
	writeApplyFile(t, root, "- op: log\n  id: other\n  text: progress\n")

	stdout := mustRun(t, root, "apply", applyFile)

	if strings.Contains(stdout, lockWarnAdvice) {
		t.Errorf("stdout = %q, want no lock warning", stdout)
	}
}
