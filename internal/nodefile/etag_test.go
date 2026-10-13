package nodefile_test

import (
	"regexp"
	"slices"
	"testing"

	"github.com/airiclenz/capcom/internal/nodefile"
)

func TestETag_HashesContent(t *testing.T) {
	hexDigits := regexp.MustCompile(`^[0-9a-f]{16}$`)
	first := []byte("---\nid: first\n---\n\nBody.\n")
	second := []byte("---\nid: second\n---\n\nBody.\n")

	firstTag := nodefile.ETag(first)

	if !hexDigits.MatchString(firstTag) {
		t.Errorf("ETag = %q, want 16 lowercase hex digits", firstTag)
	}
	if again := nodefile.ETag(slices.Clone(first)); again != firstTag {
		t.Errorf("ETag of equal bytes = %q, want %q", again, firstTag)
	}
	if other := nodefile.ETag(second); other == firstTag {
		t.Errorf("ETag of different files are both %q", other)
	}
	if empty := nodefile.ETag(nil); empty != "cbf29ce484222325" {
		t.Errorf("ETag(nil) = %q, want the FNV-1a 64 offset basis", empty)
	}
}
