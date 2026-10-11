package tree

// SetMoveRename replaces the rename a Move makes its renames with, so a test can inject a failure.
func SetMoveRename(move *Move, rename func(oldPath string, newPath string) error) {
	move.rename = rename
}
