package load

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/enginetest"
)

// TestNewEntriesConstEntries requires an enum's entries, each a const with no
// other key, to be matched by their const: a reworded description is no new
// entry, and a new value is one, at its index. A keyless entry is still
// matched by its shape, its annotations aside
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G15).
func TestNewEntriesConstEntries(t *testing.T) {
	t.Parallel()
	value := func(v, desc string) *Definition { return &Definition{Type: "string", Const: v, Description: desc} }
	ours := &Definition{OneOf: []*Definition{value("a", "A."), value("b", "B.")}}
	tests := []struct {
		name string
		live *Definition
		want []int
	}{
		{"a reworded description", &Definition{OneOf: []*Definition{value("a", "A. Reworded."), value("b", "B.")}}, nil},
		{"a new value", &Definition{OneOf: []*Definition{value("a", "A."), value("b", "B."), value("c", "C.")}}, []int{2}},
		{"the values reordered", &Definition{OneOf: []*Definition{value("b", "B."), value("a", "A.")}}, nil},
	}
	for _, tt := range tests {
		if got := newEntries(ours, tt.live); !slices.Equal(got, tt.want) {
			t.Errorf("%s: newEntries = %v, want %v", tt.name, got, tt.want)
		}
	}
	// The key itself, which a manifest's frozen order names an enum's entries
	// by; checked last, so a titled entry keeps its title's key.
	if got := UnionEntryKey(value("a", "A."), ""); got != `value "a"` {
		t.Errorf(`UnionEntryKey of the const "a" = %q, want value "a"`, got)
	}
	if got := UnionEntryKey(&Definition{Title: "T", Const: "a"}, ""); got != "title T" {
		t.Errorf("UnionEntryKey of a titled const = %q, want title T", got)
	}
	keyless := func(desc string) *Definition {
		return &Definition{Type: "array", Description: desc, Items: &Definition{Type: "string"}}
	}
	if got := newEntries(&Definition{AnyOf: []*Definition{keyless("old")}}, &Definition{AnyOf: []*Definition{keyless("new")}}); len(got) != 0 {
		t.Errorf("a keyless entry reworded: newEntries = %v, want none", got)
	}
	if got := newEntries(&Definition{AnyOf: []*Definition{keyless("")}}, &Definition{AnyOf: []*Definition{{Type: "array", Items: &Definition{Type: "integer"}}}}); !slices.Equal(got, []int{0}) {
		t.Errorf("a keyless entry of another shape: newEntries = %v, want [0]", got)
	}
}

// TestCommittedFollowsTheManifest requires Committed, which the generator and
// the protocol table's tests both read the schema through, to take the
// unstable-origin API from the manifest when there is one: a method the live
// unstable meta adds and the manifest lacks is not in its meta
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G18).
func TestCommittedFollowsTheManifest(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "engine", "accept-unstable-empty.txtar"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := enginetest.ParseArchive(b)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, f := range a.Files {
		if !strings.HasSuffix(f.Name, ".json") {
			continue
		}
		data := f.Data
		switch f.Name {
		case "meta.unstable.json":
			data = jsonEdit(t, data, func(m map[string]any) { m["agentMethods"].(map[string]any)["late"] = "x/late" })
		case "schema.unstable.json":
			data = jsonEdit(t, data, func(m map[string]any) {
				defs := m["$defs"].(map[string]any)
				defs["LateRequest"] = map[string]any{"type": "object", "x-side": "agent", "x-method": "x/late"}
				defs["LateResponse"] = map[string]any{"type": "object", "x-side": "agent", "x-method": "x/late"}
			})
		}
		if err := os.WriteFile(filepath.Join(dir, f.Name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	meta, schema, _, err := Committed(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := meta.AgentMethods["late"]; ok || schema.Defs["LateRequest"] != nil {
		t.Errorf("Committed has the live unstable x/late, which the manifest lacks: methods %v", meta.AgentMethods)
	}
	if meta.AgentMethods["new"] != "x/new" {
		t.Errorf("Committed lacks the manifest's x/new: methods %v", meta.AgentMethods)
	}
	live, _, _, err := MergeStableAndUnstable(mustMeta(t, dir), mustSchema(t, dir), mustUnstableMeta(t, dir), mustUnstableSchema(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if live.AgentMethods["late"] != "x/late" {
		t.Fatalf("the control: the live merge lacks x/late: %v", live.AgentMethods)
	}
}

// jsonEdit decodes b as a JSON object, applies fn, and encodes it again.
func jsonEdit(t *testing.T, b []byte, fn func(map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	fn(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mustMeta(t *testing.T, dir string) *Meta {
	t.Helper()
	m, err := ReadMeta(dir)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func mustSchema(t *testing.T, dir string) *Schema {
	t.Helper()
	s, err := ReadSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustUnstableMeta(t *testing.T, dir string) *Meta {
	t.Helper()
	m, _, err := ReadMetaUnstable(dir)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func mustUnstableSchema(t *testing.T, dir string) *Schema {
	t.Helper()
	s, _, err := ReadSchemaUnstable(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
