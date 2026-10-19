package emit

import (
	"go/ast"
	"testing"
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
