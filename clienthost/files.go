package clienthost

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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
// the file's end gives no content. WriteTextFile writes content to the
// file, creating it when it does not exist, as the specification requires,
// and replacing what it held. A path that does not exist is answered with
// acp.NewResourceNotFound, a path that is not absolute with
// acp.NewInvalidParams, and a path that is not a regular file, such as a
// directory or a device, is refused.
//
// Files applies no policy. The specification asks a client to check each
// path against the session's roots, and to ask its user before a write
// changes a file that controls execution or approval; a client does so in a
// ReadTextFile or WriteTextFile of its own, which wins over the embedded
// one, and calls Files' when it allows the request. An editor that holds
// unsaved changes returns them instead. The zero Files is ready to use.
type Files struct {
	// MkdirAll has WriteTextFile create the missing directories of a path,
	// with permission 0755 before the umask. False answers such a path with
	// acp.NewResourceNotFound, naming the directory.
	MkdirAll bool
}

// ReadTextFile returns the lines of the file at req.Path that req.Line and
// req.Limit select.
func (Files) ReadTextFile(_ context.Context, req acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	if err := regular(req.Path, false); err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	if req.Line != nil && *req.Line == 0 {
		return acp.ReadTextFileResponse{}, acp.NewInvalidParams(map[string]any{"line": "lines are numbered from 1"})
	}
	b, err := os.ReadFile(req.Path)
	if err != nil {
		return acp.ReadTextFileResponse{}, pathError(req.Path, err)
	}
	return acp.ReadTextFileResponse{Content: selectLines(string(b), req.Line, req.Limit)}, nil
}

// WriteTextFile writes req.Content to the file at req.Path.
func (f Files) WriteTextFile(_ context.Context, req acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	if err := regular(req.Path, true); err != nil {
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
	if err := os.WriteFile(req.Path, []byte(req.Content), 0o644); err != nil { //nolint:gosec // G306: a text file of the user's, which the umask narrows; an existing file keeps its mode.
		return acp.WriteTextFileResponse{}, pathError(req.Path, err)
	}
	return acp.WriteTextFileResponse{}, nil
}

// regular refuses a path that is not absolute, or that names something other
// than a regular file; a missing path is refused too, unless missingOK.
func regular(path string, missingOK bool) error {
	if !filepath.IsAbs(path) {
		return acp.NewInvalidParams(map[string]any{"path": "not an absolute path: " + path})
	}
	fi, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist) && missingOK:
		return nil
	case err != nil:
		return pathError(path, err)
	case !fi.Mode().IsRegular():
		return fmt.Errorf("clienthost: %s is not a regular file", path)
	}
	return nil
}

// pathError is acp.NewResourceNotFound for a path that does not exist, and
// err otherwise.
func pathError(path string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return acp.NewResourceNotFound(path)
	}
	return fmt.Errorf("clienthost: %w", err)
}

// selectLines returns the lines of content from line, 1-based, at most limit
// of them, each with its end as content has it.
func selectLines(content string, line, limit *uint32) string {
	if line == nil && limit == nil {
		return content
	}
	first := 0
	if line != nil {
		first = int(*line) - 1
	}
	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if first >= len(lines) {
		return ""
	}
	lines = lines[first:]
	if limit != nil && int(*limit) < len(lines) {
		lines = lines[:*limit]
	}
	return strings.Join(lines, "")
}
