package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun_StaleAndUnrelatedFiles_RemovesOnlyTheStalePage(t *testing.T) {
	out := t.TempDir()
	stale := filepath.Join(out, "capcomold.md")
	unrelated := filepath.Join(out, "notes.md")
	for _, path := range []string{stale, unrelated} {
		if err := os.WriteFile(path, []byte("seeded\n"), 0o600); err != nil {
			t.Fatalf("seed %s: %v", path, err)
		}
	}

	err := run(out)

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stat %s = %v, want the stale page removed", stale, err)
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Errorf("stat %s = %v, want the unrelated file kept", unrelated, err)
	}
	if _, err := os.Stat(filepath.Join(out, "capcom.md")); err != nil {
		t.Errorf("stat capcom.md = %v, want the root reference page written", err)
	}
}
