package emit

import (
	"go/ast"
	"go/token"
)

// The unexported decoding helpers types_gen.go carries, each emitted once when
// the file's code calls it (0012-MADR-schema-fidelity-framework.md).
const (
	helperObjectKeys   = "objectKeys"
	helperOrDefault    = "orDefault"
	helperSkipInvalid  = "skipInvalid"
	helperAbsentIfNull = "absentIfNull"
)

// emitDecodeHelpers appends the helpers f uses.
func emitDecodeHelpers(f *goFile) {
	if f.uses[helperObjectKeys] {
		emitObjectKeys(f)
	}
	if f.uses[helperOrDefault] {
		emitOrDefault(f)
	}
	if f.uses[helperSkipInvalid] {
		emitSkipInvalid(f)
	}
	if f.uses[helperAbsentIfNull] {
		emitAbsentIfNull(f)
	}
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
// which decodes b as a JSON object and fails, naming typ and the key, when a
// required key is absent. JSON null is an object without keys.
func emitObjectKeys(f *goFile) {
	raw := func() ast.Expr { return rawMap(f.qual(pkgJSON, "RawMessage")) }
	f.line()
	f.comment("objectKeys decodes b as a JSON object and fails, naming typ and the key,",
		"when a required key is absent. JSON null is an object without keys.")
	f.add(function(helperObjectKeys,
		funcType([]*ast.Field{
			param("b", byteSlice()),
			param("typ", id("string")),
			param("required", &ast.Ellipsis{Elt: id("string")}),
		}, param("", raw()), param("", id("error"))),
		varStmt("m", raw()),
		ifStmt(define(exprs(id("err")), call(f.qual(pkgJSON, "Unmarshal"), id("b"), addr(id("m")))), errNotNil(),
			ret(nilExpr(), call(f.qual(pkgFmt, "Errorf"), str("%s: %w"), id("typ"), id("err")))),
		rangeStmt(id("_"), id("k"), id("required"),
			ifStmt(define(exprs(id("_"), id("ok")), index(id("m"), id("k"))), not(id("ok")),
				ret(nilExpr(), call(f.qual(pkgFmt, "Errorf"), str("%s: missing required property %q"), id("typ"), id("k"))))),
		ret(id("m"), nilExpr()),
	))
}
