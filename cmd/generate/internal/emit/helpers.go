package emit

import (
	"fmt"
	"go/ast"
	"maps"
	"slices"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// emitHelperConstructors emits func New<Union><Field>(required...) <Union>
// for each constructor the model lists for t.
func emitHelperConstructors(f *goFile, t *model.Type) {
	if len(t.Helpers) == 0 {
		return
	}
	for _, h := range t.Helpers {
		vi := t.Variants[h.Variant]
		// Parameters: the required keys of the variant's Go type, except the
		// discriminator the constructor writes (model.Helper.Params).
		var params []*ast.Field
		assigns := map[string]ast.Expr{}
		for _, p := range h.Params {
			params = append(params, param(p.Key, typeAST(f, p.Type)))
			assigns[p.Field] = id(p.Key)
		}
		if t.DiscKey != "" && vi.DiscValue != "" {
			assigns[t.DiscField] = str(vi.DiscValue)
		}
		var elts []ast.Expr
		for _, field := range slices.Sorted(maps.Keys(assigns)) {
			elts = append(elts, kv(id(field), assigns[field]))
		}
		variant := addr(composite(id(vi.TypeName), elts...))

		f.comment(fmt.Sprintf("%s constructs a %s using the '%s' variant.", h.Name, t.Name, vi.DiscValue))
		f.add(function(h.Name, funcType(params, param("", id(t.Name))),
			ret(composite(id(t.Name), kv(id(vi.FieldName), variant))),
		))
		f.line()
	}
}

// helpersFile builds helpers_gen.go: constructor helpers for union variants.
func helpersFile(m *model.Model) *goFile {
	f := newFile()
	for _, t := range m.Types {
		emitHelperConstructors(f, t)
	}
	return f
}
