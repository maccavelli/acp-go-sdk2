// Package output writes the generated files into the output directory.
package output

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/emit"
)

// fileMode is the mode of every file the generator writes, as git records
// committed source.
const fileMode = 0o644

// Write replaces the files of dir that differ from files, staging all of them
// before replacing any.
//
// A file whose bytes equal the existing file's is left alone. Every other file
// is written to a temporary file in dir, synced, set to fileMode, and closed;
// if any of that fails, the temporary files are removed and nothing is
// replaced. Each is then renamed into place in the order of files. A rename
// that fails ends the run with an error naming the files already replaced and
// those left as they were.
func Write(dir string, files []emit.File) error { return writeWith(dir, files, os.Rename) }

// staged is a file written to its temporary path and not yet renamed.
type staged struct {
	name, tmp string
}

// writeWith is Write with the rename it uses, so a test can make one fail.
func writeWith(dir string, files []emit.File, rename func(oldpath, newpath string) error) error {
	var changed []emit.File
	for _, f := range files {
		old, err := os.ReadFile(filepath.Join(dir, f.Name)) //nolint:gosec // G304: dir is the generator's explicit output directory.
		switch {
		case err == nil && bytes.Equal(old, f.Src):
			continue
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			return fmt.Errorf("compare %s: %w", f.Name, err)
		}
		changed = append(changed, f)
	}

	temps := make([]staged, 0, len(changed))
	for _, f := range changed {
		tmp, err := stage(dir, f)
		if err != nil {
			return errors.Join(fmt.Errorf("stage %s: %w", f.Name, err), remove(temps))
		}
		temps = append(temps, staged{name: f.Name, tmp: tmp})
	}

	for i, s := range temps {
		if err := rename(s.tmp, filepath.Join(dir, s.name)); err != nil {
			// A renamed temporary file is the destination now; only the
			// rest are removed.
			return errors.Join(fmt.Errorf("replace %s: %w; replaced: %s; left as they were: %s",
				s.name, err, nameList(temps[:i]), nameList(temps[i:])), remove(temps[i:]))
		}
	}
	return nil
}

// stage writes f to a new temporary file in dir and returns its path. The
// name starts with a dot, so the go command and gofmt ignore a leftover.
func stage(dir string, f emit.File) (string, error) {
	t, err := os.CreateTemp(dir, "."+f.Name+".*.tmp")
	if err != nil {
		return "", err
	}
	_, err = t.Write(f.Src)
	if err == nil {
		err = t.Sync()
	}
	if err == nil {
		err = t.Chmod(fileMode)
	}
	if cerr := t.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", errors.Join(err, os.Remove(t.Name()))
	}
	return t.Name(), nil
}

// remove deletes the temporary files of temps.
func remove(temps []staged) error {
	var errs []error
	for _, s := range temps {
		if err := os.Remove(s.tmp); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// nameList lists the destination names of temps, or "none".
func nameList(temps []staged) string {
	if len(temps) == 0 {
		return "none"
	}
	names := make([]string, 0, len(temps))
	for _, s := range temps {
		names = append(names, s.name)
	}
	return strings.Join(names, ", ")
}
