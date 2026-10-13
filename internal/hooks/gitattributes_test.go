package hooks_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/airiclenz/capcom/internal/hooks"
)

// mergeAttributeLine is the .gitattributes line AppendMergeAttribute adds, without its line ending.
const mergeAttributeLine = ".capcom/issues/**/*.md merge=capcom-log"

func TestAppendMergeAttribute_Content_AppendsTheLineOnce(t *testing.T) {
	cases := []struct {
		name         string
		content      string
		wantAppended string
		wantIsAdded  bool
	}{
		{"empty", "", mergeAttributeLine + "\n", true},
		{"LF", "* text=auto\n", "* text=auto\n" + mergeAttributeLine + "\n", true},
		{"CRLF", "* text=auto\r\n*.png binary\r\n", "* text=auto\r\n*.png binary\r\n" + mergeAttributeLine + "\r\n", true},
		{"no final LF", "* text=auto", "* text=auto\n" + mergeAttributeLine + "\n", true},
		{"no final CRLF", "* text=auto\r\n*.png binary", "* text=auto\r\n*.png binary\r\n" + mergeAttributeLine + "\r\n", true},
		{
			"already present with extra spaces",
			"* text=auto\n  .capcom/issues/**/*.md \t merge=capcom-log  \n",
			"* text=auto\n  .capcom/issues/**/*.md \t merge=capcom-log  \n",
			false,
		},
		{"already present without a final line ending", mergeAttributeLine, mergeAttributeLine, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			appended, isAdded := hooks.AppendMergeAttribute([]byte(testCase.content))

			if isAdded != testCase.wantIsAdded || string(appended) != testCase.wantAppended {
				t.Errorf("AppendMergeAttribute = %v, appended %q; want %v, %q",
					isAdded, appended, testCase.wantIsAdded, testCase.wantAppended)
			}
		})
	}
}

func TestAppendMergeAttribute_SpareCapacity_LeavesContentUntouched(t *testing.T) {
	content := make([]byte, 0, 64)
	content = append(content, "* text=auto\n"...)
	spare := content[:cap(content)]
	before := bytes.Clone(spare)

	appended, isAdded := hooks.AppendMergeAttribute(content)

	if !isAdded || !bytes.Equal(spare, before) {
		t.Errorf("AppendMergeAttribute wrote into the caller's array (isAdded %v): %q", isAdded, spare)
	}
	if again, isAddedAgain := hooks.AppendMergeAttribute(appended); isAddedAgain || !bytes.Equal(again, appended) {
		t.Errorf("second AppendMergeAttribute = %v, %q; want false, unchanged", isAddedAgain, again)
	}
}

func TestInstallMergeAttribute_UnusableFile_FailsWithPathAndStaysAsItIs(t *testing.T) {
	for _, unusable := range unusableFiles {
		t.Run(unusable.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, hooks.GitattributesPath)
			unusable.make(t, path)
			wantType, wantSize := entryMode(t, path)

			status, err := hooks.InstallMergeAttribute(root)

			var failure *hooks.Error
			want := hooks.Error{Path: hooks.GitattributesPath, Message: unusable.wantMessage}
			if !errors.As(err, &failure) || *failure != want || status != "" {
				t.Errorf("InstallMergeAttribute = %q, %v; want %+v", status, err, want)
			}
			if gotType, gotSize := entryMode(t, path); gotType != wantType || gotSize != wantSize {
				t.Errorf(".gitattributes entry = %v, %d bytes; want %v, %d bytes untouched", gotType, gotSize, wantType, wantSize)
			}
		})
	}
}
