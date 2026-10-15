package emit

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// The unexported decoding helpers types_gen.go carries, each emitted once when
// the file's code calls it (0012-MADR-schema-fidelity-framework.md).
const (
	helperObjectKeys   = "objectKeys"
	helperOrDefault    = "orDefault"
	helperSkipInvalid  = "skipInvalid"
	helperAbsentIfNull = "absentIfNull"
	helperMergeObject  = "mergeObject"
	helperFoldKeys     = "foldKeys"
	helperLastKey      = "lastKey"
	helperPatchOf      = "patchOf"
	helperPatchItems   = "patchItems"
	// The helpers of a key-order rule
	// (0022-MADR-keep-elicitation-property-order-before-v1-15-0.md).
	helperKeyOrder      = "keyOrder"
	helperOrderedObject = "orderedObject"
)

// typePatch is the exported generic type of a property with patch semantics,
// emitted with the helpers that read it
// (0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md, issue #5).
const typePatch = "Patch"

// emitDecodeHelpers appends the helpers f uses.
func emitDecodeHelpers(f *goFile) {
	if f.uses[helperPatchItems] {
		f.uses[helperSkipInvalid] = true
		f.uses[typePatch] = true
	}
	if f.uses[helperPatchOf] {
		f.uses[typePatch] = true
	}
	if f.uses[typePatch] {
		emitPatchType(f)
	}
	if f.uses[helperObjectKeys] {
		emitObjectKeys(f)
		f.uses[helperFoldKeys] = true
	}
	if f.uses[helperFoldKeys] {
		emitFoldKeys(f)
	}
	if f.uses[helperOrDefault] {
		emitOrDefault(f)
	}
	if f.uses[helperSkipInvalid] {
		emitSkipInvalid(f)
	}
	if f.uses[helperPatchOf] {
		emitPatchOf(f)
	}
	if f.uses[helperPatchItems] {
		emitPatchItems(f)
	}
	if f.uses[helperAbsentIfNull] {
		emitAbsentIfNull(f)
	}
	if f.uses[helperMergeObject] {
		emitMergeObject(f)
	}
	if f.uses[helperKeyOrder] {
		emitKeyOrder(f)
	}
	if f.uses[helperOrderedObject] {
		emitOrderedObject(f)
	}
}

// emitKeyOrder emits
//
//	func keyOrder(raw json.RawMessage) []string
//
// which returns the keys of the JSON object raw in the order they appear, each
// once at its first appearance, or nil when raw is empty, null, or not an
// object (0022-MADR-keep-elicitation-property-order-before-v1-15-0.md).
func emitKeyOrder(f *goFile) {
	delim := call(f.qual(pkgJSON, "Delim"), &ast.BasicLit{Kind: token.CHAR, Value: "'{'"})
	strs := func() ast.Expr { return sliceOf(id("string")) }
	fn := function(helperKeyOrder,
		funcType([]*ast.Field{param("raw", f.qual(pkgJSON, "RawMessage"))}, param("", strs())),
		define(exprs(id("d")), call(f.qual(pkgJSON, "NewDecoder"), call(f.qual(pkgBytes, "NewReader"), id("raw")))),
		ifStmt(define(exprs(id("t"), id("err")), call(dot("d", "Token"))),
			binary(errNotNil(), token.LOR, binary(id("t"), token.NEQ, delim)), ret(nilExpr())),
		varStmt("keys", strs()),
		forCond(call(dot("d", "More")),
			define(exprs(id("t"), id("err")), call(dot("d", "Token"))),
			define(exprs(id("k"), id("ok")), typeAssert(id("t"), id("string"))),
			varStmt("x", f.qual(pkgJSON, "RawMessage")),
			ifStmt(nil, binary(binary(errNotNil(), token.LOR, not(id("ok"))), token.LOR,
				binary(call(dot("d", "Decode"), addr(id("x"))), token.NEQ, nilExpr())), ret(id("keys"))),
			ifStmt(nil, not(call(f.qual("slices", "Contains"), id("keys"), id("k"))),
				assign(exprs(id("keys")), call(id("append"), id("keys"), id("k")))),
		),
		ret(id("keys")),
	)
	f.line()
	f.comment("keyOrder returns the keys of the JSON object raw in the order they appear, each",
		"once at its first appearance, or nil when raw is empty, null, or not an object.")
	f.add(fn)
}

// emitOrderedObject emits
//
//	func orderedObject[V any](m map[string]V, order []string) (json.RawMessage, error)
//
// which encodes m as a JSON object whose keys are the names of order that m
// holds, in that order and each once, then m's other keys, sorted
// (0022-MADR-keep-elicitation-property-order-before-v1-15-0.md).
func emitOrderedObject(f *goFile) {
	marshal := func(x ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Marshal"), x) }
	add := func(x ast.Expr) ast.Stmt {
		return ifStmt(nil, not(call(f.qual("slices", "Contains"), id("keys"), id("k"))),
			assign(exprs(id("keys")), call(id("append"), id("keys"), x)))
	}
	char := func(c string) ast.Expr { return &ast.BasicLit{Kind: token.CHAR, Value: "'" + c + "'"} }
	fn := function(helperOrderedObject,
		funcType([]*ast.Field{param("m", mapOf(id("string"), id("V"))), param("order", sliceOf(id("string")))},
			param("", f.qual(pkgJSON, "RawMessage")), param("", id("error"))),
		define(exprs(id("keys")), call(id("make"), sliceOf(id("string")), intLit(0), call(id("len"), id("m")))),
		rangeStmt(id("_"), id("k"), id("order"),
			ifStmt(define(exprs(id("_"), id("ok")), index(id("m"), id("k"))), id("ok"), add(id("k")))),
		rangeStmt(id("_"), id("k"), call(f.qual("slices", "Sorted"), call(f.qual("maps", "Keys"), id("m"))), add(id("k"))),
		define(exprs(id("b")), call(byteSlice(), str("{"))),
		rangeStmt(id("i"), id("k"), id("keys"),
			ifStmt(nil, binary(id("i"), token.GTR, intLit(0)), assign(exprs(id("b")), call(id("append"), id("b"), char(",")))),
			define(exprs(id("kb"), id("err")), marshal(id("k"))),
			ifStmt(nil, errNotNil(), ret(nilExpr(), id("err"))),
			define(exprs(id("vb"), id("err")), marshal(index(id("m"), id("k")))),
			ifStmt(nil, errNotNil(), ret(nilExpr(), id("err"))),
			assign(exprs(id("b")), &ast.CallExpr{Fun: id("append"), Args: exprs(id("b"), id("kb")), Ellipsis: placeholderPos}),
			assign(exprs(id("b")), call(id("append"), id("b"), char(":"))),
			assign(exprs(id("b")), &ast.CallExpr{Fun: id("append"), Args: exprs(id("b"), id("vb")), Ellipsis: placeholderPos}),
		),
		ret(call(id("append"), id("b"), char("}")), nilExpr()),
	)
	fn.Type.TypeParams = fields(&ast.Field{Names: []*ast.Ident{id("V")}, Type: id("any")})
	f.line()
	f.comment("orderedObject encodes m as a JSON object whose keys are the names of order that",
		"m holds, in that order and each once, then m's other keys, sorted.")
	f.add(fn)
}

// emitMergeObject emits
//
//	func mergeObject(b []byte, x any) ([]byte, error)
//
// which adds the keys of x's JSON object that b's object lacks, for a struct
// with flattened members (0011-MADR-harden-code-generator-engine.md, "Found in
// step 7 and decided"). Values are carried as raw JSON, so numbers keep their
// digits.
func emitMergeObject(f *goFile) {
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }
	raw := func() ast.Expr { return rawMap(f.qual(pkgJSON, "RawMessage")) }
	fn := function(helperMergeObject,
		funcType([]*ast.Field{param("b", byteSlice()), param("x", id("any"))}, param("", byteSlice()), param("", id("error"))),
		varStmt("m", raw()),
		ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("m")))), errNotNil(), ret(nilExpr(), id("err"))),
		define(exprs(id("xb"), id("err")), call(f.qual(pkgJSON, "Marshal"), id("x"))),
		ifStmt(nil, errNotNil(), ret(nilExpr(), id("err"))),
		varStmt("xm", raw()),
		ifStmt(define(exprs(id("err")), unmarshal(id("xb"), addr(id("xm")))), errNotNil(), ret(nilExpr(), id("err"))),
		rangeStmt(id("k"), id("v"), id("xm"),
			ifStmt(define(exprs(id("_"), id("ok")), index(id("m"), id("k"))), not(id("ok")),
				assign(exprs(index(id("m"), id("k"))), id("v")))),
		ret(call(f.qual(pkgJSON, "Marshal"), id("m"))),
	)
	f.line()
	f.comment("mergeObject adds the keys of x's JSON object that b's object lacks: the keys of",
		"a flattened member. Values stay raw JSON, so numbers keep every digit.")
	f.add(fn)
}

// emitAbsentIfNull emits
//
//	func absentIfNull(raw json.RawMessage) json.RawMessage
//
// which returns raw, or nil when raw is the JSON null: ACP v1 defines a null raw
// value to be an absent one (0012-MADR-schema-fidelity-framework.md, amendment
// of 2026-10-11).
func emitAbsentIfNull(f *goFile) {
	raw := func() ast.Expr { return f.qual(pkgJSON, "RawMessage") }
	fn := function(helperAbsentIfNull,
		funcType([]*ast.Field{param("raw", raw())}, param("", raw())),
		ifStmt(nil, binary(call(id("string"), id("raw")), token.EQL, str("null")), ret(nilExpr())),
		ret(id("raw")),
	)
	f.line()
	f.comment("absentIfNull returns raw, or nil when raw is the JSON null: a null raw value",
		"is an absent one, as ACP v1 defines it for rawInput and rawOutput.")
	f.add(fn)
}

// typeParamT is the type parameter list [T any].
func typeParamT() *ast.FieldList {
	return fields(&ast.Field{Names: []*ast.Ident{id("T")}, Type: id("any")})
}

// patchOfT is Patch[t].
func patchOfT(t ast.Expr) ast.Expr { return index(id(typePatch), t) }

// emitPatchType emits Patch[T], the three states of a property with patch
// semantics, its constructors, and its methods. Its state is unexported, so
// no value is both null and set, and its zero value is absent, which
// omitzero omits (0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md).
func emitPatchType(f *goFile) {
	pt := func() ast.Expr { return patchOfT(id("T")) }
	recv := func() *ast.Field { return param("p", pt()) }
	f.line()
	f.comment(
		"Patch is a property with patch semantics: absent leaves the stored value",
		"unchanged, null clears it, and a value replaces it. The zero value is absent.",
		"A field of this type is tagged omitzero, so an absent one is not sent; a null",
		"one is sent as null. A value is sent as T encodes, so a value holding a nil",
		"slice or map is sent as null: send an empty one for [] or {}. Where the schema",
		"says so, as for a compaction's summary, an empty value also clears.",
	)
	f.add(&ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&ast.TypeSpec{Name: id(typePatch), TypeParams: typeParamT(),
		Type: &ast.StructType{Fields: fields(param("present", id("bool")), param("null", id("bool")), param("value", id("T")))}}}})

	value := function("PatchValue", funcType([]*ast.Field{param("v", id("T"))}, param("", pt())),
		ret(composite(pt(), kv(id("present"), boolLit(true)), kv(id("value"), id("v")))))
	value.Type.TypeParams = typeParamT()
	f.line()
	f.comment("PatchValue returns a Patch that replaces the stored value with v.")
	f.add(value)

	null := function("PatchNull", funcType(nil, param("", pt())),
		ret(composite(pt(), kv(id("present"), boolLit(true)), kv(id("null"), boolLit(true)))))
	null.Type.TypeParams = typeParamT()
	f.line()
	f.comment("PatchNull returns a Patch that clears the stored value: it is sent as null.")
	f.add(null)

	f.line()
	f.comment("IsZero reports whether p is absent: the stored value is left unchanged.")
	f.add(method(recv(), "IsZero", funcType(nil, param("", id("bool"))), ret(not(dot("p", "present")))))
	f.line()
	f.comment("IsNull reports whether p is null: the stored value is cleared.")
	f.add(method(recv(), "IsNull", funcType(nil, param("", id("bool"))),
		ret(binary(dot("p", "present"), token.LAND, dot("p", "null")))))
	f.line()
	f.comment("Value returns the value p replaces the stored value with, and whether it",
		"holds one.")
	f.add(method(recv(), "Value", funcType(nil, param("", id("T")), param("", id("bool"))),
		ret(dot("p", "value"), binary(dot("p", "present"), token.LAND, not(dot("p", "null"))))))
	f.line()
	f.comment("MarshalJSON encodes p's value, or null when p is null or absent.")
	f.add(method(recv(), "MarshalJSON", funcType(nil, param("", byteSlice()), param("", id("error"))),
		ifStmt(nil, binary(not(dot("p", "present")), token.LOR, dot("p", "null")), ret(bytesOf(schemaTypeNull), nilExpr())),
		ret(call(f.qual(pkgJSON, "Marshal"), dot("p", "value")))))
	f.line()
	f.comment("UnmarshalJSON decodes null as a null p and anything else as a value.")
	f.add(method(param("p", star(pt())), "UnmarshalJSON", funcType([]*ast.Field{param("b", byteSlice())}, param("", id("error"))),
		ifStmt(nil, binary(call(id("string"), id("b")), token.EQL, str(schemaTypeNull)),
			assign(exprs(star(id("p"))), call(index(id("PatchNull"), id("T")))), ret(nilExpr())),
		varStmt("v", id("T")),
		ifStmt(define(exprs(id("err")), call(f.qual(pkgJSON, "Unmarshal"), id("b"), addr(id("v")))), errNotNil(), ret(id("err"))),
		assign(exprs(star(id("p"))), call(id("PatchValue"), id("v"))),
		ret(nilExpr())))
}

// emitPatchOf emits
//
//	func patchOf[T any](raw json.RawMessage) Patch[T]
//
// which reads a patch property's raw value: empty is absent, null is null, a
// value that decodes into a T is that value, and one that does not is absent,
// as x-deserialize-default-on-error has it.
func emitPatchOf(f *goFile) {
	pt := func() ast.Expr { return patchOfT(id("T")) }
	fn := function(helperPatchOf,
		funcType([]*ast.Field{param("raw", f.qual(pkgJSON, "RawMessage"))}, param("", pt())),
		ifStmt(nil, binary(call(id("len"), id("raw")), token.EQL, intLit(0)), ret(composite(pt()))),
		ifStmt(nil, binary(call(id("string"), id("raw")), token.EQL, str(schemaTypeNull)), ret(call(index(id("PatchNull"), id("T"))))),
		varStmt("v", id("T")),
		ifStmt(nil, binary(call(f.qual(pkgJSON, "Unmarshal"), id("raw"), addr(id("v"))), token.NEQ, nilExpr()), ret(composite(pt()))),
		ret(call(id("PatchValue"), id("v"))),
	)
	fn.Type.TypeParams = typeParamT()
	f.line()
	f.comment("patchOf reads a patch property's raw value: empty is absent, null is null, a",
		"value that decodes into a T is that value, and one that does not is absent.")
	f.add(fn)
}

// emitPatchItems emits
//
//	func patchItems[T any](raw json.RawMessage) Patch[[]T]
//
// which is patchOf for an array with x-deserialize-skip-invalid-items: the
// items that do not decode into a T are dropped, as skipInvalid drops them.
func emitPatchItems(f *goFile) {
	pt := func() ast.Expr { return patchOfT(sliceOf(id("T"))) }
	fn := function(helperPatchItems,
		funcType([]*ast.Field{param("raw", f.qual(pkgJSON, "RawMessage"))}, param("", pt())),
		ifStmt(nil, binary(call(id("len"), id("raw")), token.EQL, intLit(0)), ret(composite(pt()))),
		ifStmt(nil, binary(call(id("string"), id("raw")), token.EQL, str(schemaTypeNull)), ret(call(index(id("PatchNull"), sliceOf(id("T")))))),
		define(exprs(id("items"), id("err")), call(index(id(helperSkipInvalid), id("T")), id("raw"))),
		ifStmt(nil, errNotNil(), ret(composite(pt()))),
		ret(call(id("PatchValue"), id("items"))),
	)
	fn.Type.TypeParams = typeParamT()
	f.line()
	f.comment("patchItems is patchOf for an array, dropping the items that do not decode",
		"into a T, as skipInvalid does.")
	f.add(fn)
}

// emitOrDefault emits
//
//	func orDefault[T any](raw json.RawMessage, def string) T
//
// which decodes raw into a T, or, when raw is empty or does not decode, def,
// the schema default, or the zero value when def is empty.
func emitOrDefault(f *goFile) {
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }
	fn := function(helperOrDefault,
		funcType([]*ast.Field{param("raw", f.qual(pkgJSON, "RawMessage")), param("def", id("string"))}, param("", id("T"))),
		varStmt("v", id("T")),
		ifStmt(nil, binary(binary(call(id("len"), id("raw")), token.GTR, intLit(0)), token.LAND,
			binary(unmarshal(id("raw"), addr(id("v"))), token.EQL, nilExpr())), ret(id("v"))),
		varStmt("d", id("T")),
		ifStmt(nil, binary(binary(id("def"), token.NEQ, str("")), token.LAND,
			binary(unmarshal(call(byteSlice(), id("def")), addr(id("d"))), token.NEQ, nilExpr())),
			varStmt("zero", id("T")), ret(id("zero"))),
		ret(id("d")),
	)
	fn.Type.TypeParams = typeParamT()
	f.line()
	f.comment("orDefault decodes raw into a T, or, when raw is empty or does not decode, def,",
		"the schema default, or the zero value when def is empty.")
	f.add(fn)
}

// emitSkipInvalid emits
//
//	func skipInvalid[T any](raw json.RawMessage) ([]T, error)
//
// which decodes raw as a JSON array, dropping the items that do not decode
// into a T. An empty raw value or null is no items.
func emitSkipInvalid(f *goFile) {
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }
	fn := function(helperSkipInvalid,
		funcType([]*ast.Field{param("raw", f.qual(pkgJSON, "RawMessage"))}, param("", sliceOf(id("T"))), param("", id("error"))),
		ifStmt(nil, binary(call(id("len"), id("raw")), token.EQL, intLit(0)), ret(nilExpr(), nilExpr())),
		varStmt("items", sliceOf(f.qual(pkgJSON, "RawMessage"))),
		ifStmt(define(exprs(id("err")), unmarshal(id("raw"), addr(id("items")))), errNotNil(), ret(nilExpr(), id("err"))),
		ifStmt(nil, binary(id("items"), token.EQL, nilExpr()), ret(nilExpr(), nilExpr())),
		define(exprs(id("out")), call(id("make"), sliceOf(id("T")), intLit(0), call(id("len"), id("items")))),
		rangeStmt(id("_"), id("it"), id("items"),
			varStmt("v", id("T")),
			ifStmt(nil, binary(unmarshal(id("it"), addr(id("v"))), token.EQL, nilExpr()),
				assign(exprs(id("out")), call(id("append"), id("out"), id("v"))))),
		ret(id("out"), nilExpr()),
	)
	fn.Type.TypeParams = typeParamT()
	f.line()
	f.comment("skipInvalid decodes raw as a JSON array, dropping the items that do not decode",
		"into a T. An empty raw value or null is no items.")
	f.add(fn)
}

// emitObjectKeys emits
//
//	func objectKeys(b []byte, typ string, required ...string) (map[string]json.RawMessage, error)
//
// which decodes b as a JSON object, folds the required keys as foldKeys does,
// and fails, naming typ and the key, when a required key is absent. JSON null
// is an object without keys.
func emitObjectKeys(f *goFile) {
	raw := func() ast.Expr { return rawMap(f.qual(pkgJSON, "RawMessage")) }
	f.line()
	f.comment("objectKeys decodes b as a JSON object, folds the required keys as foldKeys",
		"does, and fails, naming typ and the key, when a required key is absent. JSON",
		"null is an object without keys.")
	f.add(function(helperObjectKeys,
		funcType([]*ast.Field{
			param("b", byteSlice()),
			param("typ", id("string")),
			param("required", &ast.Ellipsis{Elt: id("string")}),
		}, param("", raw()), param("", id("error"))),
		varStmt("m", raw()),
		ifStmt(define(exprs(id("err")), call(f.qual(pkgJSON, "Unmarshal"), id("b"), addr(id("m")))), errNotNil(),
			ret(nilExpr(), call(f.qual(pkgFmt, "Errorf"), str("%s: %w"), id("typ"), id("err")))),
		exprStmt(&ast.CallExpr{Fun: id(helperFoldKeys), Args: exprs(id("b"), id("m"), id("required")), Ellipsis: 1}),
		rangeStmt(id("_"), id("k"), id("required"),
			ifStmt(define(exprs(id("_"), id("ok")), index(id("m"), id("k"))), not(id("ok")),
				ret(nilExpr(), call(f.qual(pkgFmt, "Errorf"), str("%s: missing required property %q"), id("typ"), id("k"))))),
		ret(id("m"), nilExpr()),
	))
}

// emitFoldKeys emits
//
//	func foldKeys(b []byte, m map[string]json.RawMessage, names ...string)
//	func lastKey(b []byte, name string) string
//
// foldKeys makes m, the keys of the JSON object b, hold for each of names the
// value encoding/json gives a struct field of that name: the value of the last
// key of b that matches it under Unicode case folding. It removes the other
// matching keys and keeps every key no name matches
// (0011-MADR-harden-code-generator-engine.md, "Found in step 8 and decided").
// lastKey finds that key when several match.
func emitFoldKeys(f *goFile) {
	raw := func() ast.Expr { return rawMap(f.qual(pkgJSON, "RawMessage")) }
	fold := func(k string) ast.Expr { return call(f.qual(pkgStrings, "EqualFold"), id(k), id("name")) }
	f.line()
	f.comment("foldKeys makes m, the keys of the JSON object b, hold for each of names the",
		"value encoding/json gives a struct field of that name: the value of the last key",
		"of b that matches it under Unicode case folding. It removes the other matching",
		"keys and keeps every key no name matches.")
	f.add(function(helperFoldKeys,
		funcType([]*ast.Field{
			param("b", byteSlice()),
			param("m", raw()),
			param("names", &ast.Ellipsis{Elt: id("string")}),
		}),
		rangeStmt(id("_"), id("name"), id("names"),
			varStmt("keys", sliceOf(id("string"))),
			rangeStmt(id("k"), nil, id("m"),
				ifStmt(nil, fold("k"), assign(exprs(id("keys")), call(id("append"), id("keys"), id("k"))))),
			ifStmt(nil, binary(
				binary(call(id("len"), id("keys")), token.EQL, intLit(0)),
				token.LOR,
				binary(binary(call(id("len"), id("keys")), token.EQL, intLit(1)), token.LAND,
					binary(index(id("keys"), intLit(0)), token.EQL, id("name")))),
				continueStmt()),
			define(exprs(id("last")), index(id("keys"), intLit(0))),
			ifStmt(nil, binary(call(id("len"), id("keys")), token.GTR, intLit(1)),
				assign(exprs(id("last")), call(id(helperLastKey), id("b"), id("name")))),
			define(exprs(id("v")), index(id("m"), id("last"))),
			rangeStmt(id("_"), id("k"), id("keys"), exprStmt(call(id("delete"), id("m"), id("k")))),
			assign(exprs(index(id("m"), id("name"))), id("v")),
		),
	))
	f.line()
	f.comment("lastKey returns the last key of the JSON object b that matches name under",
		"Unicode case folding. b has decoded as an object already.")
	f.add(function(helperLastKey,
		funcType([]*ast.Field{param("b", byteSlice()), param("name", id("string"))}, param("", id("string"))),
		define(exprs(id("d")), call(f.qual(pkgJSON, "NewDecoder"), call(f.qual(pkgBytes, "NewReader"), id("b")))),
		varStmt("last", id("string")),
		ifStmt(define(exprs(id("_"), id("err")), call(dot("d", "Token"))), errNotNil(), ret(id("last"))),
		forCond(call(dot("d", "More")),
			define(exprs(id("t"), id("err")), call(dot("d", "Token"))),
			ifStmt(nil, errNotNil(), ret(id("last"))),
			ifStmt(define(exprs(id("k"), id("ok")), typeAssert(id("t"), id("string"))), binary(id("ok"), token.LAND, fold("k")),
				assign(exprs(id("last")), id("k"))),
			varStmt("v", f.qual(pkgJSON, "RawMessage")),
			ifStmt(define(exprs(id("err")), call(dot("d", "Decode"), addr(id("v")))), errNotNil(), ret(id("last"))),
		),
		ret(id("last")),
	))
}

// foldNames returns names, the keys a decoder of owner reads, each once in the
// order given. Two names that differ only in case would be read as one, so it
// records a failure instead.
func foldNames(f *goFile, owner string, names []string) []string {
	var out []string
	for _, n := range names {
		if slices.Contains(out, n) {
			continue
		}
		for _, o := range out {
			if strings.EqualFold(o, n) {
				f.fail(fmt.Errorf("%s: the keys %q and %q differ only in case, and would be read as one", owner, o, n))
			}
		}
		out = append(out, n)
	}
	return out
}

// foldCall is foldKeys(raw, m, names...), or nil when names is empty.
func foldCall(f *goFile, raw, m string, names []string) ast.Stmt {
	if len(names) == 0 {
		return nil
	}
	f.uses[helperFoldKeys] = true
	args := []ast.Expr{id(raw), id(m)}
	for _, n := range names {
		args = append(args, str(n))
	}
	return exprStmt(call(id(helperFoldKeys), args...))
}
