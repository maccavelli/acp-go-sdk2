package clienthost

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// Files reads and writes text files for fs/read_text_file and
// fs/write_text_file, from this process's file system
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 3).
//
// ReadTextFile returns the file's lines from line, 1-based, the first when
// it is absent, and at most limit of them, all of them when it is absent;
// a line keeps its end, "\n" or "\r\n", as the file has it, and a line past
// the file's end gives no content. It reads the file only as far as the lines
// it returns, the whole file only when neither is set. WriteTextFile writes
// content to the file, creating it when it does not exist, as the
// specification requires, and replacing what it held. ReadTextFile answers a
// file that does not exist with acp.NewResourceNotFound, and WriteTextFile a
// file whose directory does not exist, unless MkdirAll; both answer a path
// that is not absolute with acp.NewInvalidParams, and refuse a path that is
// not a regular file, such as a directory, a device or a FIFO. They check the
// file they opened, not the path before opening it, and on Unix an open does
// not wait for a FIFO's other end, so a path swapped for a FIFO is refused,
// not waited on (0049-PLAN-subpackage-fixes.md, phase 2, P3 and P4).
//
// Files applies no policy. The specification asks a client to check each
// path against the session's roots, and to ask its user before a write
// changes a file that controls execution or approval; a client does so in a
// ReadTextFile or WriteTextFile of its own, which wins over the embedded
// one, and calls Files' when it allows the request. The specification has
// fs/read_text_file return a file's unsaved changes in the client's editor;
// a client that holds such changes answers from them, not from Files, which
// reads only what is on disk. The zero Files is ready to use.
type Files struct {
	// MkdirAll has WriteTextFile create the missing directories of a path,
	// with permission 0755 before the umask. False answers such a path with
	// acp.NewResourceNotFound, naming the directory.
	MkdirAll bool
}

// readBufferSize is the buffer ReadTextFile reads lines through; a longer
// line is read in pieces of it.
const readBufferSize = 64 << 10

// errNotRegular is openFile's error for a path whose open shows it is not a
// regular file before a handle exists to check.
var errNotRegular = errors.New("not a regular file")

// ReadTextFile returns the lines of the file at req.Path that req.Line and
// req.Limit select.
func (Files) ReadTextFile(_ context.Context, req acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	if err := absolute(req.Path); err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	f, fi, err := openRegular(req.Path, os.O_RDONLY)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	defer f.Close() //nolint:errcheck // a read-only file's Close reports nothing the answer needs.
	if req.Line != nil && *req.Line == 0 {
		return acp.ReadTextFileResponse{}, acp.NewInvalidParams(map[string]any{"line": "lines are numbered from 1"})
	}
	content, err := readLines(f, fi.Size(), req.Line, req.Limit)
	if err != nil {
		return acp.ReadTextFileResponse{}, fmt.Errorf("clienthost: reading %s: %w", req.Path, err)
	}
	return acp.ReadTextFileResponse{Content: content}, nil
}

// WriteTextFile writes req.Content to the file at req.Path.
func (f Files) WriteTextFile(_ context.Context, req acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	if err := absolute(req.Path); err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	dir := filepath.Dir(req.Path)
	if f.MkdirAll {
		if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // G301: a directory of the user's files, which the umask narrows.
			return acp.WriteTextFileResponse{}, pathError(dir, err)
		}
	} else if _, err := os.Stat(dir); err != nil {
		return acp.WriteTextFileResponse{}, pathError(dir, err)
	}
	file, _, err := openRegular(req.Path, os.O_WRONLY|os.O_CREATE)
	if err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	if err := file.Truncate(0); err != nil {
		_ = file.Close() //nolint:errcheck // the truncate's error is the one to report.
		return acp.WriteTextFileResponse{}, pathError(req.Path, err)
	}
	if _, err := file.WriteString(req.Content); err != nil {
		_ = file.Close() //nolint:errcheck // the write's error is the one to report.
		return acp.WriteTextFileResponse{}, pathError(req.Path, err)
	}
	if err := file.Close(); err != nil {
		return acp.WriteTextFileResponse{}, pathError(req.Path, err)
	}
	return acp.WriteTextFileResponse{}, nil
}

// absolute refuses a path that is not absolute.
func absolute(path string) error {
	if !filepath.IsAbs(path) {
		return acp.NewInvalidParams(map[string]any{"path": "not an absolute path: " + path})
	}
	return nil
}

// openRegular opens path with flag, a new file with permission 0644 before
// the umask and an existing one keeping its own, and refuses what the handle
// shows is not a regular file; it returns the file and its info. A missing
// path is acp.NewResourceNotFound.
func openRegular(path string, flag int) (*os.File, fs.FileInfo, error) {
	f, err := openFile(path, flag, 0o644)
	switch {
	case errors.Is(err, errNotRegular):
		return nil, nil, fmt.Errorf("clienthost: %s is not a regular file", path)
	case err != nil:
		return nil, nil, pathError(path, err)
	}
	fi, err := f.Stat()
	if err == nil && !fi.Mode().IsRegular() {
		err = fmt.Errorf("clienthost: %s is not a regular file", path)
	} else if err != nil {
		err = fmt.Errorf("clienthost: %w", err)
	}
	if err != nil {
		_ = f.Close() //nolint:errcheck // the refusal is the error to report.
		return nil, nil, err
	}
	return f, fi, nil
}

// pathError is acp.NewResourceNotFound for a path that does not exist, and
// err otherwise.
func pathError(path string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return acp.NewResourceNotFound(path)
	}
	return fmt.Errorf("clienthost: %w", err)
}

// readLines returns the lines of r from line, 1-based, at most limit of them,
// each with its end as r has it, reading r no further than the last of them;
// with neither, all of r, into a string grown to size, r's size. The counts are uint64, so a line or a limit past
// what int holds on a 32-bit target is a count like any other
// (0049-PLAN-subpackage-fixes.md, phase 2, P5).
func readLines(r io.Reader, size int64, line, limit *uint32) (string, error) {
	var b strings.Builder
	if line == nil && limit == nil {
		if size > 0 && size < math.MaxInt {
			b.Grow(int(size) + 1)
		}
		_, err := io.Copy(&b, r)
		return b.String(), err
	}
	br := bufio.NewReaderSize(r, readBufferSize)
	if line != nil {
		for skip := uint64(*line) - 1; skip > 0; skip-- {
			ok, err := readLine(br, nil)
			if err != nil || !ok {
				return "", err
			}
		}
	}
	keep := func(piece []byte) { b.Write(piece) }
	for n := uint64(0); limit == nil || n < uint64(*limit); n++ {
		ok, err := readLine(br, keep)
		if err != nil {
			return "", err
		}
		if !ok {
			break
		}
	}
	return b.String(), nil
}

// readLine reads one line of br, with its end, giving each piece of it to
// keep unless keep is nil, so a line longer than br's buffer is never held
// whole. It reports whether there was a line: false at the end of br, and
// true for a last line without its end.
func readLine(br *bufio.Reader, keep func([]byte)) (bool, error) {
	read := false
	for {
		piece, err := br.ReadSlice('\n')
		if len(piece) > 0 {
			read = true
			if keep != nil {
				keep(piece)
			}
		}
		switch {
		case err == nil:
			return true, nil
		case errors.Is(err, bufio.ErrBufferFull):
		case errors.Is(err, io.EOF):
			return read, nil
		default:
			return read, err
		}
	}
}
