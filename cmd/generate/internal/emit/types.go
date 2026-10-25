package emit

import (
	"fmt"
	"go/ast"
	"go/token"
	"math"
	"slices"
	"strconv"
	"strings"

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

// fieldList returns the fields. A doc comment with no field after it is
// dropped and recorded as an error on f.
func (s *fieldSet) fieldList(f *goFile) *ast.FieldList {
	if len(s.doc) > 0 {
		f.fail(fmt.Errorf("doc comment %q has no field after it", s.doc[0]))
		s.doc = nil
	}
	return fields(s.list...)
}

func (s *fieldSet) structType(f *goFile) *ast.StructType {
	return &ast.StructType{Fields: s.fieldList(f)}
}

func (s *fieldSet) interfaceType(f *goFile) *ast.InterfaceType {
	return &ast.InterfaceType{Methods: s.fieldList(f)}
}

// structField is name typ `json:"tag"`.
func structField(name string, typ ast.Expr, tag string) *ast.Field {
	return &ast.Field{Names: []*ast.Ident{id(name)}, Type: typ, Tag: structTag(tag)}
}

// typesFile builds types_gen.go: all types and the Agent/Client interfaces.
func typesFile(m *model.Model) *goFile {
	f := newFile()
	f.keyDecoders = keyDecoders(m)
	for _, t := range m.Types {
		emitSchemaDef(f, t)
	}
	emitRPCInterfaces(f, m)
	emitDecodeHelpers(f)
	if mi := m.Initialize; mi != nil {
		f.line()
		f.comment("Negotiation is what initialize agreed on a connection: the request the",
			"client sent and the response the agent answered",
			"(0017-MADR-remediate-acp-v1-conformance-violations.md, G1 and N1).")
		f.add(typeDecl("Negotiation", &ast.StructType{Fields: fields(param("Request", id(mi.Req)), param("Response", id(mi.Resp)))}))
		emitGates(f, m)
	}

	return f
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
	target.comment(methodNotice(mi.Doc, mi)...)
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

// emitRPCInterfaces emits each interface of the model, in its order, with the
// methods bound to it; an interface with no methods is still emitted.
func emitRPCInterfaces(f *goFile, m *model.Model) {
	sets := map[model.Binding]*fieldSet{}
	for _, in := range m.Interfaces {
		sets[in.Binding] = &fieldSet{}
	}
	for _, mi := range slices.Concat(m.AgentMethods, m.ClientMethods) {
		addRPCMethod(f, sets[mi.Binding], mi)
	}
	for _, in := range m.Interfaces {
		f.comment(withNotice(in.Doc, "", in.Deprecation)...)
		f.add(typeDecl(in.Name, sets[in.Binding].interfaceType(f)))
	}
}

// integerLit is the literal of a decimal integer, a negative one as a unary
// minus.
func integerLit(v string) ast.Expr {
	if digits, ok := strings.CutPrefix(v, "-"); ok {
		return &ast.UnaryExpr{Op: token.SUB, X: &ast.BasicLit{Kind: token.INT, Value: digits}}
	}
	return &ast.BasicLit{Kind: token.INT, Value: v}
}

// enumConsts declares <Const> name = "value" for each value of t.
func enumConsts(f *goFile, t *model.Type) {
	if len(t.Enum) == 0 {
		return
	}
	var specs []*ast.ValueSpec
	for _, v := range t.Enum {
		specs = append(specs, enumSpec(v, id(t.Name), str(v.Value), strconv.Quote(v.Value), defPlace(t)))
	}
	f.add(constDecl(true, specs...))
}

// enumSpec is <Const> typ = value, documented as the value shown, then the
// value's schema description (0019-PLAN-examples-docs-tests-tooling.md, I12),
// then its deprecation notice, naming it a variant of owner.
func enumSpec(v model.EnumValue, typ, value ast.Expr, shown, owner string) *ast.ValueSpec {
	s := constSpec(v.Const, typ, value)
	lines := []string{v.Const + " is the " + shown + " value."}
	if len(v.Doc) > 0 {
		lines = append(append(lines, ""), v.Doc...)
	}
	s.Doc = docLines(withNotice(lines, "the "+shown+" variant of "+owner, v.Deprecation)...)
	return s
}

// defPlace names the schema definition t was built from, by its own key, in
// the docs of the methods emitted for it; a variant or a nested struct is
// named by where it sits in it (0019-PLAN-examples-docs-tests-tooling.md,
// I12, and its deviation of 2026-10-17).
func defPlace(t *model.Type) string { return "the " + t.Origin.Key + " schema definition" }

// typeDoc is a generated type's doc comment: "<name> is <place>.", then the
// schema's description after a blank line, unless the description already
// starts with the name (0024-PLAN-documentation.md, A7).
func typeDoc(name, place string, doc []string) []string {
	if len(doc) > 0 && strings.HasPrefix(doc[0], name+" ") {
		return doc
	}
	lines := []string{name + " is " + place + "."}
	if len(doc) > 0 {
		lines = append(append(lines, ""), doc...)
	}
	return lines
}

// frozenNotice is the deprecation notice of a declaration the unstable
// manifest keeps: it names no replacement, since there is none
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 1).
const frozenNotice = "Deprecated: this declaration comes from the unstable ACP schema; it stays under v1 for " +
	"compatibility only (0039-MADR-five-adopter-additions-on-schema-1-25-0.md)."

// withNotice returns doc with the deprecation notice of the node at place as
// its last paragraph, after a blank line, when d marks the node; doc
// otherwise. The notice is a fixed sentence, then the protocol table's note
// when there is one (0024-PLAN-api-gate-and-deprecation.md), or for a frozen
// mark frozenNotice. doc is not changed.
func withNotice(doc []string, place string, d *model.Deprecation) []string {
	if d == nil {
		return doc
	}
	text := frozenNotice
	if !d.Frozen {
		text = "Deprecated: the ACP schema marks " + place + " deprecated."
		if d.Note != "" {
			text += " " + d.Note
		}
	}
	out := slices.Clone(doc)
	if len(out) > 0 {
		out = append(out, "")
	}
	return append(out, wrapDoc(text)...)
}

// methodNotice is withNotice for a method's doc: its notice names the
// request or notification definition whose mark it carries.
func methodNotice(doc []string, mi *model.Method) []string {
	if mi.Deprecation == nil {
		return doc
	}
	return withNotice(doc, "the "+mi.Deprecation.Definition+" schema definition", mi.Deprecation)
}

// schemaVariantPlace names variant vi of union t: by its discriminator value,
// else by its field name.
func schemaVariantPlace(t *model.Type, vi *model.Variant) string {
	which := vi.FieldName
	if vi.DiscValue != "" {
		which = strconv.Quote(vi.DiscValue)
	}
	return "the " + which + " variant of " + defPlace(t)
}

func emitSchemaDef(f *goFile, t *model.Type) {
	name := t.Name
	if t.Kind == model.KindNone {
		return
	}

	// A union emits its variant types first, so emitUnion places its
	// description; every other case starts with the type it documents.
	if t.Kind != model.KindUnion {
		f.comment(withNotice(typeDoc(name, defPlace(t), t.Doc), defPlace(t), t.Deprecation)...)
	}

	switch t.Kind {
	case model.KindEnum, model.KindStringConstUnion, model.KindOpenEnum:
		// An open enum's unknown values are still representable as strings,
		// matching the schema's extensibility.
		f.add(typeDecl(name, id("string")))
		enumConsts(f, t)
		f.line()
	case model.KindUnion:
		emitUnion(f, t)
	case model.KindObject:
		emitObjectSchema(f, t)
	case model.KindPrimitive:
		f.add(typeDecl(name, typeAST(f, *t.Underlying)))
		f.line()
	case model.KindIntegerEnum:
		f.add(typeDecl(name, typeAST(f, *t.Underlying)))
		var specs []*ast.ValueSpec
		for _, v := range t.Enum {
			specs = append(specs, enumSpec(v, id(name), integerLit(v.Value), v.Value, defPlace(t)))
		}
		f.add(constDecl(true, specs...))
		f.line()
	case model.KindEmptyObject:
		// Empty object shape: emit a concrete empty struct so methods can be defined
		// and the wire encoding is consistently {} rather than null.
		f.add(typeDecl(name, &ast.StructType{Fields: fields()}))
		f.line()
		if t.Validation != nil {
			emitValidate(f, name, defPlace(t), t.Validation)
		}
	default:
		// A definition the model gives no Go type is raw JSON, kept byte for
		// byte. An alias keeps json.RawMessage's methods; a defined type would
		// encode as base64.
		f.add(aliasDecl(name, f.qual(pkgJSON, "RawMessage")))
		f.line()
	}
}

func emitObjectSchema(f *goFile, t *model.Type) {
	// Nested struct types for inline object properties come first.
	for _, ns := range t.NestedStructs {
		place := fmt.Sprintf("the %q property of %s", ns.Key, defPlace(t))
		f.comment(withNotice(typeDoc(ns.Name, place, ns.Doc), place, ns.Deprecation)...)
		emitStruct(f, ns.Name, place, ns.Fields, nil, false, nil, "", ns.Validation)
	}
	emitStruct(f, t.Name, defPlace(t), t.Fields, t.Members, t.KeepExtra, t.KeyOrders, "", t.Validation)
}

// emitStruct emits type name struct { fields } and the methods its required
// keys, defaults, fills, flattened members, kept keys, key orders, and
// validation need; exempt is a required key decoding does not check. place
// names where the struct's schema sits, for its methods' docs.
func emitStruct(f *goFile, name, place string, fields []*model.Field, members []*model.Member, keepExtra bool, orders []*model.KeyOrder,
	exempt string, val *model.Validation) {
	var st fieldSet
	for _, fld := range fields {
		st.comment(withNotice(fld.Doc, fmt.Sprintf("the %q property of %s", fld.Key, place), fld.Deprecation)...)
		st.add(structField(fld.Name, typeAST(f, fld.Type), fld.Tag))
	}
	for _, mb := range members {
		doc := slices.Clone(mb.Doc)
		if len(doc) > 0 {
			doc = append(doc, "")
		}
		doc = append(doc, "Its keys share this struct's JSON object; exactly one member is set.")
		st.comment(withNotice(doc, "the "+mb.Field+" member of "+place, mb.Deprecation)...)
		st.add(structField(mb.Field, star(f.typeName(mb.Type)), "-"))
	}
	if keepExtra {
		st.comment("Extra holds the keys of the payload this struct does not declare, as received.",
			"MarshalJSON writes them back.")
		st.add(structField(extraField, rawMap(f.qual(pkgJSON, "RawMessage")), "-"))
	}
	for _, o := range orders {
		st.comment(o.Field+" is the order of the keys of "+o.Map+": UnmarshalJSON fills it in the order",
			"they appear, and MarshalJSON writes the keys it names first, in its order, then",
			"the others, sorted; a name with no entry is skipped. Nil keeps the sorted order.")
		st.add(structField(o.Field, sliceOf(id("string")), "-"))
	}
	f.add(typeDecl(name, st.structType(f)))
	f.line()
	emitObjectCodec(f, name, place, fields, members, keepExtra, orders, exempt)
	if val != nil {
		emitValidate(f, name, place, val)
	}
}

// typeAST renders a model type as a Go type expression, adding to f the import
// a qualified type needs. Each call builds new nodes, so no node is shared
// between two places in a file.
func typeAST(f *goFile, r model.TypeRef) ast.Expr {
	switch r.Kind {
	case model.RefPointer:
		return star(typeAST(f, *r.Elem))
	case model.RefSlice:
		return sliceOf(typeAST(f, *r.Elem))
	case model.RefMap:
		return rawMap(typeAST(f, *r.Elem))
	case model.RefRaw:
		return f.qual(pkgJSON, "RawMessage")
	case model.RefPatch:
		return index(id(typePatch), typeAST(f, *r.Elem))
	default:
		return f.typeName(r.Name)
	}
}

// keyDecoders returns the structs of m whose decoder starts with objectKeys
// and keeps no Extra, by the rule emitObjectCodec emits them by.
func keyDecoders(m *model.Model) map[string]bool {
	out := map[string]bool{}
	add := func(name string, fields []*model.Field, members []*model.Member, keepExtra bool, orders []*model.KeyOrder, exempt string) {
		if !keepExtra && decodesKeys(fields, members, keepExtra, orders, exempt) {
			out[name] = true
		}
	}
	for _, t := range m.Types {
		if t.Kind == model.KindObject {
			for _, ns := range t.NestedStructs {
				add(ns.Name, ns.Fields, nil, false, nil, "")
			}
			add(t.Name, t.Fields, t.Members, t.KeepExtra, t.KeyOrders, "")
		}
		for _, vi := range t.Variants {
			if in := vi.Inline; in != nil && in.Kind == model.InlineStruct {
				add(vi.TypeName, in.Fields, in.Members, in.KeepExtra, nil, in.Exempt)
			}
		}
	}
	return out
}

// decodesKeys reports whether a struct of these fields gets an UnmarshalJSON
// that starts with objectKeys: one with required keys, defaults, lenient or
// optional raw fields, flattened members, kept keys, or a key order.
func decodesKeys(fields []*model.Field, members []*model.Member, keepExtra bool, orders []*model.KeyOrder, exempt string) bool {
	for _, fld := range fields {
		if fld.Default != nil || fld.Facts.DefaultOnError || fld.Facts.SkipInvalidItems || fld.Patch || fld.NullIsAbsent ||
			(fld.Required && fld.Key != exempt) {
			return true
		}
	}
	return keepExtra || len(members) > 0 || len(orders) > 0
}

// emitObjectCodec synthesizes MarshalJSON for structs with fields whose
// schema defaults it injects or that are required slices or maps, an Extra
// (a catch-all's, or an object's whose additionalProperties is true), or
// flattened members, and UnmarshalJSON for structs with required keys,
// defaults, lenient fields, optional raw fields, flattened members, or an
// Extra.
func emitObjectCodec(f *goFile, name, place string, fields []*model.Field, members []*model.Member, keepExtra bool, orders []*model.KeyOrder,
	exempt string) {
	var defaults, fills, lenient, absent []*model.Field
	var required []string
	for _, fld := range fields {
		if fld.Default != nil {
			defaults = append(defaults, fld)
		}
		if fld.Facts.DefaultOnError || fld.Facts.SkipInvalidItems || fld.Patch {
			lenient = append(lenient, fld)
		}
		if fld.Fill {
			fills = append(fills, fld)
		}
		if fld.NullIsAbsent {
			absent = append(absent, fld)
		}
		if fld.Required && fld.Key != exempt {
			required = append(required, fld.Key)
		}
	}
	slices.Sort(required)
	// A MarshalJSON that would only copy into an alias and encode it is not
	// emitted (0019-PLAN-api-decisions.md, I8).
	if slices.ContainsFunc(defaults, injectsDefault) || len(fills) > 0 || keepExtra || len(members) > 0 || len(orders) > 0 {
		emitObjectMarshal(f, name, place, defaults, fills, members, keepExtra, orders)
	}
	// A default that is its type's zero value needs nothing on decode
	// (0019-PLAN-idioms-performance.md, D3); UnmarshalJSON is still emitted
	// for it, so the errors of a payload that is not an object stay as they
	// were.
	injected := slices.DeleteFunc(slices.Clone(defaults), func(fld *model.Field) bool { return fld.Default.Value.Kind == model.ValueZero })
	if decodesKeys(fields, members, keepExtra, orders, exempt) {
		emitObjectUnmarshal(f, name, place, fields, injected, lenient, absent, members, required, keepExtra, orders)
	}
}

// marshalDoc and unmarshalDoc are the docs of a codec whose receiver is recv,
// for a type whose schema sits at place.
func marshalDoc(recv, place string) string {
	return "MarshalJSON encodes " + recv + " as " + place + "'s JSON."
}

func unmarshalDoc(recv, place string) string {
	return "UnmarshalJSON decodes " + place + "'s JSON into " + recv + "."
}

// injectsDefault reports whether MarshalJSON writes fld's default into a nil
// value: a nilable array or object default. A typed object default, not
// nilable, is not injected on encode.
func injectsDefault(fld *model.Field) bool {
	dp := fld.Default
	return (dp.Kind == model.DefaultArray || dp.Kind == model.DefaultObject) && dp.Nilable
}

// emitObjectMarshal emits MarshalJSON: it fills nil slices and maps with
// their defaults, and nil required ones with empty values, before encoding,
// writes back an Extra, writes the set member's keys, and writes a map in the
// order its key order names (the model refuses an order beside an Extra or
// members).
func emitObjectMarshal(f *goFile, name, place string, defaults, fills []*model.Field, members []*model.Member, keepExtra bool,
	orders []*model.KeyOrder) {
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }
	marshal := func(x ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Marshal"), x) }

	// MarshalJSON: coerce nil slices and maps to their defaults, and nil
	// required ones to empty, before encoding.
	body := []ast.Stmt{
		typeStmt("Alias", id(name)),
		define(exprs(id("a")), call(id("Alias"), id("v"))),
	}
	for _, fld := range defaults {
		if injectsDefault(fld) {
			body = append(body, ifStmt(nil, binary(dot("a", fld.Name), token.EQL, nilExpr()),
				assign(exprs(dot("a", fld.Name)), valueExpr(f, fld.Default.Value, false)),
			))
		}
	}
	for _, fld := range fills {
		body = append(body, ifStmt(nil, binary(dot("a", fld.Name), token.EQL, nilExpr()),
			assign(exprs(dot("a", fld.Name)), composite(typeAST(f, fld.Type))),
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
	} else if len(members) > 0 {
		// Write the set member's keys into the struct's object.
		f.uses[helperMergeObject] = true
		var cases []ast.Stmt
		for _, mb := range members {
			cases = append(cases, caseOf(exprs(binary(dot("v", mb.Field), token.NEQ, nilExpr())),
				ret(call(id(helperMergeObject), id("b"), dot("v", mb.Field)))))
		}
		body = append(body,
			define(exprs(id("b"), id("err")), marshal(id("a"))),
			ifStmt(nil, errNotNil(), ret(nilExpr(), id("err"))),
			switchStmt(nil, cases...),
			ret(nilExpr(), call(f.qual(pkgErrors, "New"), str(name+": no member set"))),
		)
	} else if len(orders) > 0 {
		body = append(body, orderedMarshal(f, orders)...)
	} else {
		body = append(body, ret(marshal(id("a"))))
	}
	f.comment(marshalDoc("v", place))
	f.add(method(param("v", id(name)), "MarshalJSON",
		funcType(nil, param("", byteSlice()), param("", id("error"))), body...))
	f.line()
}

// emitObjectUnmarshal emits UnmarshalJSON, which decodes b's keys with
// objectKeys and hands them to decodeKeys. decodeKeys requires the required
// keys, decodes the fields, decodes each lenient field from its own raw
// value, empties each optional raw field that holds null, applies defaults
// when a field is missing, or null when the schema does not allow null, and
// keeps a catch-all's undeclared keys. A union that holds the keys already
// calls decodeKeys itself (0019-PLAN-idioms-performance.md, D2).
func emitObjectUnmarshal(f *goFile, name, place string, fields, defaults, lenient, absent []*model.Field, members []*model.Member, required []string,
	keepExtra bool, orders []*model.KeyOrder) {
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }
	f.uses[helperObjectKeys] = true
	m := "m"
	if len(required) == 0 && len(defaults) == 0 && !keepExtra && len(members) == 0 && len(orders) == 0 {
		m = "_"
	}
	// The keys read beyond the required ones, which requireKeys folds.
	var more []string
	for _, fld := range defaults {
		more = append(more, fld.Key)
	}
	for _, mb := range members {
		more = append(more, mb.Required...)
	}
	for _, o := range orders {
		more = append(more, o.Key)
	}
	if keepExtra {
		for _, fld := range fields {
			more = append(more, fld.Key)
		}
	}
	reads := foldNames(f, name, append(slices.Clone(required), more...))
	var body []ast.Stmt
	if len(required) > 0 {
		f.uses[helperRequireKeys] = true
		args := []ast.Expr{id("b"), id("m"), str(name)}
		for _, k := range required {
			args = append(args, str(k))
		}
		body = append(body, ifStmt(define(exprs(id("err")), call(id(helperRequireKeys), args...)), errNotNil(), ret(id("err"))))
	}
	if fold := foldCall(f, "b", "m", slices.DeleteFunc(reads, func(k string) bool { return slices.Contains(required, k) })); fold != nil {
		body = append(body, fold)
	}
	body = append(body, typeStmt("Alias", id(name)))
	wrap := func() ast.Stmt { return ret(call(f.qual(pkgFmt, "Errorf"), str(name+": %w"), id("err"))) }
	if len(lenient) == 0 {
		body = append(body,
			varStmt("a", id("Alias")),
			ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("a")))), errNotNil(), wrap()),
		)
	} else {
		body = append(body, lenientDecode(f, name, lenient, wrap)...)
	}
	for _, fld := range absent {
		f.uses[helperAbsentIfNull] = true
		body = append(body, assign(exprs(dot("a", fld.Name)), call(id(helperAbsentIfNull), dot("a", fld.Name))))
	}
	if len(members) > 0 {
		body = append(body, memberDecode(f, name, members))
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
			ifStmt(nil, cond, assign(exprs(dot("a", fld.Name)), valueExpr(f, fld.Default.Value, false))),
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
	for _, o := range orders {
		// The order the keys of the map's object appear in.
		f.uses[helperKeyOrder] = true
		body = append(body, assign(exprs(dot("a", o.Field)), call(id(helperKeyOrder), index(id("m"), str(o.Key)))))
	}
	body = append(body,
		assign(exprs(star(id("v"))), call(id(name), id("a"))),
		ret(nilExpr()),
	)
	f.comment(unmarshalDoc("v", place))
	f.add(method(param("v", star(id(name))), "UnmarshalJSON",
		funcType([]*ast.Field{param("b", byteSlice())}, param("", id("error"))),
		define(exprs(id("m"), id("err")), call(id(helperObjectKeys), id("b"), str(name))),
		ifStmt(nil, errNotNil(), ret(id("err"))),
		ret(call(dot("v", "decodeKeys"), id("b"), id("m"))),
	))
	f.line()
	f.comment("decodeKeys decodes b, a JSON object whose keys m holds, into v, as UnmarshalJSON",
		"does. It may fold the keys of m.")
	f.add(method(param("v", star(id(name))), "decodeKeys",
		funcType([]*ast.Field{param("b", byteSlice()), param(m, rawMap(f.qual(pkgJSON, "RawMessage")))}, param("", id("error"))), body...))
	f.line()
}

// orderedMarshal ends a MarshalJSON whose struct keeps key orders: with no
// order set it encodes the alias a as before; otherwise it writes each map
// whose key the encoding holds in its order, through orderedObject
// (0022-MADR-keep-elicitation-property-order-before-v1-15-0.md).
func orderedMarshal(f *goFile, orders []*model.KeyOrder) []ast.Stmt {
	f.uses[helperOrderedObject] = true
	marshal := func(x ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Marshal"), x) }
	var unset ast.Expr
	for _, o := range orders {
		c := binary(call(id("len"), dot("v", o.Field)), token.EQL, intLit(0))
		if unset == nil {
			unset = c
		} else {
			unset = binary(unset, token.LAND, c)
		}
	}
	body := []ast.Stmt{
		ifStmt(nil, unset, ret(marshal(id("a")))),
		define(exprs(id("b"), id("err")), marshal(id("a"))),
		ifStmt(nil, errNotNil(), ret(nilExpr(), id("err"))),
		varStmt("m", rawMap(f.qual(pkgJSON, "RawMessage"))),
		ifStmt(define(exprs(id("err")), call(f.qual(pkgJSON, "Unmarshal"), id("b"), addr(id("m")))), errNotNil(), ret(nilExpr(), id("err"))),
	}
	for _, o := range orders {
		body = append(body, ifStmt(define(exprs(id("_"), id("ok")), index(id("m"), str(o.Key))), id("ok"),
			define(exprs(id("x"), id("err")), call(id(helperOrderedObject), dot("a", o.Map), dot("v", o.Field))),
			ifStmt(nil, errNotNil(), ret(nilExpr(), id("err"))),
			assign(exprs(index(id("m"), str(o.Key))), id("x")),
		))
	}
	return append(body, ret(marshal(id("m"))))
}

// memberDecode decodes b into the first member, in schema order, whose
// required keys m holds, and refuses a payload no member fits.
func memberDecode(f *goFile, name string, members []*model.Member) ast.Stmt {
	var cases []ast.Stmt
	for _, mb := range members {
		var cond ast.Expr = boolLit(true)
		for i, k := range mb.Required {
			has := binary(index(id("m"), str(k)), token.NEQ, nilExpr())
			if i == 0 {
				cond = has
			} else {
				cond = binary(cond, token.LAND, has)
			}
		}
		cases = append(cases, caseOf(exprs(cond),
			varStmt("x", f.typeName(mb.Type)),
			ifStmt(define(exprs(id("err")), call(f.qual(pkgJSON, "Unmarshal"), id("b"), addr(id("x")))), errNotNil(),
				ret(call(f.qual(pkgFmt, "Errorf"), str(name+": "+mb.Field+": %w"), id("err")))),
			assign(exprs(dot("a", mb.Field)), addr(id("x"))),
		))
	}
	cases = append(cases, caseOf(nil, ret(call(f.qual(pkgErrors, "New"), str(name+": the payload fits no member")))))
	return switchStmt(nil, cases...)
}

// lenientDecode decodes b into an anonymous struct that embeds Alias and
// shadows each lenient field with its raw value, so a malformed value cannot
// fail the struct, then decodes each lenient field from its raw value:
// x-deserialize-default-on-error falls back to the schema default or the zero
// value, x-deserialize-skip-invalid-items drops the items that fail. It leaves
// the decoded fields in a.
func lenientDecode(f *goFile, name string, lenient []*model.Field, wrap func() ast.Stmt) []ast.Stmt {
	shadow := []*ast.Field{{Type: id("Alias")}}
	for _, fld := range lenient {
		shadow = append(shadow, structField(fld.Name, f.qual(pkgJSON, "RawMessage"), fld.Key))
	}
	body := []ast.Stmt{
		varStmt("s", &ast.StructType{Fields: fields(shadow...)}),
		ifStmt(define(exprs(id("err")), call(f.qual(pkgJSON, "Unmarshal"), id("b"), addr(id("s")))), errNotNil(), wrap()),
		define(exprs(id("a")), dot("s", "Alias")),
	}
	for _, fld := range lenient {
		// def is the default's Go value, nil for none or a zero one.
		var def ast.Expr
		if fld.Default != nil && fld.Default.Value.Kind != model.ValueZero {
			def = valueExpr(f, fld.Default.Value, false)
		}
		raw := dot("s", fld.Name)
		field := dot("a", fld.Name)
		if fld.Patch {
			// Absent, null, or a value (0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md).
			if elem := *fld.Type.Elem; fld.Facts.SkipInvalidItems && elem.Kind == model.RefSlice {
				f.uses[helperPatchItems] = true
				body = append(body, assign(exprs(field), call(index(id(helperPatchItems), typeAST(f, *elem.Elem)), raw)))
			} else {
				f.uses[helperPatchOf] = true
				body = append(body, assign(exprs(field), call(index(id(helperPatchOf), typeAST(f, elem)), raw)))
			}
			continue
		}
		if !fld.Facts.SkipInvalidItems {
			if def == nil {
				f.uses[helperOrZero] = true
				body = append(body, assign(exprs(field), call(index(id(helperOrZero), typeAST(f, fld.Type)), raw)))
			} else {
				f.uses[helperOrDefault] = true
				body = append(body, assign(exprs(field), call(index(id(helperOrDefault), typeAST(f, fld.Type)), raw, def)))
			}
			continue
		}
		f.uses[helperSkipInvalid] = true
		items := call(index(id(helperSkipInvalid), typeAST(f, *fld.Type.Elem)), raw)
		if !fld.Facts.DefaultOnError {
			body = append(body, block(
				define(exprs(id("items"), id("err")), items),
				ifStmt(nil, errNotNil(), ret(call(f.qual(pkgFmt, "Errorf"), str(name+": %q: %w"), str(fld.Key), id("err")))),
				assign(exprs(field), id("items")),
			))
			continue
		}
		var fallback []ast.Stmt
		if def != nil {
			fallback = []ast.Stmt{assign(exprs(field), def)}
		}
		then := []ast.Stmt{assign(exprs(field), id("items"))}
		if fallback == nil {
			body = append(body, ifStmt(define(exprs(id("items"), id("err")), items), binary(id("err"), token.EQL, nilExpr()), then...))
		} else {
			body = append(body, ifElse(define(exprs(id("items"), id("err")), items), binary(id("err"), token.EQL, nilExpr()), then, fallback))
		}
	}
	return body
}

// emitInlineVariant emits the type the model gives a union variant without a
// $ref, if any.
func emitInlineVariant(f *goFile, tname, place string, in *model.Inline, dep *model.Deprecation) {
	if in == nil {
		return
	}
	f.comment(withNotice(typeDoc(tname, place, in.Doc), place, dep)...)
	switch in.Kind {
	case model.InlineStruct:
		emitStruct(f, tname, place, in.Fields, in.Members, in.KeepExtra, nil, in.Exempt, in.Validation)
	case model.InlineNamed:
		// Non-object title-only variants can still carry payloads (e.g. arrays).
		f.add(typeDecl(tname, typeAST(f, in.Type)))
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
	return decodeChosen(f, vi, where, false)
}

// decodeChosen is decodeStrict for a union whose map of b's keys, m, is in
// scope when withKeys is set: a variant with a decodeKeys decodes from it,
// so b is read into a map once (0019-PLAN-idioms-performance.md, D2).
func decodeChosen(f *goFile, vi *model.Variant, where string, withKeys bool) []ast.Stmt {
	decode := call(f.qual(pkgJSON, "Unmarshal"), id("b"), addr(id("v")))
	if withKeys && f.keyDecoders[vi.TypeName] {
		decode = call(dot("v", "decodeKeys"), id("b"), id("m"))
	}
	return append([]ast.Stmt{
		ifStmt(define(exprs(id("err")), decode), errNotNil(),
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

// unionStart is the head of every union UnmarshalJSON: JSON null leaves the
// union as it was, unless a variant is null; then the union is reset.
func unionStart(name string, variants []*model.Variant) []ast.Stmt {
	isNull := binary(call(id("string"), id("b")), token.EQL, str(schemaTypeNull))
	var null *model.Variant
	for _, vi := range variants {
		if vi.IsNull {
			null = vi
			break
		}
	}
	var body []ast.Stmt
	if null == nil {
		body = append(body, ifStmt(nil, isNull, ret(nilExpr())))
	}
	body = append(body, assign(exprs(star(id("u"))), composite(id(name))))
	if null != nil {
		body = append(body, ifStmt(nil, isNull, append([]ast.Stmt{varStmt("v", id(null.TypeName))}, setVariant(null)...)...))
	}
	return body
}

// missingDisc is the error for a payload without its discriminator that the
// union cannot place.
func missingDisc(f *goFile, name, discKey string) ast.Stmt {
	return ret(call(f.qual(pkgErrors, "New"), str(name+": missing discriminator "+strconv.Quote(discKey))))
}

func unionUnmarshalBody(f *goFile, name string, variants []*model.Variant, discKey string, hasUnknown bool, catchAll *model.Variant) []ast.Stmt {
	body := unionStart(name, variants)
	if discKey != "" {
		return append(body, discUnmarshalBody(f, name, variants, discKey, hasUnknown, catchAll)...)
	}
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }

	// Required-key match, on keys folded as encoding/json matches names.
	var reads []string
	for _, vi := range variants {
		if vi.IsObject {
			reads = append(reads, vi.Required...)
		}
	}
	var object []ast.Stmt
	if fold := foldCall(f, "b", "m", foldNames(f, name, reads)); fold != nil {
		object = append(object, fold)
	}
	for _, vi := range variants {
		if !vi.IsObject || len(vi.Required) == 0 {
			continue
		}
		stmts := []ast.Stmt{
			varStmt("v", f.typeName(vi.TypeName)),
			define(exprs(id("match")), boolLit(true)),
		}
		for _, rk := range vi.Required {
			stmts = append(stmts, keyMatch("m", rk))
		}
		stmts = append(stmts, ifStmt(nil, id("match"), decodeChosen(f, vi, variantPlace(name, vi), true)...))
		object = append(object, block(stmts...))
	}
	body = append(body,
		varStmt("m", rawMap(f.qual(pkgJSON, "RawMessage"))),
		elseIf(ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("m")))), binary(id("err"), token.EQL, nilExpr()),
			object...),
			// Not an object (e.g., primitive union variant) or invalid JSON.
			ifStmt(define(exprs(id("_"), id("ok")),
				call(index(f.qual(pkgErrors, "AsType"), star(f.qual(pkgJSON, "UnmarshalTypeError"))), id("err"))), not(id("ok")),
				ret(id("err")))),
	)
	return append(body, arrayMatch(f, name, variants)...)
}

// arrayMatch decodes an array variant whose items carry its required keys,
// each item's keys folded as encoding/json matches names, then tries each
// variant in turn.
func arrayMatch(f *goFile, name string, variants []*model.Variant) []ast.Stmt {
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }
	var body, array []ast.Stmt
	for _, vi := range variants {
		if !vi.IsArray || len(vi.ArrayItemRequired) == 0 {
			continue
		}
		loop := []ast.Stmt{
			varStmt("elem", rawMap(f.qual(pkgJSON, "RawMessage"))),
			ifStmt(nil, binary(unmarshal(id("item"), addr(id("elem"))), token.NEQ, nilExpr()),
				assign(exprs(id("match")), boolLit(false)), breakStmt()),
			foldCall(f, "item", "elem", foldNames(f, name, vi.ArrayItemRequired)),
		}
		for _, rk := range vi.ArrayItemRequired {
			loop = append(loop, keyMatch("elem", rk))
		}
		array = append(array, block(
			varStmt("v", f.typeName(vi.TypeName)),
			define(exprs(id("match")), boolLit(true)),
			rangeStmt(id("_"), id("item"), id("arr"), loop...),
			ifStmt(nil, id("match"), decodeStrict(f, vi, variantPlace(name, vi))...),
		))
	}
	// Without an array variant to match, nothing reads the payload as an
	// array (0019-PLAN-idioms-performance.md, D4).
	if len(array) > 0 {
		body = append(body,
			varStmt("arr", sliceOf(f.qual(pkgJSON, "RawMessage"))),
			ifStmt(nil, binary(unmarshal(id("b"), addr(id("arr"))), token.EQL, nilExpr()), array...),
		)
	}

	// Fallback: try each variant in turn.
	for _, vi := range variants {
		body = append(body, block(
			varStmt("v", f.typeName(vi.TypeName)),
			ifStmt(nil, binary(unmarshal(id("b"), addr(id("v"))), token.EQL, nilExpr()), setVariant(vi)...),
		))
	}
	return append(body, ret(call(f.qual(pkgErrors, "New"), str(name+": no variant matches"))))
}

// discUnmarshalBody decodes a discriminated union, whose variants are all
// objects: by the discriminator's value when it is present, else by the rule
// for a payload without it.
func discUnmarshalBody(f *goFile, name string, variants []*model.Variant, discKey string, hasUnknown bool, catchAll *model.Variant) []ast.Stmt {
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }
	var cases []ast.Stmt
	for _, vi := range variants {
		if vi.DiscValue != "" {
			cases = append(cases, caseOf(exprs(str(vi.DiscValue)),
				append([]ast.Stmt{varStmt("v", f.typeName(vi.TypeName))},
					decodeChosen(f, vi, name+": "+discKey+" "+strconv.Quote(vi.DiscValue), true)...)...))
		}
	}
	reads := []string{discKey}
	for _, vi := range variants {
		reads = append(reads, vi.FitKeys...)
	}
	body := []ast.Stmt{
		varStmt("m", rawMap(f.qual(pkgJSON, "RawMessage"))),
		ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("m")))), errNotNil(),
			ret(call(f.qual(pkgFmt, "Errorf"), str(name+": %w"), id("err")))),
		foldCall(f, "b", "m", foldNames(f, name, reads)),
		// A discriminator that is present and not a string is an error.
		varStmt("disc", id("string")),
		ifStmt(define(exprs(id("v"), id("ok")), index(id("m"), str(discKey))), id("ok"),
			ifStmt(define(exprs(id("err")), unmarshal(id("v"), addr(id("disc")))), errNotNil(),
				ret(call(f.qual(pkgFmt, "Errorf"), str(name+": discriminator "+strconv.Quote(discKey)+": %w"), id("err"))))),
		switchOrIf(id("disc"), cases...),
	}
	present := func(v string) ast.Expr {
		return binary(id("ok"), token.LAND, binary(call(id("string"), id(v)), token.NEQ, str(schemaTypeNull)))
	}
	if catchAll != nil {
		// A present value no case took belongs to the schema's catch-all.
		body = append(body, ifStmt(define(exprs(id("raw"), id("ok")), index(id("m"), str(discKey))), present("raw"),
			append([]ast.Stmt{
				varStmt("v", id(catchAll.TypeName)),
				ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("v")))), errNotNil(),
					ret(call(f.qual(pkgFmt, "Errorf"), str(name+": "+discKey+" %q: %w"), id("disc"), id("err")))),
			}, setVariant(catchAll)...)...,
		))
	}
	if hasUnknown {
		// A present value no case took is kept as received.
		body = append(body, ifStmt(define(exprs(id("v"), id("ok")), index(id("m"), str(discKey))), present("v"),
			assign(exprs(dot("u", unknownField)), call(f.qual(pkgBytes, "Clone"), id("b"))),
			ret(nilExpr()),
		))
	}

	// The discriminator is absent or null.
	var keyLess []*model.Variant
	for _, vi := range variants {
		if vi.KeyLess {
			keyLess = append(keyLess, vi)
		}
	}
	if len(keyLess) > 0 {
		return append(body, keyLessBody(f, name, keyLess, discKey)...)
	}
	return append(body, fitBody(f, name, variants, discKey, hasUnknown, catchAll)...)
}

// keyLessBody decodes the variants the schema means when the discriminator is
// absent: each by its required keys, then each by a trial decode, in schema
// order. A variant without required keys takes every payload that reaches it.
func keyLessBody(f *goFile, name string, keyLess []*model.Variant, discKey string) []ast.Stmt {
	var body []ast.Stmt
	for _, vi := range keyLess {
		if len(vi.FitKeys) == 0 {
			return append(append(body, varStmt("v", f.typeName(vi.TypeName))), decodeChosen(f, vi, variantPlace(name, vi), true)...)
		}
		stmts := []ast.Stmt{
			varStmt("v", f.typeName(vi.TypeName)),
			define(exprs(id("match")), boolLit(true)),
		}
		for _, k := range vi.FitKeys {
			stmts = append(stmts, keyMatch("m", k))
		}
		body = append(body, block(append(stmts, ifStmt(nil, id("match"), decodeChosen(f, vi, variantPlace(name, vi), true)...))...))
	}
	for _, vi := range keyLess {
		body = append(body, block(
			varStmt("v", f.typeName(vi.TypeName)),
			ifStmt(nil, binary(call(f.qual(pkgJSON, "Unmarshal"), id("b"), addr(id("v"))), token.EQL, nilExpr()), setVariant(vi)...),
		))
	}
	return append(body, missingDisc(f, name, discKey))
}

// fitBody decodes the one variant, the catch-all aside, whose required keys
// other than the discriminator are all in the payload. When none or several
// fit, the payload is kept in Unknown, or refused in a union with a catch-all.
func fitBody(f *goFile, name string, variants []*model.Variant, discKey string, hasUnknown bool, catchAll *model.Variant) []ast.Stmt {
	// The variants without keys to match always fit: they start the list.
	var always []ast.Expr
	var matches, cases []ast.Stmt
	for _, vi := range variants {
		if vi == catchAll {
			continue
		}
		if len(vi.FitKeys) == 0 {
			always = append(always, str(vi.FieldName))
		} else {
			stmts := []ast.Stmt{define(exprs(id("match")), boolLit(true))}
			for _, k := range vi.FitKeys {
				stmts = append(stmts, keyMatch("m", k))
			}
			add := assign(exprs(id("fit")), call(id("append"), id("fit"), str(vi.FieldName)))
			matches = append(matches, block(append(stmts, ifStmt(nil, id("match"), add))...))
		}
		cases = append(cases, caseOf(exprs(str(vi.FieldName)),
			append([]ast.Stmt{varStmt("v", f.typeName(vi.TypeName))}, decodeChosen(f, vi, variantPlace(name, vi), true)...)...))
	}
	body := []ast.Stmt{varStmt("fit", sliceOf(id("string")))}
	if len(always) > 0 {
		body = []ast.Stmt{define(exprs(id("fit")), composite(sliceOf(id("string")), always...))}
	}
	body = append(append(body, matches...), ifStmt(nil, binary(call(id("len"), id("fit")), token.EQL, intLit(1)),
		switchOrIf(index(id("fit"), intLit(0)), cases...)))
	if hasUnknown {
		return append(body,
			assign(exprs(dot("u", unknownField)), call(f.qual(pkgBytes, "Clone"), id("b"))),
			ret(nilExpr()),
		)
	}
	return append(body, missingDisc(f, name, discKey))
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
			then = nil
			if vi.IsArray {
				// A nil array is sent as [], as a required array field is
				// (0014-MADR-union-constructors-take-every-required-key.md, item 7).
				then = append(then, ifStmt(nil, binary(star(dot("u", vi.FieldName)), token.EQL, nilExpr()), ret(bytesOf("[]"), nilExpr())))
			}
			then = append(then, ret(marshal(star(dot("u", vi.FieldName)))))
		case len(vi.ConstPairs) == 0:
			// An object with no constant to set encodes as it is
			// (0019-PLAN-idioms-performance.md, D1).
			then = []ast.Stmt{ret(marshal(star(dot("u", vi.FieldName))))}
		case vi.ConstFields != nil:
			// The constants are set on a copy, which encodes once.
			then = []ast.Stmt{define(exprs(id("v")), star(dot("u", vi.FieldName)))}
			for i, kv := range vi.ConstPairs {
				then = append(then, assign(exprs(dot("v", vi.ConstFields[i])), str(kv[1])))
			}
			then = append(then, ret(marshal(id("v"))))
		default:
			// A constant without a string field to hold it is set on the
			// encoded object.
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

// unionValidateBody counts the variants set, Unknown included, requires
// exactly one, refuses a catch-all holding a value its not excludes, and
// validates the set variant when its type has Validate.
func unionValidateBody(f *goFile, t *model.Type, variants []*model.Variant) []ast.Stmt {
	name, hasUnknown := t.Name, t.HasUnknown
	body := []ast.Stmt{varStmt("count", id("int"))}
	for _, vi := range variants {
		body = append(body, ifStmt(nil, binary(dot("u", vi.FieldName), token.NEQ, nilExpr()), incr(id("count"))))
	}
	if hasUnknown {
		body = append(body, ifStmt(nil, binary(dot("u", unknownField), token.NEQ, nilExpr()), incr(id("count"))))
	}
	body = append(body, ifStmt(nil, binary(id("count"), token.NEQ, intLit(1)),
		ret(validationError(f, name, nil, nil, "RuleOneVariant", str(name+" must have exactly one variant set")))))
	if ca := catchAllOf(t); ca != nil && len(ca.Excluded) > 0 {
		// The schema's not excludes these values from the catch-all; the
		// peer would read them as another variant
		// (0014-MADR-union-constructors-take-every-required-key.md, C1).
		value := dot("u", ca.FieldName, t.DiscField)
		var list []ast.Expr
		for _, x := range ca.Excluded {
			list = append(list, str(x))
		}
		body = append(body, ifStmt(nil, binary(dot("u", ca.FieldName), token.NEQ, nilExpr()),
			switchOrIf(value, caseOf(list,
				ret(validationError(f, name, str(t.DiscKey), value, "RuleExcludedValue",
					call(f.qual(pkgFmt, "Sprintf"), str(name+": variant "+ca.FieldName+": "+t.DiscKey+" %q names another variant"), value)))))))
	}
	for _, vi := range variants {
		if !vi.Validates {
			continue
		}
		body = append(body, ifStmt(nil, binary(dot("u", vi.FieldName), token.NEQ, nilExpr()),
			validateCall(f, vi.TypeName, dot("u", vi.FieldName), name+": variant "+vi.FieldName+": %w")))
	}
	return append(body, ret(nilExpr()))
}

// addValidate adds Validate, which checks with the receiving host's rule for
// absolute paths, and validate, which takes the rule and holds the body, so
// the SDK can check what it sends with the portable rule
// (0015-MADR-validate-absolute-protocol-paths.md, A4). place names where
// the type's schema sits, for Validate's doc.
func addValidate(f *goFile, recv, name, place string, body ...ast.Stmt) {
	f.comment("Validate returns an error if " + recv + " does not satisfy " + place + ".")
	f.add(method(param(recv, star(id(name))), "Validate", funcType(nil, param("", id("error"))),
		ret(call(sel(id(recv), "validate"), f.qual("path/filepath", "IsAbs")))))
	f.line()
	abs := funcType([]*ast.Field{param("", id("string"))}, param("", id("bool")))
	f.add(method(param(recv, star(id(name))), "validate", funcType([]*ast.Field{param("abs", abs)}, param("", id("error"))), body...))
}

// validateCall is if err := x.validate(abs); err != nil { return fmt.Errorf(format, args..., err) }, for x
// of the type named typeName. From package unstable, a type of acp's is
// checked through its exported Validate, by the host's rule
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, question 9).
func validateCall(f *goFile, typeName string, x ast.Expr, format string, args ...ast.Expr) ast.Stmt {
	check := call(sel(x, "validate"), id("abs"))
	if f.external(typeName) {
		check = call(sel(x, "Validate"))
	}
	return ifStmt(define(exprs(id("err")), check), errNotNil(),
		ret(call(f.qual(pkgFmt, "Errorf"), append(append([]ast.Expr{str(format)}, args...), id("err"))...)))
}

// emitValidate emits Validate for a struct from the checks of val, in field
// order (0012-MADR-schema-fidelity-framework.md).
func emitValidate(f *goFile, name, place string, val *model.Validation) {
	var body []ast.Stmt
	for _, c := range val.Checks {
		body = append(body, checkStmt(f, name, c))
	}
	if len(val.Members) > 0 {
		body = append(body, varStmt("count", id("int")))
		for _, mb := range val.Members {
			body = append(body, ifStmt(nil, binary(dot("v", mb.Field), token.NEQ, nilExpr()), incr(id("count"))))
		}
		body = append(body, ifStmt(nil, binary(id("count"), token.NEQ, intLit(1)),
			ret(validationError(f, name, nil, nil, "RuleOneMember", str(name+" must have exactly one member set")))))
		for _, mb := range val.Members {
			if mb.Validates {
				body = append(body, ifStmt(nil, binary(dot("v", mb.Field), token.NEQ, nilExpr()),
					validateCall(f, mb.Type, dot("v", mb.Field), name+": member "+mb.Field+": %w")))
			}
		}
	}
	addValidate(f, "v", name, place, append(body, ret(nilExpr()))...)
	f.line()
}

// checkStmt renders one check of a struct's Validate.
func checkStmt(f *goFile, name string, c model.Check) ast.Stmt {
	field := dot("v", c.Field)
	where := name + ": " + c.Key
	if c.Kind == model.CheckAbsolutePath {
		return absPathStmt(f, name, c)
	}
	if c.Kind == model.CheckNested {
		if c.InPatch {
			// A patch's value is checked when it holds one.
			return ifStmt(define(exprs(id("pv"), id("ok")), call(sel(field, "Value"))), id("ok"),
				nestedStmt(f, c.Type, func() ast.Expr { return id("pv") }, c.Shape, where))
		}
		return nestedStmt(f, c.Type, func() ast.Expr { return dot("v", c.Field) }, c.Shape, where)
	}
	value := field
	if c.Shape == model.RefPointer {
		value = star(dot("v", c.Field))
	}
	bound := boundLit(c)
	var cond ast.Expr
	var msg, rule string
	switch c.Kind {
	case model.CheckMinLength:
		s := value
		if c.NamedString {
			s = call(id("string"), value)
		}
		cond = binary(call(f.qual("unicode/utf8", "RuneCountInString"), s), token.LSS, bound)
		msg, rule = where+": shorter than minLength "+bound.Value, "RuleMinLength"
	case model.CheckMinimum:
		cond = binary(value, token.LSS, bound)
		msg, rule = where+": less than minimum "+bound.Value, "RuleMinimum"
	default:
		cond = binary(value, token.GTR, bound)
		msg, rule = where+": greater than maximum "+bound.Value, "RuleMaximum"
	}
	if c.Shape == model.RefPointer {
		cond = binary(binary(dot("v", c.Field), token.NEQ, nilExpr()), token.LAND, cond)
	}
	return ifStmt(nil, cond, ret(validationError(f, name, str(c.Key), value, rule, str(msg))))
}

// nestedStmt calls the Validate of what x holds, by its shape: a value, a
// pointer when set, each element of a slice, or each value of a map in key
// order.
func nestedStmt(f *goFile, typeName string, x func() ast.Expr, shape model.RefKind, where string) ast.Stmt {
	switch shape {
	case model.RefPointer:
		return ifStmt(nil, binary(x(), token.NEQ, nilExpr()), validateCall(f, typeName, x(), where+": %w"))
	case model.RefSlice:
		return rangeStmt(id("i"), nil, x(), validateCall(f, typeName, index(x(), id("i")), where+"[%d]: %w", id("i")))
	case model.RefMap:
		keys := call(f.qual("slices", "Sorted"), call(f.qual("maps", "Keys"), x()))
		return rangeStmt(id("_"), id("k"), keys,
			define(exprs(id("x")), index(x(), id("k"))),
			validateCall(f, typeName, id("x"), where+"[%q]: %w", id("k")))
	default:
		return validateCall(f, typeName, x(), where+": %w")
	}
}

// absPathStmt checks a path the protocol requires to be absolute with the
// rule abs, by the field's shape: a string, a pointer to one when set, or
// each element of a slice, named by its index
// (0015-MADR-validate-absolute-protocol-paths.md, item 2).
func absPathStmt(f *goFile, name string, c model.Check) ast.Stmt {
	sprintf := func(format string, args ...ast.Expr) ast.Expr {
		return call(f.qual(pkgFmt, "Sprintf"), append([]ast.Expr{str(format)}, args...)...)
	}
	where := name + ": " + c.Key
	fail := func(value ast.Expr) ast.Stmt {
		return ret(validationError(f, name, str(c.Key), value, "RuleAbsolutePath", sprintf(where+": %q is not an absolute path", value)))
	}
	field := dot("v", c.Field)
	switch c.Shape {
	case model.RefPointer:
		return ifStmt(nil, binary(binary(field, token.NEQ, nilExpr()), token.LAND, not(call(id("abs"), star(field)))), fail(star(field)))
	case model.RefSlice:
		elem := sprintf(c.Key+"[%d]", id("i"))
		return rangeStmt(id("i"), id("x"), field, ifStmt(nil, not(call(id("abs"), id("x"))),
			ret(validationError(f, name, elem, id("x"), "RuleAbsolutePath", sprintf(where+"[%d]: %q is not an absolute path", id("i"), id("x"))))))
	default:
		return ifStmt(nil, not(call(id("abs"), field)), fail(field))
	}
}

// validationError is &ValidationError{...} for a failed check: the Go type,
// the JSON key (empty for a check of the whole value), the offending value
// (nil for a count), the rule, and the message, which is the text Validate
// returned before the type existed
// (0015-PLAN-validate-absolute-protocol-paths.md, step 3).
func validationError(f *goFile, typeName string, field, value ast.Expr, rule string, msg ast.Expr) ast.Expr {
	elts := []ast.Expr{kv(id("Type"), str(typeName))}
	if field != nil {
		elts = append(elts, kv(id("Field"), field))
	}
	if value != nil {
		elts = append(elts, kv(id("Value"), value))
	}
	elts = append(elts, kv(id("Rule"), f.runtimeName(rule)))
	if f.local == nil {
		// Only acp's own code can set the message; a ValidationError of
		// package unstable reads as its fallback, "Type: Field: Rule"
		// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, question 9).
		elts = append(elts, kv(id("msg"), msg))
	}
	return addr(composite(f.runtimeName("ValidationError"), elts...))
}

// boundLit is a check's bound as a literal: an integer, or a float written
// without a needless fraction. An integer bound above math.MaxInt64, which
// only a uint64 holds, is written as one (0019-PLAN-generator-hardening.md,
// G21).
func boundLit(c model.Check) *ast.BasicLit {
	if c.Integer && c.Bound >= math.MaxInt64 {
		return &ast.BasicLit{Kind: token.INT, Value: strconv.FormatUint(uint64(c.Bound), 10)}
	}
	if c.Integer {
		return &ast.BasicLit{Kind: token.INT, Value: strconv.FormatInt(int64(c.Bound), 10)}
	}
	return &ast.BasicLit{Kind: token.FLOAT, Value: strconv.FormatFloat(c.Bound, 'g', -1, 64)}
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
		emitInlineVariant(f, vi.TypeName, schemaVariantPlace(t, vi), vi.Inline, vi.Deprecation)
		variants = append(variants, vi)
	}

	// Wrapper: one pointer field per variant, set by UnmarshalJSON.
	var st fieldSet
	for _, vi := range variants {
		st.comment(withNotice(vi.Doc, schemaVariantPlace(t, vi), vi.Deprecation)...)
		st.add(structField(vi.FieldName, star(f.typeName(vi.TypeName)), "-"))
	}
	if t.HasUnknown {
		st.comment(
			fmt.Sprintf("Unknown holds a payload whose %q names no variant this SDK knows, as it was received.", t.DiscKey),
			"MarshalJSON writes it back unchanged when no variant is set.",
		)
		st.add(structField(unknownField, f.qual(pkgJSON, "RawMessage"), "-"))
	}
	// The union's description belongs on the wrapper, after the variant types.
	f.comment(withNotice(typeDoc(name, defPlace(t), t.Doc), defPlace(t), t.Deprecation)...)
	f.add(typeDecl(name, st.structType(f)))
	f.line()
	f.comment(unmarshalDoc("u", defPlace(t)))
	f.add(method(param("u", star(id(name))), "UnmarshalJSON",
		funcType([]*ast.Field{param("b", byteSlice())}, param("", id("error"))),
		unionUnmarshalBody(f, name, variants, t.DiscKey, t.HasUnknown, catchAllOf(t))...))
	f.line()
	f.comment(marshalDoc("u", defPlace(t)))
	f.add(method(param("u", id(name)), "MarshalJSON",
		funcType(nil, param("", byteSlice()), param("", id("error"))),
		unionMarshalBody(f, name, variants, t.HasUnknown)...))
	f.line()

	// Every union has exactly one variant set, which validates when it can.
	addValidate(f, "u", name, defPlace(t), unionValidateBody(f, t, variants)...)
	f.line()
}
