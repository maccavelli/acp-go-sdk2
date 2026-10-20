package enginetest

import (
	"os"
	"path/filepath"
	"testing"
)

const deprecatedSource = `package acp

// Old is a type.
//
// Deprecated: use New,
// which is newer.
type Old struct {
	// A is a field.
	//
	// Deprecated: use B.
	A string
	B string
}

// Do is an interface.
type Do interface {
	// Run runs.
	//
	// Deprecated: use Go.
	Run() error
}

// Gone is a constant.
//
// Deprecated: use Here.
const Gone = 1

const (
	// Here is a constant.
	Here = 2
	// There is a constant.
	//
	// Deprecated: use Here.
	There = 3
)

// Was is a variable.
//
// Deprecated: use Is.
var Was int

// F is a function.
//
// Deprecated: use G.
func F() {}

// M is a method.
//
// Deprecated: use N.
func (o *Old) M() {}

// V is a method.
//
// Deprecated: use W.
func (o Old) V() {}

// NoBlankLine is a probe.
// Deprecated: use X.
func NoBlankLine() {}

// MidLine is a probe. Deprecated: use X.
func MidLine() {}

// Lowercase is a probe.
//
// deprecated: use X.
func Lowercase() {}

// NoColon is a probe.
//
// Deprecated, use X.
func NoColon() {}
`

// TestDeprecations requires one line per declaration whose doc comment holds
// a "Deprecated: " paragraph, of each kind, sorted, with the paragraph's
// lines joined; none for the forms staticcheck ignores (no blank line before
// it, mid-line, lower case, a comma); and none from a file keep refuses or a
// test file.
func TestDeprecations(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, src := range map[string]string{
		"gen.go":     deprecatedSource,
		"skipped.go": "package acp\n\n// S is skipped.\n//\n// Deprecated: kept out.\nfunc S() {}\n",
		"x_test.go":  "package acp\n\n// T is a test helper.\n//\n// Deprecated: a test file.\nfunc T() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Deprecations(dir, func(name string) bool { return name != "skipped.go" })
	if err != nil {
		t.Fatal(err)
	}
	want := `const Gone: Deprecated: use Here.
const There: Deprecated: use Here.
field Old.A: Deprecated: use B.
func F: Deprecated: use G.
method (*Old).M: Deprecated: use N.
method (Old).V: Deprecated: use W.
method Do.Run: Deprecated: use Go.
type Old: Deprecated: use New, which is newer.
var Was: Deprecated: use Is.
`
	if got != want {
		t.Errorf("Deprecations:\n%s\nwant:\n%s", got, want)
	}
	if got, err := Deprecations(dir, func(string) bool { return false }); err != nil || got != "" {
		t.Errorf("Deprecations of no file = %q, %v; want empty", got, err)
	}
}
