package tree

import "github.com/airiclenz/capcom/internal/nodefile"

// Forwarders to the guarded reader and the atomic writes, which moved to internal/nodefile, so
// that callers compile unchanged until they import nodefile themselves.
var (
	// ReadRegularFile is nodefile.ReadRegularFile.
	ReadRegularFile = nodefile.ReadRegularFile
	// ReadNodeFile is nodefile.ReadNodeFile.
	ReadNodeFile = nodefile.ReadNodeFile
	// WriteFileAtomic is nodefile.WriteFileAtomic.
	WriteFileAtomic = nodefile.WriteFileAtomic
	// ReplaceFileAtomic is nodefile.ReplaceFileAtomic.
	ReplaceFileAtomic = nodefile.ReplaceFileAtomic
	// WriteFileAtomicVerbatim is nodefile.WriteFileAtomicVerbatim.
	WriteFileAtomicVerbatim = nodefile.WriteFileAtomicVerbatim
	// ReplaceFileVerbatim is nodefile.ReplaceFileVerbatim.
	ReplaceFileVerbatim = nodefile.ReplaceFileVerbatim
	// WriteFileExclusive is nodefile.WriteFileExclusive.
	WriteFileExclusive = nodefile.WriteFileExclusive
)
