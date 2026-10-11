package main

import (
	"bytes"
	"cmp"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
)

// site is one place an operator changes: a mutant.
type site struct {
	File      string // relative to the module, with forward slashes
	Line, Col int
	Op        string // the operator's name
	Orig      string // the text it replaces
	Repl      string // the text it writes
	index     int    // the site's place in the walk of its file
}

// The operators (0011-PLAN-engine-tests.md, D5).
const (
	opEqNeq       = "eq-neq"
	opBoundary    = "boundary"
	opAndOr       = "and-or"
	opTrueFalse   = "true-false"
	opReturnError = "return-error"
	opBranch      = "continue-break"
	opPlusMinus   = "plus-minus-one"
)

// The identifiers and the package the operators write.
const (
	identTrue  = "true"
	identFalse = "false"
	pkgErrors  = "errors"
)

// swaps are the binary operators each replaces, and the operator's name.
var swaps = map[token.Token]struct {
	to token.Token
	op string
}{
	token.EQL: {token.NEQ, opEqNeq}, token.NEQ: {token.EQL, opEqNeq},
	token.LSS: {token.LEQ, opBoundary}, token.LEQ: {token.LSS, opBoundary},
	token.GTR: {token.GEQ, opBoundary}, token.GEQ: {token.GTR, opBoundary},
	token.LAND: {token.LOR, opAndOr}, token.LOR: {token.LAND, opAndOr},
}

// candidate is a site found in a walk, with the change that makes its mutant.
type candidate struct {
	pos   token.Pos
	op    string
	orig  string
	repl  string
	apply func(f *ast.File)
}

// sites lists the mutants of the Go source src, named file, ordered by
// position.
func sites(file string, src []byte) ([]site, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var out []site
	for i, c := range candidates(f) {
		p := fset.Position(c.pos)
		out = append(out, site{File: file, Line: p.Line, Col: p.Column, Op: c.op, Orig: c.orig, Repl: c.repl, index: i})
	}
	slices.SortStableFunc(out, func(a, b site) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
	})
	return out, nil
}

// mutant returns src with s applied, formatted.
func mutant(file string, src []byte, s site) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	cs := candidates(f)
	if s.index >= len(cs) {
		return nil, fmt.Errorf("%s: site %d of %d", file, s.index, len(cs))
	}
	cs[s.index].apply(f)
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, err
	}
	return format.Source(buf.Bytes())
}

// candidates walks f in a fixed order and returns every site, each with the
// change that makes its mutant.
func candidates(f *ast.File) []candidate {
	var out []candidate
	var stack []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		switch n := n.(type) {
		case *ast.BinaryExpr:
			out = append(out, binarySites(n)...)
		case *ast.Ident:
			if n.Name == identTrue || n.Name == identFalse {
				to := map[string]string{identTrue: identFalse, identFalse: identTrue}[n.Name]
				out = append(out, candidate{n.Pos(), opTrueFalse, n.Name, to, func(*ast.File) { n.Name = to }})
			}
		case *ast.ReturnStmt:
			if c, ok := returnSite(n, stack); ok {
				out = append(out, c)
			}
		case *ast.BranchStmt:
			if n.Label == nil && (n.Tok == token.CONTINUE || n.Tok == token.BREAK) {
				to := map[token.Token]token.Token{token.CONTINUE: token.BREAK, token.BREAK: token.CONTINUE}[n.Tok]
				out = append(out, candidate{n.Pos(), opBranch, n.Tok.String(), to.String(), func(*ast.File) { n.Tok = to }})
			}
		}
		return true
	})
	return out
}

// binarySites returns the sites of a binary expression: its operator swapped
// or moved by one, or + 1 and - 1 swapped.
func binarySites(n *ast.BinaryExpr) []candidate {
	if s, ok := swaps[n.Op]; ok {
		return []candidate{{n.OpPos, s.op, n.Op.String(), s.to.String(), func(*ast.File) { n.Op = s.to }}}
	}
	if lit, ok := n.Y.(*ast.BasicLit); ok && lit.Kind == token.INT && lit.Value == "1" && (n.Op == token.ADD || n.Op == token.SUB) {
		to := map[token.Token]token.Token{token.ADD: token.SUB, token.SUB: token.ADD}[n.Op]
		return []candidate{{n.OpPos, opPlusMinus, n.Op.String() + " 1", to.String() + " 1", func(*ast.File) { n.Op = to }}}
	}
	return nil
}

// returnSite returns the site of return nil in a function whose last result is
// error: nil becomes errors.New("mutant").
func returnSite(n *ast.ReturnStmt, stack []ast.Node) (candidate, bool) {
	if len(n.Results) == 0 {
		return candidate{}, false
	}
	last := len(n.Results) - 1
	if id, ok := n.Results[last].(*ast.Ident); !ok || id.Name != "nil" || !returnsError(stack) {
		return candidate{}, false
	}
	return candidate{n.Results[last].Pos(), opReturnError, "nil", `errors.New("mutant")`, func(f *ast.File) {
		n.Results[last] = &ast.CallExpr{
			Fun:  &ast.SelectorExpr{X: ast.NewIdent(importErrors(f)), Sel: ast.NewIdent("New")},
			Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: strconv.Quote("mutant")}},
		}
	}}, true
}

// returnsError reports whether the innermost function on stack has error as
// its last result.
func returnsError(stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		var ft *ast.FuncType
		switch fn := stack[i].(type) {
		case *ast.FuncDecl:
			ft = fn.Type
		case *ast.FuncLit:
			ft = fn.Type
		default:
			continue
		}
		if ft.Results == nil || len(ft.Results.List) == 0 {
			return false
		}
		id, ok := ft.Results.List[len(ft.Results.List)-1].Type.(*ast.Ident)
		return ok && id.Name == "error"
	}
	return false
}

// importErrors returns the name f imports the errors package by, adding the
// import when f has none.
func importErrors(f *ast.File) string {
	for _, im := range f.Imports {
		if im.Path.Value == `"errors"` {
			if im.Name != nil {
				return im.Name.Name
			}
			return pkgErrors
		}
	}
	spec := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: `"errors"`}}
	f.Imports = append(f.Imports, spec)
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			g.Specs = append([]ast.Spec{spec}, g.Specs...)
			if !g.Lparen.IsValid() {
				// A single import gains parentheses for the second.
				g.Lparen, g.Rparen = g.TokPos+1, g.End()
			}
			return pkgErrors
		}
	}
	f.Decls = append([]ast.Decl{&ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{spec}}}, f.Decls...)
	return pkgErrors
}
