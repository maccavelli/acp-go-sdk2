package emit

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// TestDefaultedNullablePropertyCompiles type-checks the output for a property
// that has a default and allows null, a shape schema 1.25.0 does not contain.
func TestDefaultedNullablePropertyCompiles(t *testing.T) {
	t.Parallel()
	def := &load.Definition{
		Type: "object",
		Properties: map[string]*load.Definition{
			"mode": {Type: []any{"string", "null"}, Default: "auto"},
		},
	}
	f := newFile()
	emitObjectSchema(f, "Options", def, map[string]bool{"Options": true})
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
