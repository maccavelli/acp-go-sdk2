package acp

import (
	"os"
	"path/filepath"
	"testing"
)

// testAbsPath is a path under the host's temporary directory, absolute on
// every host, for tests that send a path the protocol requires to be
// absolute; a POSIX literal such as "/" is not absolute on Windows
// (0015-PLAN-validate-absolute-protocol-paths.md, step 4).
func testAbsPath(elem ...string) string {
	return filepath.Join(append([]string{os.TempDir()}, elem...)...)
}

// TestIsPortableAbs pins the rule for a path the SDK sends: absolute in POSIX
// or Windows form, on every host (0015-MADR-validate-absolute-protocol-paths.md,
// A4).
func TestIsPortableAbs(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"/a", "/", `C:\a`, "C:/a", `c:\`, `\\server\share`, `\\?\C:\a`, "//server/share"} {
		if !isPortableAbs(p) {
			t.Errorf("isPortableAbs(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"", "a", "./a", "../a", `\a`, "C:a", "C:", "1:/a", ":/a", "~/a"} {
		if isPortableAbs(p) {
			t.Errorf("isPortableAbs(%q) = true, want false", p)
		}
	}
	if p := testAbsPath("x"); !filepath.IsAbs(p) || !isPortableAbs(p) {
		t.Errorf("testAbsPath(%q) is not absolute on this host", p)
	}
}
