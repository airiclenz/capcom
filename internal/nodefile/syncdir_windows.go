package nodefile

// syncDir does nothing on Windows, where a directory cannot be opened for fsync; NTFS journals
// the rename itself.
func syncDir(string) error {
	return nil
}
