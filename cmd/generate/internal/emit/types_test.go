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
	emitDecodeHelpers(f)
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
	emitDecodeHelpers(f)
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

// TestRequiredKeysDecoder requires a struct with required keys to decode
// through objectKeys with those keys in sorted order, and the output, with its
// helper, to compile.
func TestRequiredKeysDecoder(t *testing.T) {
	t.Parallel()
	schema := &load.Schema{Defs: map[string]*load.Definition{"Options": {
		Type:     "object",
		Required: []string{"zeta", "alpha"},
		Properties: map[string]*load.Definition{
			"zeta":  {Type: "string"},
			"alpha": {Type: "integer", Format: "uint32"},
			"opt":   {Type: "boolean"},
		},
	}}}
	m, err := model.Build(model.Input{Meta: &load.Meta{}, Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	f := newFile()
	emitObjectSchema(f, m.Type("Options"))
	// An inline variant of a discriminated union does not require the
	// discriminator: the union placed the payload.
	u, err := model.Build(model.Input{Meta: &load.Meta{}, Schema: &load.Schema{Defs: map[string]*load.Definition{"U": {OneOf: []*load.Definition{{
		Type: "object", Required: []string{"type", "text"},
		Properties: map[string]*load.Definition{"type": {Type: "string", Const: "text"}, "text": {Type: "string"}},
	}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	emitUnion(f, u.Type("U"))
	variant := u.Type("U").Variants[0].TypeName
	emitDecodeHelpers(f)
	src := render(t, f)
	for _, want := range []string{
		"func (v *Options) UnmarshalJSON(b []byte) error {",
		`_, err := objectKeys(b, "` + variant + `", "text")`,
		`_, err := objectKeys(b, "Options", "alpha", "zeta")`,
		"func objectKeys(b []byte, typ string, required ...string) (map[string]json.RawMessage, error) {",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("output lacks %q:\n%s", want, src)
		}
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

// TestLenientDecoder requires each lenient shape to decode from its own raw
// value: default-on-error through orDefault with the schema default,
// skip-invalid-items with default-on-error through skipInvalid with a
// fallback, and skip-invalid-items alone failing on a value that is not an
// array; and the output, with its helpers, to compile.
func TestLenientDecoder(t *testing.T) {
	t.Parallel()
	schema := &load.Schema{Defs: map[string]*load.Definition{"Options": {
		Type: "object",
		Properties: map[string]*load.Definition{
			"mode":  {Type: "string", Default: "auto", XDeserializeDefaultOnError: true},
			"tags":  {Type: "array", Items: &load.Definition{Type: "string"}, Default: []any{}, XDeserializeDefaultOnError: true, XDeserializeSkipInvalidItems: true},
			"ids":   {Type: "array", Items: &load.Definition{Type: "integer", Format: "uint32"}, XDeserializeSkipInvalidItems: true},
			"plain": {Type: "string"},
		},
	}}}
	m, err := model.Build(model.Input{Meta: &load.Meta{}, Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	f := newFile()
	emitObjectSchema(f, m.Type("Options"))
	emitDecodeHelpers(f)
	src := render(t, f)
	for _, want := range []string{
		"\t\tMode json.RawMessage `json:\"mode\"`",
		`a.Mode = orDefault[string](s.Mode, "\"auto\"")`,
		`if items, err := skipInvalid[string](s.Tags); err == nil {`,
		`a.Tags = orDefault[[]string](nil, "[]")`,
		`items, err := skipInvalid[uint32](s.Ids)`,
		`return fmt.Errorf("Options: %q: %w", "ids", err)`,
		"func orDefault[T any](raw json.RawMessage, def string) T {",
		"func skipInvalid[T any](raw json.RawMessage) ([]T, error) {",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("output lacks %q:\n%s", want, src)
		}
	}
	if strings.Contains(src, "Plain json.RawMessage") {
		t.Error("a field without a lenient keyword is shadowed")
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
