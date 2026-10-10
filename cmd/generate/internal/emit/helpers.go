package emit

import (
	"fmt"
	"go/ast"
	"maps"
	"slices"
	"strings"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/util"
)

type helperVariant struct {
	fieldName string
	typeName  string
	discKey   string
	discValue string
	required  []string
	props     map[string]*load.Definition
}

func collectHelperVariants(name string, def *load.Definition, schema *load.Schema, usedTypeNames map[string]bool) []helperVariant {
	if def == nil || def.DocsIgnore || len(def.OneOf) == 0 {
		return nil
	}
	// Skip string-const unions
	if isStringConstUnion(def) {
		return nil
	}
	// Skip generating New... helpers for unions that have stable, static helpers
	// implemented in helpers.go.
	switch name {
	case unionContentBlock, "ToolCallContent", nameSessionUpdate:
		return nil
	}
	// Build variant info using the same logic as the types emitter (emitUnion)
	// to ensure type names match exactly.
	discKey := discriminatorKey(schema, def, def.OneOf)
	variants := []helperVariant{}
	for idx, v := range def.OneOf {
		if hv, ok := oneHelperVariant(name, schema, idx, v, discKey, usedTypeNames); ok {
			variants = append(variants, hv)
		}
	}
	return variants
}

func oneHelperVariant(name string, schema *load.Schema, idx int, v *load.Definition, discKey string, usedTypeNames map[string]bool) (helperVariant, bool) {
	if v == nil {
		return helperVariant{}, false
	}
	// Match the types emitter's allOf-to-ref shortcut.
	ref := v.Ref
	if ref == "" {
		ref = singleRefVariant(v)
	}
	v = expandAllOf(schema, v)

	// Compute type name matching the types emitter (emitUnion).
	tname := ""
	if after, ok := strings.CutPrefix(ref, "#/$defs/"); ok {
		tname = after
	} else if v.Title != "" {
		tname = generateNestedTypeName(name, v.Title, usedTypeNames)
		usedTypeNames[tname] = true
	} else {
		if discKey != "" {
			if pd := v.Properties[discKey]; pd != nil && pd.Const != nil {
				tname = generateNestedTypeName(name, fmt.Sprint(pd.Const), usedTypeNames)
				usedTypeNames[tname] = true
			}
		}
		if tname == "" {
			tname = fmt.Sprintf("%sVariant%d", name, idx+1)
			usedTypeNames[tname] = true
		}
	}
	tname = util.ToExportedField(tname)

	// Derive field name with same priority as types emitter.
	fieldName := ""
	dv := ""
	if discKey != "" {
		if pd := v.Properties[discKey]; pd != nil && pd.Const != nil {
			dv = fmt.Sprint(pd.Const)
			fieldName = util.ToExportedField(dv)
		}
	}
	if fieldName == "" && v.Title != "" {
		fieldName = util.ToExportedField(v.Title)
	}
	if fieldName == "" {
		fieldName = util.ToExportedField(tname)
	}
	return helperVariant{
		fieldName: fieldName,
		typeName:  tname,
		discKey:   discKey,
		discValue: dv,
		required:  slices.Clone(v.Required),
		props:     v.Properties,
	}, true
}

// emitHelperConstructors emits func New<Union><Field>(required...) <Union> per variant.
func emitHelperConstructors(f *goFile, name string, variants []helperVariant) {
	for _, vi := range variants {
		// Parameters: every required property except the const discriminator.
		var params []*ast.Field
		assigns := map[string]ast.Expr{}
		for _, rk := range vi.required {
			if rk == vi.discKey {
				continue
			}
			pd := vi.props[rk]
			if pd == nil {
				continue
			}
			params = append(params, param(rk, typeExpr(pd)))
			assigns[util.ToExportedField(rk)] = id(rk)
		}
		if vi.discKey != "" && vi.discValue != "" {
			assigns[util.ToExportedField(vi.discKey)] = str(vi.discValue)
		}
		var elts []ast.Expr
		for _, field := range slices.Sorted(maps.Keys(assigns)) {
			elts = append(elts, kv(id(field), assigns[field]))
		}
		variant := addr(composite(id(vi.typeName), elts...))

		f.comment(fmt.Sprintf("New%s%s constructs a %s using the '%s' variant.", name, vi.fieldName, name, vi.discValue))
		f.add(function("New"+name+vi.fieldName, funcType(params, param("", id(name))),
			ret(composite(id(name), kv(id(vi.fieldName), variant))),
		))
		f.line()
	}
}

// WriteHelpers emits helpers_gen.go with constructor helpers for union variants.
func WriteHelpers(outDir string, schema *load.Schema, _ *load.Meta) error {
	f := newFile()

	// Schema-driven helpers: New<Union><Variant>(required fields only).
	// Track used type names to match the types emitter's naming (generateNestedTypeName).
	usedTypeNames := make(map[string]bool, len(schema.Defs))
	for k := range schema.Defs {
		usedTypeNames[k] = true
	}
	for _, name := range slices.Sorted(maps.Keys(schema.Defs)) {
		emitHelperConstructors(f, name, collectHelperVariants(name, schema.Defs[name], schema, usedTypeNames))
	}
	return f.write(outDir, "helpers_gen.go")
}
