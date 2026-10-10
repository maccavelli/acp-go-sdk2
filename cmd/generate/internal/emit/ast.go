package emit

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// Small constructors for go/ast nodes. They keep the emitters readable and
// return fresh nodes on every call, so no node is shared between two places
// in a tree.

// placeholderPos marks a position whose only job is to be valid, such as the
// '=' of a type alias or the '(' of a grouped declaration. The file layout
// replaces it with a real position.
const placeholderPos token.Pos = 1

func id(name string) *ast.Ident { return ast.NewIdent(name) }

// sel builds x.name1.name2...
func sel(x ast.Expr, names ...string) ast.Expr {
	for _, n := range names {
		x = &ast.SelectorExpr{X: x, Sel: id(n)}
	}
	return x
}

// dot builds name.names[0].names[1]...
func dot(name string, names ...string) ast.Expr { return sel(id(name), names...) }

func call(fun ast.Expr, args ...ast.Expr) *ast.CallExpr { return &ast.CallExpr{Fun: fun, Args: args} }

func str(s string) *ast.BasicLit { return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(s)} }

func intLit(n int) *ast.BasicLit { return &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(n)} }

func boolLit(b bool) *ast.Ident { return id(strconv.FormatBool(b)) }

func nilExpr() *ast.Ident { return id("nil") }

func star(x ast.Expr) *ast.StarExpr { return &ast.StarExpr{X: x} }

func addr(x ast.Expr) *ast.UnaryExpr { return &ast.UnaryExpr{Op: token.AND, X: x} }

func not(x ast.Expr) *ast.UnaryExpr { return &ast.UnaryExpr{Op: token.NOT, X: x} }

func binary(x ast.Expr, op token.Token, y ast.Expr) *ast.BinaryExpr {
	return &ast.BinaryExpr{X: x, Op: op, Y: y}
}

func paren(x ast.Expr) *ast.ParenExpr { return &ast.ParenExpr{X: x} }

func index(x, i ast.Expr) *ast.IndexExpr { return &ast.IndexExpr{X: x, Index: i} }

func typeAssert(x, typ ast.Expr) *ast.TypeAssertExpr { return &ast.TypeAssertExpr{X: x, Type: typ} }

func sliceOf(elt ast.Expr) *ast.ArrayType { return &ast.ArrayType{Elt: elt} }

func mapOf(key, value ast.Expr) *ast.MapType { return &ast.MapType{Key: key, Value: value} }

// byteSlice is []byte.
func byteSlice() *ast.ArrayType { return sliceOf(id("byte")) }

// bytesOf is []byte("s").
func bytesOf(s string) *ast.CallExpr { return call(byteSlice(), str(s)) }

// emptyBytes is []byte{}.
func emptyBytes() *ast.CompositeLit { return composite(byteSlice()) }

// rawMap is map[string]<value>.
func rawMap(value ast.Expr) *ast.MapType { return mapOf(id("string"), value) }

func composite(typ ast.Expr, elts ...ast.Expr) *ast.CompositeLit {
	return &ast.CompositeLit{Type: typ, Elts: elts}
}

func kv(key, value ast.Expr) *ast.KeyValueExpr { return &ast.KeyValueExpr{Key: key, Value: value} }

func exprs(xs ...ast.Expr) []ast.Expr { return xs }

func define(lhs []ast.Expr, rhs ...ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: lhs, Tok: token.DEFINE, Rhs: rhs}
}

func assign(lhs []ast.Expr, rhs ...ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{Lhs: lhs, Tok: token.ASSIGN, Rhs: rhs}
}

func exprStmt(x ast.Expr) *ast.ExprStmt { return &ast.ExprStmt{X: x} }

func ret(results ...ast.Expr) *ast.ReturnStmt { return &ast.ReturnStmt{Results: results} }

func block(stmts ...ast.Stmt) *ast.BlockStmt { return &ast.BlockStmt{List: stmts} }

func ifStmt(init ast.Stmt, cond ast.Expr, body ...ast.Stmt) *ast.IfStmt {
	return &ast.IfStmt{Init: init, Cond: cond, Body: block(body...)}
}

func ifElse(init ast.Stmt, cond ast.Expr, then, els []ast.Stmt) *ast.IfStmt {
	s := ifStmt(init, cond, then...)
	s.Else = block(els...)
	return s
}

func incr(x ast.Expr) *ast.IncDecStmt { return &ast.IncDecStmt{X: x, Tok: token.INC} }

func rangeStmt(key, value, x ast.Expr, body ...ast.Stmt) *ast.RangeStmt {
	return &ast.RangeStmt{Key: key, Value: value, Tok: token.DEFINE, X: x, Body: block(body...)}
}

func switchStmt(tag ast.Expr, clauses ...ast.Stmt) *ast.SwitchStmt {
	return &ast.SwitchStmt{Tag: tag, Body: block(clauses...)}
}

// caseOf is a case clause. A nil list is the default clause.
func caseOf(list []ast.Expr, body ...ast.Stmt) *ast.CaseClause {
	return &ast.CaseClause{List: list, Body: body}
}

// varStmt is var name typ [= value].
func varStmt(name string, typ ast.Expr, value ast.Expr) *ast.DeclStmt {
	spec := &ast.ValueSpec{Names: []*ast.Ident{id(name)}, Type: typ}
	if value != nil {
		spec.Values = []ast.Expr{value}
	}
	return &ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{spec}}}
}

// typeStmt is a local type declaration, type name typ.
func typeStmt(name string, typ ast.Expr) *ast.DeclStmt {
	return &ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&ast.TypeSpec{Name: id(name), Type: typ}}}}
}

// param is one parameter or result. An empty name leaves it unnamed.
func param(name string, typ ast.Expr) *ast.Field {
	f := &ast.Field{Type: typ}
	if name != "" {
		f.Names = []*ast.Ident{id(name)}
	}
	return f
}

func fields(list ...*ast.Field) *ast.FieldList { return &ast.FieldList{List: list} }

// results is nil for no results, so the printer omits the list.
func results(list ...*ast.Field) *ast.FieldList {
	if len(list) == 0 {
		return nil
	}
	return fields(list...)
}

func funcType(params []*ast.Field, res ...*ast.Field) *ast.FuncType {
	return &ast.FuncType{Params: fields(params...), Results: results(res...)}
}

// method is func (recv) name(params) results { body }.
func method(recv *ast.Field, name string, typ *ast.FuncType, body ...ast.Stmt) *ast.FuncDecl {
	return &ast.FuncDecl{Recv: fields(recv), Name: id(name), Type: typ, Body: block(body...)}
}

// function is func name(params) results { body }.
func function(name string, typ *ast.FuncType, body ...ast.Stmt) *ast.FuncDecl {
	return &ast.FuncDecl{Name: id(name), Type: typ, Body: block(body...)}
}

// typeDecl is type name typ.
func typeDecl(name string, typ ast.Expr) *ast.GenDecl {
	return &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&ast.TypeSpec{Name: id(name), Type: typ}}}
}

// aliasDecl is type name = typ.
func aliasDecl(name string, typ ast.Expr) *ast.GenDecl {
	return &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&ast.TypeSpec{Name: id(name), Assign: placeholderPos, Type: typ}}}
}

// constSpec is name [typ] = value.
func constSpec(name string, typ ast.Expr, value ast.Expr) *ast.ValueSpec {
	return &ast.ValueSpec{Names: []*ast.Ident{id(name)}, Type: typ, Values: []ast.Expr{value}}
}

// constDecl is a const declaration. A grouped declaration prints in
// parentheses even when it holds one spec.
func constDecl(grouped bool, specs ...*ast.ValueSpec) *ast.GenDecl {
	d := &ast.GenDecl{Tok: token.CONST}
	for _, s := range specs {
		d.Specs = append(d.Specs, s)
	}
	if grouped {
		d.Lparen = placeholderPos
	}
	return d
}

// structTag is a json struct tag literal, `json:"name"`.
func structTag(name string) *ast.BasicLit {
	s := jsonStructTag + ":" + strconv.Quote(name)
	if strconv.CanBackquote(s) {
		s = "`" + s + "`"
	} else {
		s = strconv.Quote(s)
	}
	return &ast.BasicLit{Kind: token.STRING, Value: s}
}

// docLines turns comment text into // lines. Text that already starts with
// a comment marker is kept as written.
func docLines(lines ...string) *ast.CommentGroup {
	if len(lines) == 0 {
		return nil
	}
	g := &ast.CommentGroup{}
	for _, line := range lines {
		g.List = append(g.List, &ast.Comment{Text: commentText(line)})
	}
	return g
}

func commentText(line string) string {
	switch {
	case strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*"):
		return line
	case line == "":
		return "//"
	default:
		return "// " + line
	}
}
