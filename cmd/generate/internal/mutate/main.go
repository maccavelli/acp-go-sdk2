// Command mutate runs mutation tests: it changes one operator at a time in the
// files given, runs go test against each mutant through an overlay, so the tree
// is never written, and reports which mutants the tests kill
// (0011-PLAN-engine-tests.md, D5). It reports and never gates.
//
// Usage, in cmd/generate:
//
//	go run ./internal/mutate [-dir module] [-files glob,...] [-pkgs pattern] [-skip regexp] [-only file:line:col,...] [-n count] [-seed n] [-j count] [-timeout d] [-o report.json]
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"maps"
	"math/rand/v2"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// config is the tool's flags.
type config struct {
	dir     string // the module to test
	files   string // comma-separated globs relative to dir; "" is every non-test file of pkgs
	pkgs    string // the packages go test runs
	skip    string // go test -skip: tests not run
	only    string // comma-separated file:line:col sites to run instead of a sample
	n       int    // mutants to run, 0 for all
	jobs    int    // mutants run at once
	seed    uint64 // the sample's seed
	timeout string // per mutant, as go test -timeout
	wait    time.Duration
	out     string // the JSON report; "" is a new file in os.TempDir()
}

// report is the JSON report.
type report struct {
	Dir     string                    `json:"dir"`
	Pkgs    string                    `json:"pkgs"`
	Skip    string                    `json:"skip,omitempty"`
	Sites   int                       `json:"sites"`
	Seed    uint64                    `json:"seed"`
	Mutants []result                  `json:"mutants"`
	Counts  map[string]map[string]int `json:"counts"`
	path    string
}

func main() {
	var cfg config
	flag.StringVar(&cfg.dir, "dir", ".", "the module to test")
	flag.StringVar(&cfg.files, "files", "", "files to mutate, comma-separated globs relative to -dir; default every non-test .go file of -pkgs")
	flag.StringVar(&cfg.pkgs, "pkgs", "./...", "packages to test")
	flag.StringVar(&cfg.skip, "skip", "", "go test -skip: tests not run against the mutants")
	flag.StringVar(&cfg.only, "only", "", "run only these sites, file:line:col as a report lists them, comma-separated")
	flag.IntVar(&cfg.n, "n", 0, "mutants to run, sampled with -seed; 0 runs all")
	flag.IntVar(&cfg.jobs, "j", 1, "mutants run at once, each through its own overlay")
	flag.Uint64Var(&cfg.seed, "seed", 1, "the sample's seed")
	flag.StringVar(&cfg.timeout, "timeout", "3m", "go test -timeout for each mutant")
	flag.StringVar(&cfg.out, "o", "", "the JSON report; default a new file in the temporary directory")
	flag.Parse()
	if _, err := run(cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "mutate:", err)
		os.Exit(1)
	}
}

// run mutates the files of cfg, runs each mutant, writes the report, and
// prints a summary that lists each survivor to w.
func run(cfg config, w io.Writer) (*report, error) {
	wait, err := time.ParseDuration(cfg.timeout)
	if err != nil {
		return nil, fmt.Errorf("-timeout: %w", err)
	}
	cfg.wait = wait
	files, err := listFiles(cfg)
	if err != nil {
		return nil, err
	}
	var all []site
	srcs := map[string][]byte{}
	for _, f := range files {
		src, err := os.ReadFile(filepath.Join(cfg.dir, filepath.FromSlash(f)))
		if err != nil {
			return nil, err
		}
		ss, err := sites(f, src)
		if err != nil {
			return nil, err
		}
		srcs[f] = src
		all = append(all, ss...)
	}
	chosen := sample(all, cfg.n, cfg.seed)
	if cfg.only != "" {
		if chosen, err = pick(all, cfg.only); err != nil {
			return nil, err
		}
	}
	work, err := os.MkdirTemp("", "mutate-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work) //nolint:errcheck // a temporary directory.
	rep := &report{Dir: cfg.dir, Pkgs: cfg.pkgs, Skip: cfg.skip, Sites: len(all), Seed: cfg.seed, Counts: map[string]map[string]int{}}
	rep.Mutants = runAll(cfg, work, chosen, srcs)
	for _, r := range rep.Mutants {
		pkg := path.Dir(r.File)
		if rep.Counts[pkg] == nil {
			rep.Counts[pkg] = map[string]int{}
		}
		rep.Counts[pkg][r.Outcome]++
	}
	if err := writeReport(rep, cfg.out); err != nil {
		return nil, err
	}
	if _, err := io.WriteString(w, summary(rep)); err != nil {
		return nil, err
	}
	return rep, nil
}

// runAll runs the mutants of chosen, cfg.jobs at a time, and returns their
// results in the order of chosen. Each mutant has its own file and overlay in
// work, so mutants run at once do not meet.
func runAll(cfg config, work string, chosen []site, srcs map[string][]byte) []result {
	out := make([]result, len(chosen))
	next := make(chan int)
	var mu sync.Mutex
	done := 0
	var wg sync.WaitGroup
	for range max(cfg.jobs, 1) {
		wg.Go(func() {
			for i := range next {
				s := chosen[i]
				r := runMutant(context.Background(), cfg, work, i, s, srcs[s.File])
				out[i] = r
				mu.Lock()
				done++
				fmt.Fprintf(os.Stderr, "[%d/%d] %s:%d:%d %s %s -> %s: %s (%.1fs)\n", done, len(chosen), r.File, r.Line, r.Column, r.Operator, r.Original, r.Mutant, r.Outcome, r.Seconds)
				mu.Unlock()
			}
		})
	}
	for i := range chosen {
		next <- i
	}
	close(next)
	wg.Wait()
	return out
}

// listFiles returns the files to mutate, relative to cfg.dir with forward
// slashes: the matches of cfg.files, or every non-test Go file of cfg.pkgs.
func listFiles(cfg config) ([]string, error) {
	var out []string
	if cfg.files == "" {
		cmd := exec.Command("go", append([]string{"list", "-f", `{{$d := .Dir}}{{range .GoFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}`}, strings.Fields(cfg.pkgs)...)...) //nolint:gosec // G204: the go command.
		cmd.Dir = cfg.dir
		b, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("go list: %w", err)
		}
		root, err := filepath.Abs(cfg.dir)
		if err != nil {
			return nil, err
		}
		for _, p := range strings.Fields(string(b)) {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return nil, err
			}
			out = append(out, filepath.ToSlash(rel))
		}
	}
	for _, g := range strings.Split(cfg.files, ",") {
		if g = strings.TrimSpace(g); g == "" {
			continue
		}
		matches, err := filepath.Glob(filepath.Join(cfg.dir, filepath.FromSlash(g)))
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("-files %s matches no file", g)
		}
		for _, m := range matches {
			if strings.HasSuffix(m, ".go") && !strings.HasSuffix(m, "_test.go") {
				rel, err := filepath.Rel(cfg.dir, m)
				if err != nil {
					return nil, err
				}
				out = append(out, filepath.ToSlash(rel))
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// sample returns n of all, chosen with seed and kept in file order, or all
// when n is 0 or not less than their number.
func sample(all []site, n int, seed uint64) []site {
	if n <= 0 || n >= len(all) {
		return all
	}
	picked := rand.New(rand.NewPCG(seed, 0)).Perm(len(all))[:n] //nolint:gosec // G404: a reproducible sample, not a secret.
	slices.Sort(picked)
	out := make([]site, 0, n)
	for _, i := range picked {
		out = append(out, all[i])
	}
	return out
}

// pick returns the sites of all that only names, as file:line:col, in file
// order, or an error naming one that is not a site.
func pick(all []site, only string) ([]site, error) {
	want := map[string]bool{}
	for _, s := range strings.Split(only, ",") {
		if s = strings.TrimSpace(s); s != "" {
			want[s] = true
		}
	}
	var out []site
	for _, s := range all {
		k := fmt.Sprintf("%s:%d:%d", s.File, s.Line, s.Col)
		if want[k] {
			out = append(out, s)
			delete(want, k)
		}
	}
	if len(want) > 0 {
		return nil, fmt.Errorf("-only names no site at %s", strings.Join(slices.Sorted(maps.Keys(want)), ", "))
	}
	return out, nil
}

// writeReport writes rep as indented JSON to out, or to a new file in the
// temporary directory, and records its path.
func writeReport(rep *report, out string) error {
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	if out == "" {
		f, err := os.CreateTemp("", "mutate-*.json")
		if err != nil {
			return err
		}
		out = f.Name()
		if err := f.Close(); err != nil {
			return err
		}
	}
	rep.path = out
	return os.WriteFile(out, append(b, '\n'), 0o600)
}

// summary returns the counts by package and outcome, and each survivor.
func summary(rep *report) string {
	var w strings.Builder
	fmt.Fprintf(&w, "mutate: %d of %d sites run; report %s\n", len(rep.Mutants), rep.Sites, rep.path)
	for _, pkg := range slices.Sorted(func(yield func(string) bool) {
		for k := range rep.Counts {
			if !yield(k) {
				return
			}
		}
	}) {
		c := rep.Counts[pkg]
		fmt.Fprintf(&w, "  %-24s killed %d, survived %d, invalid %d, timeout %d\n", pkg, c[killed], c[survived], c[invalid], c[timedOut])
	}
	var surv []result
	for _, r := range rep.Mutants {
		if r.Outcome == survived {
			surv = append(surv, r)
		}
	}
	slices.SortFunc(surv, func(a, b result) int {
		return cmp.Or(strings.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Column, b.Column))
	})
	if len(surv) > 0 {
		w.WriteString("survived:\n")
	}
	for _, r := range surv {
		fmt.Fprintf(&w, "  %s:%d:%d %s %s -> %s\n", r.File, r.Line, r.Column, r.Operator, r.Original, r.Mutant)
	}
	return w.String()
}
