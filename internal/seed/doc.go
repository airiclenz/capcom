// Package seed holds the base set of Knowledge Notes that capcom ships: rules that hold in any
// repository, written as markdown files and embedded in the binary. The files sit in the directory
// notes in a store's Knowledge layout: one root Note file, and the Notes below it in the directory
// named after their parent. A Note is added to the set, or dropped from it, by adding or deleting a
// file there; no Go file names a Note of the set.
//
// Load reads the embedded set and LoadFS reads a set from any file system, so a caller's tests can
// load a made-up one. Both check the whole set and fail naming the file that breaks it. A Set
// answers with its root Note, with every Note root first, and with one Note by its id.
package seed
