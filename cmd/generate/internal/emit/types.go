package emit

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/ir"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/util"
)

// splitCamelCase splits a CamelCase string into words.
// Example: "RequestPermissionRequest" -> ["Request", "Permission", "Request"]
func splitCamelCase(s string) []string {
	if s == "" {
		return nil
	}
	var words []string
	var current strings.Builder

	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			// Start new word on uppercase letter
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	return words
}

// generateNestedTypeName applies multiple heuristics to create idiomatic nested type names
// while guaranteeing stability (name depends only on parent, field, and collision check).
//
// If a collision is detected (e.g. the schema already defines a type with the same name we
// would otherwise generate), fall back to a deterministic suffixed name. Panics only if it
// cannot find a unique name (which would indicate a codegen bug).
func generateNestedTypeName(parentName, rawFieldName string, usedNames map[string]bool) string {
	// Heuristic 1: Ensure field name is properly capitalized first
	fieldName := util.ToExportedField(rawFieldName)

	// Heuristic 2: Try RPC suffix stripping for Request/Response/Notification parents
	// This handles cases like RequestPermissionRequest + ToolCall → RequestPermissionToolCall
	for _, suffix := range []string{"Request", "Response", "Notification"} {
		if baseName, ok := strings.CutSuffix(parentName, suffix); ok && baseName != "" {
			candidate := baseName + fieldName
			if !usedNames[candidate] {
				return candidate
			}
		}
	}

	// Heuristic 3: Try word-boundary deduplication
	// This handles cases like AgentNotification + ExtNotification → AgentExtNotification
	parentWords := splitCamelCase(parentName)
	fieldWords := splitCamelCase(fieldName)

	// Need at least 2 words in parent to consider deduplication
	if len(parentWords) >= 2 && len(fieldWords) > 0 {
		lastParentWord := parentWords[len(parentWords)-1]
		firstFieldWord := fieldWords[0]
		lastFieldWord := fieldWords[len(fieldWords)-1]

		if strings.EqualFold(lastParentWord, firstFieldWord) || strings.EqualFold(lastParentWord, lastFieldWord) {
			// Remove last word from parent, then concatenate
			parentWithoutLast := strings.Join(parentWords[:len(parentWords)-1], "")
			if parentWithoutLast != "" {
				candidate := parentWithoutLast + fieldName
				if !usedNames[candidate] {
					return candidate
				}
			}
		}
	}

	// Heuristic 4: Fall back to full concatenation
	fullName := parentName + fieldName
	if !usedNames[fullName] {
		return fullName
	}

	// Collision: this can happen when the schema already defines a top-level type with the
	// same name as an inline property we'd otherwise generate.
	base := fullName + "Inline"
	if !usedNames[base] {
		return base
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s%d", base, i)
		if !usedNames[candidate] {
			return candidate
		}
	}

	// DEFENSIVE PROGRAMMING: If we get here, something is deeply wrong with name tracking.
	panic(fmt.Sprintf("unable to generate unique nested type name (parent: %q, field: %q)", parentName, rawFieldName))
}

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
func WriteTypes(outDir string, schema *load.Schema, meta *load.Meta) error {
	f := newFile()

	// Track all type names to avoid collisions when generating nested types
	usedTypeNames := make(map[string]bool, len(schema.Defs))
	for k := range schema.Defs {
		usedTypeNames[k] = true
	}
	for _, name := range slices.Sorted(maps.Keys(schema.Defs)) {
		emitSchemaDef(f, name, schema.Defs[name], schema, usedTypeNames)
	}
	emitRPCInterfaces(f, schema, meta)

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
func addRPCMethod(f *goFile, target *fieldSet, schema *load.Schema, methodKey string, mi *ir.MethodInfo) {
	if desc := methodDescription(schema, mi); desc != "" {
		target.comment(util.FormatDocComment(desc)...)
	}
	switch {
	case mi.Notif != "":
		target.add(interfaceMethod(f, ir.DispatchMethodNameForNotification(methodKey, mi.Notif), mi.Notif, ""))
	case mi.Req != "":
		methodName := strings.TrimSuffix(mi.Req, "Request")
		respName := methodName + "Response"
		if ir.IsNullResponse(schema.Defs[respName]) {
			respName = ""
		}
		target.add(interfaceMethod(f, methodName, mi.Req, respName))
	}
}

func emitRPCInterfaces(f *goFile, schema *load.Schema, meta *load.Meta) {
	// Append Agent & Client interfaces from method groups
	groups := ir.BuildMethodGroups(schema, meta)

	var agent, agentLoader, agentExperimental fieldSet
	for _, k := range ir.SortedKeys(meta.AgentMethods) {
		mi := groups["agent|"+meta.AgentMethods[k]]
		if mi == nil {
			continue
		}
		target := &agent
		switch mi.Binding {
		case ir.BindAgentLoader:
			target = &agentLoader
		case ir.BindAgentExperimental:
			target = &agentExperimental
		}
		addRPCMethod(f, target, schema, k, mi)
	}
	f.add(typeDecl("Agent", agent.interfaceType()))
	if len(agentLoader.list) > 0 {
		f.comment("AgentLoader defines optional support for loading sessions. Implement and advertise the capability to enable 'session/load'.")
		f.add(typeDecl("AgentLoader", agentLoader.interfaceType()))
	}
	// Always emit AgentExperimental, even if empty.
	f.comment("AgentExperimental defines unstable methods that are not part of the official spec. These may change or be removed without notice.")
	f.add(typeDecl("AgentExperimental", agentExperimental.interfaceType()))

	var client, clientExperimental, clientTerminal fieldSet
	for _, k := range ir.SortedKeys(meta.ClientMethods) {
		mi := groups["client|"+meta.ClientMethods[k]]
		if mi == nil {
			continue
		}
		target := &client
		switch mi.Binding {
		case ir.BindClientExperimental:
			target = &clientExperimental
		case ir.BindClientTerminal:
			target = &clientTerminal
		}
		addRPCMethod(f, target, schema, k, mi)
	}
	f.add(typeDecl("Client", client.interfaceType()))
	if len(clientTerminal.list) > 0 {
		f.comment("ClientTerminal defines terminal-related experimental methods (x-docs-ignore). Implement and advertise 'terminal: true' to enable 'terminal/*'.")
		f.add(typeDecl("ClientTerminal", clientTerminal.interfaceType()))
	}
	// Always emit ClientExperimental, even if empty.
	f.comment("ClientExperimental defines unstable methods that are not part of the official spec. These may change or be removed without notice.")
	f.add(typeDecl("ClientExperimental", clientExperimental.interfaceType()))
}

// enumConsts declares <name><Value> name = "value" for each value.
func enumConsts(f *goFile, name string, values []string) {
	if len(values) == 0 {
		return
	}
	var specs []*ast.ValueSpec
	for _, s := range values {
		specs = append(specs, constSpec(util.ToEnumConst(name, s), id(name), str(s)))
	}
	f.add(constDecl(true, specs...))
}

func emitSchemaDef(f *goFile, name string, def *load.Definition, schema *load.Schema, usedTypeNames map[string]bool) {
	if def == nil {
		return
	}

	// DEFENSIVE PROGRAMMING: Ensure we're not about to emit a type that conflicts
	// with a previously generated nested type. This should never happen since we
	// pre-populate usedTypeNames with all schema definitions.
	if usedTypeNames[name] && name != "" {
		if _, inSchema := schema.Defs[name]; !inSchema {
			panic(fmt.Sprintf("BUG: attempting to emit type %q that was already generated as a nested type", name))
		}
	}

	if def.Description != "" {
		f.comment(util.FormatDocComment(def.Description)...)
	}

	switch {
	case len(def.Enum) > 0:
		f.add(typeDecl(name, id("string")))
		var values []string
		for _, v := range def.Enum {
			values = append(values, fmt.Sprint(v))
		}
		enumConsts(f, name, values)
		f.line()
	case isStringConstUnion(def):
		f.add(typeDecl(name, id("string")))
		var values []string
		for _, v := range def.OneOf {
			if v != nil && v.Const != nil {
				values = append(values, fmt.Sprint(v.Const))
			}
		}
		enumConsts(f, name, values)
		f.line()
	case len(def.AnyOf) > 0 && isOpenStringEnum(def):
		// "Open" string enum: `anyOf` of string consts plus (typically) a
		// free-form string catch-all. Emit as a named string type with
		// constants for the known values. Unknown values are still
		// representable as strings, matching the schema's extensibility.
		f.add(typeDecl(name, id("string")))
		var values []string
		for _, v := range def.AnyOf {
			if v == nil || v.Const == nil {
				continue
			}
			if s, ok := v.Const.(string); ok {
				values = append(values, s)
			}
		}
		enumConsts(f, name, values)
		f.line()
	case len(def.AnyOf) > 0:
		emitUnion(f, name, schema, def, def.AnyOf, false, usedTypeNames)
	case len(def.OneOf) > 0 && !isStringConstUnion(def):
		// Generic union generation for non-enum oneOf
		// Use the same implementation, but require exactly one variant
		emitUnion(f, name, schema, def, def.OneOf, true, usedTypeNames)
	case ir.PrimaryType(def) == schemaTypeObject && len(def.Properties) > 0:
		emitObjectSchema(f, name, def, usedTypeNames)
	case slices.Contains([]string{schemaTypeString, schemaTypeInteger, schemaTypeNumber, schemaTypeBoolean}, ir.PrimaryType(def)):
		f.add(typeDecl(name, primitiveType(ir.PrimaryType(def))))
		f.line()
	case ir.PrimaryType(def) == schemaTypeObject && len(def.Properties) == 0:
		// Empty object shape: emit a concrete empty struct so methods can be defined
		// and the wire encoding is consistently {} rather than null.
		f.add(typeDecl(name, &ast.StructType{Fields: fields()}))
		f.line()
	default:
		f.comment(fmt.Sprintf("%s is a union or complex schema; represented generically.", name))
		f.add(typeDecl(name, id("any")))
		f.line()
	}

	// Validators for selected types. Union wrappers get Validate from
	// emitUnion; emitValidate reads properties on the wrapper, which is the
	// wrong shape for those types.
	emittedUnion := (len(def.OneOf) > 0 && !isStringConstUnion(def)) || (len(def.AnyOf) > 0 && !isOpenStringEnum(def))
	if !emittedUnion && (isRPCTypeName(name) || name == "ToolCallUpdate") {
		emitValidate(f, name, def)
	}
}

func isRPCTypeName(name string) bool {
	return strings.HasSuffix(name, "Request") || strings.HasSuffix(name, "Response") || strings.HasSuffix(name, "Notification")
}

// requiredSet returns the names in required as a set.
func requiredSet(required []string) map[string]bool {
	set := make(map[string]bool, len(required))
	for _, r := range required {
		set[r] = true
	}
	return set
}

func emitObjectSchema(f *goFile, name string, def *load.Definition, usedTypeNames map[string]bool) {
	req := requiredSet(def.Required)
	pkeys := slices.Sorted(maps.Keys(def.Properties))

	// Pre-generate nested struct types for inline object properties
	nestedTypes := emitInlineNestedTypes(f, name, pkeys, def, usedTypeNames)

	// Track fields with schema defaults for generic (de)serialization
	var defaults []defaultProp
	var fills []requiredFill
	var st fieldSet
	for _, pk := range pkeys {
		prop := def.Properties[pk]
		field := util.ToExportedField(pk)
		if prop.Description != "" {
			st.comment(util.FormatDocComment(prop.Description)...)
		}
		tag := pk
		var dp *defaultProp
		if prop.Default != nil {
			// Whether field zero is nil (slice/map) for Marshal fill-in
			nilable := ir.PrimaryType(prop) == schemaTypeArray || (ir.PrimaryType(prop) == schemaTypeObject && len(prop.Properties) == 0 && prop.Ref == "")
			// Capture canonical JSON of default
			defJSON := schemaTypeNull
			if b, err := json.Marshal(prop.Default); err == nil {
				defJSON = string(b)
			}
			dp = &defaultProp{
				fieldName:   field,
				propName:    pk,
				defaultJSON: defJSON,
				kind:        defaultKindOf(prop.Default),
				allowNull:   includesNull(prop),
				nilable:     nilable,
			}
			defaults = append(defaults, *dp)
		}
		if !req[pk] {
			// Default: omit if empty for optional fields.
			// Keep always-present behavior only for defaults where the zero value is nil (slice/map).
			// For typed object defaults (non-nilable), still allow omission on the wire.
			if dp == nil || (dp.kind != kindArray && dp.kind != kindObject) || !dp.nilable {
				tag = pk + ",omitempty"
			}
		}
		// Emit an additional comment line indicating the default, if any.
		if dp != nil && dp.defaultJSON != schemaTypeNull {
			// An empty comment line separates the default from the description.
			if prop.Description != "" {
				st.comment("")
			}
			st.comment(fmt.Sprintf("Defaults to %s if unset.", dp.defaultJSON))
		}
		var fieldType ast.Expr
		if nestedTypeName, ok := nestedTypes[pk]; ok {
			fieldType = id(nestedTypeName)
		} else {
			fieldType = optionalTypeExpr(prop)
			if req[pk] && dp == nil {
				fills = appendRequiredFill(fills, field, prop)
			}
		}
		st.add(structField(field, fieldType, tag))
	}
	f.add(typeDecl(name, st.structType()))
	f.line()

	emitObjectDefaults(f, name, defaults, fills)
}

// requiredFill is a required slice or map field. Its MarshalJSON encodes nil
// as an empty literal of typ, because the schema does not accept null there.
type requiredFill struct {
	fieldName string
	typ       ast.Expr
}

// appendRequiredFill appends a fill for prop when its Go type is a slice or a
// map. typ is built afresh so the struct field and the literal share no node.
func appendRequiredFill(fills []requiredFill, field string, prop *load.Definition) []requiredFill {
	switch t := optionalTypeExpr(prop).(type) {
	case *ast.ArrayType:
		if t.Len == nil {
			return append(fills, requiredFill{fieldName: field, typ: t})
		}
	case *ast.MapType:
		return append(fills, requiredFill{fieldName: field, typ: t})
	}
	return fills
}

func emitInlineNestedTypes(f *goFile, name string, pkeys []string, def *load.Definition, usedTypeNames map[string]bool) map[string]string {
	nestedTypes := map[string]string{}
	for _, pk := range pkeys {
		prop := def.Properties[pk]
		// Detect inline objects: no $ref, type is object, has properties
		if prop.Ref != "" || ir.PrimaryType(prop) != schemaTypeObject || len(prop.Properties) == 0 {
			continue
		}
		nestedTypeName := generateNestedTypeName(name, pk, usedTypeNames)
		nestedTypes[pk] = nestedTypeName

		// DEFENSIVE PROGRAMMING: Assert no collision before registering
		if usedTypeNames[nestedTypeName] {
			panic(fmt.Sprintf("BUG: nested type name collision: %q already registered", nestedTypeName))
		}
		usedTypeNames[nestedTypeName] = true

		if prop.Description != "" {
			f.comment(util.FormatDocComment(prop.Description)...)
		}
		nestedReq := requiredSet(prop.Required)
		var fills []requiredFill
		var st fieldSet
		for _, npk := range slices.Sorted(maps.Keys(prop.Properties)) {
			nprop := prop.Properties[npk]
			if nprop.Description != "" {
				st.comment(util.FormatDocComment(nprop.Description)...)
			}
			ntag := npk
			if !nestedReq[npk] {
				ntag = npk + ",omitempty"
			} else {
				fills = appendRequiredFill(fills, util.ToExportedField(npk), nprop)
			}
			st.add(structField(util.ToExportedField(npk), optionalTypeExpr(nprop), ntag))
		}
		f.add(typeDecl(nestedTypeName, st.structType()))
		f.line()
		emitObjectDefaults(f, nestedTypeName, nil, fills)
	}
	return nestedTypes
}

type defaultKind int

const (
	kindNone defaultKind = iota
	kindScalar
	kindArray
	kindObject
)

type defaultProp struct {
	fieldName   string
	propName    string
	defaultJSON string
	kind        defaultKind
	allowNull   bool
	nilable     bool // whether zero-value is nil (slice/map)
}

// emitObjectDefaults synthesizes MarshalJSON for structs with fields that
// carry schema defaults or are required slices or maps, and UnmarshalJSON for
// structs with defaults.
func emitObjectDefaults(f *goFile, name string, defaults []defaultProp, fills []requiredFill) {
	if len(defaults) == 0 && len(fills) == 0 {
		return
	}
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }

	// MarshalJSON: coerce nil slices and maps to their defaults, and nil
	// required ones to empty, before encoding.
	body := []ast.Stmt{
		typeStmt("Alias", id(name)),
		varStmt("a", id("Alias"), nil),
		assign(exprs(id("a")), call(id("Alias"), id("v"))),
	}
	for _, dp := range defaults {
		// Typed object defaults (non-nilable) are not injected on encode.
		if (dp.kind == kindArray || dp.kind == kindObject) && dp.nilable {
			body = append(body, ifStmt(nil, binary(dot("a", dp.fieldName), token.EQL, nilExpr()),
				exprStmt(unmarshal(bytesOf(dp.defaultJSON), addr(dot("a", dp.fieldName)))),
			))
		}
	}
	for _, rf := range fills {
		body = append(body, ifStmt(nil, binary(dot("a", rf.fieldName), token.EQL, nilExpr()),
			assign(exprs(dot("a", rf.fieldName)), composite(rf.typ)),
		))
	}
	body = append(body, ret(call(f.qual(pkgJSON, "Marshal"), id("a"))))
	f.add(method(param("v", id(name)), "MarshalJSON",
		funcType(nil, param("", byteSlice()), param("", id("error"))), body...))
	f.line()
	if len(defaults) == 0 {
		return
	}

	// UnmarshalJSON: apply defaults when a field is missing, or null when
	// the schema does not allow null.
	body = []ast.Stmt{
		varStmt("m", rawMap(f.qual(pkgJSON, "RawMessage")), nil),
		ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("m")))), errNotNil(), ret(id("err"))),
		typeStmt("Alias", id(name)),
		varStmt("a", id("Alias"), nil),
		ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("a")))), errNotNil(), ret(id("err"))),
	}
	for _, dp := range defaults {
		var cond ast.Expr = not(id("_ok"))
		if !dp.allowNull {
			cond = binary(not(id("_ok")), token.LOR, paren(binary(call(id("string"), id("_rm")), token.EQL, str(schemaTypeNull))))
		}
		body = append(body, block(
			define(exprs(id("_rm"), id("_ok")), index(id("m"), str(dp.propName))),
			ifStmt(nil, cond, exprStmt(unmarshal(bytesOf(dp.defaultJSON), addr(dot("a", dp.fieldName))))),
		))
	}
	body = append(body,
		assign(exprs(star(id("v"))), call(id(name), id("a"))),
		ret(nilExpr()),
	)
	f.add(method(param("v", star(id(name))), "UnmarshalJSON",
		funcType([]*ast.Field{param("b", byteSlice())}, param("", id("error"))), body...))
	f.line()
}

// emitInlineUnionStruct emits the type for an inline (non-$ref) union variant.
func emitInlineUnionStruct(f *goFile, name, tname, ref string, v *load.Definition, isObj, isNull bool, sharedProps map[string]*load.Definition, sharedRequired map[string]bool, usedTypeNames map[string]bool) {
	if ref != "" || (!isObj && !isNull && v.Title == "") {
		return
	}
	// DEFENSIVE PROGRAMMING: Verify tname is registered before emitting
	if !usedTypeNames[tname] {
		panic(fmt.Sprintf("BUG: attempting to emit unregistered type: %q (parent union: %q)", tname, name))
	}

	var st fieldSet
	var fills []requiredFill
	switch {
	case !isNull && isObj:
		req := requiredSet(v.Required)
		maps.Copy(req, sharedRequired)
		mergedProps := make(map[string]*load.Definition, len(sharedProps)+len(v.Properties))
		maps.Copy(mergedProps, sharedProps)
		maps.Copy(mergedProps, v.Properties)
		if v.Description != "" {
			f.comment(util.FormatDocComment(v.Description)...)
		}
		for _, pk := range slices.Sorted(maps.Keys(mergedProps)) {
			pDef := mergedProps[pk]
			if pDef.Description != "" {
				st.comment(util.FormatDocComment(pDef.Description)...)
			}
			tag := pk
			if !req[pk] {
				tag = pk + ",omitempty"
			} else {
				fills = appendRequiredFill(fills, util.ToExportedField(pk), pDef)
			}
			st.add(structField(util.ToExportedField(pk), optionalTypeExpr(pDef), tag))
		}
	case !isNull && !isObj && v.Title != "":
		// Title-only variants. Extension types keep the raw payload instead
		// of dropping it.
		isExtension := strings.Contains(strings.ToLower(v.Description), "extension") || strings.Contains(v.Title, "Ext")
		if v.Description != "" {
			f.comment(util.FormatDocComment(v.Description)...)
		}
		switch {
		case isExtension:
			// An alias keeps json.RawMessage's methods, unlike a defined type.
			f.add(aliasDecl(tname, f.qual(pkgJSON, "RawMessage")))
		case ir.PrimaryType(v) != "":
			// Non-object title-only variants can still carry payloads (e.g. arrays).
			// Emit a concrete type so sequential decode can succeed for valid payloads.
			f.add(typeDecl(tname, typeExpr(v)))
		default:
			// Emit as empty struct only for truly empty variants.
			f.add(typeDecl(tname, st.structType()))
		}
		f.line()
		return
	}
	f.add(typeDecl(tname, st.structType()))
	f.line()
	emitObjectDefaults(f, tname, nil, fills)
}

// invalidPayload returns errors.New("invalid variant payload") with the given leading results.
func invalidPayload(f *goFile, lead ...ast.Expr) ast.Stmt {
	return ret(append(lead, call(f.qual(pkgErrors, "New"), str("invalid variant payload")))...)
}

// setVariant is u.<field> = &v; return nil.
func setVariant(vi variantInfo) []ast.Stmt {
	return []ast.Stmt{
		assign(exprs(dot("u", vi.fieldName)), addr(id("v"))),
		ret(nilExpr()),
	}
}

// decodeStrict decodes b into v, fails on error, and sets the variant.
func decodeStrict(f *goFile, vi variantInfo) []ast.Stmt {
	return append([]ast.Stmt{
		ifStmt(nil, binary(call(f.qual(pkgJSON, "Unmarshal"), id("b"), addr(id("v"))), token.NEQ, nilExpr()),
			invalidPayload(f)),
	}, setVariant(vi)...)
}

// keyMatch sets match = false when key is missing from m.
func keyMatch(m, key string) ast.Stmt {
	return ifStmt(define(exprs(id("_"), id("ok")), index(id(m), str(key))), not(id("ok")),
		assign(exprs(id("match")), boolLit(false)))
}

func unionUnmarshalBody(f *goFile, variants []variantInfo, discKey string) []ast.Stmt {
	unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }
	var body []ast.Stmt

	// Handle literal null with the first null-only variant.
	for _, vi := range variants {
		if vi.isNull {
			body = append(body, ifStmt(nil, binary(call(id("string"), id("b")), token.EQL, str(schemaTypeNull)),
				append([]ast.Stmt{varStmt("v", id(vi.typeName), nil)}, setVariant(vi)...)...))
			break
		}
	}

	var object []ast.Stmt
	// Prefer discriminator-based dispatch when available (e.g. "type", "outcome").
	if discKey != "" {
		var cases []ast.Stmt
		for _, vi := range variants {
			if vi.discValue != "" {
				cases = append(cases, caseOf(exprs(str(vi.discValue)),
					append([]ast.Stmt{varStmt("v", id(vi.typeName), nil)}, decodeStrict(f, vi)...)...))
			}
		}
		object = append(object, block(
			varStmt("disc", id("string"), nil),
			ifStmt(define(exprs(id("v"), id("ok")), index(id("m"), str(discKey))), id("ok"),
				exprStmt(unmarshal(id("v"), addr(id("disc"))))),
			switchStmt(id("disc"), cases...),
		))
	}
	// Required-key match.
	for _, vi := range variants {
		if !vi.isObject || len(vi.required) == 0 {
			continue
		}
		stmts := []ast.Stmt{
			varStmt("v", id(vi.typeName), nil),
			varStmt("match", id("bool"), boolLit(true)),
		}
		for _, rk := range vi.required {
			stmts = append(stmts, keyMatch("m", rk))
		}
		stmts = append(stmts, ifStmt(nil, id("match"), decodeStrict(f, vi)...))
		object = append(object, block(stmts...))
	}
	body = append(body,
		varStmt("m", rawMap(f.qual(pkgJSON, "RawMessage")), nil),
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
		if !vi.isArray || len(vi.arrayItemRequired) == 0 {
			continue
		}
		var loop []ast.Stmt
		for _, rk := range vi.arrayItemRequired {
			loop = append(loop, keyMatch("elem", rk))
		}
		array = append(array, block(
			varStmt("v", id(vi.typeName), nil),
			varStmt("match", id("bool"), boolLit(true)),
			rangeStmt(id("_"), id("elem"), id("arr"), loop...),
			ifStmt(nil, id("match"), decodeStrict(f, vi)...),
		))
	}
	body = append(body,
		varStmt("arr", sliceOf(rawMap(f.qual(pkgJSON, "RawMessage"))), nil),
		ifStmt(nil, binary(unmarshal(id("b"), addr(id("arr"))), token.EQL, nilExpr()), array...),
	)

	// Fallback: try each variant in turn.
	for _, vi := range variants {
		body = append(body, block(
			varStmt("v", id(vi.typeName), nil),
			ifStmt(nil, binary(unmarshal(id("b"), addr(id("v"))), token.EQL, nilExpr()), setVariant(vi)...),
		))
	}
	return append(body, ret(call(f.qual(pkgErrors, "New"), str("no matching variant for union"))))
}

const keyMimeType = "mimeType"

// contentBlockShape lists the keys a ContentBlock variant keeps on the wire:
// required keys always, optional keys only when present.
var contentBlockShape = map[string]struct{ required, optional []string }{
	"text":          {required: []string{"text"}},
	"image":         {required: []string{"data", keyMimeType}, optional: []string{"uri"}},
	"audio":         {required: []string{"data", keyMimeType}},
	"resource_link": {required: []string{"name", "uri"}, optional: []string{"description", keyMimeType, "size", "title"}},
	"resource":      {required: []string{"resource"}},
}

// contentBlockMarshal rebuilds the variant map with only the keys of its
// wire shape, to preserve exact ContentBlock JSON.
func contentBlockMarshal(f *goFile, disc string) ast.Stmt {
	shape, ok := contentBlockShape[disc]
	if !ok {
		return nil
	}
	nm := func(key string) ast.Expr { return index(id("nm"), str(key)) }
	stmts := []ast.Stmt{
		varStmt("nm", rawMap(id("any")), nil),
		assign(exprs(id("nm")), call(id("make"), rawMap(id("any")))),
		assign(exprs(nm("type")), str(disc)),
	}
	for _, k := range shape.required {
		stmts = append(stmts, assign(exprs(nm(k)), index(id("m"), str(k))))
	}
	for i, k := range shape.optional {
		// The single optional key of image uses _v/_ok; resource_link numbers them.
		v, okName := "_v", "_ok"
		if disc == "resource_link" {
			v, okName = fmt.Sprintf("v%d", i+1), fmt.Sprintf("ok%d", i+1)
		}
		stmts = append(stmts, ifStmt(define(exprs(id(v), id(okName)), index(id("m"), str(k))), id(okName),
			assign(exprs(nm(k)), id(v))))
	}
	stmts = append(stmts, ret(call(f.qual(pkgJSON, "Marshal"), id("nm"))))
	return block(stmts...)
}

func unionMarshalBody(f *goFile, name string, variants []variantInfo) []ast.Stmt {
	marshal := func(x ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Marshal"), x) }
	var body []ast.Stmt
	for _, vi := range variants {
		var then []ast.Stmt
		switch {
		case vi.isNull:
			// Null-only variant encodes to JSON null.
			then = []ast.Stmt{ret(marshal(nilExpr()))}
		case !vi.isObject:
			// Non-object variants (e.g., arrays/primitives) are already in final wire shape.
			then = []ast.Stmt{
				define(exprs(id("_b"), id("_e")), marshal(star(dot("u", vi.fieldName)))),
				ifStmt(nil, binary(id("_e"), token.NEQ, nilExpr()), ret(emptyBytes(), id("_e"))),
				ret(id("_b"), nilExpr()),
			}
		default:
			then = []ast.Stmt{
				define(exprs(id("_b"), id("_e")), marshal(star(dot("u", vi.fieldName)))),
				ifStmt(nil, binary(id("_e"), token.NEQ, nilExpr()), ret(emptyBytes(), id("_e"))),
				// Marshal object variant to map for discriminant injection and shaping.
				varStmt("m", rawMap(id("any")), nil),
				ifStmt(nil, binary(call(f.qual(pkgJSON, "Unmarshal"), id("_b"), addr(id("m"))), token.NEQ, nilExpr()),
					invalidPayload(f, emptyBytes())),
			}
			// Inject const discriminants.
			for _, kv := range vi.constPairs {
				then = append(then, assign(exprs(index(id("m"), str(kv[0]))), str(kv[1])))
			}
			if name == unionContentBlock {
				// Special shaping for ContentBlock variants to preserve exact wire JSON.
				if s := contentBlockMarshal(f, vi.discValue); s != nil {
					then = append(then, s)
				}
			} else {
				// Default: remarshal, possibly with an injected discriminant.
				then = append(then, ret(marshal(id("m"))))
			}
		}
		body = append(body, ifStmt(nil, binary(dot("u", vi.fieldName), token.NEQ, nilExpr()), then...))
	}
	return append(body, ret(nilExpr(), call(f.qual(pkgErrors, "New"), str(name+": no variant set"))))
}

type sharedFieldCheck struct {
	field string
	prop  string
	kind  string
}

func sharedFieldChecks(def *load.Definition) []sharedFieldCheck {
	if def == nil {
		return nil
	}
	var out []sharedFieldCheck
	for _, propName := range slices.Sorted(slices.Values(def.Required)) {
		pDef := def.Properties[propName]
		if pDef == nil {
			continue
		}
		switch kind := ir.PrimaryType(pDef); kind {
		case schemaTypeString, schemaTypeArray:
			out = append(out, sharedFieldCheck{field: util.ToExportedField(propName), prop: propName, kind: kind})
		}
	}
	return out
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

func sharedUnionValidateBody(f *goFile, variants []variantInfo, checks []sharedFieldCheck) []ast.Stmt {
	carriers := 0
	for _, vi := range variants {
		if vi.hasSharedFields {
			carriers++
		}
	}
	if len(checks) == 0 || carriers == 0 {
		return []ast.Stmt{ret(nilExpr())}
	}
	body := []ast.Stmt{varStmt("set", id("bool"), nil)}
	for _, vi := range variants {
		if !vi.hasSharedFields {
			continue
		}
		var then []ast.Stmt
		for _, c := range checks {
			then = append(then, requiredCheck(f, dot("v", vi.fieldName, c.field), c.kind, c.prop))
		}
		then = append(then, assign(exprs(id("set")), boolLit(true)))
		body = append(body, ifStmt(nil, binary(dot("v", vi.fieldName), token.NEQ, nilExpr()), then...))
	}
	return append(body,
		ifStmt(nil, not(id("set")), ret(call(f.qual(pkgFmt, "Errorf"), str(checks[0].prop+" is required")))),
		ret(nilExpr()),
	)
}

func unionValidateBody(f *goFile, name string, variants []variantInfo) []ast.Stmt {
	body := []ast.Stmt{varStmt("count", id("int"), nil)}
	for _, vi := range variants {
		body = append(body, ifStmt(nil, binary(dot("u", vi.fieldName), token.NEQ, nilExpr()), incr(id("count"))))
	}
	return append(body,
		ifStmt(nil, binary(id("count"), token.NEQ, intLit(1)),
			ret(call(f.qual(pkgErrors, "New"), str(name+" must have exactly one variant set")))),
		ret(nilExpr()),
	)
}

func isStringConstUnion(def *load.Definition) bool {
	if def == nil || len(def.OneOf) == 0 {
		return false
	}
	for _, v := range def.OneOf {
		if v == nil || v.Const == nil {
			return false
		}
		if _, ok := v.Const.(string); !ok {
			return false
		}
	}
	return true
}

// isOpenStringEnum reports whether def is an "open" string enum: an `anyOf`
// where every variant is a plain string schema (type: "string") and at least
// one variant has a string `const`. Such schemas describe a string value with
// a recommended set of known values while still permitting arbitrary strings
// for extensibility (see e.g. SessionConfigOptionCategory).
//
// These are emitted as a Go string-derived type with named constants for each
// known value, rather than as a discriminated union wrapper.
func isOpenStringEnum(def *load.Definition) bool {
	if def == nil || len(def.AnyOf) == 0 {
		return false
	}
	sawConst := false
	for _, v := range def.AnyOf {
		if v == nil {
			return false
		}
		// Each variant must be a plain string schema: type "string", no nested
		// structure (properties, items, refs, nested unions, allOf, etc.).
		if ir.PrimaryType(v) != schemaTypeString {
			return false
		}
		if v.Ref != "" || len(v.Properties) > 0 || v.Items != nil ||
			len(v.AnyOf) > 0 || len(v.OneOf) > 0 || len(v.AllOf) > 0 ||
			len(v.Enum) > 0 {
			return false
		}
		if v.Const != nil {
			if _, ok := v.Const.(string); !ok {
				return false
			}
			sawConst = true
		}
	}
	return sawConst
}

// validateMethod is func (recv *name) Validate() error { body }.
func validateMethod(recv, name string, body ...ast.Stmt) *ast.FuncDecl {
	return method(param(recv, star(id(name))), "Validate", funcType(nil, param("", id("error"))), body...)
}

// emitValidate generates Validate for RPC object types and ToolCallUpdate.
func emitValidate(f *goFile, name string, def *load.Definition) {
	if name == "ToolCallUpdate" {
		f.add(validateMethod("t", name,
			requiredCheck(f, dot("t", "ToolCallId"), schemaTypeString, "toolCallId"),
			ret(nilExpr()),
		))
		return
	}
	if def == nil || ir.PrimaryType(def) != schemaTypeObject || !isRPCTypeName(name) {
		return
	}
	var body []ast.Stmt
	for _, propName := range slices.Sorted(maps.Keys(def.Properties)) {
		if !slices.Contains(def.Required, propName) {
			continue
		}
		switch kind := ir.PrimaryType(def.Properties[propName]); kind {
		case schemaTypeString, schemaTypeArray:
			body = append(body, requiredCheck(f, dot("v", util.ToExportedField(propName)), kind, propName))
		}
	}
	f.add(validateMethod("v", name, append(body, ret(nilExpr()))...))
}

func methodDescription(schema *load.Schema, mi *ir.MethodInfo) string {
	if mi == nil || mi.DocsIgnored {
		return ""
	}
	// Prefer notification descriptions when present, otherwise fall back to request/response types.
	for _, name := range []string{mi.Notif, mi.Req, mi.Resp} {
		if name == "" {
			continue
		}
		if def := schema.Defs[name]; def != nil && def.Description != "" {
			return def.Description
		}
	}
	return ""
}

// primitiveType maps a JSON Schema primitive type to its Go type.
func primitiveType(t string) ast.Expr {
	switch t {
	case schemaTypeString:
		return id("string")
	case schemaTypeInteger:
		return id("int")
	case schemaTypeNumber:
		return id("float64")
	case schemaTypeBoolean:
		return id("bool")
	default:
		return id("any")
	}
}

// defaultKindOf classifies the JSON Schema default value into a coarse kind.
func defaultKindOf(val any) defaultKind {
	switch val.(type) {
	case nil:
		return kindNone
	case []any:
		return kindArray
	case map[string]any:
		return kindObject
	case string, float64, bool:
		return kindScalar
	default:
		// Fallback: classify by fmt string
		s := fmt.Sprint(val)
		if strings.HasPrefix(s, "[") {
			return kindArray
		}
		if strings.HasPrefix(s, "map[") || strings.HasPrefix(s, "{") {
			return kindObject
		}
		return kindScalar
	}
}

// includesNull reports whether the property's type union contains null.
func includesNull(d *load.Definition) bool {
	if d == nil {
		return false
	}
	arr, ok := d.Type.([]any)
	return ok && slices.Contains(arr, any(schemaTypeNull))
}

// expandAllOf merges JSON Schema allOf nodes into a shallow composite definition.
//
// ACP's schema frequently uses `allOf: [{"$ref": "#/$defs/Type"}]` so a property or union
// variant can attach local metadata (description/default/const constraints) while reusing
// a referenced shape.
//
// For codegen purposes we approximate allOf by merging object properties + required lists.
// We intentionally do not attempt to resolve semantic constraints beyond that.
func expandAllOf(schema *load.Schema, d *load.Definition) *load.Definition {
	if d == nil || len(d.AllOf) == 0 {
		return d
	}

	merged := *d
	// Avoid re-processing in recursive calls.
	merged.AllOf = nil
	merged.Properties = maps.Clone(d.Properties)
	merged.Required = slices.Clone(d.Required)
	reqSet := requiredSet(merged.Required)

	for _, part := range d.AllOf {
		if part == nil {
			continue
		}
		resolved := part
		if name, ok := strings.CutPrefix(part.Ref, "#/$defs/"); ok && schema != nil {
			if def := schema.Defs[name]; def != nil {
				resolved = def
			}
		}
		resolved = expandAllOf(schema, resolved)

		copyMissingAllOfFields(&merged, resolved)

		if len(resolved.Properties) > 0 {
			if merged.Properties == nil {
				merged.Properties = make(map[string]*load.Definition)
			}
			for k, v := range resolved.Properties {
				if _, ok := merged.Properties[k]; !ok {
					merged.Properties[k] = v
				}
			}
		}
		for _, r := range resolved.Required {
			if !reqSet[r] {
				merged.Required = append(merged.Required, r)
				reqSet[r] = true
			}
		}
	}

	return &merged
}

func copyMissingAllOfFields(merged *load.Definition, resolved *load.Definition) {
	if merged.Type == nil && resolved.Type != nil {
		merged.Type = resolved.Type
	}
	if merged.Items == nil && resolved.Items != nil {
		merged.Items = resolved.Items
	}
	if merged.Ref == "" && resolved.Ref != "" {
		merged.Ref = resolved.Ref
	}
	if len(merged.Enum) == 0 && len(resolved.Enum) > 0 {
		merged.Enum = resolved.Enum
	}
	if len(merged.AnyOf) == 0 && len(resolved.AnyOf) > 0 {
		merged.AnyOf = resolved.AnyOf
	}
	if len(merged.OneOf) == 0 && len(resolved.OneOf) > 0 {
		merged.OneOf = resolved.OneOf
	}
}

// typeExpr maps a schema node to a Go type expression.
func typeExpr(d *load.Definition) ast.Expr {
	if d == nil {
		return id("any")
	}

	// JSON Schema 2020-12 commonly uses allOf wrappers around a single $ref so a property can
	// attach local metadata like descriptions and defaults.
	if len(d.AllOf) > 0 && d.Ref == "" {
		// Prefer a $ref element when present, otherwise the first non-nil schema.
		for _, e := range d.AllOf {
			if e != nil && e.Ref != "" {
				return typeExpr(e)
			}
		}
		for _, e := range d.AllOf {
			if e != nil {
				return typeExpr(e)
			}
		}
	}

	if d.Ref != "" {
		if name, ok := strings.CutPrefix(d.Ref, "#/$defs/"); ok {
			return id(name)
		}
		return id("any")
	}
	if len(d.Enum) > 0 {
		return id("string")
	}
	switch t := ir.PrimaryType(d); t {
	case schemaTypeString, schemaTypeInteger, schemaTypeNumber, schemaTypeBoolean:
		return primitiveType(t)
	case schemaTypeArray:
		return sliceOf(typeExpr(d.Items))
	case schemaTypeObject:
		return rawMap(id("any"))
	default:
		return id("any")
	}
}

// pointerPrimitive returns *T for a primitive schema type, or nil.
func pointerPrimitive(t string) ast.Expr {
	switch t {
	case schemaTypeString, schemaTypeInteger, schemaTypeNumber, schemaTypeBoolean:
		return star(primitiveType(t))
	default:
		return nil
	}
}

// optionalTypeExpr maps unions that include null to pointer types where applicable.
func optionalTypeExpr(d *load.Definition) ast.Expr {
	if d == nil {
		return id("any")
	}
	// Case: property type is a union like ["string","null"]
	if arr, ok := d.Type.([]any); ok && len(arr) == 2 {
		var other string
		for _, v := range arr {
			if s, ok := v.(string); ok && s != schemaTypeNull {
				other = s
			}
		}
		if p := pointerPrimitive(other); p != nil {
			return p
		}
	}
	list := d.AnyOf
	if len(list) == 0 {
		list = d.OneOf
	}
	if len(list) == 2 {
		var nonNull *load.Definition
		for _, e := range list {
			if e == nil {
				continue
			}
			if s, ok := e.Type.(string); ok && s == schemaTypeNull {
				continue
			}
			if e.Const != nil {
				nn := *e
				nn.Type = schemaTypeString
				nonNull = &nn
			} else {
				nonNull = e
			}
		}
		if nonNull != nil {
			if name, ok := strings.CutPrefix(nonNull.Ref, "#/$defs/"); ok {
				return star(id(name))
			}
			if p := pointerPrimitive(ir.PrimaryType(nonNull)); p != nil {
				return p
			}
		}
	}
	return typeExpr(d)
}

type variantInfo struct {
	fieldName         string
	typeName          string
	required          []string
	isObject          bool
	isArray           bool
	arrayItemRequired []string
	discValue         string
	constPairs        [][2]string
	isNull            bool
	description       string
	hasSharedFields   bool
}

// singleRefVariant returns the $ref when v is only an allOf wrapper around one $ref.
func singleRefVariant(v *load.Definition) string {
	if v.Type == nil &&
		len(v.Properties) == 0 &&
		len(v.Required) == 0 &&
		len(v.Enum) == 0 &&
		v.Items == nil &&
		len(v.AnyOf) == 0 &&
		len(v.OneOf) == 0 &&
		len(v.AllOf) == 1 &&
		v.AllOf[0] != nil &&
		v.AllOf[0].Ref != "" {
		return v.AllOf[0].Ref
	}
	return ""
}

// discriminatorKey returns the schema's discriminator property, or the first
// const property found on a variant, in sorted property order.
func discriminatorKey(schema *load.Schema, parent *load.Definition, variants []*load.Definition) string {
	if parent != nil && parent.Discriminator != nil && parent.Discriminator.PropertyName != "" {
		return parent.Discriminator.PropertyName
	}
	for _, v := range variants {
		if v == nil {
			continue
		}
		v = expandAllOf(schema, v)
		for _, k := range slices.Sorted(maps.Keys(v.Properties)) {
			if pd := v.Properties[k]; pd != nil && pd.Const != nil {
				return k
			}
		}
	}
	return ""
}

// registerName records tname as used and panics on a collision.
func registerName(usedTypeNames map[string]bool, tname, context string) {
	if usedTypeNames[tname] {
		panic(fmt.Sprintf("BUG: variant type name collision: %q already registered (%s)", tname, context))
	}
	usedTypeNames[tname] = true
}

func oneUnionVariant(f *goFile, name string, schema *load.Schema, idx int, v *load.Definition, discKey string, sharedProps map[string]*load.Definition, sharedRequired map[string]bool, usedTypeNames map[string]bool) (variantInfo, bool) {
	if v == nil {
		return variantInfo{}, false
	}
	ref := v.Ref
	// If this is an allOf wrapper around a single $ref with no additional structure, treat it
	// as a $ref variant (keeps types stable and avoids duplicate structs).
	if ref == "" {
		ref = singleRefVariant(v)
	}
	v = expandAllOf(schema, v)
	// Detect null-only variant
	s, ok := v.Type.(string)
	isNull := ok && s == schemaTypeNull

	// Determine type name: prefer $ref target name when present; do not treat Title as a rename for $ref.
	tname := ""
	if after, ok := strings.CutPrefix(ref, "#/$defs/"); ok {
		tname = after
	} else if v.Title != "" {
		// Scope inline variant titles to parent union name with smart naming
		tname = generateNestedTypeName(name, v.Title, usedTypeNames)
		registerName(usedTypeNames, tname, fmt.Sprintf("parent: %q, title: %q", name, v.Title))
	} else {
		if discKey != "" {
			if pd := v.Properties[discKey]; pd != nil && pd.Const != nil {
				c := fmt.Sprint(pd.Const)
				tname = generateNestedTypeName(name, c, usedTypeNames)
				registerName(usedTypeNames, tname, fmt.Sprintf("parent: %q, discriminator: %q", name, c))
			}
		}
		if tname == "" {
			tname = fmt.Sprintf("%sVariant%d", name, idx+1)
			registerName(usedTypeNames, tname, fmt.Sprintf("parent: %q", name))
		}
	}
	// Ensure Title-derived names are exported (e.g., "stdio" -> "Stdio").
	tname = util.ToExportedField(tname)

	// Derive field name with priority: discriminator value > title > type name
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
	isObj := len(v.Properties) > 0
	isArray := ir.PrimaryType(v) == schemaTypeArray
	arrayItemRequired := []string{}
	if isArray && v.Items != nil {
		item := v.Items
		if itemName, ok := strings.CutPrefix(item.Ref, "#/$defs/"); ok {
			if d := schema.Defs[itemName]; d != nil {
				item = d
			}
		}
		item = expandAllOf(schema, item)
		if ir.PrimaryType(item) == schemaTypeObject && len(item.Required) > 0 {
			arrayItemRequired = slices.Sorted(slices.Values(item.Required))
		}
	}
	// Skip phantom variants that have neither $ref nor object shape nor null nor title
	// (but allow title-only variants like ExtMethodRequest to generate as empty structs)
	if !isObj && ref == "" && !isNull && v.Title == "" {
		return variantInfo{}, false
	}
	// Collect const properties (e.g., type, outcome).
	var consts [][2]string
	for _, pk := range slices.Sorted(maps.Keys(v.Properties)) {
		if pd := v.Properties[pk]; pd != nil && pd.Const != nil {
			if c, ok := pd.Const.(string); ok {
				consts = append(consts, [2]string{pk, c})
			}
		}
	}
	emitInlineUnionStruct(f, name, tname, ref, v, isObj, isNull, sharedProps, sharedRequired, usedTypeNames)
	return variantInfo{
		fieldName:         fieldName,
		typeName:          tname,
		required:          v.Required,
		isObject:          isObj,
		isArray:           isArray,
		arrayItemRequired: arrayItemRequired,
		discValue:         dv,
		constPairs:        consts,
		isNull:            isNull,
		description:       v.Description,
		hasSharedFields:   ref == "" && isObj && !isNull,
	}, true
}

func emitUnion(f *goFile, name string, schema *load.Schema, parentDef *load.Definition, defs []*load.Definition, exactlyOne bool, usedTypeNames map[string]bool) {
	discKey := discriminatorKey(schema, parentDef, defs)
	sharedProps := map[string]*load.Definition{}
	sharedRequired := map[string]bool{}
	if parentDef != nil {
		maps.Copy(sharedProps, parentDef.Properties)
		sharedRequired = requiredSet(parentDef.Required)
	}

	var variants []variantInfo
	for idx, v := range defs {
		if vi, ok := oneUnionVariant(f, name, schema, idx, v, discKey, sharedProps, sharedRequired, usedTypeNames); ok {
			variants = append(variants, vi)
		}
	}

	// Wrapper: one pointer field per variant, set by UnmarshalJSON.
	var st fieldSet
	for _, vi := range variants {
		if vi.description != "" {
			st.comment(util.FormatDocComment(vi.description)...)
		}
		st.add(structField(vi.fieldName, star(id(vi.typeName)), "-"))
	}
	f.add(typeDecl(name, st.structType()))
	f.line()
	f.add(method(param("u", star(id(name))), "UnmarshalJSON",
		funcType([]*ast.Field{param("b", byteSlice())}, param("", id("error"))),
		unionUnmarshalBody(f, variants, discKey)...))
	f.add(method(param("u", id(name)), "MarshalJSON",
		funcType(nil, param("", byteSlice()), param("", id("error"))),
		unionMarshalBody(f, name, variants)...))
	f.line()

	// oneOf: exactly one variant. anyOf: shared required fields live on the variants.
	if exactlyOne {
		f.add(validateMethod("u", name, unionValidateBody(f, name, variants)...))
	} else {
		f.add(validateMethod("v", name, sharedUnionValidateBody(f, variants, sharedFieldChecks(parentDef))...))
	}
	f.line()
}
