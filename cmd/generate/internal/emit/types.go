package emit

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
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
	emitDecodeHelpers(f)

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
		f.comment(in.Doc...)
		f.add(typeDecl(in.Name, sets[in.Binding].interfaceType()))
	}
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
		f.add(typeDecl(name, typeAST(f, *t.Underlying)))
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
	case model.ValidateFields, model.ValidateListed:
		emitFieldValidate(f, t)
	}
}

func emitObjectSchema(f *goFile, t *model.Type) {
	// Nested struct types for inline object properties come first.
	for _, ns := range t.NestedStructs {
		f.comment(ns.Doc...)
		emitStruct(f, ns.Name, ns.Fields, false, "")
	}
	emitStruct(f, t.Name, t.Fields, false, "")
}

// emitStruct emits type name struct { fields } and the methods its required
// keys, defaults, and fills need; exempt is a required key decoding does not
// check.
func emitStruct(f *goFile, name string, fields []*model.Field, keepExtra bool, exempt string) {
	var st fieldSet
	for _, fld := range fields {
		st.comment(fld.Doc...)
		st.add(structField(fld.Name, typeAST(f, fld.Type), fld.Tag))
	}
	if keepExtra {
		st.comment("Extra holds the keys of the payload this struct does not declare, as received.",
			"MarshalJSON writes them back.")
		st.add(structField(extraField, rawMap(f.qual(pkgJSON, "RawMessage")), "-"))
	}
	f.add(typeDecl(name, st.structType()))
	f.line()
	emitObjectCodec(f, name, fields, keepExtra, exempt)
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
	default:
		return id(r.Name)
	}
}

// emitObjectCodec synthesizes MarshalJSON for structs with fields that carry
// schema defaults or are required slices or maps, and UnmarshalJSON for
// structs with required keys, defaults, or a catch-all's Extra.
func emitObjectCodec(f *goFile, name string, fields []*model.Field, keepExtra bool, exempt string) {
	var defaults, fills, lenient []*model.Field
	var required []string
	for _, fld := range fields {
		if fld.Default != nil {
			defaults = append(defaults, fld)
		}
		if fld.Facts.DefaultOnError || fld.Facts.SkipInvalidItems {
			lenient = append(lenient, fld)
		}
		if fld.Fill {
			fills = append(fills, fld)
		}
		if fld.Required && fld.Key != exempt {
			required = append(required, fld.Key)
		}
	}
	slices.Sort(required)
	if len(defaults) > 0 || len(fills) > 0 || keepExtra {
		emitObjectMarshal(f, name, defaults, fills, keepExtra)
	}
	if len(defaults) > 0 || keepExtra || len(required) > 0 || len(lenient) > 0 {
		emitObjectUnmarshal(f, name, fields, defaults, lenient, required, keepExtra)
	}
}

// emitObjectMarshal emits MarshalJSON: it fills nil slices and maps with
// their defaults, and nil required ones with empty values, before encoding,
// and writes back a catch-all's Extra.
func emitObjectMarshal(f *goFile, name string, defaults, fills []*model.Field, keepExtra bool) {
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
	} else {
		body = append(body, ret(marshal(id("a"))))
	}
	f.add(method(param("v", id(name)), "MarshalJSON",
		funcType(nil, param("", byteSlice()), param("", id("error"))), body...))
	f.line()
}

// emitObjectUnmarshal emits UnmarshalJSON: it requires the required keys,
// decodes the fields, decodes each lenient field from its own raw value,
// applies defaults when a field is missing, or null when the schema does not
// allow null, and keeps a catch-all's undeclared keys.
func emitObjectUnmarshal(f *goFile, name string, fields, defaults, lenient []*model.Field, required []string, keepExtra bool) {
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }
	f.uses[helperObjectKeys] = true
	keys := []ast.Expr{id("b"), str(name)}
	for _, k := range required {
		keys = append(keys, str(k))
	}
	m := id("m")
	if len(defaults) == 0 && !keepExtra {
		m = id("_")
	}
	body := []ast.Stmt{
		define(exprs(m, id("err")), call(id(helperObjectKeys), keys...)),
		ifStmt(nil, errNotNil(), ret(id("err"))),
		typeStmt("Alias", id(name)),
	}
	wrap := func() ast.Stmt { return ret(call(f.qual(pkgFmt, "Errorf"), str(name+": %w"), id("err"))) }
	if len(lenient) == 0 {
		body = append(body,
			varStmt("a", id("Alias")),
			ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("a")))), errNotNil(), wrap()),
		)
	} else {
		body = append(body, lenientDecode(f, name, lenient, wrap)...)
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
		def := ""
		if fld.Default != nil {
			def = fld.Default.JSON
		}
		raw := dot("s", fld.Name)
		field := dot("a", fld.Name)
		if !fld.Facts.SkipInvalidItems {
			f.uses[helperOrDefault] = true
			body = append(body, assign(exprs(field), call(index(id(helperOrDefault), typeAST(f, fld.Type)), raw, str(def))))
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
		if def != "" {
			f.uses[helperOrDefault] = true
			fallback = []ast.Stmt{assign(exprs(field), call(index(id(helperOrDefault), typeAST(f, fld.Type)), nilExpr(), str(def)))}
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
func emitInlineVariant(f *goFile, tname string, in *model.Inline) {
	if in == nil {
		return
	}
	f.comment(in.Doc...)
	switch in.Kind {
	case model.InlineStruct:
		emitStruct(f, tname, in.Fields, in.KeepExtra, in.Exempt)
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

	// Required-key match.
	var object []ast.Stmt
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

// discUnmarshalBody decodes a discriminated union, whose variants are all
// objects: by the discriminator's value when it is present, else by the rule
// for a payload without it.
func discUnmarshalBody(f *goFile, name string, variants []*model.Variant, discKey string, hasUnknown bool, catchAll *model.Variant) []ast.Stmt {
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }
	var cases []ast.Stmt
	for _, vi := range variants {
		if vi.DiscValue != "" {
			cases = append(cases, caseOf(exprs(str(vi.DiscValue)),
				append([]ast.Stmt{varStmt("v", id(vi.TypeName))},
					decodeStrict(f, vi, name+": "+discKey+" "+strconv.Quote(vi.DiscValue))...)...))
		}
	}
	body := []ast.Stmt{
		varStmt("m", rawMap(f.qual(pkgJSON, "RawMessage"))),
		ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("m")))), errNotNil(),
			ret(call(f.qual(pkgFmt, "Errorf"), str(name+": %w"), id("err")))),
		// A discriminator that is present and not a string is an error.
		varStmt("disc", id("string")),
		ifStmt(define(exprs(id("v"), id("ok")), index(id("m"), str(discKey))), id("ok"),
			ifStmt(define(exprs(id("err")), unmarshal(id("v"), addr(id("disc")))), errNotNil(),
				ret(call(f.qual(pkgFmt, "Errorf"), str(name+": discriminator "+strconv.Quote(discKey)+": %w"), id("err"))))),
		switchStmt(id("disc"), cases...),
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
			return append(append(body, varStmt("v", id(vi.TypeName))), decodeStrict(f, vi, variantPlace(name, vi))...)
		}
		stmts := []ast.Stmt{
			varStmt("v", id(vi.TypeName)),
			define(exprs(id("match")), boolLit(true)),
		}
		for _, k := range vi.FitKeys {
			stmts = append(stmts, keyMatch("m", k))
		}
		body = append(body, block(append(stmts, ifStmt(nil, id("match"), decodeStrict(f, vi, variantPlace(name, vi))...))...))
	}
	for _, vi := range keyLess {
		body = append(body, block(
			varStmt("v", id(vi.TypeName)),
			ifStmt(nil, binary(call(f.qual(pkgJSON, "Unmarshal"), id("b"), addr(id("v"))), token.EQL, nilExpr()), setVariant(vi)...),
		))
	}
	return append(body, missingDisc(f, name, discKey))
}

// fitBody decodes the one variant, the catch-all aside, whose required keys
// other than the discriminator are all in the payload. When none or several
// fit, the payload is kept in Unknown, or refused in a union with a catch-all.
func fitBody(f *goFile, name string, variants []*model.Variant, discKey string, hasUnknown bool, catchAll *model.Variant) []ast.Stmt {
	body := []ast.Stmt{varStmt("fit", sliceOf(id("string")))}
	var cases []ast.Stmt
	for _, vi := range variants {
		if vi == catchAll {
			continue
		}
		add := assign(exprs(id("fit")), call(id("append"), id("fit"), str(vi.FieldName)))
		if len(vi.FitKeys) == 0 {
			body = append(body, add)
		} else {
			stmts := []ast.Stmt{define(exprs(id("match")), boolLit(true))}
			for _, k := range vi.FitKeys {
				stmts = append(stmts, keyMatch("m", k))
			}
			body = append(body, block(append(stmts, ifStmt(nil, id("match"), add))...))
		}
		cases = append(cases, caseOf(exprs(str(vi.FieldName)),
			append([]ast.Stmt{varStmt("v", id(vi.TypeName))}, decodeStrict(f, vi, variantPlace(name, vi))...)...))
	}
	body = append(body, ifStmt(nil, binary(call(id("len"), id("fit")), token.EQL, intLit(1)),
		switchStmt(index(id("fit"), intLit(0)), cases...)))
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
	if t.Validate == model.ValidateListed {
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
