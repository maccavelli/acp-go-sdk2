package model

import (
	"cmp"
	"slices"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// applyFrozenDeprecations marks every declaration of unstable origin that the
// unstable manifest keeps under v1
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 1):
//   - each type built from a definition the manifest keeps, with its nested
//     structs, its variants (their inline types, wrapper fields and
//     constructors) and its enum constants;
//   - each field and variant the manifest adds to a stable type, a field's
//     nested struct included;
//   - each method whose request or notification type the manifest holds: its
//     interface method, its constant and its outbound method;
//   - each interface whose every method is so marked.
//
// It runs after applyDeprecations, so a protocol-table note never names a
// mark of its own, and a mark the schema gives is kept.
func (m *Model) applyFrozenDeprecations() {
	for _, t := range m.Types {
		if t.Origin.File == load.ManifestFile && t.Kind != KindNone {
			m.markFrozenType(t)
		}
	}
	frozenOnly := map[Binding]bool{}
	for _, mi := range slices.Concat(m.AgentMethods, m.ClientMethods, m.Protocol) {
		t := m.Type(cmp.Or(mi.Notif, mi.Req))
		frozen := mi.Unstable && t != nil && t.Origin.File == load.ManifestFile
		if frozen && mi.Deprecation == nil {
			mi.Deprecation = &Deprecation{Frozen: true, Definition: t.Origin.Key}
		}
		if seen, ok := frozenOnly[mi.Binding]; !ok || seen {
			frozenOnly[mi.Binding] = frozen
		}
	}
	for i := range m.Interfaces {
		if frozenOnly[m.Interfaces[i].Binding] {
			m.Interfaces[i].Deprecation = &Deprecation{Frozen: true}
		}
	}
}

// markFrozen marks d frozen, unless it is marked already.
func markFrozen(d **Deprecation) {
	if *d == nil {
		*d = &Deprecation{Frozen: true}
	}
}

// markFrozenType marks the declarations of t, a type the manifest holds, as
// applyFrozenDeprecations lists them: all of an unstable type's, and of a
// stable type's, what the manifest adds.
func (m *Model) markFrozenType(t *Type) {
	if t.Unstable {
		markFrozen(&t.Deprecation)
		for _, ns := range t.NestedStructs {
			markFrozen(&ns.Deprecation)
		}
		for _, v := range t.Variants {
			if !v.Skip {
				markFrozen(&v.Deprecation)
			}
		}
		for i := range t.Enum {
			markFrozen(&t.Enum[i].Deprecation)
		}
		return
	}
	for _, f := range t.Fields {
		if !f.Unstable {
			continue
		}
		// The field's copies too, in nested structs and inline variants
		// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G25).
		for _, c := range m.fieldsHolding(f.prop) {
			markFrozen(&c.Deprecation)
		}
		for _, ns := range t.NestedStructs {
			if ns.Key == f.Key {
				markFrozen(&ns.Deprecation)
			}
		}
	}
	for _, v := range t.Variants {
		if v.Unstable && !v.Skip {
			markFrozen(&v.Deprecation)
		}
	}
}
