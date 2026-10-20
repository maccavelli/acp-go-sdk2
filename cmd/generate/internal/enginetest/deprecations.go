package enginetest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Deprecations lists the declarations of the non-test Go files in dir that
// keep accepts whose doc comment holds a paragraph starting "Deprecated: ",
// one line each, sorted: "<kind> <name>: <the paragraph, its lines joined by
// spaces>". The kinds are type, const, var, func, field (T.F) and method:
// T.M for an interface's, (*T).M or (T).M for a concrete type's. A notice
// in any other form is not listed, as staticcheck does not honour it
// (0024-PLAN-api-gate-and-deprecation.md).
func Deprecations(dir string, keep func(file string) bool) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	fset := token.NewFileSet()
	var lines []string
	add := func(kind, name string, doc *ast.CommentGroup) {
		if n := notice(doc); n != "" {
			lines = append(lines, kind+" "+name+": "+n)
		}
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || !keep(name) {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return "", err
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					add("func", d.Name.Name, d.Doc)
				} else {
					add("method", "("+types.ExprString(d.Recv.List[0].Type)+")."+d.Name.Name, d.Doc)
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					declDeprecations(d, s, add)
				}
			}
		}
	}
	slices.Sort(lines)
	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n") + "\n", nil
}

// declDeprecations adds the notices of one spec of d: a type with its fields
// or interface methods, or a constant or variable. A lone spec's doc comment
// is the declaration's.
func declDeprecations(d *ast.GenDecl, s ast.Spec, add func(kind, name string, doc *ast.CommentGroup)) {
	docOf := func(doc *ast.CommentGroup) *ast.CommentGroup {
		if doc == nil && len(d.Specs) == 1 {
			return d.Doc
		}
		return doc
	}
	switch s := s.(type) {
	case *ast.TypeSpec:
		add("type", s.Name.Name, docOf(s.Doc))
		switch t := s.Type.(type) {
		case *ast.StructType:
			for _, fl := range t.Fields.List {
				for _, n := range fl.Names {
					add("field", s.Name.Name+"."+n.Name, fl.Doc)
				}
			}
		case *ast.InterfaceType:
			for _, m := range t.Methods.List {
				for _, n := range m.Names {
					add("method", s.Name.Name+"."+n.Name, m.Doc)
				}
			}
		}
	case *ast.ValueSpec:
		kind := "var"
		if d.Tok == token.CONST {
			kind = "const"
		}
		for _, n := range s.Names {
			add(kind, n.Name, docOf(s.Doc))
		}
	}
}

// notice returns the paragraph of doc that starts "Deprecated: ", its lines
// joined by spaces, or "".
func notice(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	for p := range strings.SplitSeq(doc.Text(), "\n\n") {
		if strings.HasPrefix(p, "Deprecated: ") {
			return strings.Join(strings.Fields(p), " ")
		}
	}
	return ""
}
