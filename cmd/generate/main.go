// Command generate reads the ACP JSON schemas and writes the Go bindings.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/emit"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
}

// run generates the bindings. It loads the schema and builds the model before
// it creates or writes anything, so a schema the model refuses leaves the
// output directory as it was.
func run(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	schemaDir := fs.String("schema", "", "path to schema directory (defaults to <repo>/schema)")
	outDir := fs.String("out", "", "output directory for generated go files (defaults to <repo>)")
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
	meta, schema, prov, err := loadSchema(*schemaDir)
	if err != nil {
		return err
	}
	// The model refuses a schema it cannot represent before any file is written.
	m, err := model.Build(model.Input{Meta: meta, Schema: schema, Provenance: prov, Protocol: &model.ACP})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*outDir, 0o750); err != nil {
		return err
	}

	if err := emit.WriteConstants(*outDir, m); err != nil {
		return err
	}
	if err := emit.WriteTypes(*outDir, m); err != nil {
		return err
	}
	if err := emit.WriteDispatch(*outDir, m); err != nil {
		return err
	}
	// Emit helpers after types so they can reference generated structs.
	return emit.WriteHelpers(*outDir, m)
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
	if !unstableMetaFound {
		return meta, schema, nil, nil
	}
	return load.MergeStableAndUnstable(meta, schema, unstableMeta, unstableSchema)
}

// findRepoRoot walks up from the working directory to the first directory
// with schema/meta.json and schema/schema.json.
func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
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
