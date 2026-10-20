package acp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestDeprecationNotices requires each notice in the package's hand-written
// non-test files to be in the one form staticcheck honours: a comment line
// whose text starts with "deprecated", in any case, must start with
// "Deprecated: " and begin a paragraph, and no other comment line may hold
// "deprecated:" in any case. gocritic's deprecatedComment, which make lint
// runs, reports three of the four forms staticcheck ignores; the notice in
// the middle of a line only this test reports. The generated files are left
// to the generator's engine cases (0024-PLAN-api-gate-and-deprecation.md).
func TestDeprecationNotices(t *testing.T) {
	t.Parallel()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		if ast.IsGenerated(f) {
			continue
		}
		for _, g := range f.Comments {
			for _, c := range g.List {
				for _, problem := range noticeProblems(g, c) {
					t.Errorf("%s: %s", fset.Position(c.Slash), problem)
				}
			}
		}
	}
}

// noticeProblems returns what is wrong with the deprecation notice comment
// c of group g holds, if it holds one.
func noticeProblems(g *ast.CommentGroup, c *ast.Comment) []string {
	text, ok := strings.CutPrefix(c.Text, "//")
	if !ok {
		// A block comment is checked line by line, each line its own paragraph start.
		var out []string
		for line := range strings.Lines(strings.TrimSuffix(strings.TrimPrefix(c.Text, "/*"), "*/")) {
			out = append(out, lineProblems(line, true)...)
		}
		return out
	}
	start := c == g.List[0]
	for i, prev := range g.List {
		if prev == c && i > 0 {
			start = strings.TrimSpace(strings.TrimPrefix(g.List[i-1].Text, "//")) == ""
		}
	}
	return lineProblems(text, start)
}

// lineProblems returns what is wrong with a comment line's notice; start is
// whether the line begins a paragraph.
func lineProblems(line string, start bool) []string {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(lower, "deprecated") && !strings.HasPrefix(trimmed, "Deprecated: "):
		return []string{"a notice must start with \"Deprecated: \": " + trimmed}
	case strings.HasPrefix(lower, "deprecated") && !start:
		return []string{"a notice must begin a paragraph, after a blank comment line: " + trimmed}
	case !strings.HasPrefix(lower, "deprecated") && strings.Contains(lower, "deprecated:"):
		return []string{"a notice must begin its own line and paragraph: " + trimmed}
	}
	return nil
}
