package output

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/emit"
)

// stub declares the hand-written name the generated file of uses refers to.
const stub = "package acp\n\ntype RequestError struct{}\n"

// uses is a generated file that refers to the hand-written RequestError.
var uses = []emit.File{{Name: "types_gen.go", Src: []byte("package acp\n\ntype U struct{}\n\nfunc F() *RequestError { return nil }\n")}}

// dirWith returns a new directory holding files, by name.
func dirWith(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCheckAcceptsHandWrittenPackage(t *testing.T) {
	t.Parallel()
	if err := Check(dirWith(t, map[string]string{"hand.go": stub}), "acp", uses); err != nil {
		t.Fatal(err)
	}
}

func TestCheckRefuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		dir   func(t *testing.T) string
		files []emit.File
		want  []string
	}{
		{"an empty directory", func(t *testing.T) string {
			t.Helper()
			return t.TempDir()
		}, uses,
			[]string{"does not type-check", "types_gen.go:5:", "undefined: RequestError"}},
		{"a missing directory", func(t *testing.T) string {
			t.Helper()
			return filepath.Join(t.TempDir(), "missing")
		}, uses,
			[]string{"output directory", "missing: does not exist"}},
		{"a hand-written file that redeclares a generated type", func(t *testing.T) string {
			t.Helper()
			return dirWith(t, map[string]string{"hand.go": stub + "\ntype U int\n"})
		}, uses, []string{"redeclared"}},
		{"a feature newer than the go line of go.mod", func(t *testing.T) string {
			t.Helper()
			return dirWith(t, map[string]string{
				"go.mod":  "module example.com/acp\n\ngo 1.21\n",
				"hand.go": stub + "\nfunc loop() {\n\tfor range 3 {\n\t}\n}\n",
			})
		}, uses, []string{"requires go1.22"}},
		{"a file of another package", func(t *testing.T) string {
			t.Helper()
			return dirWith(t, map[string]string{"hand.go": stub, "stray.go": "package other\n"})
		}, uses, []string{"output directory", "other"}},
		{"a cgo file", func(t *testing.T) string {
			t.Helper()
			return dirWith(t, map[string]string{"hand.go": stub, "c.go": "package acp\n\nimport \"C\"\n"})
		}, uses, []string{"cgo file c.go"}},
		// The sites below no test ran (0019-PLAN-generator-hardening.md, G19).
		{"a go.mod that cannot be read", func(t *testing.T) string {
			t.Helper()
			dir := dirWith(t, map[string]string{"hand.go": stub})
			if err := os.Mkdir(filepath.Join(dir, "go.mod"), 0o700); err != nil {
				t.Fatal(err)
			}
			return dir
		}, uses, []string{"go.mod"}},
		{"a hand-written file that does not parse", func(t *testing.T) string {
			t.Helper()
			return dirWith(t, map[string]string{"hand.go": stub, "broken.go": "package acp\n\nfunc {\n"})
		}, uses, []string{"broken.go:3:"}},
		{"a generated file that does not parse", func(t *testing.T) string {
			t.Helper()
			return dirWith(t, map[string]string{"hand.go": stub})
		}, []emit.File{{Name: "types_gen.go", Src: []byte("package acp\n\nfunc {\n")}}, []string{"types_gen.go:3:"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Check(tt.dir(t), "acp", tt.files)
			if err == nil {
				t.Fatal("Check accepted the output")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

// TestCheckSelectsLikeTheGoCommand requires the files the go command would not
// build or test with the package to stay out of the check, each of which
// would fail it. An in-package test file is checked
// (0019-PLAN-generator-hardening.md, G15), and so is an external test file,
// as its own package (0049-MADR-remediate-the-2026-11-01-debugging-pass.md,
// G27, which TestCheckExternalTests shows refusing).
func TestCheckSelectsLikeTheGoCommand(t *testing.T) {
	t.Parallel()
	dir := dirWith(t, map[string]string{
		"hand.go":         stub,
		"ignored.go":      "//go:build ignore\n\npackage main\n\nvar _ = undefinedInIgnored\n",
		"hand_test.go":    "package acp\n\nvar _ = F\n",
		"hand_x_test.go":  "package acp_test\n\nimport \"acp\"\n\nvar _ = acp.F\n",
		".x.tmp":          "package acp\n\nvar _ = undefinedInDotFile\n",
		".types_gen.go.1": "package acp\n\nvar _ = undefinedInDotFile\n",
		"other_plan9.go":  "package acp\n\nvar _ = undefinedOnPlan9\n",
	})
	if err := Check(dir, "acp", uses); err != nil {
		t.Fatal(err)
	}
}

func TestCheckReportsAtMostTen(t *testing.T) {
	t.Parallel()
	var src strings.Builder
	src.WriteString("package acp\n")
	for i := range 12 {
		src.WriteString("\nvar _ = undefined" + string(rune('A'+i)) + "\n")
	}
	err := Check(t.TempDir(), "acp", []emit.File{{Name: "types_gen.go", Src: []byte(src.String())}})
	if err == nil {
		t.Fatal("Check accepted the output")
	}
	if got := strings.Count(err.Error(), "undefined: undefined"); got != 10 {
		t.Errorf("error reports %d errors, want 10:\n%v", got, err)
	}
	if !strings.HasSuffix(err.Error(), "and 2 more") {
		t.Errorf("error does not end with the count of the rest:\n%v", err)
	}
	if !strings.Contains(err.Error(), "types_gen.go:3:9: undefined: undefinedA") {
		t.Errorf("error does not start with the first error in position order:\n%v", err)
	}
}

// TestCheckConcurrently runs checks at once, as the engine tests do. Every call
// shares one importer (sharedImporter), so a missing lock shows under -race.
func TestCheckConcurrently(t *testing.T) {
	t.Parallel()
	if first, second := sharedImporter(), sharedImporter(); first != second {
		t.Fatal("Check calls do not share one importer")
	}
	importing := []emit.File{{Name: "types_gen.go", Src: []byte("package acp\n\nimport (\n\t\"encoding/json\"\n\t\"strings\"\n)\n\n" +
		"var _ = json.Marshal\n\nvar _ = strings.Cut\n\nfunc F() *RequestError { return nil }\n")}}
	errs := make(chan error, 8)
	for range 8 {
		dir := dirWith(t, map[string]string{"hand.go": stub})
		go func() { errs <- Check(dir, "acp", importing) }()
	}
	for range 8 {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}

// TestCheckIncludesHandWrittenTestFiles requires a name that a hand-written
// test file and a generated one both declare to be refused, as go vet would,
// and a generated file that is not a test to be checked without the test
// files, as the package is built (0019-PLAN-generator-hardening.md, G15).
func TestCheckIncludesHandWrittenTestFiles(t *testing.T) {
	t.Parallel()
	gen := append(slices.Clone(uses), emit.File{Name: "schema_gen_test.go", Src: []byte("package acp\n\ntype recordingAgent struct{}\n")})
	dir := dirWith(t, map[string]string{"hand.go": stub, "agent_test.go": "package acp\n\ntype recordingAgent struct{}\n"})
	err := Check(dir, "acp", gen)
	if err == nil || !strings.Contains(err.Error(), "recordingAgent redeclared") {
		t.Errorf("Check = %v, want it to refuse recordingAgent redeclared", err)
	}
	if err != nil && strings.Index(err.Error(), "schema_gen_test.go") > strings.Index(err.Error(), "agent_test.go") {
		t.Errorf("the generated file's error is not reported first:\n%v", err)
	}
	onlyInTests := []emit.File{
		{Name: "types_gen.go", Src: []byte("package acp\n\nvar _ = helperInTests\n")},
		{Name: "schema_gen_test.go", Src: []byte("package acp\n\nvar helperInTests int\n")},
	}
	if err := Check(dirWith(t, map[string]string{"hand.go": stub}), "acp", onlyInTests); err == nil || !strings.Contains(err.Error(), "undefined: helperInTests") {
		t.Errorf("Check = %v, want it to refuse a built file that needs a test file's name", err)
	}
}

// TestCheckIncludesGeneratedTestFiles type-checks a generated _test.go file
// with the package: one that refers to a name nothing declares is refused,
// and one that uses the package's names is accepted.
func TestCheckIncludesGeneratedTestFiles(t *testing.T) {
	t.Parallel()
	dir := dirWith(t, map[string]string{"hand.go": stub})
	bad := append(slices.Clone(uses), emit.File{Name: "schema_gen_test.go", Src: []byte("package acp\n\nvar _ = missingName\n")})
	if err := Check(dir, "acp", bad); err == nil || !strings.Contains(err.Error(), "schema_gen_test.go") || !strings.Contains(err.Error(), "undefined: missingName") {
		t.Errorf("Check = %v, want it to refuse undefined: missingName in schema_gen_test.go", err)
	}
	good := append(slices.Clone(uses), emit.File{Name: "schema_gen_test.go", Src: []byte("package acp\n\nvar _ = F\n\nvar _ U\n")})
	if err := Check(dir, "acp", good); err != nil {
		t.Errorf("Check refused a test file that uses the package: %v", err)
	}
}

// TestCheckSeesTheTreeWriteLeaves requires Check to read the directory as
// Write will leave it: a stale generated file, which Write removes, is not
// read as hand-written, and the hand-written files of package unstable are
// checked even when nothing renders into it
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G1).
func TestCheckSeesTheTreeWriteLeaves(t *testing.T) {
	t.Parallel()
	header := "// " + emit.GeneratedHeader + "\n\n"
	t.Run("a hand-written file that uses a stale generated one", func(t *testing.T) {
		t.Parallel()
		dir := dirWith(t, map[string]string{
			"hand.go":       stub,
			"uses_stale.go": "package acp\n\nvar _ = staleHelper()\n",
			"old_gen.go":    header + "package acp\n\nfunc staleHelper() int { return 1 }\n",
		})
		if err := Check(dir, "acp", uses); err == nil || !strings.Contains(err.Error(), "undefined: staleHelper") {
			t.Errorf("Check = %v, want it to refuse undefined: staleHelper", err)
		}
	})
	t.Run("a stale generated file that collides with the render", func(t *testing.T) {
		t.Parallel()
		dir := dirWith(t, map[string]string{"hand.go": stub, "old_gen.go": header + "package acp\n\ntype U struct{}\n"})
		if err := Check(dir, "acp", uses); err != nil {
			t.Errorf("Check refused a stale file that Write removes: %v", err)
		}
	})
	t.Run("package unstable with nothing rendered into it", func(t *testing.T) {
		t.Parallel()
		dir := dirWith(t, map[string]string{"hand.go": stub})
		udir := filepath.Join(dir, emit.UnstableDir)
		if err := os.Mkdir(udir, 0o750); err != nil {
			t.Fatal(err)
		}
		for name, src := range map[string]string{
			"doc.go":          "// Package unstable is a probe.\npackage unstable\n\nvar _ = staleUnstable\n",
			"unstable_gen.go": header + "package unstable\n\nvar staleUnstable int\n",
		} {
			if err := os.WriteFile(filepath.Join(udir, name), []byte(src), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := Check(dir, "acp", uses); err == nil || !strings.Contains(err.Error(), "undefined: staleUnstable") {
			t.Errorf("Check = %v, want it to refuse undefined: staleUnstable in package unstable", err)
		}
	})
}

// writeTree writes files, by slash path under dir, making their directories.
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// TestCheckOtherPlatforms requires a file that only windows/amd64 builds to
// be checked for that platform: one that collides with a generated name is
// refused, naming it, and one that uses the standard library's windows-only
// API is accepted, since the go command compiles it for windows
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G9, as amended by
// 0049-PLAN-generator-fixes.md's phase 3, deviation 1).
func TestCheckOtherPlatforms(t *testing.T) {
	t.Parallel()
	module := "module acp\n\ngo 1.26\n"
	t.Run("a windows file that collides", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeTree(t, dir, map[string]string{"go.mod": module, "hand.go": stub, "x_windows.go": "package acp\n\ntype U int\n"})
		err := Check(dir, "acp", uses)
		if err == nil || !strings.Contains(err.Error(), "redeclared") {
			t.Fatalf("Check = %v, want it to refuse U redeclared", err)
		}
		if runtime.GOOS != "windows" && !strings.Contains(err.Error(), "for windows/amd64") {
			t.Errorf("Check = %v, want it to name windows/amd64", err)
		}
	})
	t.Run("a windows file that uses windows-only API", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeTree(t, dir, map[string]string{"go.mod": module, "hand.go": stub,
			"x_windows.go": "package acp\n\nimport \"syscall\"\n\nvar _ syscall.Handle\n"})
		if err := Check(dir, "acp", uses); err != nil {
			t.Errorf("Check refused a correct windows-only file: %v", err)
		}
	})
}

// TestCheckExternalTests requires the root's external test package to be
// checked against the package as rendered, with a package of the module that
// it imports checked from source against the same root: a name the render
// lacks is refused, and the types the two packages share are one
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G27).
func TestCheckExternalTests(t *testing.T) {
	t.Parallel()
	tree := func(use string) string {
		dir := t.TempDir()
		writeTree(t, dir, map[string]string{
			"go.mod":             "module example.com/m\n\ngo 1.26\n",
			"hand.go":            stub,
			"acptest/acptest.go": "package acptest\n\nimport \"example.com/m\"\n\nfunc Use() *acp.RequestError { return acp.F() }\n",
			"x_test.go": "package acp_test\n\nimport (\n\t\"testing\"\n\n\t\"example.com/m\"\n\t\"example.com/m/acptest\"\n)\n\n" +
				"func TestX(t *testing.T) {\n\tvar _ *acp.RequestError = acptest.Use()\n\t" + use + "\n}\n",
		})
		return dir
	}
	if err := Check(tree("_ = acp.F"), "example.com/m", uses); err != nil {
		t.Errorf("Check refused an external test that uses the render: %v", err)
	}
	if err := Check(tree("_ = acp.ToolKindSearch"), "example.com/m", uses); err == nil || !strings.Contains(err.Error(), "undefined: acp.ToolKindSearch") {
		t.Errorf("Check = %v, want it to refuse undefined: acp.ToolKindSearch", err)
	}
}

// TestCheckRootImportsModulePackage requires a root whose hand-written file,
// and in-package test file, import another package of the module, an internal
// one, to be checked with that package from source: accepted when it checks,
// and refused, naming it, when it does not
// (0052-PLAN-agent-tool-call-snapshot-stream-and-message-size-default.md,
// phase 3, the deviation of 2026-11-10).
func TestCheckRootImportsModulePackage(t *testing.T) {
	t.Parallel()
	tree := func(buf string) string {
		dir := t.TempDir()
		writeTree(t, dir, map[string]string{
			"go.mod":              "module example.com/m\n\ngo 1.26\n",
			"hand.go":             stub,
			"uses.go":             "package acp\n\nimport \"example.com/m/internal/buf\"\n\nvar Limit = buf.N\n",
			"uses_test.go":        "package acp\n\nimport (\n\t\"testing\"\n\n\t\"example.com/m/internal/buf\"\n)\n\nfunc TestN(t *testing.T) { _ = buf.N }\n",
			"internal/buf/buf.go": buf,
		})
		return dir
	}
	if err := Check(tree("package buf\n\nconst N = 1\n"), "example.com/m", uses); err != nil {
		t.Errorf("Check refused a root that imports a package of its module: %v", err)
	}
	err := Check(tree("package buf\n\nconst N = undefinedName\n"), "example.com/m", uses)
	if err == nil || !strings.Contains(err.Error(), "example.com/m/internal/buf") || !strings.Contains(err.Error(), "undefinedName") {
		t.Errorf("Check = %v, want it to refuse the module package that does not check, naming it", err)
	}
}

// TestCheckVersionFromEnclosingModule requires the language version to come
// from the go.mod that holds the output directory, found walking up as the
// go command finds it, not only from one inside it
// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G28).
func TestCheckVersionFromEnclosingModule(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	writeTree(t, parent, map[string]string{
		"go.mod":      "module m\n\ngo 1.21\n",
		"sub/hand.go": stub,
		"sub/loop.go": "package acp\n\nfunc L() {\n\tfor range 3 {\n\t}\n}\n",
	})
	err := Check(filepath.Join(parent, "sub"), "m/sub", uses)
	if err == nil || !strings.Contains(err.Error(), "requires go1.22 or later") {
		t.Errorf("Check = %v, want it to refuse the range over an int under go 1.21", err)
	}
}
