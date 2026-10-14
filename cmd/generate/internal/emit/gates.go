package emit

import (
	"go/ast"
	"go/token"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// emitGater emits gater, the interface of a value with gates.
func emitGater(f *goFile) {
	f.line()
	f.comment("gater is a value that holds a property or variant the capabilities gate.")
	f.add(typeDecl("gater", &ast.InterfaceType{Methods: fields(&ast.Field{
		Names: []*ast.Ident{id("gates")},
		Type:  funcType([]*ast.Field{param("n", star(id("Negotiation"))), param("method", id("string"))}, param("", id("error"))),
	})}))
}

// gateFuncLit is func(method string, result any) error { body }, a gate
// built where the dispatch has what it needs.
func gateFuncLit(body ...ast.Stmt) *ast.FuncLit {
	return &ast.FuncLit{
		Type: funcType([]*ast.Field{param("method", id("string")), param("result", id("any"))}, param("", id("error"))),
		Body: block(body...),
	}
}

// emitValueGates emits gates on each type of m.Gated: the property and
// variant gates of the protocol table checked against the record n, and a
// call into each value held that has gates
// (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
func emitValueGates(f *goFile, m *model.Model) {
	for _, g := range m.Gated {
		recv := "v"
		if g.Union {
			recv = "u"
		}
		var body []ast.Stmt
		for _, c := range g.Checks {
			body = append(body, gateStmt(f, m, recv, c))
		}
		body = append(body, ret(nilExpr()))
		f.line()
		f.add(method(param(recv, star(id(g.Type))), "gates",
			funcType([]*ast.Field{param("n", star(id("Negotiation"))), param("method", id("string"))}, param("", id("error"))), body...))
	}
}

// gateStmt renders one check of a gates method.
func gateStmt(f *goFile, m *model.Model, recv string, c model.GateCheck) ast.Stmt {
	field := func() ast.Expr { return dot(recv, c.Field) }
	switch c.Kind {
	case model.GateCheckNested:
		return nestedGate(f, field, c)
	case model.GateCheckUndeclaredVariant:
		// The catch-all holds a value no capability advertises
		// (0019-PLAN-api-decisions.md, B20).
		v := func() ast.Expr { return dot(recv, c.Variant) }
		return ifStmt(nil, binary(v(), token.NEQ, nilExpr()),
			ret(addr(composite(id("CapabilityError"), kv(id("Method"), id("method")),
				kv(id("Capability"), binary(str(c.Capability+"."), token.ADD, sel(v(), c.Disc))),
				kv(id("Variant"), sel(v(), c.Disc))))))
	case model.GateCheckTerminalAuth:
		a := func() ast.Expr { return dot("a", c.Variant) }
		return rangeStmt(id("_"), id("a"), dot("n", "Response", c.List),
			ifStmt(nil, binary(binary(a(), token.NEQ, nilExpr()), token.LAND, binary(sel(a(), c.ID), token.EQL, field())),
				ret(call(f.qual(pkgFmt, "Errorf"), str("%s: "+c.Key+" %q: %w"), id("method"), field(), id("ErrTerminalAuthMethod")))))
	}
	root := func() ast.Expr { return dot("n", "Response", m.Initialize.RespCapsField) }
	if c.FromRequest {
		root = func() ast.Expr { return dot("n", "Request", m.Initialize.ReqCapsField) }
	}
	missing := func() ast.Expr {
		x := capabilityMissing(root, c.Steps)
		if b, ok := x.(*ast.BinaryExpr); ok && b.Op == token.LOR {
			return paren(x)
		}
		return x
	}
	refuse := func(fieldName ast.Expr) ast.Stmt {
		elts := []ast.Expr{kv(id("Method"), id("method")), kv(id("Capability"), str(c.Capability))}
		if fieldName != nil {
			elts = append(elts, kv(id("Field"), fieldName))
		}
		if c.Value != "" {
			elts = append(elts, kv(id("Variant"), str(c.Value)))
		}
		return ret(addr(composite(id("CapabilityError"), elts...)))
	}
	if c.Kind == model.GateCheckNonEmpty {
		return ifStmt(nil, binary(binary(call(id("len"), field()), token.GTR, intLit(0)), token.LAND, missing()), refuse(str(c.Key)))
	}
	// A variant: of the union itself, or of the union a field holds.
	set := func(x ast.Expr) ast.Expr {
		return binary(binary(sel(x, c.Variant), token.NEQ, nilExpr()), token.LAND, missing())
	}
	switch {
	case c.Field == "":
		return ifStmt(nil, set(id(recv)), refuse(nil))
	case c.Shape == model.RefSlice:
		elem := call(f.qual(pkgFmt, "Sprintf"), str(c.Key+"[%d]"), id("i"))
		return rangeStmt(id("i"), nil, field(), ifStmt(nil, set(index(field(), id("i"))), refuse(elem)))
	case c.Shape == model.RefPointer:
		return ifStmt(nil, binary(binary(field(), token.NEQ, nilExpr()), token.LAND, set(field())), refuse(str(c.Key)))
	default:
		return ifStmt(nil, set(field()), refuse(str(c.Key)))
	}
}

// nestedGate is the call into the gates of a held value, by its shape.
func nestedGate(f *goFile, field func() ast.Expr, c model.GateCheck) ast.Stmt {
	check := func(x ast.Expr) ast.Stmt {
		return ifStmt(define(exprs(id("err")), call(sel(x, "gates"), id("n"), id("method"))), errNotNil(), ret(id("err")))
	}
	switch c.Shape {
	case model.RefPointer:
		return ifStmt(nil, binary(field(), token.NEQ, nilExpr()), check(field()))
	case model.RefSlice:
		return rangeStmt(id("i"), nil, field(), check(index(field(), id("i"))))
	case model.RefMap:
		keys := call(f.qual("slices", "Sorted"), call(f.qual("maps", "Keys"), field()))
		return rangeStmt(id("_"), id("k"), keys, define(exprs(id("x")), index(field(), id("k"))), check(id("x")))
	default:
		return check(field())
	}
}
