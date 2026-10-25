package emit

import (
	"fmt"
	"go/ast"
	"go/token"
	"maps"
	"slices"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// UnstableDir is package unstable's directory under the root's: the API the
// live unstable schema adds beyond the manifest
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 1 and
// question 5).
const UnstableDir = "unstable"

const (
	unstablePackage  = "unstable"
	unstableFileName = "unstable_gen.go"
)

// Unstable is what package unstable declares from its model, as
// load.Unstable lists it: Root is the root package's import path; Local the
// new definitions, by name; Variants, for each stable union with a new
// variant, the indexes of those variants; Wires the new methods' wires.
type Unstable struct {
	Root     string
	Local    map[string]bool
	Variants map[string][]int
	Wires    map[string]bool
}

// RenderUnstable builds and formats package unstable's generated file from m,
// the model of load.Unstable's inputs. It returns no file when u adds
// nothing.
func RenderUnstable(m *model.Model, u Unstable) ([]File, error) {
	if len(u.Local) == 0 && len(u.Variants) == 0 && len(u.Wires) == 0 {
		return nil, nil
	}
	src, err := unstableFile(m, u).render()
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", UnstableDir, unstableFileName, err)
	}
	return []File{{Dir: UnstableDir, Name: unstableFileName, Src: src}}, nil
}

// unstableFile builds unstable_gen.go: the new methods' constants; each new
// definition's type, with its nested and inline types and its constructors;
// and for each new variant of a stable union its inline type and its
// Decode function. Every other name is acp's.
func unstableFile(m *model.Model, u Unstable) *goFile {
	f := newFile()
	f.pkg, f.root, f.local = unstablePackage, u.Root, map[string]bool{}
	for _, t := range m.Types {
		if !u.Local[t.Name] {
			continue
		}
		f.local[t.Name] = true
		for _, ns := range t.NestedStructs {
			f.local[ns.Name] = true
		}
		for _, v := range t.Variants {
			if v.Inline != nil {
				f.local[v.TypeName] = true
			}
		}
	}
	unions := slices.Sorted(maps.Keys(u.Variants))
	for _, name := range unions {
		t := m.Type(name)
		for _, i := range u.Variants[name] {
			if v := t.Variants[i]; v.Inline != nil {
				f.local[v.TypeName] = true
			}
		}
	}
	f.keyDecoders = map[string]bool{}
	for name := range keyDecoders(m) {
		if f.local[name] {
			f.keyDecoders[name] = true
		}
	}

	var methods []*model.Method
	for _, mi := range slices.Concat(m.AgentMethods, m.ClientMethods, m.Protocol) {
		if u.Wires[mi.Wire] {
			methods = append(methods, mi)
		}
	}
	if len(methods) > 0 {
		f.comment("Method names of the unstable methods acp does not declare: acp's",
			"dispatch answers a received one as a method it does not know.")
		f.add(methodConsts(methods))
		f.line()
	}
	for _, t := range m.Types {
		if u.Local[t.Name] {
			emitSchemaDef(f, t)
		}
	}
	for _, t := range m.Types {
		if u.Local[t.Name] {
			emitHelperConstructors(f, t)
		}
	}
	for _, name := range unions {
		t := m.Type(name)
		for _, i := range u.Variants[name] {
			vi := t.Variants[i]
			if vi.Skip {
				continue
			}
			emitInlineVariant(f, vi.TypeName, schemaVariantPlace(t, vi), vi.Inline, nil)
			emitDecodeVariant(f, t, vi)
		}
	}
	emitDecodeHelpers(f)
	return f
}

// emitDecodeVariant emits Decode<Union><Field>, which decodes the payload of
// a variant of acp's union t that acp does not declare, such as the Unknown
// of a discriminated union, into the variant's type. For a variant with a
// discriminator value, a payload whose discriminator is another is refused.
func emitDecodeVariant(f *goFile, t *model.Type, vi *model.Variant) {
	name := "Decode" + t.Name + vi.FieldName
	typ := f.typeName(vi.TypeName)
	place := t.Name + " variant " + vi.FieldName
	body := []ast.Stmt{varStmt("v", f.typeName(vi.TypeName))}
	if t.DiscKey != "" && vi.DiscValue != "" {
		tag := &ast.StructType{Fields: fields(structField("Value", id("string"), t.DiscKey))}
		body = append(body,
			varStmt("tag", tag),
			ifStmt(define(exprs(id("err")), call(f.qual(pkgJSON, "Unmarshal"), id("raw"), addr(id("tag")))), errNotNil(),
				ret(id("v"), call(f.qual(pkgFmt, "Errorf"), str(place+": %w"), id("err")))),
			ifStmt(nil, binary(dot("tag", "Value"), token.NEQ, str(vi.DiscValue)),
				ret(id("v"), call(f.qual(pkgFmt, "Errorf"),
					str(place+": "+t.DiscKey+" is %q, not "+fmt.Sprintf("%q", vi.DiscValue)), dot("tag", "Value")))))
	}
	body = append(body,
		ifStmt(define(exprs(id("err")), call(f.qual(pkgJSON, "Unmarshal"), id("raw"), addr(id("v")))), errNotNil(),
			ret(id("v"), call(f.qual(pkgFmt, "Errorf"), str(place+": %w"), id("err")))),
		ret(id("v"), nilExpr()))
	which := vi.FieldName
	if vi.DiscValue != "" {
		which = fmt.Sprintf("%q", vi.DiscValue)
	}
	f.comment(name+" decodes raw, the payload of the "+which+" variant of acp's "+t.Name+",",
		"which acp does not declare, into its type. For a discriminated union, raw is",
		"its Unknown (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, question 5).")
	f.add(function(name, funcType([]*ast.Field{param("raw", f.qual(pkgJSON, "RawMessage"))}, param("", typ), param("", id("error"))),
		body...))
	f.line()
}
