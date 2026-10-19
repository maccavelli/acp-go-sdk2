package output

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
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/emit"
)

// maxReported is how many type errors Check lists before it counts the rest.
const maxReported = 10

// Check type-checks files together with the hand-written Go files of dir, and
// returns an error listing what does not check.
//
// It checks the package as built, the hand-written files with the generated
// files that are not tests, and then the package as tested, which adds the
// hand-written in-package test files and the generated ones, so a name a test
// file of either kind declares twice is refused
// (0019-PLAN-generator-hardening.md, G15). The hand-written files are those
// the go command would build or test in dir for the host, less the generated
// names; dir must exist (G16). The language version is the go line of
// dir/go.mod, when there is one. Imports are read from source, by one
// importer that every call in the process shares.
func Check(dir string, files []emit.File) error {
	hand, handTests, err := handWritten(dir, files)
	if err != nil {
		return err
	}
	version, err := goVersion(dir)
	if err != nil {
		return err
	}
	built := slices.DeleteFunc(slices.Clone(files), func(f emit.File) bool { return strings.HasSuffix(f.Name, "_test.go") })
	if err := checkPackage(dir, version, hand, built); err != nil {
		return err
	}
	if len(built) == len(files) && len(handTests) == 0 {
		return nil
	}
	return checkPackage(dir, version, slices.Concat(hand, handTests), files)
}

// checkPackage type-checks the hand-written files named hand, in dir, with
// the generated files.
func checkPackage(dir, version string, hand []string, files []emit.File) error {
	imp := sharedImporter()
	fset := imp.fset
	parsed := make([]*ast.File, 0, len(hand)+len(files))
	for _, name := range hand {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		parsed = append(parsed, f)
	}
	pkg := ""
	for _, gf := range files {
		f, err := parser.ParseFile(fset, filepath.Join(dir, gf.Name), gf.Src, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		pkg = f.Name.Name
		parsed = append(parsed, f)
	}

	var errs []types.Error
	conf := types.Config{
		GoVersion: version,
		Importer:  imp,
		Error: func(err error) {
			te, ok := errors.AsType[types.Error](err)
			if !ok {
				te = types.Error{Fset: fset, Msg: err.Error()}
			}
			errs = append(errs, te)
		},
	}
	_, _ = conf.Check(pkg, fset, parsed, nil) //nolint:errcheck // Error collects every error, including the one returned.
	if len(errs) == 0 {
		return nil
	}
	generated := map[string]bool{}
	for _, gf := range files {
		generated[filepath.Join(dir, gf.Name)] = true
	}
	return report(dir, fset, errs, generated)
}

// sharedImporter is the source importer every Check uses. It keeps what it
// has imported, the standard library above all, so a process that checks
// many outputs, as the engine tests do, imports it once
// (0011-MADR-harden-code-generator-engine.md, phase 6 amendment). It is not
// safe for concurrent use, so every import holds mu; the files of every
// check are parsed into its file set.
var sharedImporter = sync.OnceValue(func() *lockedImporter {
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

// handWritten returns the names of the Go files the go command would build in
// dir for the host, and of its in-package test files, less the generated
// ones. A missing dir is refused: the generated code needs the hand-written
// runtime beside it (0019-PLAN-generator-hardening.md, G16).
func handWritten(dir string, files []emit.File) (hand, tests []string, err error) {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil, nil, fmt.Errorf("output directory %s: does not exist", dir)
	}
	// With cgo on, a file that imports "C" is listed as such whatever the
	// environment says, so it is refused rather than silently left out.
	ctxt := build.Default
	ctxt.CgoEnabled = true
	bp, err := ctxt.ImportDir(dir, 0)
	if _, ok := errors.AsType[*build.NoGoError](err); ok {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("output directory: %w", err)
	}
	if len(bp.CgoFiles) > 0 {
		return nil, nil, fmt.Errorf("output directory: cgo file %s is not supported", bp.CgoFiles[0])
	}
	generated := func(name string) bool {
		return slices.ContainsFunc(files, func(f emit.File) bool { return f.Name == name })
	}
	for _, name := range bp.GoFiles {
		if !generated(name) {
			hand = append(hand, name)
		}
	}
	for _, name := range bp.TestGoFiles {
		if !generated(name) {
			tests = append(tests, name)
		}
	}
	return hand, tests, nil
}

// goVersion returns the go line of dir/go.mod as a types.Config GoVersion, or
// "" when there is none.
func goVersion(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod")) //nolint:gosec // G304: dir is the generator's explicit output directory.
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	for line := range strings.Lines(string(b)) {
		line, _, _ = strings.Cut(line, "//")
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "go "); ok {
			return "go" + strings.TrimSpace(v), nil
		}
	}
	return "", nil
}

// report lists errs, those in generated files first and each part in
// position order, a secondary error after its primary, at most maxReported of
// them, and counts the rest.
func report(dir string, fset *token.FileSet, errs []types.Error, generated map[string]bool) error {
	// A secondary error's message starts with a tab and belongs to the
	// primary error before it.
	var groups [][]types.Error
	for _, e := range errs {
		if strings.HasPrefix(e.Msg, "\t") && len(groups) > 0 {
			groups[len(groups)-1] = append(groups[len(groups)-1], e)
			continue
		}
		groups = append(groups, []types.Error{e})
	}
	slices.SortStableFunc(groups, func(a, b []types.Error) int {
		pa, pb := fset.Position(a[0].Pos), fset.Position(b[0].Pos)
		return cmp.Or(compareBool(generated[pb.Filename], generated[pa.Filename]), cmp.Compare(pa.Filename, pb.Filename),
			cmp.Compare(pa.Line, pb.Line), cmp.Compare(pa.Column, pb.Column))
	})
	lines := make([]string, 0, len(errs))
	for _, g := range groups {
		for _, e := range g {
			lines = append(lines, e.Error())
		}
	}
	msg := fmt.Sprintf("generated code does not type-check with %s:\n\t%s", dir, strings.Join(lines[:min(len(lines), maxReported)], "\n\t"))
	if n := len(lines) - maxReported; n > 0 {
		msg += fmt.Sprintf("\n\tand %d more", n)
	}
	return errors.New(msg)
}

// compareBool orders false before true.
func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}
