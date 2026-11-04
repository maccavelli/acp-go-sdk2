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

// markGateHelpers marks the decoding helpers the gates call, which
// typesFile emits before them: unknownKind, for a check that reads a raw
// Unknown (0049-PLAN-surface-fixes.md, phase 4, deviation 4).
func markGateHelpers(f *goFile, m *model.Model) {
	for _, g := range m.Gated {
		for _, c := range g.Checks {
			if c.RawKey != "" {
				f.uses[helperUnknownKind] = true
			}
		}
	}
}

// gateStmt renders one check of a gates method.
func gateStmt(f *goFile, m *model.Model, recv string, c model.GateCheck) ast.Stmt {
	field := func() ast.Expr { return dot(recv, c.Field) }
	switch c.Kind {
	case model.GateCheckNested:
		return nestedGate(f, field, c)
	case model.GateCheckUndeclaredVariant:
		return undeclaredGate(f, recv, c)
	case model.GateCheckAdvertisedAuth:
		return advertisedGate(f, field, c)
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
	set := func(x func() ast.Expr) ast.Expr {
		return binary(binary(sel(x(), c.Variant), token.NEQ, nilExpr()), token.LAND, missing())
	}
	return eachUnion(f, recv, c, set, func(_ func() ast.Expr, name ast.Expr) []ast.Stmt { return []ast.Stmt{refuse(name)} })
}

// eachUnion checks the union a gate reads, recv itself or the one field c
// names, held by value, pointer or array: where cond(x) holds of it, body(x,
// name) runs, with name the field's JSON key, indexed for an array, or nil
// for recv.
func eachUnion(f *goFile, recv string, c model.GateCheck, cond func(x func() ast.Expr) ast.Expr,
	body func(x func() ast.Expr, name ast.Expr) []ast.Stmt,
) ast.Stmt {
	field := func() ast.Expr { return dot(recv, c.Field) }
	switch {
	case c.Field == "":
		self := func() ast.Expr { return id(recv) }
		return ifStmt(nil, cond(self), body(self, nil)...)
	case c.Shape == model.RefSlice:
		elem := func() ast.Expr { return index(field(), id("i")) }
		name := call(f.qual(pkgFmt, "Sprintf"), str(c.Key+"[%d]"), id("i"))
		return rangeStmt(id("i"), nil, field(), ifStmt(nil, cond(elem), body(elem, name)...))
	case c.Shape == model.RefPointer:
		return ifStmt(nil, binary(binary(field(), token.NEQ, nilExpr()), token.LAND, cond(field)), body(field, str(c.Key))...)
	default:
		return ifStmt(nil, cond(field), body(field, str(c.Key))...)
	}
}

// undeclaredGate refuses a union's catch-all, set, whose value no capability
// advertises: a variant whose Disc field holds the value, or the raw Unknown,
// from which unknownKind reads it (0019-PLAN-api-decisions.md, B20;
// 0049-PLAN-surface-fixes.md, phase 4, S9).
func undeclaredGate(f *goFile, recv string, c model.GateCheck) ast.Stmt {
	set := func(x func() ast.Expr) ast.Expr { return binary(sel(x(), c.Variant), token.NEQ, nilExpr()) }
	return eachUnion(f, recv, c, set, func(x func() ast.Expr, name ast.Expr) []ast.Stmt {
		var body []ast.Stmt
		value := func() ast.Expr { return sel(x(), c.Variant, c.Disc) }
		if c.RawKey != "" {
			body = append(body, define(exprs(id("kind")), call(id(helperUnknownKind), sel(x(), c.Variant), str(c.RawKey))))
			value = func() ast.Expr { return id("kind") }
		}
		elts := []ast.Expr{kv(id("Method"), id("method")), kv(id("Capability"), binary(str(c.Capability+"."), token.ADD, value()))}
		if name != nil {
			elts = append(elts, kv(id("Field"), name))
		}
		elts = append(elts, kv(id("Variant"), value()))
		return append(body, ret(addr(composite(id("CapabilityError"), elts...))))
	})
}

// advertisedGate refuses an id that names no entry of the initialize
// response's list: no variant's id, nor a raw entry's, which unknownKind
// reads (0049-PLAN-surface-fixes.md, phase 4, S8).
func advertisedGate(f *goFile, field func() ast.Expr, c model.GateCheck) ast.Stmt {
	var match ast.Expr
	or := func(x ast.Expr) {
		if match == nil {
			match = x
			return
		}
		match = binary(match, token.LOR, x)
	}
	for _, a := range c.Advertised {
		v := func() ast.Expr { return dot("a", a.Variant) }
		or(binary(binary(v(), token.NEQ, nilExpr()), token.LAND, binary(sel(v(), a.Field), token.EQL, field())))
	}
	if c.RawKey != "" {
		var raw ast.Expr = call(id(helperUnknownKind), dot("a", "Unknown"), str(c.RawKey))
		if c.IDType != "string" {
			raw = call(id(c.IDType), raw)
		}
		or(binary(binary(dot("a", "Unknown"), token.NEQ, nilExpr()), token.LAND, binary(raw, token.EQL, field())))
	}
	named := &ast.FuncLit{Type: funcType([]*ast.Field{param("a", id(c.Elem))}, param("", id("bool"))), Body: block(ret(match))}
	return ifStmt(nil, not(call(f.qual("slices", "ContainsFunc"), dot("n", "Response", c.List), named)),
		ret(call(f.qual(pkgFmt, "Errorf"), str("%s: "+c.Key+" %q: %w"), id("method"), field(), id("ErrUnadvertisedAuthMethod"))))
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
