package acp

import (
	"go/ast"
	"go/parser"
	"go/token"
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
