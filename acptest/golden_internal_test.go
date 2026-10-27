package acptest

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

// TestMethodTables requires ValidateGolden's tables to hold every method
// constant of package acp, an agent's method Sent, a client's Received and
// a protocol method both ways, and nothing else, so a method a schema bump
// adds fails here (0041-PLAN-close-the-gaps-the-consumers-wait-on.md,
// phase 13).
func TestMethodTables(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("../constants_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	want := map[Direction][]string{}
	for _, m := range regexp.MustCompile(`\b(Agent|Client|Protocol)Method\w+\s*=\s*"([^"]+)"`).FindAllSubmatch(b, -1) {
		switch string(m[1]) {
		case "Agent":
			want[Sent] = append(want[Sent], string(m[2]))
		case "Client":
			want[Received] = append(want[Received], string(m[2]))
		default:
			want[Sent] = append(want[Sent], string(m[2]))
			want[Received] = append(want[Received], string(m[2]))
		}
	}
	if len(want[Sent]) < 28 || len(want[Received]) < 12 {
		t.Fatalf("constants_gen.go gave %d agent and %d client methods, want at least the 28 and 12 of schema 1.25.0", len(want[Sent]), len(want[Received]))
	}
	for dir, methods := range want {
		var have []string
		for m := range methodTables[dir] {
			have = append(have, m)
		}
		slices.Sort(have)
		slices.Sort(methods)
		if !slices.Equal(have, methods) {
			t.Errorf("%s: the table holds %q, want the constants' %q", dir, have, methods)
		}
	}
}
