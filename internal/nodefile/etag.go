package nodefile

import (
	"fmt"
	"hash/fnv"
)

// ETag returns the entity tag of a file's content: its FNV-1a 64-bit hash as 16 lowercase hex
// digits. A write can compare it to detect that the file changed since it was read.
func ETag(data []byte) string {
	hash := fnv.New64a()
	hash.Write(data) // a hash.Hash never returns an error
	return fmt.Sprintf("%016x", hash.Sum64())
}
