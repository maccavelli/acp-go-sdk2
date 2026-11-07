package emit

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// TestUnknownKindWithoutRawKeyGate requires unknownKind, which helpers.go's
// Kind methods call for every union that keeps an Unknown, to be generated
// when no gate reads a raw key: with ACP's undeclared-variant rules removed,
// the one check that marked it is gone, and the root's hand-written files
// no longer compiled (0049-MADR-remediate-the-2026-11-01-debugging-pass.md,
// G30).
func TestUnknownKindWithoutRawKeyGate(t *testing.T) {
	t.Parallel()
	meta, schema, prov, err := load.Committed(filepath.Join("..", "..", "..", "..", "schema"))
	if err != nil {
		t.Fatal(err)
	}
	p := model.ACP
	p.CapabilityRules = slices.DeleteFunc(slices.Clone(model.ACP.CapabilityRules), func(r model.CapabilityRule) bool {
		return r.Kind == model.GateUndeclaredVariant
	})
	m, err := model.Build(model.Input{Meta: meta, Schema: schema, Provenance: prov, Protocol: &p})
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range m.Gated {
		for _, c := range g.Checks {
			// An advertised-auth check reads its raw key by unknownString.
			if c.RawKey != "" && c.Kind != model.GateCheckAdvertisedAuth {
				t.Fatalf("%s: a check still reads the raw key %q by unknownKind", g.Type, c.RawKey)
			}
		}
	}
	files, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Name == "types_gen.go" && !strings.Contains(string(f.Src), "func unknownKind(") {
			t.Error("types_gen.go lacks unknownKind, which helpers.go calls")
		}
	}
}
