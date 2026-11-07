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
			if c.Kind == model.GateCheckAdvertisedAuth {
				emitAcceptedIDs(f, g.Type, c)
			}
			body = append(body, gateStmt(f, m, g.Type, recv, c))
		}
		body = append(body, ret(nilExpr()))
		f.line()
		f.add(method(param(recv, star(id(g.Type))), "gates",
			funcType([]*ast.Field{param("n", star(id("Negotiation"))), param("method", id("string"))}, param("", id("error"))), body...))
	}
}

// markGateHelpers marks the decoding helpers the gates call, which
// typesFile emits before them: unknownKind, for a check that reads a raw
// Unknown's discriminator (0049-PLAN-surface-fixes.md, phase 4, deviation 4);
// for an advertised check, unknownString, which reads an Unknown's id with no
// fallback, and quotedIDs, which its refusal names the accepted ids with
// (0050-PLAN-auth-method-ids-and-the-prompt-result-for-issue-14.md, phase 1).
func markGateHelpers(f *goFile, m *model.Model) {
	for _, g := range m.Gated {
		for _, c := range g.Checks {
			switch {
			case c.Kind == model.GateCheckAdvertisedAuth:
				f.uses[helperQuotedIDs] = true
				if c.RawKey != "" {
					f.uses[helperUnknownString] = true
				}
			case c.RawKey != "":
				f.uses[helperUnknownKind] = true
			}
		}
	}
}

// acceptedIDsName is the name of the function that lists the ids the
// advertised check c of the gated type typ accepts: accepted, the type, the
// field and "s", AuthenticateRequest's MethodId giving
// acceptedAuthenticateRequestMethodIds.
func acceptedIDsName(typ string, c model.GateCheck) string {
	return "accepted" + typ + c.Field + "s"
}

// emitAcceptedIDs emits, for the advertised check c of the gated type typ,
//
//	func acceptedAuthenticateRequestMethodIds(list []AuthMethod) []AuthMethodId
//
// which returns the ids of list, an initialize answer's, that the check
// accepts, in list's order, each once: every advertised variant's id but the
// terminal one's, and an Unknown entry's id when it is a JSON string; and no
// id a terminal entry names, even one another variant holds, since the
// terminal check refuses it first. The gate and helpers.go's
// AuthenticateMethodIDs both read it, so the two cannot differ
// (0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md, D1 and D2).
func emitAcceptedIDs(f *goFile, typ string, c model.GateCheck) {
	idType := func() ast.Expr { return id(c.IDType) }
	var body []ast.Stmt
	if c.Terminal != "" {
		for _, a := range c.Advertised {
			if a.Variant != c.Terminal {
				continue
			}
			body = append(body, varStmt("terminal", sliceOf(idType())),
				rangeStmt(id("_"), id("a"), id("list"),
					ifStmt(nil, binary(dot("a", a.Variant), token.NEQ, nilExpr()),
						assign(exprs(id("terminal")), call(id("append"), id("terminal"), dot("a", a.Variant, a.Field))))))
		}
	}
	var cases []ast.Stmt
	for _, a := range c.Advertised {
		if a.Variant == c.Terminal {
			continue
		}
		cases = append(cases, caseOf(exprs(binary(dot("a", a.Variant), token.NEQ, nilExpr())),
			assign(exprs(id("x")), dot("a", a.Variant, a.Field))))
	}
	if c.RawKey != "" {
		var conv ast.Expr = id("s")
		if c.IDType != "string" {
			conv = call(idType(), id("s"))
		}
		cases = append(cases, caseOf(exprs(binary(dot("a", "Unknown"), token.NEQ, nilExpr())),
			define(exprs(id("s"), id("ok")), call(id(helperUnknownString), dot("a", "Unknown"), str(c.RawKey))),
			ifStmt(nil, not(id("ok")), continueStmt()),
			assign(exprs(id("x")), conv)))
	}
	cases = append(cases, caseOf(nil, continueStmt()))
	var keep ast.Expr = not(call(f.qual("slices", "Contains"), id("ids"), id("x")))
	if c.Terminal != "" {
		keep = binary(not(call(f.qual("slices", "Contains"), id("terminal"), id("x"))), token.LAND, keep)
	}
	body = append(body, varStmt("ids", sliceOf(idType())),
		rangeStmt(id("_"), id("a"), id("list"),
			varStmt("x", idType()),
			switchStmt(nil, cases...),
			ifStmt(nil, keep, assign(exprs(id("ids")), call(id("append"), id("ids"), id("x"))))),
		ret(id("ids")))
	name := acceptedIDsName(typ, c)
	f.line()
	f.comment(name+" returns the ids of list, an initialize",
		"answer's, that "+typ+"'s "+c.Field+" may name, in list's order, each",
		"once: no id a terminal entry names, and an Unknown entry's only when it is a",
		"JSON string.")
	f.add(function(name, funcType([]*ast.Field{param("list", sliceOf(id(c.Elem)))}, param("", sliceOf(idType()))), body...))
}

// gateStmt renders one check of a gates method of the gated type typ.
func gateStmt(f *goFile, m *model.Model, typ, recv string, c model.GateCheck) ast.Stmt {
	field := func() ast.Expr { return dot(recv, c.Field) }
	switch c.Kind {
	case model.GateCheckNested:
		return nestedGate(f, field, c)
	case model.GateCheckUndeclaredVariant:
		return undeclaredGate(f, recv, c)
	case model.GateCheckAdvertisedAuth:
		return advertisedGate(f, field, acceptedIDsName(typ, c), c)
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
// response's list (0049-PLAN-surface-fixes.md, phase 4, S8): one the list
// function accepted, emitted by emitAcceptedIDs, does not hold. Its refusal
// names the ids that function accepts, "none" for none
// (0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md, D2 and
// D4). The terminal check runs before it, so a terminal id keeps its own
// refusal.
func advertisedGate(f *goFile, field func() ast.Expr, accepted string, c model.GateCheck) ast.Stmt {
	return ifStmt(define(exprs(id("ids")), call(id(accepted), dot("n", "Response", c.List))),
		not(call(f.qual("slices", "Contains"), id("ids"), field())),
		ret(call(f.qual(pkgFmt, "Errorf"), str("%s: "+c.Key+" %q: %w; %s accepts %s"), id("method"), field(),
			id("ErrUnadvertisedAuthMethod"), id("method"), call(id(helperQuotedIDs), id("ids")))))
}

// nestedGate is the call into the gates of a held value, by its shape.
func nestedGate(f *goFile, field func() ast.Expr, c model.GateCheck) ast.Stmt {
	check := func(x ast.Expr) ast.Stmt {
		return ifStmt(define(exprs(id("err")), call(sel(x, "gates"), id("n"), id("method"))), errNotNil(), ret(id("err")))
	}
	if c.Items {
		// A union's variant that is a named slice or map: each item's gates,
		// when the variant is set (0049-MADR-remediate-the-2026-11-01-
		// debugging-pass.md, G19).
		inner := c
		inner.Items = false
		return ifStmt(nil, binary(field(), token.NEQ, nilExpr()), nestedGate(f, func() ast.Expr { return paren(star(field())) }, inner))
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
