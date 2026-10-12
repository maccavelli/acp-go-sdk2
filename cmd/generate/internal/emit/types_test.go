package emit

import (
	"encoding/json"
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
	src := mustRender(t, f)
	if want := "\tif a.Env == nil {\n\t\ta.Env = map[string]any{}\n\t}\n"; !strings.Contains(src, want) {
		t.Errorf("MarshalJSON does not fill the nil map; want\n%s\nin\n%s", want, src)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "options.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("acp", fset, withRuntimeStub(t, fset, file), nil); err != nil {
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
	src := mustRender(t, f)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "options.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("acp", fset, withRuntimeStub(t, fset, file), nil); err != nil {
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
	src := mustRender(t, f)
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
	if _, err := conf.Check("acp", fset, withRuntimeStub(t, fset, file), nil); err != nil {
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
	src := mustRender(t, f)
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
	if _, err := conf.Check("acp", fset, withRuntimeStub(t, fset, file), nil); err != nil {
		t.Fatalf("generated code does not compile: %v\n%s", err, src)
	}
}

// TestValidateMethods requires each check of a Validation to render, a union
// to require exactly one variant and validate the set one, and the output to
// compile.
func TestValidateMethods(t *testing.T) {
	t.Parallel()
	var schema load.Schema
	if err := json.Unmarshal([]byte(`{"$defs":{
		"Leaf":{"type":"object","properties":{"t":{"type":"string","minLength":2},"d":{"type":["number","null"],"format":"double","maximum":1}}},
		"U":{"oneOf":[{"$ref":"#/$defs/Leaf"},{"type":"string","title":"str"}]},
		"Holder":{"type":"object","properties":{
			"p":{"anyOf":[{"$ref":"#/$defs/U"},{"type":"null"}]},
			"s":{"type":"array","items":{"$ref":"#/$defs/U"}},
			"m":{"type":"object","additionalProperties":{"$ref":"#/$defs/U"}}}}}}`), &schema); err != nil {
		t.Fatal(err)
	}
	m, err := model.Build(model.Input{Meta: &load.Meta{}, Schema: &schema})
	if err != nil {
		t.Fatal(err)
	}
	f := newFile()
	for _, name := range []string{"Leaf", "U", "Holder"} {
		emitSchemaDef(f, m.Type(name))
	}
	emitDecodeHelpers(f)
	src := mustRender(t, f)
	for _, want := range []string{
		"if utf8.RuneCountInString(string(v.T)) < 2 {",
		"return &ValidationError{",
		`"Leaf: t: shorter than minLength 2",`,
		"RuleMinLength,",
		"if v.D != nil && *v.D > 1 {",
		"if count != 1 {",
		"if u.Leaf != nil {",
		`return fmt.Errorf("U: variant Leaf: %w", err)`,
		"for i := range v.S {",
		"for _, k := range slices.Sorted(maps.Keys(v.M)) {",
		`return fmt.Errorf("Holder: p: %w", err)`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("output lacks %q:\n%s", want, src)
		}
	}
	if strings.Contains(src, "u.Str.Validate()") {
		t.Error("a variant without Validate is validated")
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "v.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("acp", fset, withRuntimeStub(t, fset, file), nil); err != nil {
		t.Fatalf("generated code does not compile: %v\n%s", err, src)
	}
}

// runtimeStub declares the names of errors.go that generated Validate
// methods refer to, for output type-checked without the runtime files
// (0015-PLAN-validate-absolute-protocol-paths.md, step 3).
const runtimeStub = `package acp

type ValidationRule string

const (
	RuleAbsolutePath  ValidationRule = "absolutePath"
	RuleMinLength     ValidationRule = "minLength"
	RuleMinimum       ValidationRule = "minimum"
	RuleMaximum       ValidationRule = "maximum"
	RuleOneVariant    ValidationRule = "oneVariant"
	RuleOneMember     ValidationRule = "oneMember"
	RuleExcludedValue ValidationRule = "excludedValue"
)

type ValidationError struct {
	Type, Field string
	Value       any
	Rule        ValidationRule
	msg         string
}

func (e *ValidationError) Error() string { return e.msg }
`

// withRuntimeStub returns file with the parsed runtimeStub beside it.
func withRuntimeStub(t *testing.T, fset *token.FileSet, file *ast.File) []*ast.File {
	t.Helper()
	stub, err := parser.ParseFile(fset, "runtime_stub.go", runtimeStub, 0)
	if err != nil {
		t.Fatal(err)
	}
	return []*ast.File{file, stub}
}
