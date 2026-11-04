//go:build !unix

package clienthost

import (
	"io/fs"
	"os"
)

// openFile is os.OpenFile. Off Unix an open does not wait for a pipe's other
// end, so the caller refuses what is not a regular file by its handle, as on
// Unix (0049-PLAN-subpackage-fixes.md, phase 2, P4).
func openFile(path string, flag int, perm fs.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag, perm) //nolint:gosec // G304: the agent's path, which the client's policy checks before Files is called.
}
