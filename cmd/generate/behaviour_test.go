package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/enginetest"
)

// behaviourModule is the module path of the packages TestEngineBehaviour
// builds, one per case.
const behaviourModule = "enginetest"

// TestEngineBehaviour compiles the code generated for every case that has a
// behaviour_test.go, together with the runtime files, and runs that file's
// tests: one temporary module, one package per case, one go test
// (0011-PLAN-engine-tests.md, D2). A failed module is kept in the test's
// artifact directory (go test -artifacts).
func TestEngineBehaviour(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and runs the generated code with the go command")
	}
	goCmd, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("the go command is needed to run generated code: %v", err)
	}
	version, err := goLine(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	mod := t.TempDir()
	if err := os.WriteFile(filepath.Join(mod, "go.mod"), []byte("module "+behaviourModule+"\n\ngo "+version+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var cases []behaviourCase
	for _, path := range engineCases(t) {
		if c, ok := readBehaviourCase(t, path); ok {
			cases = append(cases, c)
		}
	}
	if len(cases) == 0 {
		t.Fatal("no engine case has a behaviour_test.go")
	}
	// Generating is most of the work outside go test, so the cases are
	// generated in parallel.
	t.Run("generate", func(t *testing.T) {
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				t.Parallel()
				writeBehaviourCase(t, c, mod)
			})
		}
	})
	if t.Failed() {
		return
	}

	cmd := exec.CommandContext(t.Context(), goCmd, "test", "-json", "-count=1", "./...")
	cmd.Dir = mod
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOPROXY=off")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	results := testResults(t, stdout.Bytes())

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := results[behaviourModule+"/"+c.name]
			switch {
			case r == nil:
				t.Fatalf("go test reported nothing for the package; stderr:\n%s", stderr.String())
			case r.action != "pass":
				t.Errorf("the package's tests did not pass (%s):\n%s", r.action, r.output.String())
			case r.tests == 0:
				t.Errorf("the package ran no tests:\n%s", r.output.String())
			}
		})
	}
	if runErr != nil && !t.Failed() {
		t.Errorf("go test failed with every package passing: %v\n%s", runErr, stderr.String())
	}
	if t.Failed() {
		dir := filepath.Join(t.ArtifactDir(), "module")
		if err := os.CopyFS(dir, os.DirFS(mod)); err != nil {
			t.Errorf("keeping the module: %v", err)
		} else {
			t.Logf("module kept in %s", dir)
		}
	}
}

// behaviourCase is an engine case with a behaviour_test.go: its name, which
// is also its package's directory, and its archive.
type behaviourCase struct {
	name    string
	archive *enginetest.Archive
}

// readBehaviourCase reads the case at path, and reports false when it has no
// behaviour_test.go.
func readBehaviourCase(t *testing.T, path string) (behaviourCase, bool) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := enginetest.ParseArchive(b)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if _, ok := a.File("behaviour_test.go"); !ok {
		return behaviourCase{}, false
	}
	name := strings.TrimSuffix(filepath.Base(path), ".txtar")
	if _, refuse := a.File("want.err"); refuse {
		t.Fatalf("%s: a refused case cannot have a behaviour_test.go", name)
	}
	return behaviourCase{name: name, archive: a}, true
}

// writeBehaviourCase generates c into its own package of mod, with the
// runtime files and the case's behaviour_test.go.
func writeBehaviourCase(t *testing.T, c behaviourCase, mod string) {
	t.Helper()
	schemaDir, out := t.TempDir(), filepath.Join(mod, c.name)
	if err := os.Mkdir(out, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, f := range c.archive.Files {
		if strings.HasSuffix(f.Name, ".json") {
			if err := os.WriteFile(filepath.Join(schemaDir, f.Name), f.Data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := enginetest.CopyRuntime(repoRoot, out); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-schema", schemaDir, "-out", out}); err != nil {
		t.Fatal(err)
	}
	behaviour, _ := c.archive.File("behaviour_test.go")
	if err := os.WriteFile(filepath.Join(out, "behaviour_test.go"), behaviour, 0o600); err != nil {
		t.Fatal(err)
	}
}

// goLine returns the version of the go line of a go.mod file.
func goLine(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for line := range strings.Lines(string(b)) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "go "); ok {
			return strings.TrimSpace(v), nil
		}
	}
	return "", errors.New(path + " has no go line")
}

// pkgResult is what go test -json reported for one package: its last
// package-level action (pass, fail, or skip), how many top-level tests ended,
// and all of its output, build output included.
type pkgResult struct {
	action string
	tests  int
	output strings.Builder
}

// testResults reads go test -json events and groups them by package.
func testResults(t *testing.T, events []byte) map[string]*pkgResult {
	t.Helper()
	out := map[string]*pkgResult{}
	get := func(pkg string) *pkgResult {
		if out[pkg] == nil {
			out[pkg] = &pkgResult{}
		}
		return out[pkg]
	}
	sc := bufio.NewScanner(bytes.NewReader(events))
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	for sc.Scan() {
		var e struct {
			Action, Package, Test, Output, ImportPath string
		}
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("go test -json: %v: %s", err, sc.Bytes())
		}
		switch {
		case e.ImportPath != "": // build output, keyed by the package being built
			get(strings.Fields(e.ImportPath)[0]).output.WriteString(e.Output)
		case e.Package == "":
		case e.Action == "output":
			get(e.Package).output.WriteString(e.Output)
		case e.Action != "pass" && e.Action != "fail" && e.Action != "skip":
		case e.Test == "":
			get(e.Package).action = e.Action
		case !strings.Contains(e.Test, "/"):
			get(e.Package).tests++
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
