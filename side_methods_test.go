package acp

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"
	"testing"
)

// TestSideConnectionMethodsAreProtocolMethods requires every exported method
// of AgentSideConnection and ClientSideConnection that takes a context to be
// a protocol method: declared in a generated file, or one of the extension
// methods of extensions.go. TestTypedMethodDocsNameErrors, TestNotInitialized
// and TestConcurrentUseOfOneConnection treat each such method as a wire call,
// so a hand-written helper that takes a side connection is a package-level
// function, as AllSessions and SendRequest are; this test names the rule
// where the others would only miscount
// (0038-PLAN-consumer-helpers-kind-and-agent-refusals.md, step 2's deviation).
func TestSideConnectionMethodsAreProtocolMethods(t *testing.T) {
	t.Parallel()
	fset, files := parsePackage(t)
	seen := 0
	for _, f := range files {
		name := filepath.Base(fset.Position(f.Pos()).Filename)
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !fn.Name.IsExported() {
				continue
			}
			recv := receiverName(fn.Recv.List[0].Type)
			if recv != "AgentSideConnection" && recv != "ClientSideConnection" {
				continue
			}
			if p := fn.Type.Params.List; len(p) == 0 || types.ExprString(p[0].Type) != "context.Context" {
				continue
			}
			seen++
			if strings.HasSuffix(name, "_gen.go") || name == "extensions.go" {
				continue
			}
			t.Errorf("%s: %s.%s takes a context but is declared in %s, not in a generated file or extensions.go; a helper "+
				"that takes a side connection is a package-level function, as AllSessions is", fset.Position(fn.Pos()), recv,
				fn.Name.Name, name)
		}
	}
	if seen == 0 {
		t.Fatal("no method of a side connection takes a context: the walk found nothing")
	}
}
