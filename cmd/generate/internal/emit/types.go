package emit

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// fieldSet collects struct fields or interface methods, each with the doc
// comment lines added before it.
type fieldSet struct {
	list []*ast.Field
	doc  []string
}

// comment adds the lines of desc as doc for the next field.
func (s *fieldSet) comment(lines ...string) { s.doc = append(s.doc, lines...) }

func (s *fieldSet) add(fld *ast.Field) {
	fld.Doc = docLines(s.doc...)
	s.doc = nil
	s.list = append(s.list, fld)
}

func (s *fieldSet) fieldList() *ast.FieldList {
	if len(s.doc) > 0 {
		panic("emit: doc comment with no field after it")
	}
	return fields(s.list...)
}

func (s *fieldSet) structType() *ast.StructType { return &ast.StructType{Fields: s.fieldList()} }

func (s *fieldSet) interfaceType() *ast.InterfaceType {
	return &ast.InterfaceType{Methods: s.fieldList()}
}

// structField is name typ `json:"tag"`.
func structField(name string, typ ast.Expr, tag string) *ast.Field {
	return &ast.Field{Names: []*ast.Ident{id(name)}, Type: typ, Tag: structTag(tag)}
}

// WriteTypes emits types_gen.go with all types and the Agent/Client interfaces.
func WriteTypes(outDir string, m *model.Model) error {
	f := newFile()
	for _, t := range m.Types {
		emitSchemaDef(f, t)
	}
	emitRPCInterfaces(f, m)

	return f.write(outDir, "types_gen.go")
}

// interfaceMethod is name(ctx context.Context, params paramType) error, or
// (respType, error) when respType is set.
func interfaceMethod(f *goFile, name, paramType, respType string) *ast.Field {
	res := []*ast.Field{param("", id("error"))}
	if respType != "" {
		res = []*ast.Field{param("", id(respType)), param("", id("error"))}
	}
	return &ast.Field{
		Names: []*ast.Ident{id(name)},
		Type:  funcType([]*ast.Field{param("ctx", f.qual(pkgContext, "Context")), param("params", id(paramType))}, res...),
	}
}

// addRPCMethod adds the interface method for mi to target.
func addRPCMethod(f *goFile, target *fieldSet, mi *model.Method) {
	target.comment(mi.Doc...)
	if mi.Notif != "" {
		target.add(interfaceMethod(f, mi.GoName, mi.Notif, ""))
		return
	}
	resp := ""
	if mi.HasResult {
		resp = mi.Resp
	}
	target.add(interfaceMethod(f, mi.GoName, mi.Req, resp))
}

func emitRPCInterfaces(f *goFile, m *model.Model) {
	var agent, agentLoader, agentExperimental fieldSet
	for _, mi := range m.AgentMethods {
		target := &agent
		switch mi.Binding {
		case model.BindAgentLoader:
			target = &agentLoader
		case model.BindAgentExperimental:
			target = &agentExperimental
		}
		addRPCMethod(f, target, mi)
	}
	f.add(typeDecl("Agent", agent.interfaceType()))
	if len(agentLoader.list) > 0 {
		f.comment("AgentLoader defines optional support for loading sessions. Implement and advertise the capability to enable 'session/load'.")
		f.add(typeDecl("AgentLoader", agentLoader.interfaceType()))
	}
	// Always emit AgentExperimental, even if empty.
	f.comment("AgentExperimental defines unstable methods that are not part of the official spec. These may change or be removed without notice.")
	f.add(typeDecl("AgentExperimental", agentExperimental.interfaceType()))

	var client, clientExperimental, clientTerminal fieldSet
	for _, mi := range m.ClientMethods {
		target := &client
		switch mi.Binding {
		case model.BindClientExperimental:
			target = &clientExperimental
		case model.BindClientTerminal:
			target = &clientTerminal
		}
		addRPCMethod(f, target, mi)
	}
	f.add(typeDecl("Client", client.interfaceType()))
	if len(clientTerminal.list) > 0 {
		f.comment("ClientTerminal defines terminal-related experimental methods (x-docs-ignore). Implement and advertise 'terminal: true' to enable 'terminal/*'.")
		f.add(typeDecl("ClientTerminal", clientTerminal.interfaceType()))
	}
	// Always emit ClientExperimental, even if empty.
	f.comment("ClientExperimental defines unstable methods that are not part of the official spec. These may change or be removed without notice.")
	f.add(typeDecl("ClientExperimental", clientExperimental.interfaceType()))
}

// enumConsts declares <Const> name = "value" for each value.
func enumConsts(f *goFile, name string, values []model.EnumValue) {
	if len(values) == 0 {
		return
	}
	var specs []*ast.ValueSpec
	for _, v := range values {
		specs = append(specs, constSpec(v.Const, id(name), str(v.Value)))
	}
	f.add(constDecl(true, specs...))
}

func emitSchemaDef(f *goFile, t *model.Type) {
	name := t.Name
	if t.Kind == model.KindNone {
		return
	}

	// A union emits its variant types first, so emitUnion places its
	// description; every other case starts with the type it documents.
	if t.Kind != model.KindUnion {
		f.comment(t.Doc...)
	}

	switch t.Kind {
	case model.KindEnum, model.KindStringConstUnion, model.KindOpenEnum:
		// An open enum's unknown values are still representable as strings,
		// matching the schema's extensibility.
		f.add(typeDecl(name, id("string")))
		enumConsts(f, name, t.Enum)
		f.line()
	case model.KindUnion:
		emitUnion(f, t)
	case model.KindObject:
		emitObjectSchema(f, t)
	case model.KindPrimitive:
		f.add(typeDecl(name, typeAST(*t.Underlying)))
		f.line()
	case model.KindEmptyObject:
		// Empty object shape: emit a concrete empty struct so methods can be defined
		// and the wire encoding is consistently {} rather than null.
		f.add(typeDecl(name, &ast.StructType{Fields: fields()}))
		f.line()
	default:
		f.comment(fmt.Sprintf("%s is a union or complex schema; represented generically.", name))
		f.add(typeDecl(name, id("any")))
		f.line()
	}

	// Union wrappers get Validate from emitUnion.
	switch t.Validate {
	case model.ValidateFields, model.ValidateToolCall:
		emitFieldValidate(f, t)
	}
}

func emitObjectSchema(f *goFile, t *model.Type) {
	// Nested struct types for inline object properties come first.
	for _, ns := range t.NestedStructs {
		f.comment(ns.Doc...)
		emitStruct(f, ns.Name, ns.Fields, false)
	}
	emitStruct(f, t.Name, t.Fields, false)
}

// emitStruct emits type name struct { fields } and the methods its defaults
// and fills need.
func emitStruct(f *goFile, name string, fields []*model.Field, keepExtra bool) {
	var st fieldSet
	for _, fld := range fields {
		st.comment(fld.Doc...)
		st.add(structField(fld.Name, typeAST(fld.Type), fld.Tag))
	}
	if keepExtra {
		st.comment("Extra holds the keys of the payload this struct does not declare, as received.",
			"MarshalJSON writes them back.")
		st.add(structField(extraField, rawMap(f.qual(pkgJSON, "RawMessage")), "-"))
	}
	f.add(typeDecl(name, st.structType()))
	f.line()
	emitObjectDefaults(f, name, fields, keepExtra)
}

// typeAST renders a model type as a Go type expression. Each call builds new
// nodes, so no node is shared between two places in a file.
func typeAST(r model.TypeRef) ast.Expr {
	switch r.Kind {
	case model.RefPointer:
		return star(typeAST(*r.Elem))
	case model.RefSlice:
		return sliceOf(typeAST(*r.Elem))
	case model.RefMap:
		return rawMap(typeAST(*r.Elem))
	default:
		return id(r.Name)
	}
}

// emitObjectDefaults synthesizes MarshalJSON for structs with fields that
// carry schema defaults or are required slices or maps, and UnmarshalJSON for
// structs with defaults.
func emitObjectDefaults(f *goFile, name string, fields []*model.Field, keepExtra bool) {
	var defaults, fills []*model.Field
	for _, fld := range fields {
		if fld.Default != nil {
			defaults = append(defaults, fld)
		}
		if fld.Fill {
			fills = append(fills, fld)
		}
	}
	if len(defaults) == 0 && len(fills) == 0 && !keepExtra {
		return
	}
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }
	marshal := func(x ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Marshal"), x) }

	// MarshalJSON: coerce nil slices and maps to their defaults, and nil
	// required ones to empty, before encoding.
	body := []ast.Stmt{
		typeStmt("Alias", id(name)),
		varStmt("a", id("Alias")),
		assign(exprs(id("a")), call(id("Alias"), id("v"))),
	}
	for _, fld := range defaults {
		// Typed object defaults (non-nilable) are not injected on encode.
		dp := fld.Default
		if (dp.Kind == model.DefaultArray || dp.Kind == model.DefaultObject) && dp.Nilable {
			body = append(body, ifStmt(nil, binary(dot("a", fld.Name), token.EQL, nilExpr()),
				exprStmt(unmarshal(bytesOf(dp.JSON), addr(dot("a", fld.Name)))),
			))
		}
	}
	for _, fld := range fills {
		body = append(body, ifStmt(nil, binary(dot("a", fld.Name), token.EQL, nilExpr()),
			assign(exprs(dot("a", fld.Name)), composite(typeAST(fld.Type))),
		))
	}
	if keepExtra {
		// Add each Extra key the declared fields did not write.
		body = append(body,
			define(exprs(id("b"), id("err")), marshal(id("a"))),
			ifStmt(nil, binary(errNotNil(), token.LOR, binary(call(id("len"), dot("v", extraField)), token.EQL, intLit(0))), ret(id("b"), id("err"))),
			varStmt("m", rawMap(f.qual(pkgJSON, "RawMessage"))),
			ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("m")))), errNotNil(), ret(nilExpr(), id("err"))),
			rangeStmt(id("k"), id("x"), dot("v", extraField),
				ifStmt(define(exprs(id("_"), id("ok")), index(id("m"), id("k"))), not(id("ok")),
					assign(exprs(index(id("m"), id("k"))), id("x")))),
			ret(marshal(id("m"))),
		)
	} else {
		body = append(body, ret(marshal(id("a"))))
	}
	f.add(method(param("v", id(name)), "MarshalJSON",
		funcType(nil, param("", byteSlice()), param("", id("error"))), body...))
	f.line()
	if len(defaults) == 0 && !keepExtra {
		return
	}

	// UnmarshalJSON: apply defaults when a field is missing, or null when
	// the schema does not allow null.
	body = []ast.Stmt{
		varStmt("m", rawMap(f.qual(pkgJSON, "RawMessage"))),
		ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("m")))), errNotNil(), ret(id("err"))),
		typeStmt("Alias", id(name)),
		varStmt("a", id("Alias")),
		ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("a")))), errNotNil(), ret(id("err"))),
	}
	for _, fld := range defaults {
		// Only a property that rejects null reads the raw value.
		raw := "_"
		var cond ast.Expr = not(id("_ok"))
		if !fld.Default.AllowNull {
			raw = "_rm"
			cond = binary(not(id("_ok")), token.LOR, paren(binary(call(id("string"), id("_rm")), token.EQL, str(schemaTypeNull))))
		}
		body = append(body, block(
			define(exprs(id(raw), id("_ok")), index(id("m"), str(fld.Key))),
			ifStmt(nil, cond, exprStmt(unmarshal(bytesOf(fld.Default.JSON), addr(dot("a", fld.Name))))),
		))
	}
	if keepExtra {
		// Keep the keys no field declares.
		var declared []ast.Expr
		for _, fld := range fields {
			declared = append(declared, str(fld.Key))
		}
		body = append(body,
			rangeStmt(id("_"), id("k"), composite(sliceOf(id("string")), declared...), exprStmt(call(id("delete"), id("m"), id("k")))),
			ifStmt(nil, binary(call(id("len"), id("m")), token.GTR, intLit(0)), assign(exprs(dot("a", extraField)), id("m"))),
		)
	}
	body = append(body,
		assign(exprs(star(id("v"))), call(id(name), id("a"))),
		ret(nilExpr()),
	)
	f.add(method(param("v", star(id(name))), "UnmarshalJSON",
		funcType([]*ast.Field{param("b", byteSlice())}, param("", id("error"))), body...))
	f.line()
}

// emitInlineVariant emits the type the model gives a union variant without a
// $ref, if any.
func emitInlineVariant(f *goFile, tname string, in *model.Inline) {
	if in == nil {
		return
	}
	f.comment(in.Doc...)
	switch in.Kind {
	case model.InlineStruct:
		emitStruct(f, tname, in.Fields, in.KeepExtra)
	case model.InlineRaw:
		// An alias keeps json.RawMessage's methods, unlike a defined type.
		f.add(aliasDecl(tname, f.qual(pkgJSON, "RawMessage")))
		f.line()
	case model.InlineNamed:
		// Non-object title-only variants can still carry payloads (e.g. arrays).
		f.add(typeDecl(tname, typeAST(in.Type)))
		f.line()
	default:
		f.add(typeDecl(tname, &ast.StructType{Fields: fields()}))
		f.line()
	}
}

// setVariant is u.<field> = &v; return nil.
func setVariant(vi *model.Variant) []ast.Stmt {
	return []ast.Stmt{
		assign(exprs(dot("u", vi.FieldName)), addr(id("v"))),
		ret(nilExpr()),
	}
}

// decodeStrict decodes b into v and sets the variant. A failure returns
// "<where>: <cause>".
func decodeStrict(f *goFile, vi *model.Variant, where string) []ast.Stmt {
	return append([]ast.Stmt{
		ifStmt(define(exprs(id("err")), call(f.qual(pkgJSON, "Unmarshal"), id("b"), addr(id("v")))), errNotNil(),
			ret(call(f.qual(pkgFmt, "Errorf"), str(where+": %w"), id("err")))),
	}, setVariant(vi)...)
}

// variantPlace names a variant in an error: "<Union>: variant <Field>".
func variantPlace(union string, vi *model.Variant) string { return union + ": variant " + vi.FieldName }

// keyMatch sets match = false when key is missing from m.
func keyMatch(m, key string) ast.Stmt {
	return ifStmt(define(exprs(id("_"), id("ok")), index(id(m), str(key))), not(id("ok")),
		assign(exprs(id("match")), boolLit(false)))
}

func unionUnmarshalBody(f *goFile, name string, variants []*model.Variant, discKey string, hasUnknown bool, catchAll *model.Variant) []ast.Stmt {
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }
	var body []ast.Stmt

	// Handle literal null with the first null-only variant.
	for _, vi := range variants {
		if vi.IsNull {
			body = append(body, ifStmt(nil, binary(call(id("string"), id("b")), token.EQL, str(schemaTypeNull)),
				append([]ast.Stmt{varStmt("v", id(vi.TypeName))}, setVariant(vi)...)...))
			break
		}
	}

	var object []ast.Stmt
	// Prefer discriminator-based dispatch when available (e.g. "type", "outcome").
	if discKey != "" {
		var cases []ast.Stmt
		for _, vi := range variants {
			if vi.DiscValue != "" {
				cases = append(cases, caseOf(exprs(str(vi.DiscValue)),
					append([]ast.Stmt{varStmt("v", id(vi.TypeName))},
						decodeStrict(f, vi, name+": "+discKey+" "+strconv.Quote(vi.DiscValue))...)...))
			}
		}
		// A discriminator that is present and not a string is an error.
		disc := []ast.Stmt{
			varStmt("disc", id("string")),
			ifStmt(define(exprs(id("v"), id("ok")), index(id("m"), str(discKey))), id("ok"),
				ifStmt(define(exprs(id("err")), unmarshal(id("v"), addr(id("disc")))), errNotNil(),
					ret(call(f.qual(pkgFmt, "Errorf"), str(name+": discriminator "+strconv.Quote(discKey)+": %w"), id("err"))))),
			switchStmt(id("disc"), cases...),
		}
		if catchAll != nil {
			// A present value no case took belongs to the schema's catch-all.
			disc = append(disc, ifStmt(define(exprs(id("raw"), id("ok")), index(id("m"), str(discKey))),
				binary(id("ok"), token.LAND, binary(call(id("string"), id("raw")), token.NEQ, str(schemaTypeNull))),
				append([]ast.Stmt{
					varStmt("v", id(catchAll.TypeName)),
					ifStmt(define(exprs(id("err")), call(f.qual(pkgJSON, "Unmarshal"), id("b"), addr(id("v")))), errNotNil(),
						ret(call(f.qual(pkgFmt, "Errorf"), str(name+": "+discKey+" %q: %w"), id("disc"), id("err")))),
				}, setVariant(catchAll)...)...,
			))
		}
		if hasUnknown {
			// A present value no case took is kept as received.
			disc = append(disc, ifStmt(define(exprs(id("v"), id("ok")), index(id("m"), str(discKey))),
				binary(id("ok"), token.LAND, binary(call(id("string"), id("v")), token.NEQ, str(schemaTypeNull))),
				assign(exprs(dot("u", unknownField)), call(f.qual(pkgBytes, "Clone"), id("b"))),
				ret(nilExpr()),
			))
		}
		object = append(object, block(disc...))
	}
	// Required-key match.
	for _, vi := range variants {
		if !vi.IsObject || len(vi.Required) == 0 {
			continue
		}
		stmts := []ast.Stmt{
			varStmt("v", id(vi.TypeName)),
			define(exprs(id("match")), boolLit(true)),
		}
		for _, rk := range vi.Required {
			stmts = append(stmts, keyMatch("m", rk))
		}
		stmts = append(stmts, ifStmt(nil, id("match"), decodeStrict(f, vi, variantPlace(name, vi))...))
		object = append(object, block(stmts...))
	}
	body = append(body,
		varStmt("m", rawMap(f.qual(pkgJSON, "RawMessage"))),
		ifElse(define(exprs(id("err")), unmarshal(id("b"), addr(id("m")))), binary(id("err"), token.EQL, nilExpr()),
			object,
			// Not an object (e.g., primitive union variant) or invalid JSON.
			[]ast.Stmt{ifStmt(define(exprs(id("_"), id("ok")), typeAssert(id("err"), star(f.qual(pkgJSON, "UnmarshalTypeError")))), not(id("ok")),
				ret(id("err")))},
		),
	)

	// For array variants with required keys on object items, try key-based matching first.
	var array []ast.Stmt
	for _, vi := range variants {
		if !vi.IsArray || len(vi.ArrayItemRequired) == 0 {
			continue
		}
		var loop []ast.Stmt
		for _, rk := range vi.ArrayItemRequired {
			loop = append(loop, keyMatch("elem", rk))
		}
		array = append(array, block(
			varStmt("v", id(vi.TypeName)),
			define(exprs(id("match")), boolLit(true)),
			rangeStmt(id("_"), id("elem"), id("arr"), loop...),
			ifStmt(nil, id("match"), decodeStrict(f, vi, variantPlace(name, vi))...),
		))
	}
	body = append(body,
		varStmt("arr", sliceOf(rawMap(f.qual(pkgJSON, "RawMessage")))),
		ifStmt(nil, binary(unmarshal(id("b"), addr(id("arr"))), token.EQL, nilExpr()), array...),
	)

	// Fallback: try each variant in turn.
	for _, vi := range variants {
		body = append(body, block(
			varStmt("v", id(vi.TypeName)),
			ifStmt(nil, binary(unmarshal(id("b"), addr(id("v"))), token.EQL, nilExpr()), setVariant(vi)...),
		))
	}
	return append(body, ret(call(f.qual(pkgErrors, "New"), str(name+": no variant matches"))))
}

func unionMarshalBody(f *goFile, name string, variants []*model.Variant, hasUnknown bool) []ast.Stmt {
	marshal := func(x ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Marshal"), x) }
	var body []ast.Stmt
	for _, vi := range variants {
		var then []ast.Stmt
		switch {
		case vi.IsNull:
			// Null-only variant encodes to JSON null.
			then = []ast.Stmt{ret(marshal(nilExpr()))}
		case !vi.IsObject:
			// Non-object variants (e.g., arrays/primitives) are already in final wire shape.
			then = []ast.Stmt{
				define(exprs(id("_b"), id("_e")), marshal(star(dot("u", vi.FieldName)))),
				ifStmt(nil, binary(id("_e"), token.NEQ, nilExpr()), ret(emptyBytes(), id("_e"))),
				ret(id("_b"), nilExpr()),
			}
		default:
			then = []ast.Stmt{
				define(exprs(id("_b"), id("_e")), marshal(star(dot("u", vi.FieldName)))),
				ifStmt(nil, binary(id("_e"), token.NEQ, nilExpr()), ret(emptyBytes(), id("_e"))),
				// Reshape as raw values, so numbers keep every digit.
				varStmt("m", rawMap(f.qual(pkgJSON, "RawMessage"))),
				ifStmt(define(exprs(id("err")), call(f.qual(pkgJSON, "Unmarshal"), id("_b"), addr(id("m")))), errNotNil(),
					ret(nilExpr(), call(f.qual(pkgFmt, "Errorf"), str(name+": %w"), id("err")))),
			}
			// Inject const discriminants, as raw JSON strings.
			for _, kv := range vi.ConstPairs {
				then = append(then, assign(exprs(index(id("m"), str(kv[0]))), call(f.qual(pkgJSON, "RawMessage"), str(strconv.Quote(kv[1])))))
			}
			// Remarshal, with the injected discriminant.
			then = append(then, ret(marshal(id("m"))))
		}
		body = append(body, ifStmt(nil, binary(dot("u", vi.FieldName), token.NEQ, nilExpr()), then...))
	}
	if hasUnknown {
		body = append(body, ifStmt(nil, binary(dot("u", unknownField), token.NEQ, nilExpr()), ret(dot("u", unknownField), nilExpr())))
	}
	return append(body, ret(nilExpr(), call(f.qual(pkgErrors, "New"), str(name+": no variant set"))))
}

// requiredCheck returns fmt.Errorf("<prop> is required") when x is the zero
// value for a required string or array property.
func requiredCheck(f *goFile, x ast.Expr, kind, prop string) ast.Stmt {
	var zero ast.Expr = nilExpr()
	if kind == schemaTypeString {
		zero = str("")
	}
	return ifStmt(nil, binary(x, token.EQL, zero), ret(call(f.qual(pkgFmt, "Errorf"), str(prop+" is required"))))
}

func sharedUnionValidateBody(f *goFile, variants []*model.Variant, checks []model.RequiredCheck) []ast.Stmt {
	carriers := 0
	for _, vi := range variants {
		if vi.HasSharedFields {
			carriers++
		}
	}
	if len(checks) == 0 || carriers == 0 {
		return []ast.Stmt{ret(nilExpr())}
	}
	body := []ast.Stmt{varStmt("set", id("bool"))}
	for _, vi := range variants {
		if !vi.HasSharedFields {
			continue
		}
		var then []ast.Stmt
		for _, c := range checks {
			then = append(then, requiredCheck(f, dot("v", vi.FieldName, c.Field), c.Kind, c.Prop))
		}
		then = append(then, assign(exprs(id("set")), boolLit(true)))
		body = append(body, ifStmt(nil, binary(dot("v", vi.FieldName), token.NEQ, nilExpr()), then...))
	}
	return append(body,
		ifStmt(nil, not(id("set")), ret(call(f.qual(pkgFmt, "Errorf"), str(checks[0].Prop+" is required")))),
		ret(nilExpr()),
	)
}

func unionValidateBody(f *goFile, name string, variants []*model.Variant, hasUnknown bool) []ast.Stmt {
	body := []ast.Stmt{varStmt("count", id("int"))}
	for _, vi := range variants {
		body = append(body, ifStmt(nil, binary(dot("u", vi.FieldName), token.NEQ, nilExpr()), incr(id("count"))))
	}
	if hasUnknown {
		body = append(body, ifStmt(nil, binary(dot("u", unknownField), token.NEQ, nilExpr()), incr(id("count"))))
	}
	return append(body,
		ifStmt(nil, binary(id("count"), token.NEQ, intLit(1)),
			ret(call(f.qual(pkgErrors, "New"), str(name+" must have exactly one variant set")))),
		ret(nilExpr()),
	)
}

// validateMethod is func (recv *name) Validate() error { body }.
func validateMethod(recv, name string, body ...ast.Stmt) *ast.FuncDecl {
	return method(param(recv, star(id(name))), "Validate", funcType(nil, param("", id("error"))), body...)
}

// emitFieldValidate generates Validate for an RPC object type or
// ToolCallUpdate from the checks the model lists.
func emitFieldValidate(f *goFile, t *model.Type) {
	recv := "v"
	if t.Validate == model.ValidateToolCall {
		recv = "t"
	}
	var body []ast.Stmt
	for _, c := range t.Checks {
		body = append(body, requiredCheck(f, dot(recv, c.Field), c.Kind, c.Prop))
	}
	f.add(validateMethod(recv, t.Name, append(body, ret(nilExpr()))...))
}

// catchAllOf returns a union's catch-all variant, or nil.
func catchAllOf(t *model.Type) *model.Variant {
	if t.CatchAll < 0 {
		return nil
	}
	return t.Variants[t.CatchAll]
}

func emitUnion(f *goFile, t *model.Type) {
	name := t.Name
	var variants []*model.Variant
	for _, vi := range t.Variants {
		if vi.Skip {
			continue
		}
		emitInlineVariant(f, vi.TypeName, vi.Inline)
		variants = append(variants, vi)
	}

	// Wrapper: one pointer field per variant, set by UnmarshalJSON.
	var st fieldSet
	for _, vi := range variants {
		st.comment(vi.Doc...)
		st.add(structField(vi.FieldName, star(id(vi.TypeName)), "-"))
	}
	if t.HasUnknown {
		st.comment(
			fmt.Sprintf("Unknown holds a payload whose %q names no variant this SDK knows, as it was received.", t.DiscKey),
			"MarshalJSON writes it back unchanged when no variant is set.",
		)
		st.add(structField(unknownField, f.qual(pkgJSON, "RawMessage"), "-"))
	}
	// The union's description belongs on the wrapper, after the variant types.
	f.comment(t.Doc...)
	f.add(typeDecl(name, st.structType()))
	f.line()
	f.add(method(param("u", star(id(name))), "UnmarshalJSON",
		funcType([]*ast.Field{param("b", byteSlice())}, param("", id("error"))),
		unionUnmarshalBody(f, name, variants, t.DiscKey, t.HasUnknown, catchAllOf(t))...))
	f.line()
	f.add(method(param("u", id(name)), "MarshalJSON",
		funcType(nil, param("", byteSlice()), param("", id("error"))),
		unionMarshalBody(f, name, variants, t.HasUnknown)...))
	f.line()

	// oneOf: exactly one variant. anyOf: shared required fields live on the variants.
	if t.Validate == model.ValidateExactlyOne {
		f.add(validateMethod("u", name, unionValidateBody(f, name, variants, t.HasUnknown)...))
	} else {
		f.add(validateMethod("v", name, sharedUnionValidateBody(f, variants, t.Checks)...))
	}
	f.line()
}
