package output

import (
	"os"
	"path/filepath"
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
	if err := Check(dirWith(t, map[string]string{"hand.go": stub}), uses); err != nil {
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
			err := Check(tt.dir(t), tt.files)
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
// would fail it: an external test package is another package. An in-package
// test file is checked (0019-PLAN-generator-hardening.md, G15).
func TestCheckSelectsLikeTheGoCommand(t *testing.T) {
	t.Parallel()
	dir := dirWith(t, map[string]string{
		"hand.go":         stub,
		"ignored.go":      "//go:build ignore\n\npackage main\n\nvar _ = undefinedInIgnored\n",
		"hand_test.go":    "package acp\n\nvar _ = F\n",
		"hand_x_test.go":  "package acp_test\n\nvar _ = undefinedInExternalTest\n",
		".x.tmp":          "package acp\n\nvar _ = undefinedInDotFile\n",
		".types_gen.go.1": "package acp\n\nvar _ = undefinedInDotFile\n",
		"other_plan9.go":  "package acp\n\nvar _ = undefinedOnPlan9\n",
	})
	if err := Check(dir, uses); err != nil {
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
	err := Check(t.TempDir(), []emit.File{{Name: "types_gen.go", Src: []byte(src.String())}})
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
		go func() { errs <- Check(dir, importing) }()
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
	err := Check(dir, gen)
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
	if err := Check(dirWith(t, map[string]string{"hand.go": stub}), onlyInTests); err == nil || !strings.Contains(err.Error(), "undefined: helperInTests") {
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
	if err := Check(dir, bad); err == nil || !strings.Contains(err.Error(), "schema_gen_test.go") || !strings.Contains(err.Error(), "undefined: missingName") {
		t.Errorf("Check = %v, want it to refuse undefined: missingName in schema_gen_test.go", err)
	}
	good := append(slices.Clone(uses), emit.File{Name: "schema_gen_test.go", Src: []byte("package acp\n\nvar _ = F\n\nvar _ U\n")})
	if err := Check(dir, good); err != nil {
		t.Errorf("Check refused a test file that uses the package: %v", err)
	}
}
