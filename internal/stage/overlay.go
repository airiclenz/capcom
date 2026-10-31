// Package stage stages node writes in memory: an Overlay is a nodefile.FS that reads through to the
// disk and keeps every write to itself, so a batch of operations runs against the files as they
// would be, and Overlay.Plan turns what was staged into one tree.Plan that lands every write or
// none.
package stage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/airiclenz/capcom/internal/config"
	"github.com/airiclenz/capcom/internal/nodefile"
	"github.com/airiclenz/capcom/internal/store"
)

// Ops of the *fs.PathError values the Overlay fails with, as the Disk calls they stand for name
// theirs.
const (
	readOp    = "read"
	replaceOp = "replace"
	createOp  = "create"
	mkdirOp   = "mkdir"
)

// Permissions a staged file and a staged directory report, the ones nodefile's writes give them.
const (
	filePermissions fs.FileMode = 0o644
	dirPermissions  fs.FileMode = 0o755
)

// crlf and lf are the line endings the Disk writes convert from and to.
const (
	crlf = "\r\n"
	lf   = "\n"
)

// errNotDirectory reports an ancestor of a staged create that exists but is not a directory, as
// os.MkdirAll refuses it.
var errNotDirectory = errors.New("not a directory")

// Overlay is a nodefile.FS over the disk with every write kept in memory. Reads see the staged
// files first and the disk below them; ReplaceFile and CreateFile change nothing on disk, and a
// create records the directories it would make, which Stat, Lstat and ReadDir then report. The
// first disk read of each file is kept as its Original, the file as the batch found it. RaiseFormat
// stands in for store.Store.RaiseFormat and writes nothing either. Plan returns the tree.Plan that
// makes every staged write on disk at once. An Overlay is not safe for concurrent use.
type Overlay struct {
	// store is the store whose files the Overlay stages: its Catalog reads the issue tree's
	// directory, working tree root and config from it.
	store *store.Store
	// configPath is the store's config.yaml; configName names it relative to the working tree
	// root, as store.Store names it in its errors.
	configPath string
	configName string
	// files holds the staged files by clean absolute path, and order their paths in the order
	// each was first written.
	files map[string]*stagedFile
	order []string
	// originals holds the first disk read of every file read or replaced, by clean absolute path.
	originals map[string][]byte
	// dirs holds the directories the staged creates make, missing on disk, by clean absolute path.
	dirs map[string]bool
	// config is config.yaml as the first RaiseFormat read it from disk.
	config stagedConfig
	// format is the highest store format RaiseFormat staged, 0 when none; raisedConfig is
	// config.yaml raised to it.
	format       int
	raisedConfig []byte
}

// stagedFile is one file a staged write left in memory.
type stagedFile struct {
	// data is the file's content, with LF line endings as the Disk writes leave it.
	data []byte
	// isNew says whether CreateFile staged it: it is missing on disk.
	isNew bool
}

// stagedConfig is config.yaml as the Overlay first read it from disk.
type stagedConfig struct {
	isRead    bool
	isMissing bool
	data      []byte
}

// New returns an empty Overlay over the disk whose RaiseFormat stages a raise of the config.yaml
// of s.
func New(s *store.Store) *Overlay {
	configPath := s.ConfigPath()
	configName, _ := store.RelativePath(s.WorktreeRoot, configPath)
	return &Overlay{
		store:      s,
		configPath: configPath,
		configName: configName,
		files:      map[string]*stagedFile{},
		originals:  map[string][]byte{},
		dirs:       map[string]bool{},
	}
}

// Stat returns the FileInfo of a staged file or directory, or os.Stat of path.
func (o *Overlay) Stat(path string) (fs.FileInfo, error) {
	if info, isStaged := o.stagedInfo(filepath.Clean(path)); isStaged {
		return info, nil
	}
	return os.Stat(path)
}

// Lstat returns the FileInfo of a staged file or directory, or os.Lstat of path.
func (o *Overlay) Lstat(path string) (fs.FileInfo, error) {
	if info, isStaged := o.stagedInfo(filepath.Clean(path)); isStaged {
		return info, nil
	}
	return os.Lstat(path)
}

// ReadDir lists dir as the disk holds it, with every staged file and directory directly inside it
// added or put in place of the disk entry of that name, sorted by name. A directory only a staged
// create makes lists the staged entries alone.
func (o *Overlay) ReadDir(dir string) ([]fs.DirEntry, error) {
	dir = filepath.Clean(dir)
	entries, err := os.ReadDir(dir)
	if err != nil && !(o.dirs[dir] && errors.Is(err, fs.ErrNotExist)) {
		return nil, err
	}
	byName := make(map[string]fs.DirEntry, len(entries))
	for _, entry := range entries {
		byName[entry.Name()] = entry
	}
	for path := range o.files {
		if filepath.Dir(path) == dir {
			info, _ := o.stagedInfo(path)
			byName[info.Name()] = fs.FileInfoToDirEntry(info)
		}
	}
	for path := range o.dirs {
		if filepath.Dir(path) == dir {
			info, _ := o.stagedInfo(path)
			byName[info.Name()] = fs.FileInfoToDirEntry(info)
		}
	}
	merged := make([]fs.DirEntry, 0, len(byName))
	for _, entry := range byName {
		merged = append(merged, entry)
	}
	slices.SortFunc(merged, func(a fs.DirEntry, b fs.DirEntry) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return merged, nil
}

// ReadRegularFile returns a staged file's content, refused with nodefile.ErrTooLarge over 1 MiB as
// the disk read would refuse it, and a staged directory with nodefile.ErrNotRegular. Any other path
// is read from disk with nodefile.ReadRegularFile, and its first successful read kept as the file's
// Original.
func (o *Overlay) ReadRegularFile(path string) ([]byte, error) {
	path = filepath.Clean(path)
	if file, isStaged := o.files[path]; isStaged {
		if len(file.data) > nodefile.MaxNodeFileSize {
			return nil, &fs.PathError{Op: readOp, Path: path, Err: nodefile.ErrTooLarge}
		}
		return bytes.Clone(file.data), nil
	}
	if o.dirs[path] {
		return nil, &fs.PathError{Op: readOp, Path: path, Err: nodefile.ErrNotRegular}
	}
	return o.original(path)
}

// ReplaceFile stages everything read from content, CRLF line endings turned to LF, as the new
// content of the file at path, a staged one or one on disk. A path that is neither fails with a
// *fs.PathError naming it that wraps fs.ErrNotExist; a file on disk not read before is read now,
// for its Original.
func (o *Overlay) ReplaceFile(path string, content io.Reader) error {
	path = filepath.Clean(path)
	data, err := readContent(path, content)
	if err != nil {
		return err
	}
	if file, isStaged := o.files[path]; isStaged {
		file.data = data
		return nil
	}
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		return &fs.PathError{Op: replaceOp, Path: path, Err: fs.ErrNotExist}
	}
	if _, err := o.original(path); err != nil {
		return err
	}
	o.files[path] = &stagedFile{data: data}
	o.order = append(o.order, path)
	return nil
}

// CreateFile stages everything read from content, CRLF line endings turned to LF, as a new file at
// path, and every missing directory above it as made. A path that is staged or on disk fails with
// a *fs.PathError naming it that wraps fs.ErrExist, and an ancestor that is not a directory as
// os.MkdirAll fails on it.
func (o *Overlay) CreateFile(path string, content io.Reader) error {
	path = filepath.Clean(path)
	data, err := readContent(path, content)
	if err != nil {
		return err
	}
	if _, err := o.Lstat(path); err == nil {
		return &fs.PathError{Op: createOp, Path: path, Err: fs.ErrExist}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	missing, err := o.missingDirs(filepath.Dir(path))
	if err != nil {
		return err
	}
	for _, dir := range missing {
		o.dirs[dir] = true
	}
	o.files[path] = &stagedFile{data: data, isNew: true}
	o.order = append(o.order, path)
	return nil
}

// OriginalETag returns the ETag of the file at path as the batch found it on disk, its Original,
// for an if-match check: a file not read yet is read now. A file only a staged create made has no
// Original and fails with a *fs.PathError naming it that wraps fs.ErrNotExist, as a file missing
// on disk does.
func (o *Overlay) OriginalETag(path string) (string, error) {
	path = filepath.Clean(path)
	if file, isStaged := o.files[path]; isStaged && file.isNew {
		return "", &fs.PathError{Op: readOp, Path: path, Err: fs.ErrNotExist}
	}
	data, err := o.original(path)
	if err != nil {
		return "", err
	}
	return nodefile.ETag(data), nil
}

// RaiseFormat stages a raise of config.yaml to declare at least store format version, as
// store.Store.RaiseFormat makes it, and reports whether it staged one: false when config.yaml on
// disk already declares version or above, or an earlier call staged version or above. It reads
// config.yaml from disk once and fails as store.Store.RaiseFormat fails on it, a *config.Error for
// a file config.RaiseFormat cannot raise; it writes nothing; Plan makes the raise.
func (o *Overlay) RaiseFormat(version int) (bool, error) {
	if version <= o.format {
		return false, nil
	}
	if err := o.readConfig(); err != nil {
		return false, err
	}
	raised, isChanged, err := config.RaiseFormat(o.configName, o.config.data, version)
	if err != nil || !isChanged {
		return false, err
	}
	o.format = version
	o.raisedConfig = raised
	return true, nil
}

// readConfig reads config.yaml from disk into o.config unless an earlier call did, failing as
// store.Store's own read does: a refusal or any other failure, but a missing file, names the file
// relative to the working tree root.
func (o *Overlay) readConfig() error {
	if o.config.isRead {
		return nil
	}
	data, err := nodefile.ReadRegularFile(o.configPath)
	isMissing := errors.Is(err, fs.ErrNotExist)
	if err != nil && !isMissing {
		for _, refusal := range []error{nodefile.ErrNotRegular, nodefile.ErrTooLarge} {
			if errors.Is(err, refusal) {
				return fmt.Errorf("%s: %w", o.configName, refusal)
			}
		}
		return fmt.Errorf("%s: %w", o.configName, err)
	}
	o.config = stagedConfig{isRead: true, isMissing: isMissing, data: data}
	return nil
}

// original returns the Original of the file at path, reading it from disk with
// nodefile.ReadRegularFile and keeping it when no earlier read did.
func (o *Overlay) original(path string) ([]byte, error) {
	if data, isKept := o.originals[path]; isKept {
		return bytes.Clone(data), nil
	}
	data, err := nodefile.ReadRegularFile(path)
	if err != nil {
		return nil, err
	}
	o.originals[path] = bytes.Clone(data)
	return data, nil
}

// missingDirs returns dir and each of its ancestors missing from the overlay, up to the nearest one
// that exists, deepest first. An existing ancestor that is not a directory fails as os.MkdirAll
// fails on it.
func (o *Overlay) missingDirs(dir string) ([]string, error) {
	var missing []string
	for current := dir; ; current = filepath.Dir(current) {
		info, err := o.Stat(current)
		if err == nil {
			if !info.IsDir() {
				return nil, &fs.PathError{Op: mkdirOp, Path: current, Err: errNotDirectory}
			}
			return missing, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		missing = append(missing, current)
		if filepath.Dir(current) == current {
			return missing, nil
		}
	}
}

// stagedInfo returns the FileInfo of the staged file or directory at the clean path, and whether
// one is staged there.
func (o *Overlay) stagedInfo(path string) (fs.FileInfo, bool) {
	if file, isStaged := o.files[path]; isStaged {
		return stagedFileInfo{name: filepath.Base(path), size: int64(len(file.data))}, true
	}
	if o.dirs[path] {
		return stagedFileInfo{name: filepath.Base(path), isDir: true}, true
	}
	return nil, false
}

// readContent reads everything from content, the content a write of path stages, with CRLF line
// endings turned to LF; a failed read names path.
func readContent(path string, content io.Reader) ([]byte, error) {
	data, err := io.ReadAll(content)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return bytes.ReplaceAll(data, []byte(crlf), []byte(lf)), nil
}

// stagedFileInfo is the fs.FileInfo of a staged file or directory: the mode nodefile's writes give
// it, the staged size, and no modification time, since none was made on disk.
type stagedFileInfo struct {
	name  string
	size  int64
	isDir bool
}

// Name returns the base name of the staged entry.
func (i stagedFileInfo) Name() string { return i.name }

// Size returns the staged file's length in bytes, 0 for a directory.
func (i stagedFileInfo) Size() int64 { return i.size }

// Mode returns 0644 for a staged file and a directory with 0755 for a staged directory.
func (i stagedFileInfo) Mode() fs.FileMode {
	if i.isDir {
		return fs.ModeDir | dirPermissions
	}
	return filePermissions
}

// ModTime returns the zero time: a staged entry was never written to disk.
func (i stagedFileInfo) ModTime() time.Time { return time.Time{} }

// IsDir reports whether the staged entry is a directory.
func (i stagedFileInfo) IsDir() bool { return i.isDir }

// Sys returns nil: a staged entry has no underlying data source.
func (i stagedFileInfo) Sys() any { return nil }
