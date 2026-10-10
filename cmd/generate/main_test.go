package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/emit"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// repoRoot is the SDK module root, two directories above cmd/generate.
const repoRoot = "../.."

var generatedFiles = []string{
	"constants_gen.go",
	"types_gen.go",
	"agent_gen.go",
	"client_gen.go",
	"helpers_gen.go",
}

// TestGeneratedFilesUpToDate regenerates the bindings from schema/ and
// requires the committed *_gen.go files to match byte for byte.
func TestGeneratedFilesUpToDate(t *testing.T) {
	meta, schema, err := loadSchema(filepath.Join(repoRoot, "schema"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := model.Build(meta, schema)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := emit.WriteConstants(out, m); err != nil {
		t.Fatal(err)
	}
	if err := emit.WriteTypes(out, m); err != nil {
		t.Fatal(err)
	}
	if err := emit.WriteDispatch(out, m); err != nil {
		t.Fatal(err)
	}
	if err := emit.WriteHelpers(out, m); err != nil {
		t.Fatal(err)
	}
	for _, name := range generatedFiles {
		got, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join(repoRoot, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is out of date with schema/; run make version (first difference at line %d)", name, firstDiffLine(got, want))
		}
	}
}

// firstDiffLine returns the 1-based line of the first byte where a and b differ.
func firstDiffLine(a, b []byte) int {
	n := min(len(a), len(b))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return bytes.Count(a[:i], []byte("\n")) + 1
}
