package emit

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// TestRequiredMapFilledOnEncode requires the MarshalJSON of an object with a
// required map to send a nil map as {}, a shape schema 1.25.0 does not contain.
func TestRequiredMapFilledOnEncode(t *testing.T) {
	t.Parallel()
	schema := &load.Schema{Defs: map[string]*load.Definition{"Options": {
		Type:     "object",
		Required: []string{"env"},
		Properties: map[string]*load.Definition{
			"env": {Type: "object"},
		},
	}}}
	m, err := model.Build(model.Input{Meta: &load.Meta{}, Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	f := newFile()
	emitObjectSchema(f, m.Type("Options"))
	src := render(t, f)
	if want := "\tif a.Env == nil {\n\t\ta.Env = map[string]any{}\n\t}\n"; !strings.Contains(src, want) {
		t.Errorf("MarshalJSON does not fill the nil map; want\n%s\nin\n%s", want, src)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "options.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("acp", fset, []*ast.File{file}, nil); err != nil {
		t.Fatalf("generated code does not compile: %v\n%s", err, src)
	}
}

// TestDefaultedNullablePropertyCompiles type-checks the output for a property
// that has a default and allows null, a shape schema 1.25.0 does not contain.
func TestDefaultedNullablePropertyCompiles(t *testing.T) {
	t.Parallel()
	schema := &load.Schema{Defs: map[string]*load.Definition{"Options": {
		Type: "object",
		Properties: map[string]*load.Definition{
			"mode": {Type: []any{"string", "null"}, Default: "auto"},
		},
	}}}
	m, err := model.Build(model.Input{Meta: &load.Meta{}, Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	f := newFile()
	emitObjectSchema(f, m.Type("Options"))
	src := render(t, f)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "options.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("acp", fset, []*ast.File{file}, nil); err != nil {
		t.Fatalf("generated code does not compile: %v\n%s", err, src)
	}
}
