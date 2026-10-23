package lock

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/airiclenz/capcom/internal/nodefile"
)

// Layout of a token file: `<key>.<purpose digest>.token` beside the lock record, holding the Lock
// token on its first line and the purpose after it.
const (
	tokenFileExtension   = ".token"
	tokenFilePermissions = 0o600
	tokenFileSeparator   = "."
	tokenLineEnd         = "\n"
	purposeDigestLength  = sha256.Size * 2
)

// SessionToken returns the Lock token a session acquiring key's lock for purpose holds: token when
// it is not empty, else the token kept in key's token file for purpose, else "". A missing or
// malformed token file, or one kept for another purpose, gives ""; only a token file that cannot
// be read fails, naming its path. It reads without the guard, as token files are replaced
// atomically, and creates nothing.
func (l *Locks) SessionToken(key, token, purpose string) (string, error) {
	if token != "" || purpose == "" {
		return token, nil
	}

	path := l.tokenPath(key, purpose)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}

	kept, keptPurpose, isWellFormed := parseTokenFile(data)
	if !isWellFormed || keptPurpose != purpose {
		return "", nil
	}
	return kept, nil
}

// tokenPath returns the token file that keeps key's Lock token for purpose.
func (l *Locks) tokenPath(key, purpose string) string {
	digest := sha256.Sum256([]byte(purpose))
	name := key + tokenFileSeparator + hex.EncodeToString(digest[:]) + tokenFileExtension
	return filepath.Join(l.dir, name)
}

// writeTokenFile keeps token in key's token file for purpose, owner-only and replaced atomically;
// an empty purpose writes nothing. The caller holds the guard.
func (l *Locks) writeTokenFile(key, token, purpose string) error {
	if purpose == "" {
		return nil
	}
	content := token + tokenLineEnd + purpose + tokenLineEnd
	return nodefile.WriteFileAtomicVerbatim(
		l.tokenPath(key, purpose),
		strings.NewReader(content),
		tokenFilePermissions,
	)
}

// removeTokenFiles deletes every token file of key that keeps token; an empty token deletes
// nothing. A token file that cannot be read or deleted fails, naming its path, after the others
// are tried. The caller holds the guard.
func (l *Locks) removeTokenFiles(key, token string) error {
	if token == "" {
		return nil
	}
	paths, err := l.tokenFilePaths(key)
	if err != nil {
		return err
	}

	var errs []error
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		if kept, _, isWellFormed := parseTokenFile(data); isWellFormed && kept == token {
			errs = append(errs, removeFile(path))
		}
	}
	return errors.Join(errs...)
}

// removeAllTokenFiles deletes every token file of key, whatever token it keeps. A token file that
// cannot be deleted fails, naming its path, after the others are tried. The caller holds the guard.
func (l *Locks) removeAllTokenFiles(key string) error {
	paths, err := l.tokenFilePaths(key)
	if err != nil {
		return err
	}

	errs := make([]error, 0, len(paths))
	for _, path := range paths {
		errs = append(errs, removeFile(path))
	}
	return errors.Join(errs...)
}

// tokenFilePaths returns the path of every token file of key in the lock directory. A file
// belongs to key only when its name is key, a dot, a 64-digit lowercase hex digest and `.token`,
// so `wt-a` never claims the token files of `wt-a.b`. A missing lock directory has none.
func (l *Locks) tokenFilePaths(key string) ([]string, error) {
	entries, err := os.ReadDir(l.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", l.dir, err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && isTokenFileOf(entry.Name(), key) {
			paths = append(paths, filepath.Join(l.dir, entry.Name()))
		}
	}
	return paths, nil
}

// isTokenFileOf reports whether name is the name of one of key's token files.
func isTokenFileOf(name, key string) bool {
	rest, hasKey := strings.CutPrefix(name, key+tokenFileSeparator)
	digest, hasExtension := strings.CutSuffix(rest, tokenFileExtension)
	return hasKey && hasExtension && len(digest) == purposeDigestLength && isLowercaseHex(digest)
}

// isLowercaseHex reports whether every character of text is a lowercase hex digit.
func isLowercaseHex(text string) bool {
	for _, character := range text {
		isDigit := character >= '0' && character <= '9'
		isLetter := character >= 'a' && character <= 'f'
		if !isDigit && !isLetter {
			return false
		}
	}
	return true
}

// parseTokenFile splits a token file's content into its token, the first line, and its purpose,
// the rest with one trailing line end removed. A file with no line end or an empty token is not
// well formed.
func parseTokenFile(data []byte) (string, string, bool) {
	token, purpose, hasLineEnd := strings.Cut(string(data), tokenLineEnd)
	if !hasLineEnd || token == "" {
		return "", "", false
	}
	return token, strings.TrimSuffix(purpose, tokenLineEnd), true
}

// removeFile deletes the file at path; a file already gone is no failure. Errors name path.
func removeFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
