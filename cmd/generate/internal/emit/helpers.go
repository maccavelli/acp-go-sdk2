package emit

import (
	"fmt"
	"go/ast"
	"maps"
	"slices"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/util"
)

// emitHelperConstructors emits func New<Union><Field>(required...) <Union>
// for each constructor the model lists for t.
func emitHelperConstructors(f *goFile, m *model.Model, t *model.Type) {
	if len(t.Helpers) == 0 {
		return
	}
	discKey := model.DiscriminatorKey(m.Schema, t.Def, t.Def.OneOf)
	for _, h := range t.Helpers {
		vi := t.Variants[h.Variant]
		props := model.ExpandAllOf(m.Schema, t.Def.OneOf[h.Variant]).Properties
		// Parameters: every required property except the const discriminator.
		var params []*ast.Field
		assigns := map[string]ast.Expr{}
		for _, rk := range h.Params {
			params = append(params, param(rk, typeExpr(props[rk])))
			assigns[util.ToExportedField(rk)] = id(rk)
		}
		if discKey != "" && vi.DiscValue != "" {
			assigns[util.ToExportedField(discKey)] = str(vi.DiscValue)
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

// WriteHelpers emits helpers_gen.go with constructor helpers for union variants.
func WriteHelpers(outDir string, m *model.Model) error {
	f := newFile()
	for _, t := range m.Types {
		emitHelperConstructors(f, m, t)
	}
	return f.write(outDir, "helpers_gen.go")
}
