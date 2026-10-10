package acp

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// These tests cover what the generator emits beyond the populated goldens.

func TestUnsetUnionFailsByName(t *testing.T) {
	t.Parallel()
	_, err := json.Marshal(RequestPermissionResponse{})
	if err == nil {
		t.Fatal("encoded a RequestPermissionResponse with no outcome set")
	}
	for _, want := range []string{"RequestPermissionOutcome", "no variant set"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestNilRequiredArrayEncodesEmpty(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(NewSessionRequest{Cwd: "/home/<user>/project"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"mcpServers":[]`) {
		t.Errorf("got %s, want it to contain \"mcpServers\":[]", b)
	}
	var got NewSessionRequest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("decoded %s fails Validate: %v", b, err)
	}
}

// TestUnionDocAboveWrapper requires a union's description on the wrapper
// type, not on its first variant.
func TestUnionDocAboveWrapper(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "types_gen.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	docs := map[string]string{}
	for _, decl := range file.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.TYPE && len(gd.Specs) == 1 {
			docs[gd.Specs[0].(*ast.TypeSpec).Name.Name] = gd.Doc.Text()
		}
	}
	const union = "Content blocks represent displayable information"
	if !strings.HasPrefix(docs["ContentBlock"], union) {
		t.Errorf("ContentBlock doc = %q, want it to start with %q", docs["ContentBlock"], union)
	}
	if strings.Contains(docs["ContentBlockText"], union) {
		t.Errorf("ContentBlockText doc carries the union's description: %q", docs["ContentBlockText"])
	}
}
