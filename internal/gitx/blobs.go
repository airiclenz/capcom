package gitx

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Fields of a `cat-file --batch` answer header: "<object> <type> <size>", or "<object> missing".
const (
	batchHeaderFields  = 3
	batchMissingFields = 2
	batchMissing       = "missing"
)

// ErrBlobMissing reports an object name ReadBlobs asked for that git does not hold.
var ErrBlobMissing = errors.New("blob is missing")

// errNotBlob reports an object ReadBlobs was asked for that is not a blob, such as a tree.
var errNotBlob = errors.New("object is not a blob")

// Blob is what ReadBlobs read for one object: its content, or why it could not be read.
type Blob struct {
	// Data is the blob's content; nil when Err is set.
	Data []byte
	// Err wraps ErrBlobTooLarge for a blob over the limit, ErrBlobMissing for an object git does
	// not hold, and is an *Error carrying git's stderr for one git failed on while reading it.
	Err error
}

// ReadBlobs reads the blobs objects names in dir through one `git cat-file --batch` process, asking
// in the order given, and returns what it read for each object name. A blob larger than limit is
// never buffered: its content is skipped and its Err wraps ErrBlobTooLarge. When git dies part
// way, the object it was answering gets an Err carrying git's stderr and the rest are read by a
// new process. ReadBlobs fails only when git answered no object at all, as outside a repository;
// an empty list starts no process. Object names must not hold a line break.
func ReadBlobs(dir string, objects []string, limit int64) (map[string]Blob, error) {
	blobs := make(map[string]Blob, len(objects))
	pending := make([]string, 0, len(objects))
	isPending := make(map[string]bool, len(objects))
	for _, object := range objects {
		if !isPending[object] {
			isPending[object] = true
			pending = append(pending, object)
		}
	}
	for len(pending) > 0 {
		answered, hasStarted, err := readBatch(dir, pending, limit, blobs)
		if err == nil {
			break
		}
		if len(blobs) == 0 && !hasStarted {
			return nil, err
		}
		blobs[pending[answered]] = Blob{Err: err}
		pending = pending[answered+1:]
	}
	return blobs, nil
}

// readBatch reads objects in order through one `git cat-file --batch` process into blobs. It
// returns how many objects git answered and, when git died before answering them all, whether
// it had started on the next one, with the error that ended it.
func readBatch(
	dir string,
	objects []string,
	limit int64,
	blobs map[string]Blob,
) (answered int, hasStarted bool, err error) {
	args := []string{"cat-file", "--batch"}
	command := exec.Command(gitProgram, args...)
	command.Dir = dir
	command.Env = append(os.Environ(), optionalLocksOff)
	var stderr strings.Builder
	command.Stderr = &stderr
	input, err := command.StdinPipe()
	if err != nil {
		return 0, false, fmt.Errorf("run git %s: %w", strings.Join(args, " "), err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		return 0, false, fmt.Errorf("run git %s: %w", strings.Join(args, " "), err)
	}
	if err := command.Start(); err != nil {
		return 0, false, fmt.Errorf("run git %s: %w", strings.Join(args, " "), err)
	}
	go writeObjectNames(input, objects)

	reader := bufio.NewReader(output)
	var readErr error
	for answered < len(objects) {
		var blob Blob
		blob, hasStarted, readErr = readAnswer(reader, objects[answered], limit)
		if readErr != nil {
			break
		}
		blobs[objects[answered]] = blob
		answered++
	}
	// Drain what is left so git never blocks on a full pipe before Wait.
	_, _ = io.Copy(io.Discard, reader)
	waitErr := command.Wait()
	if readErr == nil {
		// Every object is answered, so how git exited changes nothing read.
		return answered, false, nil
	}
	var exitFailure *exec.ExitError
	if errors.As(waitErr, &exitFailure) {
		return answered, hasStarted, &Error{Args: args, Stderr: stderr.String(), Err: waitErr}
	}
	if errors.Is(readErr, io.EOF) {
		readErr = io.ErrUnexpectedEOF
	}
	return answered, hasStarted, fmt.Errorf("git %s in %s: %w", strings.Join(args, " "), dir, readErr)
}

// writeObjectNames writes one object name per line to input and closes it. A write that fails
// because git died is left for the reader to notice through git's missing answers.
func writeObjectNames(input io.WriteCloser, objects []string) {
	writer := bufio.NewWriter(input)
	for _, object := range objects {
		if _, err := writer.WriteString(object + "\n"); err != nil {
			break
		}
	}
	_ = writer.Flush()
	_ = input.Close()
}

// readAnswer reads git's answer for object from reader. It fails when the stream ends or breaks
// before the answer is complete, and then reports whether any of the answer had arrived.
func readAnswer(reader *bufio.Reader, object string, limit int64) (Blob, bool, error) {
	header, err := reader.ReadString('\n')
	if err != nil {
		return Blob{}, header != "", err
	}
	fields := strings.Fields(header)
	if len(fields) == batchMissingFields && fields[1] == batchMissing {
		return Blob{Err: fmt.Errorf("%s: %w", object, ErrBlobMissing)}, true, nil
	}
	if len(fields) != batchHeaderFields {
		return Blob{}, true, fmt.Errorf("%s: unexpected answer %q", object, header)
	}
	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || size < 0 {
		return Blob{}, true, fmt.Errorf("%s: unexpected answer %q", object, header)
	}

	// The content is followed by a line feed.
	if size > limit || fields[1] != blobType {
		if _, err := io.CopyN(io.Discard, reader, size+1); err != nil {
			return Blob{}, true, err
		}
		if fields[1] != blobType {
			return Blob{Err: fmt.Errorf("%s: %w", object, errNotBlob)}, true, nil
		}
		return Blob{Err: fmt.Errorf("%s: %w", object, ErrBlobTooLarge)}, true, nil
	}
	data := make([]byte, size+1)
	if _, err := io.ReadFull(reader, data); err != nil {
		return Blob{}, true, err
	}
	return Blob{Data: data[:size]}, true, nil
}
