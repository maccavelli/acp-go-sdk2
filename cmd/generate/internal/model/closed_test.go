package model

import (
	"slices"
	"strings"
	"testing"
)

// closedDefs adds, to protocolDefs, a closed enum E, a closed oneOf of
// consts S, an open enum O, and Holder, whose fields hold them in each shape
// the check reaches.
const closedDefs = `"E":{"type":"string","enum":["a","b"]},
	"S":{"oneOf":[{"type":"string","const":"x"},{"type":"string","const":"y"}]},
	"O":{"anyOf":[{"type":"string","const":"p"},{"type":"string"}]},
	"Holder":{"type":"object","required":["req"],"properties":{
		"req":{"$ref":"#/$defs/E"},
		"opt":{"$ref":"#/$defs/S"},
		"ptr":{"anyOf":[{"$ref":"#/$defs/E"},{"type":"null"}]},
		"list":{"type":"array","items":{"$ref":"#/$defs/S"}},
		"open":{"$ref":"#/$defs/O"}}},`

// TestClosedEnumChecks requires a closed enumeration, an enum or a oneOf of
// consts, to get a membership check in each field that holds it, by value,
// required or not, by pointer, or in a slice, with its constants, and the
// field marked Closed; an open enum to get none; and one held in a map to be
// refused (0049-PLAN-surface-fixes.md, phase 3).
func TestClosedEnumChecks(t *testing.T) {
	t.Parallel()
	build := func(defs string) (*Model, error) {
		return Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, strings.Replace(protocolDefs, `"Update":`, defs+`"Update":`, 1)),
			Protocol: testProtocol()})
	}
	m, err := build(closedDefs)
	if err != nil {
		t.Fatal(err)
	}
	h := m.Type("Holder")
	got := map[string]Check{}
	for _, c := range h.Validation.Checks {
		if c.Kind == CheckClosedEnum {
			got[c.Key] = c
		}
	}
	for key, want := range map[string]Check{
		"req":  {Shape: RefNamed, Type: "E", Required: true, Values: []string{"EA", "EB"}},
		"opt":  {Shape: RefNamed, Type: "S", Values: []string{"SX", "SY"}},
		"ptr":  {Shape: RefPointer, Type: "E", Values: []string{"EA", "EB"}},
		"list": {Shape: RefSlice, Type: "S", Values: []string{"SX", "SY"}},
	} {
		c, ok := got[key]
		if !ok || c.Shape != want.Shape || c.Type != want.Type || c.Required != want.Required || !slices.Equal(c.Values, want.Values) {
			t.Errorf("%s: check %+v, want %+v", key, c, want)
		}
	}
	if _, ok := got["open"]; ok {
		t.Error("open: an open enum got a membership check")
	}
	for _, f := range h.Fields {
		if f.Closed != (f.Key != "open") {
			t.Errorf("%s: Closed %v", f.Key, f.Closed)
		}
	}

	mapped := `"E":{"type":"string","enum":["a","b"]},
	"Holder":{"type":"object","properties":{"m":{"type":"object","additionalProperties":{"$ref":"#/$defs/E"}}}},`
	if _, err := build(mapped); err == nil || !strings.Contains(err.Error(), "Holder.m: a closed enumeration in map[string]E is not checked") {
		t.Errorf("a map of a closed enumeration: Build = %v", err)
	}
}

// checkCommittedClosedEnums requires the closed enumerations of the committed
// schemas to be the fourteen the surface PLAN names, so a new one is noticed
// (0049-PLAN-surface-fixes.md, phase 3).
func checkCommittedClosedEnums(t *testing.T, m *Model) {
	t.Helper()
	var got []string
	for _, ty := range m.Types {
		if m.isClosedEnum(ty.Name) {
			got = append(got, ty.Name)
		}
	}
	slices.Sort(got)
	want := []string{"ElicitationSchemaType", "PermissionOptionKind", "PlanEntryPriority", "PlanEntryStatus", "PositionEncodingKind",
		"Role", "StopReason", "StringFormat", "TextDocumentSyncKind", "ToolCallStatus", "ToolKind",
		"UnstableNesDiagnosticSeverity", "UnstableNesRejectReason", "UnstableNesTriggerKind"}
	if !slices.Equal(got, want) {
		t.Errorf("closed enumerations: %v, want %v", got, want)
	}
}
