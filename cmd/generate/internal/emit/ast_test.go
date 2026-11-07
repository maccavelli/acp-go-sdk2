package emit

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// TestSwitchOrIf requires a switch of one case of one value to become an if,
// and every other switch to stay one: several cases, several values, a
// default, or a case whose body breaks out of the switch, which an if would
// turn into a break of the enclosing loop
// (0019-PLAN-idioms-performance.md, D9).
func TestSwitchOrIf(t *testing.T) {
	t.Parallel()
	body := func() ast.Stmt { return ret(nilExpr()) }
	brk := func() ast.Stmt { return breakStmt() }
	for _, tt := range []struct {
		name    string
		clauses []ast.Stmt
		wantIf  bool
	}{
		{"one case of one value", []ast.Stmt{caseOf(exprs(str("a")), body())}, true},
		{"a break inside its own loop", []ast.Stmt{caseOf(exprs(str("a")), rangeStmt(id("_"), id("x"), id("xs"), brk()))}, true},
		{"a break inside its own switch", []ast.Stmt{caseOf(exprs(str("a")), switchStmt(id("y"), caseOf(nil, brk())))}, true},
		{"a break inside a function literal", []ast.Stmt{caseOf(exprs(str("a")), exprStmt(call(gateFuncLit(brk()))))}, true},
		{"one case of two values", []ast.Stmt{caseOf(exprs(str("a"), str("b")), body())}, false},
		{"two cases", []ast.Stmt{caseOf(exprs(str("a")), body()), caseOf(exprs(str("b")), body())}, false},
		{"a default only", []ast.Stmt{caseOf(nil, body())}, false},
		{"a break out of the switch", []ast.Stmt{caseOf(exprs(str("a")), brk())}, false},
		{"a break in a nested if", []ast.Stmt{caseOf(exprs(str("a")), ifStmt(nil, id("ok"), brk()))}, false},
	} {
		s := switchOrIf(id("tag"), tt.clauses...)
		if _, isIf := s.(*ast.IfStmt); isIf != tt.wantIf {
			t.Errorf("%s: %T, want an if: %v", tt.name, s, tt.wantIf)
		}
	}
}

// TestNestedGateItems requires the gates call into a union's variant that is a
// named slice or map to call each item's, when the variant is set
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G19).
func TestNestedGateItems(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		shape model.RefKind
		want  []string
	}{
		{model.RefSlice, []string{"if v.Many != nil {", "for i := range *v.Many {", "(*v.Many)[i].gates(n, method)"}},
		{model.RefMap, []string{"if v.Many != nil {", "maps.Keys((*v.Many))", "x := (*v.Many)[k]", "x.gates(n, method)"}},
	} {
		s := nestedGate(newFile(), func() ast.Expr { return dot("v", "Many") },
			model.GateCheck{Kind: model.GateCheckNested, Field: "Many", Shape: tt.shape, Items: true})
		var b bytes.Buffer
		if err := printer.Fprint(&b, token.NewFileSet(), s); err != nil {
			t.Fatal(err)
		}
		flat := strings.Join(strings.Fields(b.String()), " ")
		for _, w := range tt.want {
			if !strings.Contains(flat, strings.Join(strings.Fields(w), " ")) {
				t.Errorf("shape %v: %s lacks %q", tt.shape, flat, w)
			}
		}
	}
}
