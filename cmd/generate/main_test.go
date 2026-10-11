package main

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// TestGeneratedFilesUpToDate renders the bindings from schema/ in memory and
// requires the generated files, in order, to match the committed *_gen.go
// files byte for byte.
func TestGeneratedFilesUpToDate(t *testing.T) {
	meta, schema, prov, err := loadSchema(filepath.Join(repoRoot, "schema"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := model.Build(model.Input{Meta: meta, Schema: schema, Provenance: prov, Protocol: &model.ACP})
	if err != nil {
		t.Fatal(err)
	}
	files, err := emit.Render(m)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.Name)
	}
	if !slices.Equal(names, generatedFiles) {
		t.Fatalf("Render returned %v, want %v", names, generatedFiles)
	}
	for _, f := range files {
		want, err := os.ReadFile(filepath.Join(repoRoot, f.Name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(f.Src, want) {
			t.Errorf("%s is out of date with schema/; run make version (first difference at line %d)", f.Name, firstDiffLine(f.Src, want))
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

// writeSchema writes a schema directory with an empty meta and defs as the
// definitions, and returns its path.
func writeSchema(t *testing.T, defs string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), []byte(`{"version":1,"agentMethods":{},"clientMethods":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "schema.json"), []byte(`{"$defs":`+defs+`}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestRunRefusesBeforeWriting runs the generator on the hostile schemas of
// 0011-PLAN-resolved-schema-model.md, G12. Each must fail with an error that
// names the problem, and leave the output directory as it was.
func TestRunRefusesBeforeWriting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, defs string
		want       []string
	}{
		{"enum constants collide", `{"Kind":{"type":"string","enum":["a-b","a_b"]}}`, []string{"Kind", "KindAB"}},
		{"struct fields collide", `{"F":{"type":"object","properties":{"foo_bar":{"type":"string"},"fooBar":{"type":"string"}}}}`,
			[]string{"F:", "FooBar"}},
		{"dangling reference", `{"R":{"type":"object","properties":{"m":{"$ref":"#/$defs/Missing"}},"required":["m"]}}`,
			[]string{"R.properties.m", "Missing"}},
		{"field name is not an identifier", `{"Thing":{"type":"object","properties":{"2fa":{"type":"string"}},"required":["2fa"]}}`,
			[]string{"Thing", "2fa"}},
		{"keyword as a constructor parameter", `{"Union":{"oneOf":[
			{"type":"object","properties":{"type":{"const":"x","type":"string"},"range":{"type":"string"}},"required":["type","range"]},
			{"type":"object","properties":{"type":{"const":"y","type":"string"}},"required":["type"]}]}}`, []string{"Union", "range"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := t.TempDir()
			sentinel := filepath.Join(out, "keep.txt")
			if err := os.WriteFile(sentinel, []byte("sentinel\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			err := run([]string{"-schema", writeSchema(t, tt.defs), "-out", out})
			if err == nil {
				t.Fatal("run accepted the schema")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
			entries, err := os.ReadDir(out)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			if len(names) != 1 || names[0] != "keep.txt" {
				t.Errorf("output directory holds %v, want only keep.txt", names)
			}
			if b, err := os.ReadFile(sentinel); err != nil || string(b) != "sentinel\n" {
				t.Errorf("sentinel changed: %q, %v", b, err)
			}
		})
	}
}

// handWritten declares the names of the hand-written package that the files
// generated for a schema without methods refer to.
const handWritten = `package acp

type RequestError struct{}

func NewMethodNotFound(string) *RequestError { return nil }

type AgentSideConnection struct{}

type ClientSideConnection struct{}
`

// TestRunGeneratesRecasedUnion runs the generator on the oneOf that panicked
// before the model named variants once, and type-checks what it writes.
func TestRunGeneratesRecasedUnion(t *testing.T) {
	t.Parallel()
	out := t.TempDir()
	err := run([]string{"-schema", writeSchema(t, `{"U":{"oneOf":[
		{"type":"object","properties":{"type":{"const":"x","type":"string"},"n":{"type":"string"}},"required":["type","n"]},
		{"type":"object","properties":{"type":{"const":"y","type":"string"}},"required":["type"]}]}}`), "-out", out})
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range generatedFiles {
		f, err := parser.ParseFile(fset, filepath.Join(out, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	stub, err := parser.ParseFile(fset, "hand_written.go", handWritten, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("acp", fset, append(files, stub), nil)
	if err != nil {
		t.Fatalf("generated code does not type-check: %v", err)
	}
	for _, name := range []string{"U", "UX", "UY", "NewUX", "NewUY"} {
		if pkg.Scope().Lookup(name) == nil {
			t.Errorf("generated package has no %s", name)
		}
	}
}
