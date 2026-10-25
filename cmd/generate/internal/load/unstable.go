package load

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
)

// Unstable is what the live unstable schema adds beyond the manifest, as the
// input of package unstable's model
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 1, and
// questions 5, 9 and 10). Meta and Schema are acp's with the additions:
// Schema holds every acp definition, each new definition under its schema
// key, and the live version of each stable union with a new variant.
type Unstable struct {
	Meta       *Meta
	Schema     *Schema
	Provenance *Provenance
	// Local names the new definitions, by key: the types package unstable
	// declares. Every other type of Schema is acp's.
	Local map[string]bool
	// Variants holds, for each stable union with a new variant, the indexes
	// of its new entries in its live list.
	Variants map[string][]int
	// Wires names the new methods' wire methods.
	Wires map[string]bool
}

// Empty reports whether u adds nothing.
func (u *Unstable) Empty() bool {
	return u == nil || (len(u.Local) == 0 && len(u.Variants) == 0 && len(u.Wires) == 0)
}

// NewUnstable compares the live unstable inputs with the manifest m and with
// acp's merged inputs, which MergeManifest gave:
//   - a new definition is one the live merge (MergeStableAndUnstable) keeps
//     whose key is neither a stable definition nor a fragment of m; a new
//     key that is also an acp name is refused;
//   - a new method is one whose wire is in neither the stable meta nor m;
//   - a new variant is an entry of the live version of a stable union whose
//     key, by UnionEntryKey, the acp union lacks.
//
// A new definition's reference to a frozen key acp holds only under another
// name is renamed to that name. A new field of a stable definition is
// emitted nowhere, by question 5's answer, so it is not listed.
func NewUnstable(stableMeta *Meta, stableSchema *Schema, liveMeta *Meta, liveSchema *Schema, m *Manifest,
	acpMeta *Meta, acpSchema *Schema, acpProv *Provenance) (*Unstable, error) {
	liveMerged, _, liveProv, err := MergeStableAndUnstable(stableMeta, stableSchema, liveMeta, liveSchema)
	if err != nil {
		return nil, err
	}
	u := &Unstable{Local: map[string]bool{}, Variants: map[string][]int{}, Wires: map[string]bool{}}
	for name := range liveProv.UnstableDefs {
		key := liveProv.Origins[name].Key
		if _, ok := stableSchema.Defs[key]; ok {
			continue
		}
		if _, ok := m.Defs[key]; ok {
			continue
		}
		if _, ok := acpSchema.Defs[key]; ok {
			return nil, fmt.Errorf("unstable: new definition %q has a name acp already declares", key)
		}
		u.Local[key] = true
	}

	rename := map[string]string{}
	for name, key := range m.Definitions {
		if _, ok := acpSchema.Defs[key]; !ok {
			rename[key] = name
		}
	}
	schema := &Schema{Defs: make(map[string]*Definition, len(acpSchema.Defs)+len(u.Local))}
	maps.Copy(schema.Defs, acpSchema.Defs)
	for key := range u.Local {
		d := deepCopyDefinition(liveSchema.Defs[key])
		rewriteDefinitionRefs(d, rename)
		schema.Defs[key] = d
	}
	for _, name := range slices.Sorted(maps.Keys(stableSchema.Defs)) {
		live, ours := liveSchema.Defs[name], acpSchema.Defs[name]
		if live == nil || ours == nil {
			continue
		}
		if added := newEntries(ours, live); len(added) > 0 {
			d := deepCopyDefinition(live)
			rewriteDefinitionRefs(d, rename)
			schema.Defs[name] = d
			u.Variants[name] = added
		}
	}

	frozenWires := map[string]bool{}
	for _, side := range []map[string]string{m.Methods.Agent, m.Methods.Client, m.Methods.Protocol} {
		for _, wire := range side {
			frozenWires[wire] = true
		}
	}
	meta := &Meta{
		Version:         acpMeta.Version,
		AgentMethods:    cloneStringMap(acpMeta.AgentMethods),
		ClientMethods:   cloneStringMap(acpMeta.ClientMethods),
		ProtocolMethods: cloneStringMap(acpMeta.ProtocolMethods),
	}
	for _, side := range []struct {
		dst  *map[string]string
		from map[string]string
	}{
		{&meta.AgentMethods, liveMerged.AgentMethods},
		{&meta.ClientMethods, liveMerged.ClientMethods},
		{&meta.ProtocolMethods, liveMerged.ProtocolMethods},
	} {
		for key, wire := range side.from {
			if !liveProv.UnstableWires[wire] || frozenWires[wire] {
				continue
			}
			if *side.dst == nil {
				*side.dst = map[string]string{}
			}
			if existing, ok := (*side.dst)[key]; ok && existing != wire {
				return nil, fmt.Errorf("unstable: method key %q maps to both %q and %q", key, existing, wire)
			}
			(*side.dst)[key] = wire
			u.Wires[wire] = true
		}
	}

	prov := &Provenance{UnstableDefs: map[string]bool{}, UnstableWires: map[string]bool{}, Origins: map[string]Origin{}, Changes: map[string]*Changes{}}
	maps.Copy(prov.Origins, acpProv.Origins)
	maps.Copy(prov.UnstableWires, acpProv.UnstableWires)
	for key := range u.Local {
		prov.UnstableDefs[key] = true
		prov.Origins[key] = Origin{File: UnstableSchemaFile, Key: key}
	}
	for name := range u.Variants {
		prov.Origins[name] = Origin{File: UnstableSchemaFile, Key: name}
	}
	for wire := range u.Wires {
		prov.UnstableWires[wire] = true
	}
	u.Meta, u.Schema, u.Provenance = meta, schema, prov
	return u, nil
}

// newEntries returns the indexes, in the live union's list, of the entries
// whose key the acp union lacks; an entry without a key is new unless the acp
// union holds it as it is. The list is the anyOf, or the oneOf when there is
// no anyOf, by the model's rule.
func newEntries(ours, live *Definition) []int {
	liveList, ourList := live.OneOf, ours.OneOf
	if len(live.AnyOf) > 0 {
		liveList, ourList = live.AnyOf, ours.AnyOf
	}
	if len(liveList) == 0 {
		return nil
	}
	disc := ""
	if live.Discriminator != nil {
		disc = live.Discriminator.PropertyName
	}
	var out []int
	for i, e := range liveList {
		key := UnionEntryKey(e, disc)
		known := slices.ContainsFunc(ourList, func(o *Definition) bool {
			if key == "" {
				return reflect.DeepEqual(o, e)
			}
			return UnionEntryKey(o, disc) == key
		})
		if !known {
			out = append(out, i)
		}
	}
	return out
}
