//go:build unix

package clienthost

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// openFile is os.OpenFile with O_NONBLOCK, so that opening a FIFO returns at
// once instead of waiting for its other end, for the caller to refuse it by
// its handle; a regular file ignores the flag. A FIFO that no reader has open
// fails a write's open with ENXIO, which is errNotRegular here
// (0049-PLAN-subpackage-fixes.md, phase 2, P4).
func openFile(path string, flag int, perm fs.FileMode) (*os.File, error) {
	f, err := os.OpenFile(path, flag|syscall.O_NONBLOCK, perm) //nolint:gosec // G304: the agent's path, which the client's policy checks before Files is called.
	if errors.Is(err, syscall.ENXIO) {
		return nil, errNotRegular
	}
	return f, err
}
