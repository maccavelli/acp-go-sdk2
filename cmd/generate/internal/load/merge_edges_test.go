package load

import "testing"

// Edges of the merge that the committed schema does not reach, found by the
// mutation run of 0011-PLAN-engine-tests.md, step 9.

// TestPromoteSkipsNullStableDefinition requires a stable definition that is
// JSON null to be passed over, not dereferenced.
func TestPromoteSkipsNullStableDefinition(t *testing.T) {
	t.Parallel()
	stable := &Schema{Defs: map[string]*Definition{"NullRequest": nil}}
	got := promoteChangedSharedDefs(&Schema{Defs: map[string]*Definition{}}, stable, &Schema{Defs: map[string]*Definition{}}, map[string]struct{}{"m": {}})
	if len(got) != 0 {
		t.Errorf("promoted %v", got)
	}
}

// TestPromoteContinuesPastMissingDefinition requires a stable method
// definition the unstable schema lacks to leave the others to be promoted. The
// queue comes from a map, so the call is repeated until both orders have run.
func TestPromoteContinuesPastMissingDefinition(t *testing.T) {
	t.Parallel()
	method := func(desc string) *Definition {
		return &Definition{Type: "object", XMethod: "m", XSide: "agent", Description: desc}
	}
	for range 64 {
		stable := &Schema{Defs: map[string]*Definition{"GoneRequest": method("gone"), "ChangedRequest": method("old")}}
		unstable := &Schema{Defs: map[string]*Definition{"ChangedRequest": method("new")}}
		combined := &Schema{Defs: map[string]*Definition{}}
		got := promoteChangedSharedDefs(combined, stable, unstable, map[string]struct{}{"m": {}})
		if _, ok := got["ChangedRequest"]; !ok || combined.Defs["ChangedRequest"] == nil || combined.Defs["ChangedRequest"].Description != "new" {
			t.Fatalf("promoted %v, combined %v; want ChangedRequest from the unstable schema", got, combined.Defs)
		}
	}
}

// TestEnsureRefsCopiesUnstableDependency requires a definition the roots
// reference, present only in the unstable schema, to be copied in.
func TestEnsureRefsCopiesUnstableDependency(t *testing.T) {
	t.Parallel()
	combined := &Schema{Defs: map[string]*Definition{"Root": {Type: "object", Properties: map[string]*Definition{"d": ref("Dep")}}}}
	unstable := &Schema{Defs: map[string]*Definition{"Dep": {Type: "string"}}}
	ensureRefsAvailable(combined, unstable, map[string]struct{}{"Root": {}})
	if combined.Defs["Dep"] == nil || combined.Defs["Dep"].Type != "string" {
		t.Errorf("combined %v; want Dep copied from the unstable schema", combined.Defs)
	}
}
