package emit

import "go/ast"

// The unexported decoding helpers types_gen.go carries, each emitted once when
// the file's code calls it (0012-MADR-schema-fidelity-framework.md).
const (
	helperObjectKeys = "objectKeys"
)

// emitDecodeHelpers appends the helpers f uses.
func emitDecodeHelpers(f *goFile) {
	if f.uses[helperObjectKeys] {
		emitObjectKeys(f)
	}
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
