package output

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/emit"
)

var names = []string{"constants_gen.go", "types_gen.go", "agent_gen.go", "client_gen.go", "helpers_gen.go"}

// generated returns the five files, each holding its name and version.
func generated(version string) []emit.File {
	files := make([]emit.File, 0, len(names))
	for _, n := range names {
		files = append(files, emit.File{Name: n, Src: []byte("package acp\n\n// " + n + " " + version + "\n")})
	}
	return files
}

// writeAll writes files into dir directly, at mode 0o600.
func writeAll(t *testing.T, dir string, files []emit.File) {
	t.Helper()
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.Name), f.Src, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// requireContents requires each file of files to hold its bytes in dir.
func requireContents(t *testing.T, dir string, files []emit.File) {
	t.Helper()
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(dir, f.Name))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != string(f.Src) {
			t.Errorf("%s holds %q, want %q", f.Name, b, f.Src)
		}
	}
}

// requireNoTemporaryFiles requires dir to hold no staged file.
func requireNoTemporaryFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") && strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temporary file %s left in the output directory", e.Name())
		}
	}
}

func TestWriteReplacesChangedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "types_gen.go"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := generated("v1")
	if err := Write(dir, files); err != nil {
		t.Fatal(err)
	}
	requireContents(t, dir, files)
	if runtime.GOOS != "windows" {
		for _, n := range names {
			info, err := os.Stat(filepath.Join(dir, n))
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != 0o644 {
				t.Errorf("%s has mode %v, want 0644", n, got)
			}
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "keep.txt")); err != nil || string(b) != "sentinel\n" {
		t.Errorf("sentinel changed: %q, %v", b, err)
	}
	requireNoTemporaryFiles(t, dir)
}

func TestWriteLeavesUnchangedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := generated("v1")
	if err := Write(dir, files); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour).Truncate(time.Second)
	for _, n := range names {
		if err := os.Chtimes(filepath.Join(dir, n), past, past); err != nil {
			t.Fatal(err)
		}
	}
	modTime := func(n string) time.Time {
		info, err := os.Stat(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		return info.ModTime()
	}

	if err := Write(dir, files); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if got := modTime(n); !got.Equal(past) {
			t.Errorf("unchanged %s was rewritten: modified %v, want %v", n, got, past)
		}
	}

	files[1].Src = []byte("package acp\n\n// types_gen.go v2\n")
	if err := Write(dir, files); err != nil {
		t.Fatal(err)
	}
	requireContents(t, dir, files)
	for _, n := range names {
		changed := !modTime(n).Equal(past)
		if changed != (n == "types_gen.go") {
			t.Errorf("%s: modification time moved = %v, want %v", n, changed, n == "types_gen.go")
		}
	}
	requireNoTemporaryFiles(t, dir)
}

func TestWriteStagingFailureReplacesNothing(t *testing.T) {
	t.Parallel()
	t.Run("a file that cannot be staged after others were", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		old := generated("v1")
		writeAll(t, dir, old)
		files := generated("v2")
		files = append(files[:2:2], emit.File{Name: "sub/x_gen.go", Src: []byte("package acp\n")})
		err := Write(dir, files)
		if err == nil || !strings.Contains(err.Error(), "stage sub/x_gen.go") {
			t.Fatalf("Write error = %v, want one staging sub/x_gen.go", err)
		}
		requireContents(t, dir, old)
		requireNoTemporaryFiles(t, dir)
	})
	t.Run("a read-only directory", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("needs a directory the test cannot write to")
		}
		dir := t.TempDir()
		old := generated("v1")
		writeAll(t, dir, old)
		if err := os.Chmod(dir, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		err := Write(dir, generated("v2"))
		if err == nil || !strings.Contains(err.Error(), "stage constants_gen.go") {
			t.Fatalf("Write error = %v, want one staging constants_gen.go", err)
		}
		requireContents(t, dir, old)
		requireNoTemporaryFiles(t, dir)
	})
}

// TestWriteStagedFileFailureReplacesNothing requires a temporary file that
// was created and then cannot be written to be removed, and nothing
// replaced (0019-PLAN-generator-hardening.md, G19).
func TestWriteStagedFileFailureReplacesNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old := generated("v1")
	writeAll(t, dir, old)
	closed := func(dir, pattern string) (*os.File, error) {
		f, err := os.CreateTemp(dir, pattern)
		if err == nil {
			err = f.Close()
		}
		return f, err
	}
	err := writeWith(dir, generated("v2"), closed, os.Rename)
	if err == nil || !strings.Contains(err.Error(), "stage constants_gen.go") || !strings.Contains(err.Error(), "already closed") {
		t.Fatalf("writeWith error = %v, want staging constants_gen.go to fail on a closed file", err)
	}
	requireContents(t, dir, old)
	requireNoTemporaryFiles(t, dir)
}

func TestWriteRenameFailureReports(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old := generated("v1")
	writeAll(t, dir, old)
	files := generated("v2")
	failing := func(from, to string) error {
		if filepath.Base(to) == "helpers_gen.go" {
			return errors.New("planted rename failure")
		}
		return os.Rename(from, to)
	}
	err := writeWith(dir, files, os.CreateTemp, failing)
	if err == nil {
		t.Fatal("writeWith succeeded")
	}
	for _, w := range []string{
		"replace helpers_gen.go", "planted rename failure",
		"replaced: constants_gen.go, types_gen.go, agent_gen.go, client_gen.go", "left as they were: helpers_gen.go",
	} {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q does not contain %q", err, w)
		}
	}
	requireContents(t, dir, files[:4])
	requireContents(t, dir, old[4:])
	requireNoTemporaryFiles(t, dir)
}

func TestWriteCompareFailureReplacesNothing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	old := generated("v1")
	writeAll(t, dir, old[:4])
	if err := os.Mkdir(filepath.Join(dir, "helpers_gen.go"), 0o750); err != nil {
		t.Fatal(err)
	}
	err := Write(dir, generated("v2"))
	if err == nil || !strings.Contains(err.Error(), "helpers_gen.go") {
		t.Fatalf("Write error = %v, want one naming helpers_gen.go", err)
	}
	requireContents(t, dir, old[:4])
	requireNoTemporaryFiles(t, dir)
}
