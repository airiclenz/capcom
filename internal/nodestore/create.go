package nodestore

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/airiclenz/capcom/internal/edit"
	"github.com/airiclenz/capcom/internal/frontmatter"
	"github.com/airiclenz/capcom/internal/tree"
)

// nodeFileExtension ends every node file; a node's children directory is its path without it.
const nodeFileExtension = ".md"

// hiddenNamePrefix starts the file and directory names tree.Load skips, temp files among them.
const hiddenNamePrefix = "."

// problemSeparator joins the validation problems of one node in a single message.
const problemSeparator = "; "

// writeMode says whether write may replace a file already at the node's path.
type writeMode int

const (
	// replaceFile replaces the existing file atomically, as Edit and Format do, and never creates
	// a missing file or directory.
	replaceFile writeMode = iota
	// createFile never replaces a file: one already at the path fails with an error wrapping
	// fs.ErrExist, as Create needs.
	createFile
)

// Create writes a new node of the kind, named id, below the node parent names (its id, one of its
// Aliases or a unique id prefix; "" places it at the top of the tree), with the content build
// returns. It returns the node written and the tree problems that do not concern it, their paths
// relative to the working tree root with forward slashes, for the caller to report as warnings.
//
// It runs, in this order, stopping at the first failure:
//  1. id validation: an invalid id fails with an error wrapping tree.ErrInvalidID.
//  2. the Tree load: a tree that cannot be loaded at all fails with its *tree.Error or tree.Errors.
//  3. the claim refusals, each an *ExistsError: a node of the tree answering to id, as its id or
//     one of its Aliases; then a node file inside an orphan directory named id or recording id
//     among its Aliases, which the tree leaves out; then a file a repeated-id problem of the load
//     names, named id or recording id among its Aliases, which the tree may leave out too; then a
//     node file that does not parse but whose raw frontmatter still lists id as an alias. Each
//     later refusal relies on the earlier ones having passed.
//  4. the --parent resolution: tree.ErrNotFound or tree.ErrAmbiguous, unwrapped, or, for a name
//     two files claim, a *tree.Error wrapping tree.ErrDuplicate that names both files.
//  5. triage: a tree problem naming the new file, the parent's file or the parent's children
//     directory blocks the write and comes back as the failure, a tree.Errors.
//  6. build, whose failure is returned as it is.
//  7. validation: a node that does not validate fails with a *FileError at the new file, listing
//     every problem and citing the first problem's line.
//  8. the exclusive write, which never replaces a file: right before it the parent's node file is
//     checked again, and a parent file another session removed or cannot be checked fails with a
//     *FileError at it, creating neither the parent's children directory nor the new file; a file
//     that appeared at the new node's path since the claim refusals fails with the *ExistsError
//     naming it; any other failure is returned as it is.
func (k *Kind[T]) Create(
	id string,
	parent string,
	build func() (T, error),
) (Written, tree.Errors, error) {
	if err := tree.ValidateID(id); err != nil {
		return Written{}, nil, err
	}
	nodes, problems, err := k.load()
	if err != nil {
		return Written{}, nil, err
	}
	if err := k.claimFailure(nodes, problems, id); err != nil {
		return Written{}, nil, err
	}
	path, parentPath, err := newNodePath(nodes, id, parent)
	if err != nil {
		return Written{}, nil, k.resolveFailure(err)
	}
	warnings, err := k.triage(problems, k.relative(path), k.relative(parentPath))
	if err != nil {
		return Written{}, nil, err
	}

	node, err := build()
	if err != nil {
		return Written{}, nil, err
	}
	if parentPath != "" {
		// Writing below a parent whose node file is gone would recreate its children directory
		// with no node file.
		if _, err := os.Lstat(parentPath); err != nil {
			return Written{}, nil, k.fileError(parentPath, err)
		}
	}
	written, err := k.write(path, id, node, createFile)
	if errors.Is(err, fs.ErrExist) {
		// Another writer placed the file after the claim refusals passed.
		return Written{}, nil, k.existsError(id, tree.Node{ID: id, Path: path})
	}
	if err != nil {
		return Written{}, nil, err
	}
	return written, warnings, nil
}

// load loads the kind's tree and the problems tree.Load reports while still returning the tree of
// what it could place. A failure that returns no tree is returned with its files relative to the
// working tree root.
func (k *Kind[T]) load() (*tree.Tree, tree.Errors, error) {
	nodes, err := tree.Load(k.dir, k.ReadAliases)
	if err == nil {
		return nodes, nil, nil
	}
	var problems tree.Errors
	if errors.As(err, &problems) {
		return nodes, problems, nil
	}
	var problem *tree.Error
	if errors.As(err, &problem) {
		return nil, nil, k.relativeTreeError(problem)
	}
	return nil, nil, err
}

// claimFailure refuses id, the id of a new node, when a node already answers to it or may answer
// to it; Create documents the order of the refusals. It is nil when id is free.
func (k *Kind[T]) claimFailure(nodes *tree.Tree, problems tree.Errors, id string) error {
	if existing, found := nodes.Lookup(id); found {
		return k.existsError(id, existing)
	}
	if err := k.orphanClaimFailure(problems, id); err != nil {
		return err
	}
	if err := k.duplicateClaimFailure(problems, id); err != nil {
		return err
	}
	return k.unparsedClaimFailure(nodes, problems, id)
}

// orphanClaimFailure refuses id when a node file anywhere under one of the orphan directories
// among problems is named id or records id among its Aliases. tree.Load leaves such a file out of
// the tree, so writing the id beside it would make a duplicate that surfaces only once the orphan
// directory is fixed.
func (k *Kind[T]) orphanClaimFailure(problems tree.Errors, id string) error {
	for _, problem := range problems {
		if !errors.Is(problem, tree.ErrOrphanDirectory) {
			continue
		}
		path, isAlias, err := findNodeFile(problem.Path, id, k.ReadAliases)
		if err != nil {
			return err
		}
		if path == "" {
			continue
		}
		claim := "taken by"
		if isAlias {
			claim = "an alias of"
		}
		return &ExistsError{
			Path:   k.relative(path),
			ID:     id,
			Holder: nodeID(path),
			Message: fmt.Sprintf(
				"id %q is already %s this file, inside the orphan directory %s",
				id,
				claim,
				k.relative(problem.Path),
			),
		}
	}
	return nil
}

// duplicateClaimFailure refuses id when a file a repeated-id problem among problems names is named
// id or records id among its Aliases, as far as ReadAliases reads them. tree.Load leaves such a
// file out of the tree, so writing the id beside it would make a second holder of the name that
// surfaces only once the duplicate is fixed.
func (k *Kind[T]) duplicateClaimFailure(problems tree.Errors, id string) error {
	for _, file := range duplicateIDFiles(problems) {
		claim, err := k.claimOf(file.path, id)
		if err != nil {
			return err
		}
		if claim == "" {
			continue
		}
		return &ExistsError{
			Path:   k.relative(file.path),
			ID:     id,
			Holder: nodeID(file.path),
			Message: fmt.Sprintf(
				"id %q is already %s this file, left out for the duplicate id %q",
				id,
				claim,
				file.id,
			),
		}
	}
	return nil
}

// claimOf words how the node file at path claims id: "taken by" when it is named id, "an alias of"
// when ReadAliases reads id among its Aliases, and "" when it does not claim id.
func (k *Kind[T]) claimOf(path string, id string) (string, error) {
	if nodeID(path) == id {
		return "taken by", nil
	}
	names, err := k.ReadAliases(path)
	if err != nil || !slices.Contains(names, id) {
		return "", err
	}
	return "an alias of", nil
}

// duplicateIDFile is a file a repeated-id problem of a tree load names, and that repeated id.
type duplicateIDFile struct {
	path string
	id   string
}

// duplicateIDFiles returns the files every repeated-id problem among problems names, its Path then
// its Other, each once, in order. A repeated-id problem wraps tree.ErrDuplicate and names no Alias.
func duplicateIDFiles(problems tree.Errors) []duplicateIDFile {
	var files []duplicateIDFile
	seen := map[string]bool{}
	for _, problem := range problems {
		if problem.Alias != "" || !errors.Is(problem, tree.ErrDuplicate) {
			continue
		}
		for _, path := range []string{problem.Path, problem.Other} {
			if !seen[path] {
				seen[path] = true
				files = append(files, duplicateIDFile{path: path, id: nodeID(problem.Path)})
			}
		}
	}
	return files
}

// unparsedClaimFailure refuses id when a node file that does not parse, of a node of nodes,
// anywhere under one of the orphan directories among problems or named by a repeated-id problem
// among them, still lists id as an alias in its raw frontmatter. Such a file cannot say which ids it holds, so writing id beside it would risk a
// duplicate that surfaces only once the file is fixed.
func (k *Kind[T]) unparsedClaimFailure(nodes *tree.Tree, problems tree.Errors, id string) error {
	path, err := k.findUnparsedAliasHolder(nodes, problems, id)
	if err != nil || path == "" {
		return err
	}
	holder := nodeID(path)
	return &ExistsError{
		Path:   k.relative(path),
		ID:     id,
		Holder: holder,
		Message: fmt.Sprintf(
			"id %q may be an alias of %s %s, whose file does not parse: fix it first",
			id,
			k.codec.Noun,
			holder,
		),
	}
}

// findUnparsedAliasHolder returns the first node file, of a node of nodes, anywhere under one of
// the orphan directories among problems or named by a repeated-id problem among them, from which
// RecoverAliases reads id, or "" when there is none.
func (k *Kind[T]) findUnparsedAliasHolder(
	nodes *tree.Tree,
	problems tree.Errors,
	id string,
) (string, error) {
	for _, node := range nodes.Nodes() {
		names, err := k.RecoverAliases(node.Path)
		if err != nil {
			return "", err
		}
		if slices.Contains(names, id) {
			return node.Path, nil
		}
	}
	for _, problem := range problems {
		if !errors.Is(problem, tree.ErrOrphanDirectory) {
			continue
		}
		// orphanClaimFailure has already refused a file named id, so a file found here holds id
		// as an alias.
		path, _, err := findNodeFile(problem.Path, id, k.RecoverAliases)
		if err != nil || path != "" {
			return path, err
		}
	}
	for _, file := range duplicateIDFiles(problems) {
		names, err := k.RecoverAliases(file.path)
		if err != nil {
			return "", err
		}
		if slices.Contains(names, id) {
			return file.path, nil
		}
	}
	return "", nil
}

// findNodeFile returns the first node file at any depth below dir that answers to id, skipping
// hidden names and entries that are not regular files as tree.Load does, or "" when there is none.
// A file answers to id when it is named id or, by isAlias, when aliases reads id among its Aliases.
func findNodeFile(
	dir string,
	id string,
	aliases tree.AliasFunc,
) (path string, isAlias bool, err error) {
	name := id + nodeFileExtension
	err = filepath.WalkDir(dir, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if current != dir && strings.HasPrefix(entry.Name(), hiddenNamePrefix) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), nodeFileExtension) {
			return nil
		}
		// A symlink, named pipe, socket or device is never a node file; ModeIrregular is not
		// refused, as tree.Walk does not refuse it: Windows reports it for some ordinary files.
		if entry.Type()&^fs.ModeIrregular != 0 {
			return nil
		}
		if entry.Name() == name {
			path = current
			return fs.SkipAll
		}
		names, err := aliases(current)
		if err != nil {
			return err
		}
		if slices.Contains(names, id) {
			path, isAlias = current, true
			return fs.SkipAll
		}
		return nil
	})
	return path, isAlias, err
}

// nodeID is the id a node file's name gives it: its base name without the extension.
func nodeID(path string) string {
	return strings.TrimSuffix(filepath.Base(path), nodeFileExtension)
}

// newNodePath returns the file of the new node id, below the node parent resolves to or at the top
// of the tree when parent is "", and the file of that parent node, "" at the top. A parent that
// does not resolve fails as tree.Resolve does.
func newNodePath(nodes *tree.Tree, id string, parent string) (string, string, error) {
	parentNode := tree.Node{}
	if parent != "" {
		resolved, err := nodes.Resolve(parent)
		if err != nil {
			return "", "", err
		}
		parentNode = resolved
	}
	path, err := nodes.PathFor(id, parentNode.ID)
	if err != nil {
		return "", "", err
	}
	return path, parentNode.Path, nil
}

// triage splits problems by the files a write touches, targets, given relative to the working tree
// root with forward slashes ("" entries are ignored). A problem blocks the write when it names a
// target as its file or as the other file of a duplicate, or is an orphan directory where a
// target's children go; the blocking problems come back as the failure. Every other problem is
// returned, its paths relative to the working tree root.
func (k *Kind[T]) triage(problems tree.Errors, targets ...string) (tree.Errors, error) {
	var blocking tree.Errors
	var rest tree.Errors
	for _, problem := range problems {
		relative := k.relativeTreeError(problem)
		if isBlockingProblem(relative, targets) {
			blocking = append(blocking, relative)
			continue
		}
		rest = append(rest, relative)
	}
	if len(blocking) > 0 {
		return nil, blocking
	}
	return rest, nil
}

// isBlockingProblem reports whether problem, its paths relative like targets, concerns one of
// targets: it is the target's file, the other file of the target's duplicate, or an orphan
// directory at the target's children directory.
func isBlockingProblem(problem *tree.Error, targets []string) bool {
	for _, target := range targets {
		if target == "" {
			continue
		}
		if problem.Path == target || problem.Other == target {
			return true
		}
		childrenDir := strings.TrimSuffix(target, nodeFileExtension)
		if errors.Is(problem, tree.ErrOrphanDirectory) && problem.Path == childrenDir {
			return true
		}
	}
	return false
}

// resolveFailure returns err, a failure of tree.Resolve, with the files of a duplicate's *tree.Error
// relative to the working tree root; any other failure comes back as it is.
func (k *Kind[T]) resolveFailure(err error) error {
	var problem *tree.Error
	if errors.As(err, &problem) {
		return k.relativeTreeError(problem)
	}
	return err
}

// relativeTreeError returns a copy of problem whose Path and Other are relative to the working tree
// root with forward slashes; a path outside it is kept as it is.
func (k *Kind[T]) relativeTreeError(problem *tree.Error) *tree.Error {
	relative := *problem
	relative.Path = k.relative(problem.Path)
	relative.Other = k.relative(problem.Other)
	return &relative
}

// write validates node and writes it, with LF line endings, to path, the absolute file of the node
// id: under replaceFile replacing the file there atomically, failing with a *FileError at it when
// the file or its directory is missing, which it never creates; under createFile only where no
// file is, failing with an error wrapping fs.ErrExist otherwise. A node that does not validate
// fails with a *FileError listing every problem, citing the first problem's line, and is not
// written.
func (k *Kind[T]) write(path string, id string, node T, mode writeMode) (Written, error) {
	data, err := k.encode(path, node)
	if err != nil {
		return Written{}, err
	}
	return k.place(path, id, data, mode)
}

// encode validates node, to be written to path, the absolute file of the node, and returns its
// serialized bytes with LF line endings; a node that does not validate fails as write documents.
func (k *Kind[T]) encode(path string, node T) ([]byte, error) {
	relative := k.relative(path)
	if problems := k.codec.Validate(relative, node); len(problems) > 0 {
		return nil, validationError(relative, problems)
	}
	return bytes.ReplaceAll(k.codec.Serialize(node), []byte("\r\n"), []byte("\n")), nil
}

// place writes data, the bytes encode returned, to path, the absolute file of the node id, under
// mode as write documents, and returns the node written.
func (k *Kind[T]) place(path string, id string, data []byte, mode writeMode) (Written, error) {
	writeFile := tree.ReplaceFileAtomic
	if mode == createFile {
		writeFile = tree.WriteFileExclusive
	}
	err := writeFile(path, bytes.NewReader(data))
	if mode == replaceFile && errors.Is(err, fs.ErrNotExist) {
		// Another session moved or removed the node file since it was read.
		return Written{}, k.fileError(path, err)
	}
	if err != nil {
		return Written{}, err
	}
	return Written{ID: id, Path: k.relative(path), ETag: edit.ETag(data)}, nil
}

// validationError is the *FileError of a node at path, relative to the working tree root, that
// does not validate: every problem's message, at the first problem's line.
func validationError(path string, problems []*frontmatter.Error) *FileError {
	messages := make([]string, 0, len(problems))
	for _, problem := range problems {
		messages = append(messages, problem.Message)
	}
	return &FileError{
		Path:    path,
		Line:    problems[0].Line,
		Message: strings.Join(messages, problemSeparator),
	}
}
