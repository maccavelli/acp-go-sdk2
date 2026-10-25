// Command generate reads the ACP JSON schemas and writes the Go bindings.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/emit"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/output"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
}

// run generates the bindings. It loads the schema, builds the model, renders
// every file, and type-checks the files with the hand-written package of the
// output directory, which must exist, before it writes anything, so a schema
// the model refuses, a file that fails to render, or output that does not
// type-check leaves the output directory as it was. output.Write then stages
// every changed file before it replaces any. With -write-unstable-manifest it
// only freezes the manifest (load.WriteUnstableManifest).
func run(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	schemaDir := fs.String("schema", "", "path to schema directory (defaults to <repo>/schema)")
	outDir := fs.String("out", "", "output directory for generated go files (defaults to <repo>)")
	writeManifest := fs.Bool("write-unstable-manifest", false, "freeze the unstable-origin API into <schema>/"+load.ManifestFile+", once, and generate nothing")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *schemaDir == "" || *outDir == "" {
		repoRoot, err := findRepoRoot()
		if err != nil {
			return err
		}
		if *schemaDir == "" {
			*schemaDir = filepath.Join(repoRoot, "schema")
		}
		if *outDir == "" {
			*outDir = repoRoot
		}
	}
	if *writeManifest {
		return load.WriteUnstableManifest(*schemaDir)
	}
	meta, schema, prov, err := loadSchema(*schemaDir)
	if err != nil {
		return err
	}
	// The model refuses a schema it cannot represent before any file is written.
	m, err := model.Build(model.Input{Meta: meta, Schema: schema, Provenance: prov, Protocol: &model.ACP})
	if err != nil {
		return err
	}
	files, err := emit.Render(m)
	if err != nil {
		return err
	}
	root, err := rootImportPath(*outDir)
	if err != nil {
		return err
	}
	unstable, err := renderUnstable(*schemaDir, root, meta, schema, prov)
	if err != nil {
		return err
	}
	files = append(files, unstable...)
	if err := output.Check(*outDir, root, files); err != nil {
		return err
	}
	return output.Write(*outDir, files)
}

// rootImportPath is the import path of the package in outDir: the module path
// of the go.mod that holds it, joined with outDir's directory relative to
// that go.mod's, or, with no go.mod, the package's name, acp. Package
// unstable imports the root by it
// (0039-PLAN-five-adopter-additions-on-schema-1-25-0.md, 2f).
func rootImportPath(outDir string) (string, error) {
	abs, err := filepath.Abs(outDir)
	if err != nil {
		return "", err
	}
	for dir := abs; ; {
		b, err := os.ReadFile(filepath.Join(dir, "go.mod")) //nolint:gosec // G304: a go.mod above the generator's explicit output directory.
		if err == nil {
			module, err := modulePath(b)
			if err != nil {
				return "", fmt.Errorf("%s: %w", filepath.Join(dir, "go.mod"), err)
			}
			rel, err := filepath.Rel(dir, abs)
			if err != nil {
				return "", err
			}
			if rel == "." {
				return module, nil
			}
			return module + "/" + filepath.ToSlash(rel), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "acp", nil
		}
		dir = parent
	}
}

// modulePath returns the module path of the go.mod file b.
func modulePath(b []byte) (string, error) {
	for line := range strings.Lines(string(b)) {
		line, _, _ = strings.Cut(line, "//")
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module"); ok && rest != "" && (rest[0] == ' ' || rest[0] == '\t') {
			return strings.Trim(strings.TrimSpace(rest), `"`), nil
		}
	}
	return "", errors.New("no module line")
}

// renderUnstable renders package unstable: what the live unstable files add
// beyond the manifest (load.NewUnstable). It renders nothing without a
// manifest or without the live files, and when they add nothing.
func renderUnstable(schemaDir, root string, acpMeta *load.Meta, acpSchema *load.Schema, acpProv *load.Provenance) ([]emit.File, error) {
	manifest, found, err := load.ReadUnstableManifest(schemaDir)
	if err != nil || !found {
		return nil, err
	}
	liveMeta, metaFound, err := load.ReadMetaUnstable(schemaDir)
	if err != nil {
		return nil, err
	}
	liveSchema, schemaFound, err := load.ReadSchemaUnstable(schemaDir)
	if err != nil {
		return nil, err
	}
	if !metaFound || !schemaFound {
		return nil, nil
	}
	stableMeta, err := load.ReadMeta(schemaDir)
	if err != nil {
		return nil, err
	}
	stableSchema, err := load.ReadSchema(schemaDir)
	if err != nil {
		return nil, err
	}
	u, err := load.NewUnstable(stableMeta, stableSchema, liveMeta, liveSchema, manifest, acpMeta, acpSchema, acpProv)
	if err != nil || u.Empty() {
		return nil, err
	}
	m, err := model.Build(model.Input{Meta: u.Meta, Schema: u.Schema, Provenance: u.Provenance, Protocol: &model.ACP})
	if err != nil {
		return nil, fmt.Errorf("unstable: %w", err)
	}
	return emit.RenderUnstable(m, emit.Unstable{Root: root, Local: u.Local, Variants: u.Variants, Wires: u.Wires})
}

// loadSchema reads the stable schema and, when present, merges the unstable
// one. The provenance is nil when there is no unstable schema.
func loadSchema(schemaDir string) (*load.Meta, *load.Schema, *load.Provenance, error) {
	meta, err := load.ReadMeta(schemaDir)
	if err != nil {
		return nil, nil, nil, err
	}
	schema, err := load.ReadSchema(schemaDir)
	if err != nil {
		return nil, nil, nil, err
	}
	unstableMeta, unstableMetaFound, err := load.ReadMetaUnstable(schemaDir)
	if err != nil {
		return nil, nil, nil, err
	}
	unstableSchema, unstableSchemaFound, err := load.ReadSchemaUnstable(schemaDir)
	if err != nil {
		return nil, nil, nil, err
	}
	if unstableMetaFound != unstableSchemaFound {
		return nil, nil, nil, fmt.Errorf("unstable schema/meta mismatch: meta found=%v schema found=%v", unstableMetaFound, unstableSchemaFound)
	}
	// The manifest, when present, is the unstable-origin API, frozen; the live
	// unstable files have no part in it
	// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 1).
	manifest, manifestFound, err := load.ReadUnstableManifest(schemaDir)
	if err != nil {
		return nil, nil, nil, err
	}
	if manifestFound {
		return load.MergeManifest(meta, schema, manifest)
	}
	if !unstableMetaFound {
		return meta, schema, nil, nil
	}
	return load.MergeStableAndUnstable(meta, schema, unstableMeta, unstableSchema)
}

// findRepoRoot walks up from the working directory to the first directory
// with schema/meta.json and schema/schema.json.
func findRepoRoot() (string, error) { return findRepoRootFrom(os.Getwd) }

// findRepoRootFrom is findRepoRoot with the function that returns the working
// directory, so a test can make it fail.
func findRepoRootFrom(getwd func() (string, error)) (string, error) {
	cwd, err := getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; ; {
		if hasSchema(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate repository root from %q; ensure schema files exist", cwd)
		}
		dir = parent
	}
}

func hasSchema(dir string) bool {
	for _, name := range []string{"meta.json", "schema.json"} {
		if _, err := os.Stat(filepath.Join(dir, "schema", name)); err != nil {
			return false
		}
	}
	return true
}
