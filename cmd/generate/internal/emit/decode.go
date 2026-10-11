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
)

// emitDecodeHelpers appends the helpers f uses.
func emitDecodeHelpers(f *goFile) {
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
	if f.uses[helperAbsentIfNull] {
		emitAbsentIfNull(f)
	}
	if f.uses[helperMergeObject] {
		emitMergeObject(f)
	}
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
