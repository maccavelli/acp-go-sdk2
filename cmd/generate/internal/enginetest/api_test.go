package enginetest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const apiRuntime = `package acp

type Conn struct{}

func (c *Conn) Close() error { return nil }
`

const apiGenerated = `package acp

const KindA Kind = "a"

type Kind string

type Shape struct {
	R int64 ` + "`json:\"r\"`" + `
	hidden bool
}

func (s *Shape) UnmarshalJSON(b []byte) error { return nil }

func (s Shape) Area() float64 { return 0 }

func (s Shape) secret() {}

func (c *Conn) Send(s Shape) error { return nil }

func NewShape(r int64) Shape { return Shape{R: r} }

var unexported int
`

// TestAPI lists the exported declarations of the generated file only: types
// with their fields, constants, functions, and the methods declared there,
// also on a runtime type.
func TestAPI(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, src := range map[string]string{"runtime.go": apiRuntime, "x_gen.go": apiGenerated} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pkg, fset, err := LoadPackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := API(pkg, fset, func(name string) bool { return strings.HasSuffix(name, "_gen.go") })
	want := `type Kind string
const KindA Kind = "a"
func NewShape(r int64) Shape
type Shape struct{R int64 "json:\"r\""; hidden bool}
func (*Conn).Send(s Shape) error
func (Shape).Area() float64
func (*Shape).UnmarshalJSON(b []byte) error
`
	if got != want {
		t.Errorf("API =\n%s\nwant\n%s", got, want)
	}
}

func TestLoadPackageReportsTypeErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package acp\n\nvar X = undefinedName\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadPackage(dir); err == nil || !strings.Contains(err.Error(), "undefined: undefinedName") {
		t.Errorf("error = %v, want one naming undefinedName", err)
	}
}

// TestCopyRuntime copies the runtime files of a root into an empty
// directory, and refuses a root that lacks one.
func TestCopyRuntime(t *testing.T) {
	t.Parallel()
	root, dir := t.TempDir(), t.TempDir()
	for _, name := range RuntimeFiles {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package acp // "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := CopyRuntime(root, dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range RuntimeFiles {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(b) != "package acp // "+name+"\n" {
			t.Errorf("%s = %q, %v", name, b, err)
		}
	}
	if err := os.Remove(filepath.Join(root, RuntimeFiles[0])); err != nil {
		t.Fatal(err)
	}
	if err := CopyRuntime(root, t.TempDir()); err == nil {
		t.Error("CopyRuntime accepted a root without " + RuntimeFiles[0])
	}
}
