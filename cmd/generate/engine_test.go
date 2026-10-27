package main

import (
	"bytes"
	"flag"
	"go/build"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/emit"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/enginetest"
)

var update = flag.Bool("update", false, "rewrite the want.api and want.deprecated of the engine cases")

// engineCases returns the paths of the engine cases, 0011-PLAN-engine-tests.md, D1.
func engineCases(tb testing.TB) []string {
	tb.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "engine", "*.txtar"))
	if err != nil {
		tb.Fatal(err)
	}
	if len(paths) == 0 {
		tb.Fatal("no engine cases in testdata/engine")
	}
	return paths
}

// TestEngineCases runs the generator on each synthetic case into a directory
// holding the hand-written runtime. A case with want.err must be refused with
// each of its lines and leave the directory as it was; a case with want.api
// must generate a package whose exported API, as enginetest.API lists it,
// equals want.api, and whose deprecation notices, as enginetest.Deprecations
// lists them, equal want.deprecated, or are none when the case has none.
// -update rewrites want.api, and want.deprecated when there are notices,
// instead.
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
		generated := func(name string) bool { return slices.Contains(generatedFiles, name) }
		got := enginetest.API(pkg, fset, generated)
		gotDeprecated, err := enginetest.Deprecations(out, generated)
		if err != nil {
			t.Fatal(err)
		}
		gotUnstable, gotUnstableDeprecated := unstableListings(t, out, pkg)
		if *update {
			a.Set("want.api", []byte(got))
			if gotDeprecated != "" {
				a.Set("want.deprecated", []byte(gotDeprecated))
			}
			if gotUnstable != "" {
				a.Set("want.api.unstable", []byte(gotUnstable))
			}
			if gotUnstableDeprecated != "" {
				a.Set("want.deprecated.unstable", []byte(gotUnstableDeprecated))
			}
			if err := os.WriteFile(path, enginetest.Format(a), 0o600); err != nil {
				t.Fatal(err)
			}
			return
		}
		if got != string(wantAPI) {
			t.Errorf("API differs from want.api (rerun with -update to accept):\ngot:\n%s\nwant:\n%s", got, wantAPI)
		}
		// A case without want.deprecated must generate no notice
		// (0024-PLAN-api-gate-and-deprecation.md).
		if wantDeprecated, _ := a.File("want.deprecated"); gotDeprecated != string(wantDeprecated) {
			t.Errorf("deprecation notices differ from want.deprecated (rerun with -update to accept):\ngot:\n%s\nwant:\n%s",
				gotDeprecated, wantDeprecated)
		}
		requireUnstable(t, a, out, gotUnstable, gotUnstableDeprecated)
	}
}

// requireUnstable requires package unstable's listings to equal the case's
// want.api.unstable and want.deprecated.unstable, and a case without the
// first to leave no unstable/ under out
// (0039-PLAN-five-adopter-additions-on-schema-1-25-0.md, 2f).
func requireUnstable(t *testing.T, a *enginetest.Archive, out, api, deprecated string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(out, emit.UnstableDir)); api == "" && err == nil {
		t.Errorf("the run made %s/ and generated no package unstable", emit.UnstableDir)
	}
	if want, _ := a.File("want.api.unstable"); api != string(want) {
		t.Errorf("package unstable's API differs from want.api.unstable (rerun with -update to accept):\ngot:\n%s\nwant:\n%s", api, want)
	}
	if want, _ := a.File("want.deprecated.unstable"); deprecated != string(want) {
		t.Errorf("package unstable's notices differ from want.deprecated.unstable (rerun with -update to accept):\ngot:\n%s\nwant:\n%s",
			deprecated, want)
	}
}

// unstableListings lists the API and the deprecation notices of the package
// unstable generated under out, against root, the case's root package: ""
// for each when the run generated none.
func unstableListings(t *testing.T, out string, root *types.Package) (api, deprecated string) {
	t.Helper()
	dir := filepath.Join(out, emit.UnstableDir)
	if _, err := os.Stat(filepath.Join(dir, "unstable_gen.go")); err != nil {
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return "", ""
	}
	rootPath, err := rootImportPath(out)
	if err != nil {
		t.Fatal(err)
	}
	pkg, fset, err := enginetest.LoadPackageImporting(dir, rootPath, root)
	if err != nil {
		t.Fatal(err)
	}
	generated := func(name string) bool { return strings.HasSuffix(name, "_gen.go") }
	deprecated, err = enginetest.Deprecations(dir, generated)
	if err != nil {
		t.Fatal(err)
	}
	return enginetest.API(pkg, fset, generated), deprecated
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
// case, or compaction.go, elicitation.go, helpers.go, refusals.go or
// tooltimeline.go, which use ACP types and are not.
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
	want := slices.Sorted(slices.Values(append(slices.Clone(enginetest.RuntimeFiles), "compaction.go", "elicitation.go", "helpers.go", "refusals.go", "tooltimeline.go")))
	if !slices.Equal(hand, want) {
		t.Errorf("hand-written files of the root are %v, want %v: add a new file to enginetest.RuntimeFiles if it uses no ACP type, "+
			"or exclude it here as helpers.go is", hand, want)
	}
}
