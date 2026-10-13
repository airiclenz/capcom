// Package nodefile reads repository-controlled files through one guarded reader, ReadRegularFile,
// and its store-file form, ReadNodeFile. The reader refuses an entry that is not a regular file
// with ErrNotRegular and one larger than 1 MiB with ErrTooLarge, never follows a symlink and never
// blocks on a named pipe. The package imports nothing from internal/, so every package that reads
// a file from the working tree can import it.
package nodefile
