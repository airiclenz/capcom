package tree

import "github.com/airiclenz/capcom/internal/nodefile"

// Forwarders to the guarded reader, which moved to internal/nodefile, so that callers compile
// unchanged until they import nodefile themselves.
var (
	// ReadRegularFile is nodefile.ReadRegularFile.
	ReadRegularFile = nodefile.ReadRegularFile
	// ReadNodeFile is nodefile.ReadNodeFile.
	ReadNodeFile = nodefile.ReadNodeFile
)
