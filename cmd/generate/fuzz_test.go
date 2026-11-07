package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/emit"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/enginetest"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// FuzzBuild requires a schema the model accepts to render, and nothing to
// panic (0011-PLAN-engine-tests.md, D4). The seeds are the meta.json and the
// schema.json of every engine case, so a seed's methods are listed and most
// seeds build; with an empty meta nearly every one was refused as an
// unlisted method (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G12).
// It does not type-check: TestEngineCases checks every accepted case, and a
// check per input would cut the inputs a run covers.
func FuzzBuild(f *testing.F) {
	for _, seed := range seedPairs(f) {
		f.Add(seed.meta, seed.schema)
	}
	f.Fuzz(func(t *testing.T, metaData, data []byte) {
		m, err := buildSeed(metaData, data)
		if err != nil {
			return
		}
		if _, err := emit.Render(m); err != nil {
			t.Fatalf("the model accepted %s with %s, but rendering failed: %v", data, metaData, err)
		}
	})
}

// seed is an engine case's meta.json and schema.json, by the case's name;
// plain is true for an accepted case without unstable files or protocol.json,
// which its two files alone build.
type seed struct {
	name         string
	meta, schema []byte
	plain        bool
}

// seedPairs returns the seed of every engine case that has both files.
func seedPairs(tb testing.TB) []seed {
	tb.Helper()
	var out []seed
	for _, path := range engineCases(tb) {
		b, err := os.ReadFile(path)
		if err != nil {
			tb.Fatal(err)
		}
		a, err := enginetest.ParseArchive(b)
		if err != nil {
			tb.Fatalf("%s: %v", path, err)
		}
		meta, okMeta := a.File("meta.json")
		schema, okSchema := a.File("schema.json")
		if !okMeta || !okSchema {
			continue
		}
		_, unstable := a.File("schema.unstable.json")
		_, table := a.File(protocolFile)
		name := strings.TrimSuffix(filepath.Base(path), ".txtar")
		out = append(out, seed{name: name, meta: meta, schema: schema, plain: strings.HasPrefix(name, "accept-") && !unstable && !table})
	}
	return out
}

// buildSeed builds the model of a fuzz input, a meta and a schema, with ACP's
// table; an error when either does not decode.
func buildSeed(metaData, data []byte) (*model.Model, error) {
	var meta load.Meta
	var schema load.Schema
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, fmt.Errorf("meta: %w", err)
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	return model.Build(model.Input{Meta: &meta, Schema: &schema, Protocol: &model.ACP})
}

// TestFuzzSeedsBuild requires the fuzzer's seeds to start where the engine
// cases do: every plain accepted case builds and renders from its seed, as
// FuzzBuild builds it (0049-MADR-remediate-the-2026-11-01-debugging-pass.md,
// G12).
func TestFuzzSeedsBuild(t *testing.T) {
	t.Parallel()
	built := 0
	for _, s := range seedPairs(t) {
		if !s.plain {
			continue
		}
		m, err := buildSeed(s.meta, s.schema)
		if err != nil {
			t.Errorf("%s: Build: %v", s.name, err)
			continue
		}
		if _, err := emit.Render(m); err != nil {
			t.Errorf("%s: Render: %v", s.name, err)
			continue
		}
		built++
	}
	if built == 0 {
		t.Fatal("no accepted case built")
	}
}
