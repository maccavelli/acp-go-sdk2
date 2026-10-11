package emit

import (
	"go/ast"
	"go/token"
	"slices"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// registryFile builds schema_gen_test.go: data about the generated code that
// the SDK's own tests iterate over, and peers that record every call
// (0011-MADR-harden-code-generator-engine.md, phase 6 amendment, D3). It is a
// test file, so nothing in it reaches a dependent's build.
func registryFile(m *model.Model) *goFile {
	f := newFile()
	field := func(name string, typ ast.Expr) *ast.Field {
		return &ast.Field{Names: []*ast.Ident{id(name)}, Type: typ}
	}
	newFunc := &ast.FuncType{Params: fields(), Results: results(param("", id("any")))}
	structDecl := func(name string, doc []string, list ...*ast.Field) {
		var s fieldSet
		for _, fld := range list {
			s.add(fld)
		}
		f.comment(doc...)
		f.add(typeDecl(name, s.structType(f)))
		f.line()
	}

	structDecl("schemaType", []string{
		"schemaType is a generated type and the schema definition it was built from:",
		"the definition's key in File, a file of the schema directory.",
	}, field("Type", id("string")), field("Def", id("string")), field("File", id("string")), field("New", newFunc))
	structDecl("schemaVariant", []string{
		"schemaVariant is a union's field for one variant, and the variant's",
		"discriminator value, empty for a variant without one.",
	}, field("Field", id("string")), field("Value", id("string")))
	structDecl("schemaUnion", []string{
		"schemaUnion is a union: its discriminator, empty for none, its variants, whether",
		"it keeps an unknown value in Unknown, and its catch-all variant's field, if any.",
	}, field("Type", id("string")), field("Key", id("string")), field("Variants", sliceOf(id("schemaVariant"))),
		field("Unknown", id("bool")), field("CatchAll", id("string")))
	structDecl("schemaMethod", []string{
		"schemaMethod is a wire method: the side that handles it, its parameters, and its",
		"result, nil for a notification or a request without one.",
	}, field("Side", id("string")), field("Wire", id("string")), field("Params", newFunc), field("Result", newFunc),
		field("Notification", id("bool")))

	f.comment("newOf returns a new zero T.")
	newOf := function("newOf", funcType(nil, param("", id("any"))), ret(call(id("new"), id("T"))))
	newOf.Type.TypeParams = typeParamT()
	f.add(newOf)
	f.line()
	newOfType := func(name string) ast.Expr { return index(id("newOf"), id(name)) }

	var types []ast.Expr
	var unions []ast.Expr
	for _, t := range m.Types {
		if t.Kind == model.KindNone {
			continue
		}
		types = append(types, composite(nil,
			kv(id("Type"), str(t.Name)), kv(id("Def"), str(t.Origin.Key)), kv(id("File"), str(t.Origin.File)),
			kv(id("New"), newOfType(t.Name))))
		if t.Kind == model.KindUnion {
			unions = append(unions, unionEntry(t))
		}
	}
	listFunc(f, "schemaTypes", "schemaType", types,
		"schemaTypes returns every generated type, in the order of types_gen.go.")
	listFunc(f, "schemaUnions", "schemaUnion", unions,
		"schemaUnions returns every generated union, in the order of types_gen.go.")

	var methods []ast.Expr
	for _, mi := range slices.Concat(m.AgentMethods, m.ClientMethods, m.Protocol) {
		elts := []ast.Expr{kv(id("Side"), str(mi.Side)), kv(id("Wire"), str(mi.Wire))}
		if mi.Notif != "" {
			elts = append(elts, kv(id("Params"), newOfType(mi.Notif)))
		} else {
			elts = append(elts, kv(id("Params"), newOfType(mi.Req)))
			if mi.HasResult {
				elts = append(elts, kv(id("Result"), newOfType(mi.Resp)))
			}
		}
		methods = append(methods, composite(nil, append(elts, kv(id("Notification"), boolLit(mi.Notif != "")))...))
	}
	listFunc(f, "schemaMethods", "schemaMethod", methods,
		"schemaMethods returns every wire method: the agent's, the client's, then the protocol's.")

	recorder(f, "recordingAgent", "a", m.AgentMethods, "agent")
	recorder(f, "recordingClient", "c", m.ClientMethods, "client")

	// Each interface is asserted, so the type check of the output proves the
	// recorders implement them.
	var specs []ast.Spec
	for _, in := range m.Interfaces {
		rec := "recordingClient"
		if in.Binding == model.BindAgent || in.Binding == model.BindAgentOptional || in.Binding == model.BindAgentExperimental {
			rec = "recordingAgent"
		}
		specs = append(specs, &ast.ValueSpec{Names: []*ast.Ident{id("_")}, Type: id(in.Name), Values: []ast.Expr{composite(id(rec))}})
	}
	if len(specs) > 0 {
		f.comment("The recorders implement every interface.")
		f.add(&ast.GenDecl{Tok: token.VAR, Lparen: 1, Specs: specs})
	}
	return f
}

// unionEntry is the schemaUnion element of t.
func unionEntry(t *model.Type) ast.Expr {
	var variants []ast.Expr
	for _, vi := range t.Variants {
		if !vi.Skip {
			variants = append(variants, composite(nil, kv(id("Field"), str(vi.FieldName)), kv(id("Value"), str(vi.DiscValue))))
		}
	}
	catchAll := ""
	if t.CatchAll >= 0 {
		catchAll = t.Variants[t.CatchAll].FieldName
	}
	return composite(nil,
		kv(id("Type"), str(t.Name)), kv(id("Key"), str(t.DiscKey)),
		kv(id("Variants"), composite(sliceOf(id("schemaVariant")), variants...)),
		kv(id("Unknown"), boolLit(t.HasUnknown)), kv(id("CatchAll"), str(catchAll)))
}

// listFunc adds func name() []elem { return []elem{elts...} }.
func listFunc(f *goFile, name, elem string, elts []ast.Expr, doc string) {
	f.comment(doc)
	f.add(function(name, funcType(nil, param("", sliceOf(id(elem)))), ret(composite(sliceOf(id(elem)), elts...))))
	f.line()
}

// recorder adds a type that implements every method of one side: each sends
// its parameters on calls and returns the zero result, or, when reply is set,
// what reply fills the result with and its error.
func recorder(f *goFile, name, recvName string, methods []*model.Method, side string) {
	f.comment(name+" implements every "+side+" interface. Each method sends its parameters on calls",
		"and returns the zero result, or, when reply is set, the result reply fills, nil for a method",
		"without one, and reply's error.")
	var s fieldSet
	s.add(&ast.Field{Names: []*ast.Ident{id("calls")}, Type: &ast.ChanType{Dir: ast.SEND | ast.RECV, Value: id("any")}})
	s.add(&ast.Field{Names: []*ast.Ident{id("reply")}, Type: funcType([]*ast.Field{param("result", id("any"))}, param("", id("error")))})
	f.add(typeDecl(name, s.structType(f)))
	f.line()
	for _, mi := range methods {
		paramType := mi.Req
		if mi.Notif != "" {
			paramType = mi.Notif
		}
		send := &ast.SendStmt{Chan: dot(recvName, "calls"), Arrow: token.NoPos, Value: id("params")}
		sig := []*ast.Field{param("_", f.qual(pkgContext, "Context")), param("params", id(paramType))}
		var fn *ast.FuncDecl
		if mi.Notif == "" && mi.HasResult {
			fn = method(param(recvName, id(name)), mi.GoName, funcType(sig, param("", id(mi.Resp)), param("", id("error"))),
				send, varStmt("r", id(mi.Resp)),
				ifStmt(nil, binary(dot(recvName, "reply"), token.NEQ, nilExpr()), ret(id("r"), call(dot(recvName, "reply"), addr(id("r"))))),
				ret(id("r"), nilExpr()))
		} else {
			fn = method(param(recvName, id(name)), mi.GoName, funcType(sig, param("", id("error"))), send,
				ifStmt(nil, binary(dot(recvName, "reply"), token.NEQ, nilExpr()), ret(call(dot(recvName, "reply"), nilExpr()))),
				ret(nilExpr()))
		}
		f.add(fn)
		f.line()
	}
}
