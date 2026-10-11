// Package enginetest holds what the generator's engine tests share: a reader
// of the txtar cases in testdata/engine, the hand-written runtime that the code
// generated for a synthetic schema compiles with, and a listing of the API
// that code declares. See 0011-PLAN-engine-tests.md.
package enginetest

import (
	"bytes"
	"fmt"
	"strings"
)

// File is one file of an Archive.
type File struct {
	Name string
	Data []byte
}

// Archive is a txtar archive: a comment, then files, each started by a line
// "-- name --".
type Archive struct {
	Comment []byte
	Files   []File
}

// File returns the data of the named file.
func (a *Archive) File(name string) ([]byte, bool) {
	for _, f := range a.Files {
		if f.Name == name {
			return f.Data, true
		}
	}
	return nil, false
}

// Set replaces the data of the named file, or adds the file at the end.
func (a *Archive) Set(name string, data []byte) {
	for i := range a.Files {
		if a.Files[i].Name == name {
			a.Files[i].Data = data
			return
		}
	}
	a.Files = append(a.Files, File{Name: name, Data: data})
}

// ParseArchive reads a txtar archive. Each file's data ends with a newline.
// A file name must be a plain name, not a path, and appear once.
func ParseArchive(b []byte) (*Archive, error) {
	a := &Archive{}
	var cur *bytes.Buffer
	finish := func() {
		if cur != nil {
			a.Files[len(a.Files)-1].Data = fixNewline(cur.Bytes())
		}
	}
	for line := range strings.Lines(string(b)) {
		if name, ok := marker(line); ok {
			finish()
			if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
				return nil, fmt.Errorf("file name %q is not a plain name", name)
			}
			if _, dup := a.File(name); dup {
				return nil, fmt.Errorf("duplicate file %q", name)
			}
			a.Files = append(a.Files, File{Name: name})
			cur = &bytes.Buffer{}
			continue
		}
		if cur == nil {
			a.Comment = append(a.Comment, line...)
			continue
		}
		cur.WriteString(line)
	}
	finish()
	return a, nil
}

// marker reports whether line is "-- name --" and returns the name.
func marker(line string) (string, bool) {
	line = strings.TrimRight(line, "\r\n")
	inner, ok := strings.CutPrefix(line, "-- ")
	if !ok {
		return "", false
	}
	name, ok := strings.CutSuffix(inner, " --")
	return strings.TrimSpace(name), ok
}

// fixNewline returns b ending with a newline, unless it is empty.
func fixNewline(b []byte) []byte {
	if len(b) == 0 || b[len(b)-1] == '\n' {
		return bytes.Clone(b)
	}
	return append(bytes.Clone(b), '\n')
}

// Format writes a as a txtar archive.
func Format(a *Archive) []byte {
	var buf bytes.Buffer
	buf.Write(fixNewline(a.Comment))
	for _, f := range a.Files {
		fmt.Fprintf(&buf, "-- %s --\n", f.Name)
		buf.Write(fixNewline(f.Data))
	}
	return buf.Bytes()
}
