package output

import (
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Edges found by the mutation run of 0011-PLAN-engine-tests.md, step 9.

// TestGoVersionWithoutGoLine requires a go.mod without a go directive to give
// no version and no error.
func TestGoVersionWithoutGoLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if v, err := goVersion(dir); v != "" || err != nil {
		t.Errorf("goVersion = %q, %v; want none", v, err)
	}
}

// TestReportGroupsSecondaryErrors requires a secondary error to follow its
// primary, every error to be listed, and a secondary error with no primary
// before it to stand alone.
func TestReportGroupsSecondaryErrors(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	f := fset.AddFile("a.go", -1, 100)
	f.SetLines([]int{0, 10, 20, 30, 40})
	at := func(line int, msg string) types.Error {
		return types.Error{Fset: fset, Pos: f.LineStart(line), Msg: msg}
	}
	err := report("dir", fset, []types.Error{at(1, "\torphan"), at(2, "early"), at(2, "\tdetail of early"), at(4, "late")})
	if err == nil {
		t.Fatal("no error")
	}
	got := err.Error()
	order := []string{"orphan", "early", "detail of early", "late"}
	last := -1
	for _, w := range order {
		i := strings.Index(got, w)
		if i < 0 || i < last {
			t.Fatalf("report:\n%s\nwant %v in that order", got, order)
		}
		last = i
	}
}
