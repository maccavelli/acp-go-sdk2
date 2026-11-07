package output

import (
	"cmp"
	"encoding/json"
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
	"os/exec"
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
// names; dir must exist (G16). The language version is the go line of the
// go.mod that holds dir, by FindGoMod, when there is one (0049-MADR, G28).
// Imports are read from source, by one importer that every call in the
// process shares.
//
// The files of package unstable, in emit.UnstableDir, are then checked with
// its hand-written files, when its directory exists, through an importer
// that returns the root package just checked for root, its import path, and
// the shared importer's for any other path: the root exists only in memory
// until Write (0039-PLAN-five-adopter-additions-on-schema-1-25-0.md, 2f).
// The hand-written files of package unstable are checked even when nothing
// renders into it.
//
// The tree checked is the one Write leaves: the stale generated files that
// Write removes, by staleFiles, are not read as hand-written
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G1).
//
// The root's external test package, its package *_test files, is checked
// too, against the package as tested, and so is each package of the module
// under dir that it imports, from source, against the same root
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G27).
//
// The host is not the only platform the package is built for. For linux/amd64
// and windows/amd64, when the files the go command would build or test there
// differ from the host's, the package and its tests are compiled by the go
// command for that platform, with the generated files in memory through
// -overlay, since the in-process importer reads the standard library for the
// host alone (G9, as amended by 0049-PLAN-generator-fixes.md's phase 3,
// deviation 1).
func Check(dir, root string, files []emit.File) error {
	var rootFiles, unstableFiles []emit.File
	for _, f := range files {
		if f.Dir == emit.UnstableDir {
			unstableFiles = append(unstableFiles, f)
		} else {
			rootFiles = append(rootFiles, f)
		}
	}
	stale, err := staleFiles(dir, files)
	if err != nil {
		return err
	}
	host, err := listFor(dir, rootFiles, unstableFiles, stale, platform{})
	if err != nil {
		return err
	}
	version, err := goVersion(dir)
	if err != nil {
		return err
	}
	if err := checkHost(dir, root, version, rootFiles, unstableFiles, host); err != nil {
		return err
	}
	seen := []fileLists{host}
	for _, p := range otherPlatforms {
		l, err := listFor(dir, rootFiles, unstableFiles, stale, p)
		if err != nil {
			return err
		}
		if slices.ContainsFunc(seen, l.equal) {
			continue
		}
		seen = append(seen, l)
		if err := compileFor(dir, files, stale, p, l.unstable); err != nil {
			return err
		}
	}
	return nil
}

// checkHost type-checks the package for the host in process, as Check says:
// as built, as tested, its external tests, and package unstable.
func checkHost(dir, root, version string, rootFiles, unstableFiles []emit.File, l fileLists) error {
	imp := sharedImporter()
	built := slices.DeleteFunc(slices.Clone(rootFiles), func(f emit.File) bool { return strings.HasSuffix(f.Name, "_test.go") })
	pkg, err := checkPackage(dir, version, l.hand, built, imp)
	if err != nil {
		return err
	}
	tested := pkg
	if len(built) != len(rootFiles) || len(l.tests) > 0 {
		if tested, err = checkPackage(dir, version, slices.Concat(l.hand, l.tests), rootFiles, imp); err != nil {
			return err
		}
	}
	if len(l.xtests) > 0 {
		mi := &moduleImporter{dir: dir, root: root, version: version, pkg: tested, next: imp, done: map[string]*types.Package{}}
		if _, err := checkPackage(dir, version, l.xtests, nil, mi); err != nil {
			return err
		}
	}
	if !l.unstable {
		return nil
	}
	_, err = checkPackage(filepath.Join(dir, emit.UnstableDir), version, l.uhand, unstableFiles, servingImporter{path: root, pkg: pkg, next: imp})
	return err
}

// platform is a GOOS and GOARCH; the zero platform is the host.
type platform struct{ goos, goarch string }

func (p platform) String() string {
	if p.goos == "" {
		return "the host"
	}
	return p.goos + "/" + p.goarch
}

// otherPlatforms are the platforms besides the host that Check compiles for
// when their files differ from the host's: those CI builds for
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G9).
var otherPlatforms = []platform{{"linux", "amd64"}, {"windows", "amd64"}}

// fileLists are the hand-written files of dir the go command would build or
// test for one platform: the root's, its in-package tests, its external
// tests, and package unstable's, which is checked when unstable is true.
type fileLists struct {
	hand, tests, xtests, uhand []string
	unstable                   bool
}

func (l fileLists) equal(o fileLists) bool {
	return slices.Equal(l.hand, o.hand) && slices.Equal(l.tests, o.tests) && slices.Equal(l.xtests, o.xtests) &&
		slices.Equal(l.uhand, o.uhand) && l.unstable == o.unstable
}

// listFor lists the hand-written files of dir and of its package unstable for
// p, less the generated and the stale ones.
func listFor(dir string, rootFiles, unstableFiles []emit.File, stale []string, p platform) (fileLists, error) {
	var l fileLists
	var err error
	if l.hand, l.tests, l.xtests, err = handWritten(dir, rootFiles, staleIn(stale, ""), p); err != nil {
		return l, err
	}
	udir := filepath.Join(dir, emit.UnstableDir)
	if _, err := os.Stat(udir); err == nil {
		if l.uhand, _, _, err = handWritten(udir, unstableFiles, staleIn(stale, emit.UnstableDir), p); err != nil {
			return l, err
		}
	}
	l.unstable = len(unstableFiles) > 0 || len(l.uhand) > 0
	return l, nil
}

// compileFor compiles the package in dir and its tests, and package unstable
// when unstable is set, for p with the go command, as `go test -c` builds
// them, files in place of what dir holds by -overlay: the generated files
// with their rendered bytes, and the stale files as deleted. Nothing under
// dir is written.
func compileFor(dir string, files []emit.File, stale []string, p platform, unstable bool) error {
	tmp, err := os.MkdirTemp("", "acp-generate-check-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // a temporary directory; failing to remove it loses nothing.
	// The go command resolves its working directory through symbolic links,
	// as macOS's temporary directories need, so the overlay's paths must too,
	// or they match nothing and the rendered files are silently left out.
	abs, err := filepath.Abs(dir)
	if err == nil {
		abs, err = filepath.EvalSymlinks(abs)
	}
	if err != nil {
		return err
	}
	replace := map[string]string{}
	for i, f := range files {
		src := filepath.Join(tmp, fmt.Sprintf("%d_%s", i, f.Name))
		if err := os.WriteFile(src, f.Src, 0o600); err != nil {
			return err
		}
		replace[filepath.Join(abs, f.Path())] = src
	}
	for _, s := range stale {
		replace[filepath.Join(abs, s)] = ""
	}
	overlay, err := json.Marshal(map[string]any{"Replace": replace})
	if err != nil {
		return err
	}
	overlayPath := filepath.Join(tmp, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o600); err != nil {
		return err
	}
	bin := filepath.Join(tmp, "bin") + string(filepath.Separator)
	if err := os.Mkdir(bin, 0o750); err != nil {
		return err
	}
	pkgs := []string{"."}
	if unstable {
		pkgs = append(pkgs, "./"+emit.UnstableDir)
	}
	cmd := exec.Command("go", slices.Concat([]string{"test", "-c", "-o", bin, "-overlay", overlayPath}, pkgs)...) //nolint:gosec // G204: the go command, with paths Check makes.
	cmd.Dir = abs
	cmd.Env = append(os.Environ(), "GOOS="+p.goos, "GOARCH="+p.goarch, "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("generated code does not compile for %s with %s: %w:\n\t%s", p, dir, err,
			strings.ReplaceAll(strings.TrimSpace(string(out)), "\n", "\n\t"))
	}
	return nil
}

// moduleImporter serves the external test package of the root: pkg, the root
// as tested, for root; each package of the module under dir that is imported,
// type-checked from source against that same root, cached in done; and next's
// for any other path (0049-MADR-remediate-the-2026-11-01-debugging-pass.md,
// G27). It is used by one check at a time.
type moduleImporter struct {
	dir, root, version string
	pkg                *types.Package
	next               *lockedImporter
	done               map[string]*types.Package
}

func (m *moduleImporter) Import(path string) (*types.Package, error) {
	return m.ImportFrom(path, "", 0)
}

func (m *moduleImporter) ImportFrom(path, dir string, mode types.ImportMode) (*types.Package, error) {
	if path == m.root {
		return m.pkg, nil
	}
	rel, ok := strings.CutPrefix(path, m.root+"/")
	if !ok {
		return m.next.ImportFrom(path, dir, mode)
	}
	if p, ok := m.done[path]; ok {
		return p, nil
	}
	pdir := filepath.Join(m.dir, filepath.FromSlash(rel))
	ctxt := build.Default
	ctxt.CgoEnabled = true
	bp, err := ctxt.ImportDir(pdir, 0)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	fset := sharedImporter().fset
	parsed := make([]*ast.File, 0, len(bp.GoFiles))
	for _, name := range bp.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(pdir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, f)
	}
	var first error
	conf := types.Config{GoVersion: m.version, Importer: m, Error: func(err error) {
		if first == nil {
			first = err
		}
	}}
	p, _ := conf.Check(path, fset, parsed, nil) //nolint:errcheck // Error keeps the first error, the one returned.
	if first != nil {
		return nil, fmt.Errorf("%s, checked against the generated root: %w", path, first)
	}
	m.done[path] = p
	return p, nil
}

// staleIn returns the names of the files of stale, paths under the output
// directory, that are in its package directory d.
func staleIn(stale []string, d string) map[string]bool {
	out := map[string]bool{}
	for _, p := range stale {
		if dir, name := filepath.Split(p); filepath.Clean(dir) == filepath.Clean(d) {
			out[name] = true
		}
	}
	return out
}

// servingImporter returns pkg for path and defers every other import to next.
type servingImporter struct {
	path string
	pkg  *types.Package
	next *lockedImporter
}

func (s servingImporter) Import(path string) (*types.Package, error) {
	return s.ImportFrom(path, "", 0)
}

func (s servingImporter) ImportFrom(path, dir string, mode types.ImportMode) (*types.Package, error) {
	if path == s.path {
		return s.pkg, nil
	}
	return s.next.ImportFrom(path, dir, mode)
}

// checkPackage type-checks the hand-written files named hand, in dir, with
// the generated files, importing through imp, and returns the package.
func checkPackage(dir, version string, hand []string, files []emit.File, imp types.ImporterFrom) (*types.Package, error) {
	fset := sharedImporter().fset
	parsed := make([]*ast.File, 0, len(hand)+len(files))
	for _, name := range hand {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, f)
	}
	// The package's name, from its files: a hand-written file's when nothing
	// is generated into dir.
	pkg := ""
	if len(parsed) > 0 {
		pkg = parsed[0].Name.Name
	}
	for _, gf := range files {
		f, err := parser.ParseFile(fset, filepath.Join(dir, gf.Name), gf.Src, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
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
	checked, _ := conf.Check(pkg, fset, parsed, nil) //nolint:errcheck // Error collects every error, including the one returned.
	if len(errs) == 0 {
		return checked, nil
	}
	generated := map[string]bool{}
	for _, gf := range files {
		generated[filepath.Join(dir, gf.Name)] = true
	}
	return nil, report(dir, fset, errs, generated)
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
// dir for p, of its in-package test files, and of its external test files,
// less the generated ones and the stale ones Write removes, by name. A
// missing dir is refused: the generated code needs the hand-written runtime
// beside it (0019-PLAN-generator-hardening.md, G16).
func handWritten(dir string, files []emit.File, stale map[string]bool, p platform) (hand, tests, xtests []string, err error) {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil, fmt.Errorf("output directory %s: does not exist", dir)
	}
	// With cgo on, a file that imports "C" is listed as such whatever the
	// environment says, so it is refused rather than silently left out.
	ctxt := build.Default
	ctxt.CgoEnabled = true
	if p.goos != "" {
		ctxt.GOOS, ctxt.GOARCH = p.goos, p.goarch
	}
	bp, err := ctxt.ImportDir(dir, 0)
	if _, ok := errors.AsType[*build.NoGoError](err); ok {
		return nil, nil, nil, nil
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("output directory: %w", err)
	}
	if len(bp.CgoFiles) > 0 {
		return nil, nil, nil, fmt.Errorf("output directory: cgo file %s is not supported", bp.CgoFiles[0])
	}
	generated := func(name string) bool {
		return stale[name] || slices.ContainsFunc(files, func(f emit.File) bool { return f.Name == name })
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
	for _, name := range bp.XTestGoFiles {
		if !generated(name) {
			xtests = append(xtests, name)
		}
	}
	return hand, tests, xtests, nil
}

// FindGoMod returns the path and the bytes of the go.mod that holds dir: the
// first one found walking up from dir, as the go command finds it. The path
// is "" when there is none. The generator takes the root's import path from
// it, and Check the language version, so the two read the same file
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G28).
func FindGoMod(dir string) (string, []byte, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", nil, err
	}
	for d := abs; ; {
		path := filepath.Join(d, "go.mod")
		b, err := os.ReadFile(path) //nolint:gosec // G304: a go.mod at or above the generator's explicit output directory.
		if err == nil {
			return path, b, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", nil, err
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", nil, nil
		}
		d = parent
	}
}

// goVersion returns the go line of the go.mod that holds dir, by FindGoMod,
// as a types.Config GoVersion, or "" when there is none.
func goVersion(dir string) (string, error) {
	path, b, err := FindGoMod(dir)
	if err != nil || path == "" {
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
