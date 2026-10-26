package acptest

import (
	"os"
	"path/filepath"
)

// Abs returns path as an absolute path on the host: path itself, cleaned,
// when the host reads it as absolute, and otherwise path under os.TempDir().
// The SDK checks a path it receives with filepath.IsAbs, which reads a POSIX
// path such as /work/a.txt as relative on Windows, so a fixture written so
// passes through Abs on every host, as the root's tests do with testAbsPath
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 5).
func Abs(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(os.TempDir(), path)
}
