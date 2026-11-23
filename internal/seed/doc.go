// Package seed holds the base set of Knowledge Notes that capcom ships: rules that hold in any
// repository, written as markdown files and embedded in the binary. The files sit in the directory
// notes in a store's Knowledge layout: one root Note file, and the Notes below it in the directory
// named after their parent. A Note is added to the set, or dropped from it, by adding or deleting a
// file there; no Go file names a Note of the set.
//
// Load reads the embedded set and LoadFS reads a set from any file system, so a caller's tests can
// load a made-up one. Both check the whole set and fail naming the file that breaks it. A Set
// answers with its root Note, with every Note root first, and with one Note by its id.
//
// A Note that capcom seeds carries two Labels, built by Set.Labels: `seed:<root id>` marks it as
// seeded, which Set.IsSeeded reports, and `seed-text:<version>:<hash>` records the Hash of the
// title and body it was seeded with. Set.State holds the store's Note of an id against the set's:
// Same when the text is the set's, Outdated when it is still the text an earlier set seeded, and
// Differs when it was edited or was never seeded. Set.Relabel gives the Labels of an Outdated Note
// once its text is replaced by the set's.
package seed
