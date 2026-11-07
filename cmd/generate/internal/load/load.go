// Package load provides utilities to read the ACP JSON schema and
// accompanying metadata into minimal structures used by the generator.
package load

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

// Meta mirrors schema/meta.json for method maps and version.
type Meta struct {
	Version int `json:"version"`

	AgentMethods    map[string]string `json:"agentMethods"`
	ClientMethods   map[string]string `json:"clientMethods"`
	ProtocolMethods map[string]string `json:"protocolMethods,omitempty"`
}

// Schema is a minimal view over schema/schema.json definitions used by the generator.
type Schema struct {
	Defs map[string]*Definition `json:"$defs"`
}

// Discriminator specifies which property distinguishes union variants.
// Part of JSON Schema discriminator object support.
type Discriminator struct {
	PropertyName string `json:"propertyName"`
}

// Definition is a partial JSON Schema node the generator cares about.
type Definition struct {
	Description string                 `json:"description"`
	Type        any                    `json:"type"`
	Properties  map[string]*Definition `json:"properties"`
	Required    []string               `json:"required"`
	Enum        []any                  `json:"enum"`
	Items       *Definition            `json:"items"`
	Ref         string                 `json:"$ref"`
	AnyOf       []*Definition          `json:"anyOf"`
	OneOf       []*Definition          `json:"oneOf"`
	AllOf       []*Definition          `json:"allOf"`
	DocsIgnore  bool                   `json:"x-docs-ignore"`
	Title       string                 `json:"title"`
	Const       any                    `json:"const"`
	XSide       string                 `json:"x-side"`
	XMethod     string                 `json:"x-method"`
	// Default holds the JSON Schema default value, when present.
	// Used by generators to synthesize defaulting behavior.
	Default any `json:"default"`
	// Discriminator specifies which property name distinguishes union variants.
	// Part of JSON Schema's discriminator object support.
	Discriminator *Discriminator `json:"discriminator,omitempty"`

	// Format, Minimum, Maximum, and MinLength are JSON Schema validation
	// keywords, such as format "uint64" with minimum 0.
	Format    string       `json:"format"`
	Minimum   *json.Number `json:"minimum"`
	Maximum   *json.Number `json:"maximum"`
	MinLength *int         `json:"minLength"`
	// AdditionalProperties is the schema of the values of an object's other
	// keys; boolean true decodes as a boolean schema.
	AdditionalProperties *Definition `json:"additionalProperties"`
	// Not and UnevaluatedProperties constrain union variants.
	Not                   *Definition `json:"not"`
	UnevaluatedProperties *Definition `json:"unevaluatedProperties"`
	// XDeserializeDefaultOnError asks a decoder to use the default when the
	// value fails to decode; XDeserializeSkipInvalidItems asks it to drop
	// array items that fail to decode.
	XDeserializeDefaultOnError   bool `json:"x-deserialize-default-on-error"`
	XDeserializeSkipInvalidItems bool `json:"x-deserialize-skip-invalid-items"`
	// Deprecated is JSON Schema 2020-12's annotation that applications should
	// refrain from using the node; false is the same as absent, and null is
	// refused (0024-PLAN-api-gate-and-deprecation.md).
	Deprecated bool `json:"deprecated"`

	// boolSchema records whether this definition was a boolean schema (true/false).
	// JSON Schema allows boolean schemas, where true matches anything and false matches nothing.
	// We ignore the semantic difference in codegen and treat both as permissive/unknown shapes.
	boolSchema *bool `json:"-"`
}

// BoolSchema reports whether d was the boolean schema true or false, and
// which.
func (d *Definition) BoolSchema() (value, ok bool) {
	if d == nil || d.boolSchema == nil {
		return false, false
	}
	return *d.boolSchema, true
}

// UnmarshalJSON allows Definition to decode both object and boolean JSON
// Schema forms. A keyword Definition does not model is an error, so a schema
// update that adds one fails here rather than being ignored.
func (d *Definition) UnmarshalJSON(b []byte) error {
	// Trim whitespace for simple equality checks
	tb := bytes.TrimSpace(b)
	if bytes.Equal(tb, []byte("true")) || bytes.Equal(tb, []byte("false")) {
		v := bytes.Equal(tb, []byte("true"))
		// Reset to zero-value and record that this was a boolean schema.
		*d = Definition{}
		d.boolSchema = &v
		return nil
	}
	// Fallback to normal object decoding
	type alias Definition
	var a alias
	if err := decodeStrict(b, &a); err != nil {
		return err
	}
	if err := refuseNull(b, "deprecated"); err != nil {
		return err
	}
	*d = Definition(a)
	return nil
}

// refuseNull fails when the JSON object b holds key with the value null,
// which decoding into a bool takes as false where JSON Schema requires a
// boolean (0024-PLAN-api-gate-and-deprecation.md).
func refuseNull(b []byte, key string) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	if v, ok := obj[key]; ok && bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return fmt.Errorf("%s is null; it must be true or false", key)
	}
	return nil
}

// decodeStrict decodes b into v, a pointer to a struct, and fails on a key v
// does not declare, and on one that names a field in another letter case,
// which encoding/json would match (0019-PLAN-generator-hardening.md, G13).
func decodeStrict(b []byte, v any) error {
	if err := exactKeys(b, reflect.TypeOf(v).Elem()); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// exactKeys fails when a key of the JSON object b differs only in letter case
// from the JSON name of a field of the struct type t. A b that is not an
// object is left to the decoder.
func exactKeys(b []byte, t reflect.Type) error {
	if !bytes.HasPrefix(bytes.TrimSpace(b), []byte("{")) {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	var names []string
	for f := range t.Fields() {
		if name, _, _ := strings.Cut(f.Tag.Get("json"), ","); name != "" && name != "-" {
			names = append(names, name)
		}
	}
	for _, key := range slices.Sorted(maps.Keys(obj)) {
		if slices.Contains(names, key) {
			continue
		}
		if i := slices.IndexFunc(names, func(n string) bool { return strings.EqualFold(n, key) }); i >= 0 {
			return fmt.Errorf("unknown field %q: keywords are matched exactly, and this is %q in another case", key, names[i])
		}
	}
	return nil
}

// UnmarshalJSON decodes a discriminator strictly.
func (d *Discriminator) UnmarshalJSON(b []byte) error {
	type alias Discriminator
	var a alias
	if err := decodeStrict(b, &a); err != nil {
		return err
	}
	*d = Discriminator(a)
	return nil
}

// parseMeta decodes a meta file strictly. Its version must be 1 or more: a
// missing one would make ProtocolVersionNumber 0
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G8).
func parseMeta(b []byte) (*Meta, error) {
	if err := duplicateKey(b); err != nil {
		return nil, err
	}
	var meta Meta
	if err := decodeStrict(b, &meta); err != nil {
		return nil, err
	}
	if meta.Version < 1 {
		return nil, fmt.Errorf("version %d; want 1 or more", meta.Version)
	}
	return &meta, nil
}

// duplicateKey fails on a key that one JSON object of b holds twice, naming
// where, since decoding would keep the last and drop the others silently
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G5 g). JSON that
// does not parse is left to the decoder, which reports it.
func duplicateKey(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	err := duplicateIn(dec, "")
	if errors.Is(err, errNotJSON) {
		return nil
	}
	return err
}

// errNotJSON stops duplicateIn at input the decoder will refuse.
var errNotJSON = errors.New("not JSON")

// duplicateIn reads one value from dec, failing on a repeated key in any
// object of it; path is where the value is, by its keys.
func duplicateIn(dec *json.Decoder, path string) error {
	tok, err := dec.Token()
	if err != nil {
		return errNotJSON
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return errNotJSON
			}
			key, ok := kt.(string)
			if !ok {
				return errNotJSON
			}
			at := key
			if path != "" {
				at = path + "." + key
			}
			if seen[key] {
				return fmt.Errorf("%s: key %q appears twice in one object; decoding would keep the last", at, key)
			}
			seen[key] = true
			if err := duplicateIn(dec, at); err != nil {
				return err
			}
		}
	case '[':
		for i := 0; dec.More(); i++ {
			if err := duplicateIn(dec, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	if _, err := dec.Token(); err != nil {
		return errNotJSON
	}
	return nil
}

// parseSchema decodes the definitions of a schema file one by one, so an
// error names the definition it came from. Other top-level keys are ignored.
func parseSchema(b []byte) (*Schema, error) {
	if err := duplicateKey(b); err != nil {
		return nil, err
	}
	var top struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, err
	}
	if err := exactKeys(b, reflect.TypeOf(top)); err != nil {
		return nil, err
	}
	schema := &Schema{Defs: make(map[string]*Definition, len(top.Defs))}
	for _, name := range slices.Sorted(maps.Keys(top.Defs)) {
		raw := top.Defs[name]
		// A null definition is no schema: it was dropped, and a $ref to it
		// reported that it names none
		// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G32).
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, fmt.Errorf("definition %q is null, not a schema", name)
		}
		var d Definition
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, fmt.Errorf("definition %q: %w", name, err)
		}
		schema.Defs[name] = &d
	}
	return schema, nil
}

// ReadMeta loads schema/meta.json.
func ReadMeta(schemaDir string) (*Meta, error) {
	metaBytes, err := os.ReadFile(filepath.Join(schemaDir, "meta.json")) //nolint:gosec // schemaDir is the generator's explicit input.
	if err != nil {
		return nil, fmt.Errorf("read meta.json: %w", err)
	}
	meta, err := parseMeta(metaBytes)
	if err != nil {
		return nil, fmt.Errorf("parse meta.json: %w", err)
	}
	return meta, nil
}

// ReadSchema loads schema/schema.json.
func ReadSchema(schemaDir string) (*Schema, error) {
	schemaBytes, err := os.ReadFile(filepath.Join(schemaDir, "schema.json")) //nolint:gosec // schemaDir is the generator's explicit input.
	if err != nil {
		return nil, fmt.Errorf("read schema.json: %w", err)
	}
	schema, err := parseSchema(schemaBytes)
	if err != nil {
		return nil, fmt.Errorf("parse schema.json: %w", err)
	}
	return schema, nil
}

// ReadMetaUnstable loads schema/meta.unstable.json when present.
// The returned boolean indicates whether the file was found.
func ReadMetaUnstable(schemaDir string) (*Meta, bool, error) {
	metaBytes, err := os.ReadFile(filepath.Join(schemaDir, "meta.unstable.json")) //nolint:gosec // schemaDir is the generator's explicit input.
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read meta.unstable.json: %w", err)
	}
	meta, err := parseMeta(metaBytes)
	if err != nil {
		return nil, true, fmt.Errorf("parse meta.unstable.json: %w", err)
	}
	return meta, true, nil
}

// ReadSchemaUnstable loads schema/schema.unstable.json when present.
// The returned boolean indicates whether the file was found.
func ReadSchemaUnstable(schemaDir string) (*Schema, bool, error) {
	schemaBytes, err := os.ReadFile(filepath.Join(schemaDir, "schema.unstable.json")) //nolint:gosec // schemaDir is the generator's explicit input.
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read schema.unstable.json: %w", err)
	}
	schema, err := parseSchema(schemaBytes)
	if err != nil {
		return nil, true, fmt.Errorf("parse schema.unstable.json: %w", err)
	}
	return schema, true, nil
}

// Committed reads the inputs the generator builds the SDK from in schemaDir:
// the stable schema, merged with the manifest when there is one, else with
// the live unstable schema when there is one. The provenance is nil when
// there is neither. The generator and the tests of the protocol table both
// read the schema through it, so the tests check the model the SDK ships
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G18).
func Committed(schemaDir string) (*Meta, *Schema, *Provenance, error) {
	meta, err := ReadMeta(schemaDir)
	if err != nil {
		return nil, nil, nil, err
	}
	schema, err := ReadSchema(schemaDir)
	if err != nil {
		return nil, nil, nil, err
	}
	unstableMeta, unstableMetaFound, err := ReadMetaUnstable(schemaDir)
	if err != nil {
		return nil, nil, nil, err
	}
	unstableSchema, unstableSchemaFound, err := ReadSchemaUnstable(schemaDir)
	if err != nil {
		return nil, nil, nil, err
	}
	if unstableMetaFound != unstableSchemaFound {
		return nil, nil, nil, fmt.Errorf("unstable schema/meta mismatch: meta found=%v schema found=%v", unstableMetaFound, unstableSchemaFound)
	}
	// The manifest, when present, is the unstable-origin API, frozen; the live
	// unstable files have no part in it
	// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 1).
	manifest, manifestFound, err := ReadUnstableManifest(schemaDir)
	if err != nil {
		return nil, nil, nil, err
	}
	if manifestFound {
		return MergeManifest(meta, schema, manifest)
	}
	if !unstableMetaFound {
		return meta, schema, nil, nil
	}
	return MergeStableAndUnstable(meta, schema, unstableMeta, unstableSchema)
}
