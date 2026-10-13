// Package nodefile reads repository-controlled files through one guarded reader, ReadRegularFile,
// and its store-file form, ReadNodeFile, and writes node files crash-safely. The reader refuses an
// entry that is not a regular file with ErrNotRegular and one larger than 1 MiB with ErrTooLarge,
// never follows a symlink and never blocks on a named pipe. The writes (WriteFileAtomic,
// ReplaceFileAtomic, WriteFileAtomicVerbatim, ReplaceFileVerbatim and WriteFileExclusive) stream
// into a temp file beside the target and rename or hard-link it into place, so readers see either
// the old file or the whole new one. ETag hashes a file's content into the entity tag a write
// compares to detect that the file changed since it was read. The package imports nothing from
// internal/, so every package that reads or writes a file in the working tree can import it.
package nodefile
