package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSites lists the operator sites of a fixture and checks each mutant's
// line: one change at the site, the rest of the file as it was.
func TestSites(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(filepath.Join("testdata", "ops.go"))
	if err != nil {
		t.Fatal(err)
	}
	ss, err := sites("ops.go", src)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`4:11 eq-neq == -> !=`,
		`4:16 and-or || -> &&`,
		`4:21 eq-neq != -> ==`,
		`4:26 and-or && -> ||`,
		`4:31 boundary < -> <=`,
		`8:11 boundary <= -> <`,
		`8:16 and-or && -> ||`,
		`8:21 boundary > -> >=`,
		`8:25 and-or || -> &&`,
		`8:30 boundary >= -> >`,
		`12:11 plus-minus-one + 1 -> - 1`,
		`12:15 plus-minus-one - 1 -> + 1`,
		`16:9 true-false true -> false`,
		`16:15 true-false false -> true`,
		`21:10 return-error nil -> errors.New("mutant")`,
		`23:29 return-error nil -> errors.New("mutant")`,
		`35:8 eq-neq == -> !=`,
		`36:4 continue-break continue -> break`,
		`38:8 eq-neq == -> !=`,
		`39:4 continue-break break -> continue`,
	}
	var got []string
	for _, s := range ss {
		got = append(got, fmt.Sprintf("%d:%d %s %s -> %s", s.Line, s.Col, s.Op, s.Orig, s.Repl))
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("sites:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	lines := strings.Split(string(src), "\n")
	for _, s := range ss {
		out, err := mutant("ops.go", src, s)
		if err != nil {
			t.Fatalf("%d:%d: %v", s.Line, s.Col, err)
		}
		mutated := strings.Split(string(out), "\n")
		if s.Op == "return-error" {
			// The import of errors adds two lines after the package clause.
			if !strings.Contains(string(out), "import \"errors\"") {
				t.Errorf("%d:%d: no import of errors:\n%s", s.Line, s.Col, out)
			}
			mutated = append(mutated[:2], mutated[4:]...)
		}
		if len(mutated) != len(lines) {
			t.Fatalf("%d:%d: %d lines, want %d:\n%s", s.Line, s.Col, len(mutated), len(lines), out)
		}
		for i := range lines {
			changed := lines[i] != mutated[i]
			if changed != (i == s.Line-1) {
				t.Errorf("%d:%d: line %d is %q, was %q", s.Line, s.Col, i+1, mutated[i], lines[i])
			}
		}
		if line := mutated[s.Line-1]; !strings.Contains(line, s.Repl) {
			t.Errorf("%d:%d: line %q lacks %q", s.Line, s.Col, line, s.Repl)
		}
	}
}

// moduleFiles is a module with one source file and two tests. Its mutants
// cover every outcome: a bound of Sum that TestSum catches (killed), the
// equality of the untested IsZero (survived), a break outside a loop made a
// continue (invalid), and Sum's step made - 1, which never ends (timeout).
var moduleFiles = map[string]string{
	"go.mod": "module example.com/m\n\ngo 1.27.2\n",
	"m.go": `package m

// Sum returns 0+1+...+(n-1).
func Sum(n int) int {
	s := 0
	for i := 0; i < n; i = i + 1 {
		s += i
	}
	return s
}

// IsZero reports whether x is 0.
func IsZero(x int) bool { return x == 0 }

// Skip returns x.
func Skip(x int) int {
	switch x {
	case 0:
		break
	}
	return x
}
`,
	"m_test.go": `package m

import "testing"

func TestSum(t *testing.T) {
	if Sum(3) != 3 {
		t.Fatal(Sum(3))
	}
}

func TestSkip(t *testing.T) {
	if Skip(0) != 0 {
		t.Fatal(Skip(0))
	}
}
`,
}

// TestRun mutates the module and requires each outcome, the failed test of a
// killed mutant, the same outcomes with two mutants at once, the tests that
// -skip names not run, and the module's files unchanged afterwards.
func TestRun(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test for each mutant")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("the go command is needed: %v", err)
	}
	t.Parallel()
	dir := t.TempDir()
	for name, body := range moduleFiles {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshot(t, dir)
	want := map[string]string{
		"6 boundary":        "killed TestSum",
		"6 plus-minus-one":  "timeout ",
		"13 eq-neq":         "survived ",
		"19 continue-break": "invalid ",
	}
	base := config{dir: dir, files: "m.go", pkgs: "./...", timeout: "5s", jobs: 1}
	rep := checkRun(t, base, want)
	if c := rep.Counts["."]; c["killed"] != 1 || c["survived"] != 1 || c["invalid"] != 1 || c["timeout"] != 1 {
		t.Errorf("counts %v", rep.Counts)
	}
	if b, err := os.ReadFile(rep.path); err != nil || !strings.Contains(string(b), `"outcome": "survived"`) {
		t.Errorf("report: %v\n%s", err, b)
	}
	two := base
	two.jobs = 2
	checkRun(t, two, want)
	skip := two
	skip.skip = "TestSum"
	checkRun(t, skip, map[string]string{
		"6 boundary":        "survived ",
		"6 plus-minus-one":  "survived ",
		"13 eq-neq":         "survived ",
		"19 continue-break": "invalid ",
	})
	after := snapshot(t, dir)
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Errorf("the module changed:\nbefore %v\nafter  %v", before, after)
	}
}

// TestOnly requires -only to run exactly the sites it names, each the mutant a
// full run makes there, and to refuse a site that is not one.
func TestOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test for each mutant")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("the go command is needed: %v", err)
	}
	t.Parallel()
	dir := t.TempDir()
	for name, body := range moduleFiles {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config{dir: dir, files: "m.go", pkgs: "./...", timeout: "5s", jobs: 1, only: "m.go:13:36,m.go:6:16"}
	checkRun(t, cfg, map[string]string{"6 boundary": "killed TestSum", "13 eq-neq": "survived "})
	cfg.only = "m.go:13:35"
	if _, err := run(cfg, io.Discard); err == nil || !strings.Contains(err.Error(), "m.go:13:35") {
		t.Errorf("-only of no site: %v", err)
	}
}

// checkRun runs cfg with a report in a temporary directory and requires the
// outcome and failed tests of each mutant, by line and operator, to be want's.
func checkRun(t *testing.T, cfg config, want map[string]string) *report {
	t.Helper()
	cfg.out = filepath.Join(t.TempDir(), "report.json")
	rep, err := run(cfg, os.Stdout)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for i, r := range rep.Mutants {
		if i > 0 && (r.Line < rep.Mutants[i-1].Line || r.Line == rep.Mutants[i-1].Line && r.Column < rep.Mutants[i-1].Column) {
			t.Errorf("-j %d: the report lists %d:%d after %d:%d", cfg.jobs, r.Line, r.Column, rep.Mutants[i-1].Line, rep.Mutants[i-1].Column)
		}
		if r.Seconds <= 0 {
			t.Errorf("-j %d: mutant %d %s took %v s", cfg.jobs, r.Line, r.Operator, r.Seconds)
		}
		got[fmt.Sprintf("%d %s", r.Line, r.Operator)] = r.Outcome + " " + strings.Join(r.Failed, ",")
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("-j %d -skip %q: mutants\n%v\nwant\n%v", cfg.jobs, cfg.skip, got, want)
	}
	return rep
}

// snapshot reads every file under dir.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		out[p] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
