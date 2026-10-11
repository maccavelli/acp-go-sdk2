package enginetest

import (
	"cmp"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// RuntimeFiles are the hand-written files of the SDK that refer to generated
// code only through Agent, Client, and ProtocolMethodCancelRequest, so the code
// generated for a synthetic schema compiles with them. helpers.go uses ACP
// types and is not one of them.
var RuntimeFiles = []string{"agent.go", "client.go", "connection.go", "doc.go", "errors.go", "extensions.go"}

// CopyRuntime copies RuntimeFiles from the SDK root into dir.
func CopyRuntime(root, dir string) error {
	for _, name := range RuntimeFiles {
		b, err := os.ReadFile(filepath.Join(root, name)) //nolint:gosec // G304: a file of the SDK root the tests name.
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil { //nolint:gosec // G703: name is one of RuntimeFiles, dir a test's directory.
			return err
		}
	}
	return nil
}

// shared is the one source importer LoadPackage uses for every package. It
// keeps what it has imported, the standard library above all, so each case
// does not import it again; it is not safe for concurrent use, so every
// import holds mu. Every package is parsed into its file set.
var shared = sync.OnceValue(func() *lockedImporter {
	fset := token.NewFileSet()
	return &lockedImporter{fset: fset, imp: importer.ForCompiler(fset, "source", nil)}
})

type lockedImporter struct {
	mu   sync.Mutex
	fset *token.FileSet
	imp  types.Importer
}

func (l *lockedImporter) Import(path string) (*types.Package, error) {
	return l.ImportFrom(path, "", 0)
}

func (l *lockedImporter) ImportFrom(path, dir string, mode types.ImportMode) (*types.Package, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if from, ok := l.imp.(types.ImporterFrom); ok {
		return from.ImportFrom(path, dir, mode)
	}
	return l.imp.Import(path)
}

// LoadPackage type-checks the non-test Go files the go command would build in
// dir, reading imports from source, and returns every error it finds.
func LoadPackage(dir string) (*types.Package, *token.FileSet, error) {
	bp, err := build.Default.ImportDir(dir, 0)
	if err != nil {
		return nil, nil, err
	}
	imp := shared()
	fset := imp.fset
	files := make([]*ast.File, 0, len(bp.GoFiles))
	for _, name := range bp.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, f)
	}
	var errs []error
	conf := types.Config{
		Importer: imp,
		Error:    func(err error) { errs = append(errs, err) },
	}
	pkg, _ := conf.Check(bp.Name, fset, files, nil) //nolint:errcheck // Error collects every error, including the one returned.
	if len(errs) > 0 {
		return nil, nil, fmt.Errorf("package %s does not type-check: %w", dir, errors.Join(errs...))
	}
	return pkg, fset, nil
}

// API lists the exported declarations of pkg made in the files keep accepts,
// one per line: the package-level objects in name order, a constant with its
// value, then the exported
// methods declared in those files, by receiver and name, also when the
// receiver is declared elsewhere. Types are written relative to pkg.
func API(pkg *types.Package, fset *token.FileSet, keep func(file string) bool) string {
	in := func(obj types.Object) bool { return keep(filepath.Base(fset.Position(obj.Pos()).Filename)) }
	qual := func(p *types.Package) string {
		if p == pkg {
			return ""
		}
		return p.Name()
	}
	var b strings.Builder
	type method struct{ recv, name, line string }
	var methods []method
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if obj.Exported() && in(obj) {
			b.WriteString(types.ObjectString(obj, qual))
			if c, ok := obj.(*types.Const); ok {
				b.WriteString(" = " + c.Val().ExactString())
			}
			b.WriteString("\n")
		}
		named, ok := obj.Type().(*types.Named)
		if _, isType := obj.(*types.TypeName); !ok || !isType {
			continue
		}
		for m := range named.Methods() {
			if m.Exported() && in(m) {
				methods = append(methods, method{name, m.Name(), types.ObjectString(m, qual)})
			}
		}
	}
	slices.SortFunc(methods, func(x, y method) int { return cmp.Or(cmp.Compare(x.recv, y.recv), cmp.Compare(x.name, y.name)) })
	for _, m := range methods {
		b.WriteString(m.line + "\n")
	}
	return b.String()
}
