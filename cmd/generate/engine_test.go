package main

import (
	"bytes"
	"flag"
	"go/build"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/enginetest"
)

var update = flag.Bool("update", false, "rewrite the want.api of the engine cases")

// engineCases returns the paths of the engine cases, 0011-PLAN-engine-tests.md, D1.
func engineCases(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "engine", "*.txtar"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no engine cases in testdata/engine")
	}
	return paths
}

// TestEngineCases runs the generator on each synthetic case into a directory
// holding the hand-written runtime. A case with want.err must be refused with
// each of its lines and leave the directory as it was; a case with want.api
// must generate a package whose exported API, as enginetest.API lists it,
// equals want.api. -update rewrites want.api instead.
func TestEngineCases(t *testing.T) {
	t.Parallel()
	for _, path := range engineCases(t) {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".txtar"), func(t *testing.T) {
			t.Parallel()
			runEngineCase(t, path)
		})
	}
}

func runEngineCase(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := enginetest.ParseArchive(b)
	if err != nil {
		t.Fatal(err)
	}
	schemaDir := t.TempDir()
	for _, f := range a.Files {
		if strings.HasSuffix(f.Name, ".json") {
			if err := os.WriteFile(filepath.Join(schemaDir, f.Name), f.Data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	out := t.TempDir()
	if err := enginetest.CopyRuntime(repoRoot, out); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, out)
	runErr := run([]string{"-schema", schemaDir, "-out", out})

	wantErr, refuse := a.File("want.err")
	wantAPI, accept := a.File("want.api")
	switch {
	case refuse == accept:
		t.Fatal("a case needs exactly one of want.err and want.api")
	case refuse:
		if runErr == nil {
			t.Fatal("run accepted the schema")
		}
		for line := range strings.Lines(string(wantErr)) {
			if line = strings.TrimSpace(line); line != "" && !strings.Contains(runErr.Error(), line) {
				t.Errorf("error %q does not contain %q", runErr, line)
			}
		}
		if after := snapshot(t, out); !maps.EqualFunc(before, after, bytes.Equal) {
			t.Errorf("output directory changed: holds %v, held %v", slices.Sorted(maps.Keys(after)), slices.Sorted(maps.Keys(before)))
		}
	default:
		if runErr != nil {
			t.Fatal(runErr)
		}
		pkg, fset, err := enginetest.LoadPackage(out)
		if err != nil {
			t.Fatal(err)
		}
		got := enginetest.API(pkg, fset, func(name string) bool { return slices.Contains(generatedFiles, name) })
		if *update {
			a.Set("want.api", []byte(got))
			if err := os.WriteFile(path, enginetest.Format(a), 0o600); err != nil {
				t.Fatal(err)
			}
			return
		}
		if got != string(wantAPI) {
			t.Errorf("API differs from want.api (rerun with -update to accept):\ngot:\n%s\nwant:\n%s", got, wantAPI)
		}
	}
}

// snapshot returns the name and bytes of every file in dir.
func snapshot(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte, len(entries))
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[e.Name()] = b
	}
	return files
}

// TestRuntimeFilesCoverHandWritten requires every hand-written file of the
// root to be either a runtime file, compiled with the code of each synthetic
// case, or helpers.go, which uses ACP types and is not.
func TestRuntimeFilesCoverHandWritten(t *testing.T) {
	t.Parallel()
	bp, err := build.Default.ImportDir(repoRoot, 0)
	if err != nil {
		t.Fatal(err)
	}
	var hand []string
	for _, name := range bp.GoFiles {
		if !slices.Contains(generatedFiles, name) {
			hand = append(hand, name)
		}
	}
	want := slices.Sorted(slices.Values(append(slices.Clone(enginetest.RuntimeFiles), "helpers.go")))
	if !slices.Equal(hand, want) {
		t.Errorf("hand-written files of the root are %v, want %v: add a new file to enginetest.RuntimeFiles if it uses no ACP type, "+
			"or exclude it here as helpers.go is", hand, want)
	}
}
