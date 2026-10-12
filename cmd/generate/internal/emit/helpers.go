package emit

import (
	"fmt"
	"go/ast"
	"maps"
	"slices"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// emitHelperConstructors emits func New<Union><Field>(...) <Union> for each
// constructor the model lists for t: one that takes the variant's required
// keys, or, for a Value constructor, one that takes the variant's value.
func emitHelperConstructors(f *goFile, t *model.Type) {
	if len(t.Helpers) == 0 {
		return
	}
	for _, h := range t.Helpers {
		vi := t.Variants[h.Variant]
		var params []*ast.Field
		for _, p := range h.Params {
			params = append(params, param(p.Name, typeAST(f, p.Type)))
		}
		var body []ast.Stmt
		if h.Value {
			// The parameter is the variant's type, or the type it is declared
			// with, which converts to it.
			value := ast.Expr(id(h.Params[0].Name))
			if h.Params[0].Type.String() != vi.TypeName {
				body = append(body, define(exprs(id("v")), call(id(vi.TypeName), value)))
				value = id("v")
			}
			body = append(body, ret(composite(id(t.Name), kv(id(vi.FieldName), addr(value)))))
		} else {
			// Parameters: the required keys of the variant's Go type, except
			// the discriminator the constructor writes (model.Helper.Params).
			assigns := map[string]ast.Expr{}
			for _, p := range h.Params {
				assigns[p.Field] = id(p.Name)
			}
			if t.DiscKey != "" && vi.DiscValue != "" {
				assigns[t.DiscField] = str(vi.DiscValue)
			}
			var elts []ast.Expr
			for _, field := range slices.Sorted(maps.Keys(assigns)) {
				elts = append(elts, kv(id(field), assigns[field]))
			}
			body = append(body, ret(composite(id(t.Name), kv(id(vi.FieldName), addr(composite(id(vi.TypeName), elts...))))))
		}

		f.comment(helperDoc(h, t, vi)...)
		f.add(function(h.Name, funcType(params, param("", id(t.Name))), body...))
		f.line()
	}
}

// helperDoc is a constructor's doc comment: the variant by its discriminator
// value where it has one, else by its field; for a catch-all, where the keys
// it does not declare go.
func helperDoc(h *model.Helper, t *model.Type, vi *model.Variant) []string {
	doc := []string{fmt.Sprintf("%s constructs a %s using its %s variant.", h.Name, t.Name, vi.FieldName)}
	if vi.DiscValue != "" {
		doc[0] = fmt.Sprintf("%s constructs a %s using the '%s' variant.", h.Name, t.Name, vi.DiscValue)
	}
	if t.CatchAll == h.Variant {
		doc = append(doc,
			"Keys the variant does not declare go in its Extra field, which MarshalJSON writes",
			"after the declared ones; a key a declared field writes is not taken from Extra.")
	}
	return doc
}

// helpersFile builds helpers_gen.go: constructor helpers for union variants.
func helpersFile(m *model.Model) *goFile {
	f := newFile()
	for _, t := range m.Types {
		emitHelperConstructors(f, t)
	}
	return f
}
