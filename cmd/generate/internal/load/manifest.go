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

// ManifestFile is the unstable manifest's file in the schema directory: the
// unstable-origin API the v1 module exports, frozen
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 1). It is also
// the Origin file of every definition built from it.
const ManifestFile = "unstable-v1.json"

// manifestVersion is the only manifest version the generator reads.
const manifestVersion = 1

// Manifest is the unstable manifest. Defs holds the fragments as the unstable
// schema held them, under its keys: the source of each kept definition, and
// the whole unstable version of each replaced one.
type Manifest struct {
	Version int `json:"version"`
	// Schema is the schema version the manifest was frozen from.
	Schema  string          `json:"schema"`
	Methods ManifestMethods `json:"methods"`
	// Definitions maps each name the module gives a kept definition to its
	// key in Defs.
	Definitions map[string]string `json:"definitions"`
	// Changes names, for each stable definition the unstable schema replaced,
	// what of the frozen version applies to it.
	Changes map[string]*ManifestChange `json:"changes"`
	Defs    map[string]json.RawMessage `json:"$defs"`

	// frozen is Defs decoded.
	frozen *Schema
}

// ManifestMethods are the frozen unstable methods, by side, key to wire.
type ManifestMethods struct {
	Agent    map[string]string `json:"agent,omitempty"`
	Client   map[string]string `json:"client,omitempty"`
	Protocol map[string]string `json:"protocol,omitempty"`
}

// ManifestChange is what of the frozen version of a stable definition
// applies to it: each listed property, whole, and for a union its entries in
// the frozen order, by UnionEntryKey, the keys in FrozenEntries taken from
// the frozen version and the others from the stable definition.
type ManifestChange struct {
	Properties    []string `json:"properties,omitempty"`
	Entries       []string `json:"entries,omitempty"`
	FrozenEntries []string `json:"frozenEntries,omitempty"`
}

// ParseManifest decodes a manifest strictly, its fragments included.
func ParseManifest(b []byte) (*Manifest, error) {
	var m Manifest
	if err := decodeStrict(b, &m); err != nil {
		return nil, err
	}
	if m.Version != manifestVersion {
		return nil, fmt.Errorf("version %d; this generator reads version %d", m.Version, manifestVersion)
	}
	m.frozen = &Schema{Defs: make(map[string]*Definition, len(m.Defs))}
	for _, key := range slices.Sorted(maps.Keys(m.Defs)) {
		var d Definition
		if err := json.Unmarshal(m.Defs[key], &d); err != nil {
			return nil, fmt.Errorf("$defs %q: %w", key, err)
		}
		m.frozen.Defs[key] = &d
	}
	return &m, nil
}

// ReadUnstableManifest loads the manifest of schemaDir when present. The
// boolean reports whether the file was found.
func ReadUnstableManifest(schemaDir string) (*Manifest, bool, error) {
	b, err := os.ReadFile(filepath.Join(schemaDir, ManifestFile)) //nolint:gosec // schemaDir is the generator's explicit input.
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read %s: %w", ManifestFile, err)
	}
	m, err := ParseManifest(b)
	if err != nil {
		return nil, true, fmt.Errorf("parse %s: %w", ManifestFile, err)
	}
	return m, true, nil
}

// MergeManifest combines the stable inputs with the manifest: the stable
// methods and definitions; each frozen method; each kept definition, built
// from its fragment, with its references renamed and a stable x-method
// cleared as MergeStableAndUnstable does for a definition kept under another
// name; and each replaced definition, the stable one with its change
// applied. The live unstable schema has no part in it, so a schema that
// changes or drops a frozen definition changes nothing here. Every definition
// built from the manifest has ManifestFile as its Origin's file.
func MergeManifest(stableMeta *Meta, stableSchema *Schema, m *Manifest) (*Meta, *Schema, *Provenance, error) {
	if stableMeta == nil || stableSchema == nil || m == nil || m.frozen == nil {
		return nil, nil, nil, errors.New("merge manifest: a stable input or the parsed manifest is nil")
	}
	stableWires := map[string]struct{}{}
	addWireMethods(stableWires, stableMeta.AgentMethods)
	addWireMethods(stableWires, stableMeta.ClientMethods)
	addWireMethods(stableWires, stableMeta.ProtocolMethods)
	combinedMeta := &Meta{
		Version:         stableMeta.Version,
		AgentMethods:    cloneStringMap(stableMeta.AgentMethods),
		ClientMethods:   cloneStringMap(stableMeta.ClientMethods),
		ProtocolMethods: cloneStringMap(stableMeta.ProtocolMethods),
	}
	unstableOnlyWires := map[string]struct{}{}
	var err error
	for _, side := range []struct {
		dst  *map[string]string
		from map[string]string
	}{
		{&combinedMeta.AgentMethods, m.Methods.Agent},
		{&combinedMeta.ClientMethods, m.Methods.Client},
		{&combinedMeta.ProtocolMethods, m.Methods.Protocol},
	} {
		if *side.dst, err = mergeUnstableOnlyMethods(*side.dst, side.from, stableWires, unstableOnlyWires); err != nil {
			return nil, nil, nil, fmt.Errorf("%s: %w", ManifestFile, err)
		}
	}

	combined := &Schema{Defs: make(map[string]*Definition, len(stableSchema.Defs)+len(m.Definitions))}
	maps.Copy(combined.Defs, stableSchema.Defs)
	prov := &Provenance{UnstableDefs: map[string]bool{}, UnstableWires: map[string]bool{}, Origins: map[string]Origin{}, Changes: map[string]*Changes{}}

	renamed := map[string]string{}
	for name, key := range m.Definitions {
		if name != key {
			renamed[key] = name
		}
	}
	for _, name := range slices.Sorted(maps.Keys(m.Definitions)) {
		key := m.Definitions[name]
		if _, ok := stableSchema.Defs[name]; ok {
			return nil, nil, nil, fmt.Errorf("%s: definition %q is kept as unstable, but the stable schema has it", ManifestFile, name)
		}
		frozen, ok := m.frozen.Defs[key]
		if !ok {
			return nil, nil, nil, fmt.Errorf("%s: definition %q names $defs %q, which it does not hold", ManifestFile, name, key)
		}
		d := deepCopyDefinition(frozen)
		if name != key {
			rewriteDefinitionRefs(d, renamed)
			if _, ok := stableWires[d.XMethod]; ok {
				d.XMethod, d.XSide = "", ""
			}
		}
		combined.Defs[name] = d
		prov.UnstableDefs[name] = true
		prov.Origins[name] = Origin{File: ManifestFile, Key: key}
	}

	for _, name := range slices.Sorted(maps.Keys(m.Changes)) {
		d, err := applyChange(stableSchema.Defs[name], m.frozen.Defs[name], m.Changes[name])
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s: change of %q: %w", ManifestFile, name, err)
		}
		combined.Defs[name] = d
		prov.Origins[name] = Origin{File: ManifestFile, Key: name}
		prov.Changes[name] = changesOf(stableSchema.Defs[name], d)
	}
	for wire := range unstableOnlyWires {
		prov.UnstableWires[wire] = true
	}
	return combinedMeta, combined, prov, nil
}

// applyChange returns the stable definition s with c applied from its frozen
// version u.
func applyChange(s, u *Definition, c *ManifestChange) (*Definition, error) {
	switch {
	case s == nil:
		return nil, errors.New("the stable schema has no such definition")
	case u == nil:
		return nil, errors.New("$defs does not hold its frozen version")
	case c == nil:
		return nil, errors.New("the change is null")
	}
	d := deepCopyDefinition(s)
	for _, key := range c.Properties {
		p, ok := u.Properties[key]
		if !ok {
			return nil, fmt.Errorf("the frozen version has no property %q", key)
		}
		if d.Properties == nil {
			d.Properties = map[string]*Definition{}
		}
		d.Properties[key] = deepCopyDefinition(p)
	}
	if len(c.Entries) == 0 {
		return d, nil
	}
	disc := ""
	if u.Discriminator != nil {
		disc = u.Discriminator.PropertyName
	}
	frozenList, stableList, anyOf := u.OneOf, s.OneOf, len(u.AnyOf) > 0
	if anyOf {
		frozenList, stableList = u.AnyOf, s.AnyOf
	}
	entryOf := func(list []*Definition, key string) *Definition {
		if i := slices.IndexFunc(list, func(e *Definition) bool { return UnionEntryKey(e, disc) == key }); i >= 0 {
			return list[i]
		}
		return nil
	}
	var out []*Definition
	for _, key := range c.Entries {
		if slices.Contains(c.FrozenEntries, key) {
			e := entryOf(frozenList, key)
			if e == nil {
				return nil, fmt.Errorf("the frozen version has no entry %q", key)
			}
			out = append(out, deepCopyDefinition(e))
		} else if e := entryOf(stableList, key); e != nil {
			out = append(out, deepCopyDefinition(e))
		}
	}
	for _, e := range stableList {
		if key := UnionEntryKey(e, disc); key == "" || !slices.Contains(c.Entries, key) {
			out = append(out, deepCopyDefinition(e))
		}
	}
	if anyOf {
		d.AnyOf = out
	} else {
		d.OneOf = out
	}
	return d, nil
}

// WriteUnstableManifest freezes the unstable-origin API that schemaDir's
// stable and unstable files give into its manifest. It refuses when the
// manifest exists, which is frozen once, and unless merging through the
// manifest gives exactly what MergeStableAndUnstable gives, but for the file
// the origins name.
func WriteUnstableManifest(schemaDir string) error {
	path := filepath.Join(schemaDir, ManifestFile)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s exists; the manifest is frozen once and never rewritten", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	b, err := buildManifest(schemaDir)
	if err != nil {
		return fmt.Errorf("write %s: %w", ManifestFile, err)
	}
	return os.WriteFile(path, b, 0o644) //nolint:gosec // G306: a committed schema file, read by everyone.
}

// buildManifest returns the manifest schemaDir's files give, checked.
func buildManifest(schemaDir string) ([]byte, error) {
	stableMeta, err := ReadMeta(schemaDir)
	if err != nil {
		return nil, err
	}
	stableSchema, err := ReadSchema(schemaDir)
	if err != nil {
		return nil, err
	}
	unstableMeta, metaFound, err := ReadMetaUnstable(schemaDir)
	if err != nil {
		return nil, err
	}
	unstableSchema, schemaFound, err := ReadSchemaUnstable(schemaDir)
	if err != nil {
		return nil, err
	}
	if !metaFound || !schemaFound {
		return nil, errors.New("it is frozen from meta.unstable.json and schema.unstable.json, and one is missing")
	}
	raw, err := readRawDefs(filepath.Join(schemaDir, "schema.unstable.json"))
	if err != nil {
		return nil, err
	}
	meta, _, prov, err := MergeStableAndUnstable(stableMeta, stableSchema, unstableMeta, unstableSchema)
	if err != nil {
		return nil, err
	}
	version, err := os.ReadFile(filepath.Join(schemaDir, "version")) //nolint:gosec // schemaDir is the generator's explicit input.
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	m := &Manifest{
		Version:     manifestVersion,
		Schema:      strings.TrimSpace(string(version)),
		Definitions: map[string]string{},
		Changes:     map[string]*ManifestChange{},
		Defs:        map[string]json.RawMessage{},
	}
	for _, side := range []struct {
		dst  *map[string]string
		from map[string]string
	}{
		{&m.Methods.Agent, meta.AgentMethods},
		{&m.Methods.Client, meta.ClientMethods},
		{&m.Methods.Protocol, meta.ProtocolMethods},
	} {
		for key, wire := range side.from {
			if prov.UnstableWires[wire] {
				if *side.dst == nil {
					*side.dst = map[string]string{}
				}
				(*side.dst)[key] = wire
			}
		}
	}
	for name := range prov.UnstableDefs {
		key := prov.Origins[name].Key
		m.Definitions[name] = key
		m.Defs[key] = raw[key]
	}
	for name, c := range prov.Changes {
		mc, err := manifestChange(unstableSchema.Defs[name], c)
		if err != nil {
			return nil, fmt.Errorf("change of %q: %w", name, err)
		}
		m.Changes[name] = mc
		m.Defs[name] = raw[name]
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	parsed, err := ParseManifest(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("the manifest it built does not parse: %w", err)
	}
	if err := ManifestReproduces(schemaDir, parsed); err != nil {
		return nil, fmt.Errorf("the manifest it built does not reproduce the merge: %w", err)
	}
	return buf.Bytes(), nil
}

// ManifestReproduces reports the first way merging schemaDir's stable files
// through m differs from MergeStableAndUnstable over its stable and unstable
// files, but for the file the origins name: nil when m is what
// WriteUnstableManifest would freeze from them.
func ManifestReproduces(schemaDir string, m *Manifest) error {
	stableMeta, err := ReadMeta(schemaDir)
	if err != nil {
		return err
	}
	stableSchema, err := ReadSchema(schemaDir)
	if err != nil {
		return err
	}
	unstableMeta, _, err := ReadMetaUnstable(schemaDir)
	if err != nil {
		return err
	}
	unstableSchema, _, err := ReadSchemaUnstable(schemaDir)
	if err != nil {
		return err
	}
	meta, schema, prov, err := MergeStableAndUnstable(stableMeta, stableSchema, unstableMeta, unstableSchema)
	if err != nil {
		return err
	}
	gotMeta, gotSchema, gotProv, err := MergeManifest(stableMeta, stableSchema, m)
	if err != nil {
		return err
	}
	return sameMerge(meta, schema, prov, gotMeta, gotSchema, gotProv)
}

// manifestChange is the manifest's change for the unstable version u of a
// replaced definition, whose Changes are c: every added or changed property,
// and for an added or changed entry the whole list's keys, in u's order.
func manifestChange(u *Definition, c *Changes) (*ManifestChange, error) {
	mc := &ManifestChange{Properties: slices.Sorted(slices.Values(slices.Concat(c.AddedProperties, c.ChangedProperties)))}
	frozen := slices.Concat(c.AddedEntries, c.ChangedEntries)
	if len(frozen) == 0 {
		return mc, nil
	}
	disc := ""
	if u.Discriminator != nil {
		disc = u.Discriminator.PropertyName
	}
	list := u.OneOf
	if len(u.AnyOf) > 0 {
		list = u.AnyOf
	}
	for i, e := range list {
		key := UnionEntryKey(e, disc)
		if key == "" {
			return nil, fmt.Errorf("union entry %d has no key; its order cannot be frozen", i)
		}
		if slices.Contains(mc.Entries, key) {
			return nil, fmt.Errorf("union entries share the key %q", key)
		}
		mc.Entries = append(mc.Entries, key)
		if slices.Contains(frozen, i) {
			mc.FrozenEntries = append(mc.FrozenEntries, key)
		}
	}
	return mc, nil
}

// sameMerge reports the first way the second merge differs from the first,
// whose origins name the unstable schema where the second's name the
// manifest.
func sameMerge(meta *Meta, schema *Schema, prov *Provenance, gotMeta *Meta, gotSchema *Schema, gotProv *Provenance) error {
	if !reflect.DeepEqual(meta, gotMeta) {
		return errors.New("the methods differ")
	}
	for _, name := range slices.Sorted(maps.Keys(schema.Defs)) {
		if !reflect.DeepEqual(schema.Defs[name], gotSchema.Defs[name]) {
			return fmt.Errorf("definition %q differs", name)
		}
	}
	if len(schema.Defs) != len(gotSchema.Defs) {
		return fmt.Errorf("%d definitions, not %d", len(gotSchema.Defs), len(schema.Defs))
	}
	if !maps.Equal(prov.UnstableDefs, gotProv.UnstableDefs) || !maps.Equal(prov.UnstableWires, gotProv.UnstableWires) {
		return errors.New("the unstable definitions or wires differ")
	}
	if !reflect.DeepEqual(prov.Changes, gotProv.Changes) {
		return errors.New("the changes differ")
	}
	if len(prov.Origins) != len(gotProv.Origins) {
		return fmt.Errorf("%d origins, not %d", len(gotProv.Origins), len(prov.Origins))
	}
	for name, o := range prov.Origins {
		if g := gotProv.Origins[name]; g.Key != o.Key || g.File != ManifestFile {
			return fmt.Errorf("the origin of %q is %+v, not %s at %q", name, g, ManifestFile, o.Key)
		}
	}
	return nil
}

// readRawDefs returns the $defs of the schema file at path, each as the file
// holds it.
func readRawDefs(path string) (map[string]json.RawMessage, error) {
	b, err := os.ReadFile(path) //nolint:gosec // a file of the generator's explicit schema directory.
	if err != nil {
		return nil, err
	}
	var top struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, err
	}
	return top.Defs, nil
}
