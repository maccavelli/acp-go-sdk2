package acp

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestExportedDeclarationsDocumented requires each exported type, function,
// method, constant and variable of the package's non-test files, generated
// ones included, to have a doc comment; a constant or variable is documented
// by its own doc or its group's
// (0019-PLAN-examples-docs-tests-tooling.md, I12).
func TestExportedDeclarationsDocumented(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var missing []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		missing = append(missing, undocumented(fset, f)...)
	}
	slices.Sort(missing)
	for _, m := range missing {
		t.Errorf("no doc comment: %s", m)
	}
	if len(missing) > 0 {
		t.Errorf("%d exported declarations have no doc comment", len(missing))
	}
}

// parsePackage parses the package's non-test files, with their comments.
func parsePackage(t *testing.T) (*token.FileSet, []*ast.File) {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	return fset, files
}

// TestExportedTypeDocsStartWithName requires each exported type's doc
// comment, generated or hand-written, to start with the type's name and a
// space (0024-PLAN-documentation.md, A7).
func TestExportedTypeDocsStartWithName(t *testing.T) {
	t.Parallel()
	fset, files := parsePackage(t)
	n := 0
	for _, f := range files {
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.TYPE {
				continue
			}
			for _, s := range g.Specs {
				s := s.(*ast.TypeSpec)
				if !s.Name.IsExported() {
					continue
				}
				n++
				doc := s.Doc
				if doc == nil && !g.Lparen.IsValid() {
					doc = g.Doc
				}
				if text := doc.Text(); !strings.HasPrefix(text, s.Name.Name+" ") {
					first, _, _ := strings.Cut(text, "\n")
					t.Errorf("%s: %s's doc starts %q", fset.Position(s.Pos()), s.Name.Name, first)
				}
			}
		}
	}
	if n < 378 {
		t.Errorf("%d exported types, want at least the 378 of types_gen.go", n)
	}
}

// TestTypedMethodDocsNameErrors requires the doc of each exported method of
// the side connections that takes a context.Context to name *RequestError.
// A typed method's must also name ErrNotInitialized, unless the method is
// Initialize; an extension method's, whose params are any, must not, as it
// sends before initialize too (0024-PLAN-documentation.md, A6, and its
// step 2 deviation).
func TestTypedMethodDocsNameErrors(t *testing.T) {
	t.Parallel()
	fset, files := parsePackage(t)
	typed, untyped := 0, 0
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !fn.Name.IsExported() {
				continue
			}
			if recv := receiverName(fn.Recv.List[0].Type); recv != "AgentSideConnection" && recv != "ClientSideConnection" {
				continue
			}
			if p := fn.Type.Params.List; len(p) == 0 || types.ExprString(p[0].Type) != "context.Context" {
				continue
			}
			text := fn.Doc.Text()
			if !strings.Contains(text, "*RequestError") {
				t.Errorf("%s: %s's doc does not name *RequestError", fset.Position(fn.Pos()), fn.Name.Name)
			}
			names := strings.Contains(text, "ErrNotInitialized")
			switch p := fn.Type.Params.List; {
			case types.ExprString(p[len(p)-1].Type) == "any":
				untyped++
				if names {
					t.Errorf("%s: extension method %s's doc names ErrNotInitialized", fset.Position(fn.Pos()), fn.Name.Name)
				}
			default:
				typed++
				if !names && fn.Name.Name != "Initialize" {
					t.Errorf("%s: %s's doc does not name ErrNotInitialized", fset.Position(fn.Pos()), fn.Name.Name)
				}
			}
		}
	}
	if typed != 40 || untyped != 4 {
		t.Errorf("%d typed and %d extension methods, want the 40 of agent_gen.go and client_gen.go and the 4 of extensions.go", typed, untyped)
	}
}

// TestReadmeQuickstartIsTheExample requires the README's quickstart, the Go
// block between its quickstart markers, to be the body of
// ExampleNewClientSideConnection as go/printer prints it, with its comments
// and with each tab as four spaces, as the README's other blocks are
// indented. So the snippet is compiled and run as the example, and cannot
// drift from it (0024-PLAN-documentation.md, A8).
func TestReadmeQuickstartIsTheExample(t *testing.T) {
	t.Parallel()
	want := exampleBody(t, "example_quickstart_test.go", "ExampleNewClientSideConnection")
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	const open, end = "<!-- quickstart: ExampleNewClientSideConnection -->\n", "<!-- /quickstart -->"
	_, rest, ok := strings.Cut(string(readme), open)
	block, _, closed := strings.Cut(rest, end)
	if !ok || !closed {
		t.Fatalf("README.md has no block between %q and %q", open, end)
	}
	got, fenced := strings.CutPrefix(strings.TrimSpace(block)+"\n", "```go\n")
	got, ended := strings.CutSuffix(got, "```\n")
	if !fenced || !ended {
		t.Fatalf("the quickstart is not one Go block:\n%s", block)
	}
	if got != want {
		t.Errorf("README.md's quickstart is not ExampleNewClientSideConnection's body.\nREADME:\n%s\nexample:\n%s", got, want)
	}
}

// exampleBody prints the body of the function name of file, without its
// braces, one indent removed and each remaining tab as four spaces.
func exampleBody(t *testing.T, file, name string) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name {
			continue
		}
		var b strings.Builder
		if err := printer.Fprint(&b, fset, &printer.CommentedNode{Node: fn.Body, Comments: f.Comments}); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(b.String(), "\n")
		lines = lines[1 : len(lines)-1] // the braces
		for i, l := range lines {
			lines[i] = strings.ReplaceAll(strings.TrimPrefix(l, "\t"), "\t", "    ")
		}
		return strings.Join(lines, "\n") + "\n"
	}
	t.Fatalf("%s has no function %s", file, name)
	return ""
}

// undocumented returns each exported declaration of f without a doc comment,
// as position and name.
func undocumented(fset *token.FileSet, f *ast.File) []string {
	var out []string
	report := func(pos token.Pos, name string) {
		out = append(out, fset.Position(pos).String()+" "+name)
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if !d.Name.IsExported() || d.Doc != nil {
				continue
			}
			name := d.Name.Name
			if d.Recv != nil {
				recv := receiverName(d.Recv.List[0].Type)
				if !ast.IsExported(recv) {
					continue
				}
				name = recv + "." + name
			}
			report(d.Pos(), name)
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() && s.Doc == nil && (d.Doc == nil || d.Lparen.IsValid()) {
						report(s.Pos(), s.Name.Name)
					}
				case *ast.ValueSpec:
					if s.Doc != nil || d.Doc != nil {
						continue
					}
					for _, n := range s.Names {
						if n.IsExported() {
							report(n.Pos(), n.Name)
						}
					}
				}
			}
		}
	}
	return out
}

// receiverName returns the type name of a method's receiver.
func receiverName(e ast.Expr) string {
	for {
		switch x := e.(type) {
		case *ast.StarExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.Ident:
			return x.Name
		default:
			return ""
		}
	}
}
