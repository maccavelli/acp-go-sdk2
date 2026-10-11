package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/emit"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/enginetest"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// FuzzBuild requires a schema the model accepts to render, and nothing to
// panic (0011-PLAN-engine-tests.md, D4). The meta is empty, so only types are
// built. The seeds are the schema.json of every engine case.
func FuzzBuild(f *testing.F) {
	for _, path := range engineCases(f) {
		b, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		a, err := enginetest.ParseArchive(b)
		if err != nil {
			f.Fatalf("%s: %v", path, err)
		}
		if schema, ok := a.File("schema.json"); ok {
			f.Add(schema)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var schema load.Schema
		if json.Unmarshal(data, &schema) != nil {
			return
		}
		m, err := model.Build(model.Input{Meta: &load.Meta{}, Schema: &schema, Protocol: &model.ACP})
		if err != nil {
			return
		}
		if _, err := emit.Render(m); err != nil {
			t.Fatalf("the model accepted %s, but rendering failed: %v", data, err)
		}
	})
}
